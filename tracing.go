package turtleware

import (
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"github.com/sirupsen/logrus"
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

// TracingHook is a logrus hook, allowing some interplay between logrus
// and a tracing backend, such as jaeger.
type TracingHook struct {
	// LogToTracingBackend controls whether logrus log entries should
	// be logged inside spans, too.
	LogToTracingBackend bool
}

// Levels returns the active levels of this hook - all.
func (h *TracingHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *TracingHook) Fire(e *logrus.Entry) error {
	if e.Context == nil {
		return nil
	}

	span := opentracing.SpanFromContext(e.Context)
	if span == nil {
		return nil
	}

	if h.LogToTracingBackend {
		// if we are tracing with jaeger, remove trace and span id
		// beforehand, so we don't pollute our tracing backend
		// with redundant information
		if _, ok := span.Context().(jaeger.SpanContext); ok {
			delete(e.Data, "spanID")
			delete(e.Data, "traceID")
		}

		keyValues := make([]interface{}, 2+len(e.Data)*2)
		keyValues[0] = "message"
		keyValues[1] = e.Message

		index := 2

		for key, data := range e.Data {
			keyValues[index] = key
			keyValues[index+1] = data
			index += 2
		}

		fields, err := log.InterleavedKVToFields(keyValues...)
		if err != nil {
			return err
		}

		span.LogFields(fields...)
	}

	// ----

	// if we are tracing with jaeger, attach the trace and span id
	spanContext, ok := span.Context().(jaeger.SpanContext)
	if ok {
		e.Data["spanID"] = spanContext.SpanID().String()
		e.Data["traceID"] = spanContext.TraceID().String()
	}

	return nil
}

// TagContextSpanWithError tries to retrieve an opentracing span from the given
// context, and sets some error attributes, signaling that the current span
// has failed. If no span exists, this function does nothing.
func TagContextSpanWithError(ctx context.Context, err error) {
	if span := opentracing.SpanFromContext(ctx); span != nil {
		ext.LogError(span, err)
	}
}
