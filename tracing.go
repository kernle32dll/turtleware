package turtleware

import (
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const TracerName = "github.com/kernle32dll/turtleware"

// TracingTransport is an implementation of http.RoundTripper that will inject tracing information,
// and then call the actual Transport.
type TracingTransport struct {
	tracer          trace.TracerProvider
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
	tracerProvider := c.tracer
	if tracerProvider == nil {
		tracerProvider = otel.GetTracerProvider()
	}

	tracer := tracerProvider.Tracer(TracerName)
	spanCtx, span := tracer.Start(req.Context(), fmt.Sprintf("HTTP %s: %s", req.Method, req.Host))
	defer span.End()

	span.SetAttributes(
		attribute.String("http.url", req.URL.String()),
		attribute.String("http.method", req.Method),
	)

	// Inject W3C trace context into request headers
	propagation.TraceContext{}.Inject(
		spanCtx,
		propagation.HeaderCarrier(req.Header),
	)

	filteredHeaders := filterHeaders(req, c.headerWhitelist, c.headerBlacklist)
	if len(filteredHeaders) > 0 {
		for header, values := range filteredHeaders {
			span.SetAttributes(attribute.StringSlice(
				"header."+strings.ToLower(header), values,
			))
		}
	}

	roundTripper := c.roundTripper
	if roundTripper == nil {
		roundTripper = http.DefaultTransport
	}

	resp, err := roundTripper.RoundTrip(req)
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	if resp != nil {
		span.SetAttributes(
			attribute.Int("http.status_code", resp.StatusCode),
		)
	}

	return resp, err
}

// WrapZerologTracing fetches the zerolog.Logger attached with the context
// (if existing), and creates a new logger with the context's spanID and
// traceID fields set.
func WrapZerologTracing(ctx context.Context) zerolog.Logger {
	logger := *zerolog.Ctx(ctx)

	// If there is no tracing data, we bail out directly
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return logger
	}

	spanContext := span.SpanContext()
	if spanContext.HasTraceID() {
		logger = logger.With().
			Str("traceID", spanContext.TraceID().String()).
			Logger()
	}
	if spanContext.HasSpanID() {
		logger = logger.With().
			Str("spanID", spanContext.SpanID().String()).
			Logger()
	}

	return logger
}

// TagContextSpanWithError tries to retrieve an open telemetry span from the given
// context, and sets some error attributes, signaling that the current span
// has failed. If no span exists, this function does nothing.
// This function returns the error as provided, to facilitate easy error returning
// in using functions.
func TagContextSpanWithError(ctx context.Context, err error) error {
	if span := trace.SpanFromContext(ctx); span != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return err
}
