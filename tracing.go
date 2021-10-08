package turtleware

import (
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/rs/zerolog"
	"github.com/uber/jaeger-client-go"

	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// TracingTransport is an implementation of http.RoundTripper that will inject tracing information,
// and then call the actual Transport.
type TracingTransport struct {
	tracer          opentracing.Tracer
	roundTripper    http.RoundTripper
	headerWhitelist map[string]struct{}
	headerBlacklist map[string]struct{}
}

func NewTracingTransport(opts ...TracingOption) *TracingTransport {
	// default
	config := &tracingOptions{
		tracer:          nil,
		roundTripper:    nil,
		headerWhitelist: nil,
		headerBlacklist: nil,
	}

	// apply opts
	for _, opt := range opts {
		opt(config)
	}

	return &TracingTransport{
		tracer:          config.tracer,
		roundTripper:    config.roundTripper,
		headerWhitelist: config.headerWhitelist,
		headerBlacklist: config.headerBlacklist,
	}
}

func (c TracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tracer := c.tracer
	if tracer == nil {
		tracer = opentracing.GlobalTracer()
	}

	span, spanCtx := opentracing.StartSpanFromContextWithTracer(req.Context(), tracer, fmt.Sprintf("HTTP %s: %s", req.Method, req.Host))
	defer span.Finish()

	ext.HTTPUrl.Set(span, req.URL.String())
	ext.HTTPMethod.Set(span, req.Method)

	if err := tracer.Inject(
		span.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header)); err != nil {
		return nil, err
	}

	filteredHeaders := filterHeaders(req, c.headerWhitelist, c.headerBlacklist)
	if len(filteredHeaders) > 0 {
		for header, values := range filteredHeaders {
			span.SetTag("header."+strings.ToLower(header), values)
		}
	}

	roundTripper := c.roundTripper
	if roundTripper == nil {
		roundTripper = http.DefaultTransport
	}

	resp, err := roundTripper.RoundTrip(req.WithContext(spanCtx))

	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		ext.LogError(span, err)
	}

	if resp != nil {
		ext.HTTPStatusCode.Set(span, uint16(resp.StatusCode))
	}

	return resp, err
}

// WrapZerologTracing fetches the zerolog.Logger attached with the context
// (if existing), and creates a new logger with the context's spanID and
// traceID fields set.
func WrapZerologTracing(ctx context.Context) zerolog.Logger {
	logger := zerolog.Ctx(ctx)

	// If there is no tracing data, we bail out directly
	span := opentracing.SpanFromContext(ctx)
	if span == nil {
		return *logger
	}

	spanContext, isJaeger := span.Context().(jaeger.SpanContext)
	if !isJaeger {
		// No span or trace to extract - bail out
		return *logger
	}

	return logger.With().
		Str("spanID", spanContext.SpanID().String()).
		Str("traceID", spanContext.TraceID().String()).
		Str("parentID", spanContext.ParentID().String()).
		Logger()
}

// TracingContextWithZerolog derives a zerolog.Logger from the given context (if
// existing), and
func TracingContextWithZerolog(ctx context.Context) context.Context {
	// If there is no tracing data, we just use the context as is
	//span := opentracing.SpanFromContext(ctx)
	//if span == nil {
	//	return ctx
	//}

	ctxLogger := zerolog.Ctx(ctx)

	logger := *ctxLogger

	pr, pw := io.Pipe()

	// Inspired by:
	// https://gist.github.com/asdine/f821abe6189a04250ae61b77a3048bd9
	go func() {
		dec := json.NewDecoder(pr)

		for {
			var e map[string]interface{}
			err := dec.Decode(&e)
			if err == io.EOF {
				// Shutdown
				return
			}

			if err != nil {
				continue
			}

			if spanID, ok := e["correlationID"].(string); ok {
				spanContext, err := jaeger.ContextFromString(spanID)
				if err != nil {
					continue
				}

				// Remove correlation id beforehand, so we don't pollute
				// our tracing backend with redundant information
				delete(e, "correlationID")

				keyValues := make([]interface{}, len(e)*2)

				index := 0
				for key, data := range e {
					keyValues[index] = key
					keyValues[index+1] = data
					index += 2
				}

				opentracing.ContextWithSpan(context.Background(), spanContext)

				spanContext.LogKV(keyValues...)
			}

			// if we are tracing with jaeger, remove trace and span id
			// beforehand, so we don't pollute our tracing backend
			// with redundant information
			if _, ok := span.Context().(jaeger.SpanContext); ok {
				delete(e, "spanID")
				delete(e, "parentID")
			}

		}
	}()

	originalLogger := logger

	logger = logger.Output(pw).Hook(zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		//originalLogger.
		// TODO: Delegate to originalLogger

		e.Send()
	}))

	return logger.WithContext(ctx)
}

// TagContextSpanWithError tries to retrieve an opentracing span from the given
// context, and sets some error attributes, signaling that the current span
// has failed. If no span exists, this function does nothing.
func TagContextSpanWithError(ctx context.Context, err error) {
	if span := opentracing.SpanFromContext(ctx); span != nil {
		ext.LogError(span, err)
	}
}
