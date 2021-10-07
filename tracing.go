package turtleware

import (
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/rs/zerolog"
	"github.com/uber/jaeger-client-go"

	"context"
	"errors"
	"fmt"
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

// TagContextSpanWithError tries to retrieve an opentracing span from the given
// context, and sets some error attributes, signaling that the current span
// has failed. If no span exists, this function does nothing.
func TagContextSpanWithError(ctx context.Context, err error) {
	if span := opentracing.SpanFromContext(ctx); span != nil {
		ext.LogError(span, err)
	}
}
