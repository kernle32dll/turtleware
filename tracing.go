package turtleware

import (
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"github.com/sirupsen/logrus"
	"github.com/uber/jaeger-client-go"

	"context"
	"fmt"
	"net/http"
	"strings"
)

// TracingTransport is an implementation of http.RoundTripper that will inject tracing information,
// and then call the actual Transport.
// If the Tracer is nil, opentracing.GlobalTracer() is used.
// If the Transport is nil, http.DefaultTransport is used.
type TracingTransport struct {
	Tracer opentracing.Tracer

	// The RoundTripper interface actually used to make requests
	// If nil, http.DefaultTransport is used
	Transport http.RoundTripper

	// HeaderWhitelist is a set of header names which are allowed to be
	// added to traces.
	HeaderWhitelist map[string]struct{}

	// HeaderWhitelist is a set of header names which are disallowed to
	// be added to traces.
	HeaderBlacklist map[string]struct{}
}

func (c TracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tracer := c.Tracer
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

	filteredHeaders := filterHeaders(req, c.HeaderWhitelist, c.HeaderBlacklist)
	if len(filteredHeaders) > 0 {
		for header, values := range filteredHeaders {
			span.SetTag("header."+strings.ToLower(header), values)
		}
	}

	transport := c.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	resp, err := transport.RoundTrip(req.WithContext(spanCtx))

	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		ext.Error.Set(span, true)
		span.LogFields(
			log.Object("event", "error"),
			log.Object("error.object", err),
		)
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

// Levels returns the active levels of this hook - all
func (h *TracingHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *TracingHook) Fire(e *logrus.Entry) error {
	if e.Context != nil {
		if span := opentracing.SpanFromContext(e.Context); span != nil {
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
		}
	}

	return nil
}
