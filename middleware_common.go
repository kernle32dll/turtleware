package turtleware

import (
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"context"
	"errors"
	"net/http"
)

type ctxKey int

const (
	// ctxAuthToken is the context key used to pass down the bearer token.
	ctxAuthToken ctxKey = iota

	// ctxEntityUUID is the context key used to pass down the entity UUID.
	ctxEntityUUID

	// ctxPaging is the context key used to pass down paging information.
	ctxPaging

	// ctxAuthClaims is the context key used to pass down jwt claims.
	ctxAuthClaims
)

var (
	// ErrContextMissingAuthToken is an internal error indicating a missing
	// auth token in the request context, whereas one was expected.
	ErrContextMissingAuthToken = errors.New("missing auth token in context")

	// ErrContextMissingEntityUUID is an internal error indicating a missing
	// entity UUID in the request context, whereas one was expected.
	ErrContextMissingEntityUUID = errors.New("missing entity UUID in context")

	// ErrContextMissingPaging is an internal error indicating missing paging
	// in the request context, whereas one was expected.
	ErrContextMissingPaging = errors.New("missing paging in context")

	// ErrContextMissingAuthClaims is an internal error indicating missing auth
	// claims in the request context, whereas they were expected.
	ErrContextMissingAuthClaims = errors.New("missing auth claims in context")

	// ErrMarshalling signals that an error occurred while marshalling.
	ErrMarshalling = errors.New("failed to parse message body")

	// ErrReceivingResults signals that an error occurred while receiving the results
	// from the database or similar.
	ErrReceivingResults = errors.New("error while receiving results")

	// ErrResourceNotFound indicates that a requested resource was not found.
	ErrResourceNotFound = errors.New("resource not found")

	// ErrReceivingMeta signals that an error occurred while receiving the metadata
	// from the database or remotes.
	ErrReceivingMeta = errors.New("error while receiving metadata")

	// ErrMissingUserUUID signals that a received JWT did not contain an user UUID.
	ErrMissingUserUUID = errors.New("token does not include user UUID")
)

type ResourceEntityFunc func(r *http.Request) (string, error)

// IsHandledByDefaultErrorHandler indicates if the DefaultErrorHandler has any special
// handling for the given error, or if it defaults to handing it out as-is.
func IsHandledByDefaultErrorHandler(err error) bool {
	if errors.Is(err, ErrResourceNotFound) ||
		errors.Is(err, ErrMissingUserUUID) ||
		errors.Is(err, ErrMarshalling) {
		return true
	}

	validationErr := &ValidationWrapperError{}
	return errors.As(err, &validationErr)
}

// DefaultErrorHandler is a default error handler, which sensibly handles errors known by turtleware.
func DefaultErrorHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrResourceNotFound) {
		WriteError(ctx, w, r, http.StatusNotFound, err)
		return
	}

	if errors.Is(err, ErrMissingUserUUID) || errors.Is(err, ErrMarshalling) {
		WriteError(ctx, w, r, http.StatusBadRequest, err)
		return
	}

	validationErr := &ValidationWrapperError{}
	if errors.As(err, validationErr) {
		WriteError(ctx, w, r, http.StatusBadRequest, validationErr.Errors...)
		return
	}

	WriteError(ctx, w, r, http.StatusInternalServerError, err)
}

// EntityUUIDMiddleware is a http middleware for extracting the UUID of the resource requested,
// and passing it down.
func EntityUUIDMiddleware(entityFunc ResourceEntityFunc) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			entityUUID, err := entityFunc(r)
			if err != nil {
				WriteError(r.Context(), w, r, http.StatusInternalServerError, err)

				return
			}

			h.ServeHTTP(
				w,
				r.WithContext(context.WithValue(r.Context(), ctxEntityUUID, entityUUID)),
			)
		})
	}
}

// AuthBearerHeaderMiddleware is a http middleware for extracting the bearer token from
// the authorization header, and passing it down. If the header is not existing, the
// WWW-Authenticate header is set and the handler bails out.
func AuthBearerHeaderMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := FromAuthHeader(r)
		if err != nil {
			if errors.Is(err, ErrMissingAuthHeader) {
				// If it was a browser request, give it a chance to authenticate
				w.Header().Add("WWW-Authenticate", `bearer`)
				WriteError(r.Context(), w, r, http.StatusUnauthorized, err)
			} else {
				WriteError(r.Context(), w, r, http.StatusBadRequest, err)
			}

			return
		}

		h.ServeHTTP(
			w,
			r.WithContext(context.WithValue(r.Context(), ctxAuthToken, token)),
		)
	})
}

// AuthClaimsMiddleware is a http middleware for extracting authentication claims, and
// passing them down.
func AuthClaimsMiddleware(keySet jwk.Set) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := AuthTokenFromRequestContext(r.Context())
			if err != nil {
				WriteError(r.Context(), w, r, http.StatusInternalServerError, err)

				return
			}

			claims, err := ValidateTokenBySet(token, keySet)
			if err != nil {
				WriteError(r.Context(), w, r, http.StatusBadRequest, ErrTokenValidationFailed)

				return
			}

			h.ServeHTTP(
				w,
				r.WithContext(context.WithValue(r.Context(), ctxAuthClaims, claims)),
			)
		})
	}
}

// PagingMiddleware is a http middleware for extracting paging information, and passing
// it down.
func PagingMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paging, err := ParsePagingFromRequest(r)
		if err != nil {
			WriteError(r.Context(), w, r, http.StatusInternalServerError, err)

			return
		}

		h.ServeHTTP(
			w,
			r.WithContext(context.WithValue(r.Context(), ctxPaging, paging)),
		)
	})
}

// TracingMiddleware is a http middleware for injecting a new named open telemetry
// span into the request context. If tracer is nil, otel.GetTracerProvider()
// is used.
func TracingMiddleware(name string, traceProvider trace.TracerProvider) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			locTraceProvider := traceProvider
			if locTraceProvider == nil {
				locTraceProvider = otel.GetTracerProvider()
			}

			// Fetch a zerolog logger, if already set in the context, or a fresh one
			// (will be injected into the context that is passed along later down below)
			logger := zerolog.Ctx(r.Context()).With().Logger()

			wireCtx := propagation.TraceContext{}.Extract(
				r.Context(),
				propagation.HeaderCarrier(r.Header),
			)

			requireResponse := false
			if spanContext := trace.SpanContextFromContext(wireCtx); !spanContext.HasTraceID() && !spanContext.HasSpanID() {
				requireResponse = true
				logger.Trace().Msg("Missing span context")
			}

			locTracer := locTraceProvider.Tracer(TracerName)
			spanCtx, span := locTracer.Start(wireCtx, name)
			defer span.End()

			// Create a logger, which contains the root span and trace,
			// and inject that back into the context for root level trace logging
			logger = WrapZerologTracing(spanCtx)
			spanCtx = logger.WithContext(spanCtx)

			// ---------------------

			// Write tracing headers back into response, to enable clients to debug calls without
			// sending a valid trace in the first place.
			if requireResponse {
				carrier := propagation.HeaderCarrier{}
				propagation.TraceContext{}.Inject(
					spanCtx,
					carrier,
				)

				for _, key := range carrier.Keys() {
					w.Header().Add(key, carrier.Get(key))
				}
			}

			h.ServeHTTP(
				w,
				r.WithContext(spanCtx),
			)
		})
	}
}

func EntityUUIDFromRequestContext(ctx context.Context) (string, error) {
	entityUUID, ok := ctx.Value(ctxEntityUUID).(string)
	if !ok {
		return "", ErrContextMissingEntityUUID
	}

	return entityUUID, nil
}

func AuthTokenFromRequestContext(ctx context.Context) (string, error) {
	token, ok := ctx.Value(ctxAuthToken).(string)
	if !ok {
		return "", ErrContextMissingAuthToken
	}

	return token, nil
}

func PagingFromRequestContext(ctx context.Context) (Paging, error) {
	paging, ok := ctx.Value(ctxPaging).(Paging)
	if !ok {
		return Paging{}, ErrContextMissingPaging
	}

	return paging, nil
}

func AuthClaimsFromRequestContext(ctx context.Context) (map[string]interface{}, error) {
	claims, ok := ctx.Value(ctxAuthClaims).(map[string]interface{})
	if !ok {
		return nil, ErrContextMissingAuthClaims
	}

	return claims, nil
}

func UserUUIDFromRequestContext(ctx context.Context) (string, error) {
	claims, err := AuthClaimsFromRequestContext(ctx)
	if err != nil {
		return "", err
	}

	// ----------------

	userUUID, ok := claims["uuid"].(string)
	if !ok || userUUID == "" {
		return "", ErrMissingUserUUID
	}

	// ----------------

	return userUUID, nil
}
