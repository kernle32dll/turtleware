package turtleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/justinas/alice"
	"github.com/klauspost/compress/gzhttp"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type httpMethod string

type endpoint struct {
	exposedHeaders []string
	allowedHeaders []string
	h              http.Handler
}

// Router is a simple HTTP router tailored to the turtleware framework.
// It follows REST conventions for endpoint types and allows adding middlewares
// that are applied to all endpoints.
type Router struct {
	endpoints   map[string]map[httpMethod]endpoint
	middlewares []alice.Constructor
	prefix      string
	mutex       *sync.Mutex
}

// NewRouter creates a new Router with the given prefix.
func NewRouter(prefix string) *Router {
	return &Router{
		endpoints: make(map[string]map[httpMethod]endpoint),
		prefix:    prefix,
		mutex:     &sync.Mutex{},
	}
}

func (r *Router) addEndpoints(path string, m httpMethod, ep endpoint) *Router {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.endpoints[path]; !exists {
		r.endpoints[path] = map[httpMethod]endpoint{}
	}

	r.endpoints[path][m] = ep

	return r
}

// Entity registers an endpoint for handling a single entity resource.
func (r *Router) Entity(path string, h http.Handler) *Router {
	return r.Endpoint(path, h, http.MethodGet).Endpoint(path, h, http.MethodHead)
}

// List registers an endpoint for handling a collection of entity resources.
func (r *Router) List(path string, h http.Handler) *Router {
	ep := WithExposedHeaders("X-Total-Count")
	return r.Endpoint(path, h, http.MethodGet, ep).Endpoint(path, h, http.MethodHead, ep)
}

// Update registers an endpoint for handling partial updates to an entity resource.
func (r *Router) Update(path string, h http.Handler) *Router {
	return r.Endpoint(path, h, http.MethodPatch, WithAllowedHeaders("Content-Type", "If-Unmodified-Since"))
}

// Create registers an endpoint for handling the creation of a new entity resource.
func (r *Router) Create(path string, h http.Handler) *Router {
	return r.Endpoint(path, h, http.MethodPost, WithAllowedHeaders("Content-Type"))
}

// Replace registers an endpoint for handling full replacements of an entity resource.
func (r *Router) Replace(path string, h http.Handler) *Router {
	return r.Endpoint(path, h, http.MethodPut, WithAllowedHeaders("Content-Type"))
}

// Endpoint registers an endpoint with the supplied HTTP method, giving full control over the endpoint.
func (r *Router) Endpoint(path string, h http.Handler, method string, opts ...EndpointOption) *Router {
	config := endpointOptions{}

	for _, opt := range opts {
		opt(&config)
	}

	ep := endpoint{
		exposedHeaders: config.exposedHeaders,
		allowedHeaders: config.allowedHeaders,
		h:              h,
	}

	return r.addEndpoints(path, httpMethod(method), ep)
}

// AddMiddleware adds a middleware to the router that will be applied to all endpoints.
func (r *Router) AddMiddleware(constructor alice.Constructor) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.middlewares = append(r.middlewares, constructor)
}

// Build constructs the final http.Handler with all registered endpoints and middlewares.
func (r *Router) Build() http.Handler {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	rtr := httprouter.New()

	rtr.PanicHandler = func(w http.ResponseWriter, r *http.Request, panic interface{}) {
		wireCtx := propagation.TraceContext{}.Extract(
			r.Context(),
			propagation.HeaderCarrier(r.Header),
		)
		if spanContext := trace.SpanContextFromContext(wireCtx); !spanContext.HasTraceID() && !spanContext.HasSpanID() {
			logger := zerolog.Ctx(r.Context())
			logger.Trace().Msg("Missing span context")
		}

		spanCtx, span := otel.Tracer(TracerName).Start(wireCtx, "Panic")
		defer span.End()

		// Update logger in context to use correct span
		logger := WrapZerologTracing(spanCtx)
		spanCtx = logger.WithContext(spanCtx)

		panicErr := fmt.Errorf("panic in http handler: %v", panic)
		WriteError(spanCtx, w, r, http.StatusInternalServerError, panicErr)
	}

	loggingOpts := []LoggingOption{
		LogHeaders(true),
		LogHeaderBlacklist("Authorization"),
	}

	middleware := alice.New(
		func(handler http.Handler) http.Handler {
			return gzhttp.GzipHandler(handler)
		},
		RequestTimingMiddleware(),
		RequestLoggerMiddleware(loggingOpts...),
	).Append(r.middlewares...)

	rtr.NotFound = middleware.Then(RequestNotFoundHandler(loggingOpts...))
	rtr.MethodNotAllowed = middleware.Then(RequestNotAllowedHandler(loggingOpts...))

	corsMiddleware := map[string]alice.Chain{}
	for path, corsHandler := range r.assembleCors() {
		corsMiddleware[path] = middleware.Append(corsHandler.Handler)

		rtr.Handler(http.MethodOptions, r.prefix+path, middleware.ThenFunc(corsHandler.HandlerFunc))
	}

	for path, methods := range r.endpoints {
		for method, endpoint := range methods {
			rtr.Handler(string(method), r.prefix+path, corsMiddleware[path].Then(endpoint.h))
		}
	}

	return rtr
}

func (r *Router) assembleCors() map[string]*cors.Cors {
	corsHandlers := make(map[string]*cors.Cors, len(r.endpoints))

	alwaysExposedHeaders := []string{
		// Tracing
		"Uber-Trace-Id",

		// Response style
		"Accept",
	}

	alwaysAllowedHeaders := []string{
		"Authorization",
	}

	for path, methods := range r.endpoints {
		allowedMethods := make(map[string]struct{}, len(methods))
		allowedHeaders := make(map[string]struct{})
		exposedHeaders := make(map[string]struct{})
		for method, ep := range methods {
			allowedMethods = addSliceToSet(allowedMethods, string(method))
			allowedHeaders = addSliceToSet(allowedHeaders, ep.allowedHeaders...)
			exposedHeaders = addSliceToSet(exposedHeaders, ep.exposedHeaders...)
		}

		corsHandlers[path] = cors.New(cors.Options{
			AllowOriginFunc: func(origin string) bool {
				return true
			},
			AllowedMethods:   keySet(allowedMethods),
			AllowCredentials: true,
			AllowedHeaders:   append(alwaysAllowedHeaders, keySet(allowedHeaders)...),
			ExposedHeaders:   append(alwaysExposedHeaders, keySet(exposedHeaders)...),
			MaxAge:           int(time.Hour * 24 / time.Second),
		})
	}

	return corsHandlers
}

func keySet[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func addSliceToSet[T comparable](set map[T]struct{}, array ...T) map[T]struct{} {
	for _, item := range array {
		set[item] = struct{}{}
	}
	return set
}
