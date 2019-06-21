package turtleware

import (
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"

	"context"
	"fmt"
	"net/http"
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
