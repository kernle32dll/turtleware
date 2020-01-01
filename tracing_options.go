package turtleware

import (
	"github.com/opentracing/opentracing-go"

	"net/http"
	"strings"
)

type tracingOptions struct {
	tracer opentracing.Tracer

	// The RoundTripper interface actually used to make requests.
	// If nil, http.DefaultTransport is used
	roundTripper http.RoundTripper

	headerWhitelist map[string]struct{}
	headerBlacklist map[string]struct{}
}

// TracingOption represents an option for the tracing parameters.
type TracingOption func(*tracingOptions)

// TracingTracer sets the Tracer interface used for tracing.
// The default is nil, which means opentracing.GlobalTracer().
func TracingTracer(tracer opentracing.Tracer) TracingOption {
	return func(c *tracingOptions) {
		c.tracer = tracer
	}
}

// TracingRoundTripper sets the RoundTripper interface actually used to make requests.
// The default is nil, which means http.DefaultTransport.
func TracingRoundTripper(roundTripper http.RoundTripper) TracingOption {
	return func(c *tracingOptions) {
		c.roundTripper = roundTripper
	}
}

// TraceHeaderWhitelist sets a whitelist of headers to allow.
// Automatically replaces the blacklist if used.
// The default is not set, which means "deny all".
func TraceHeaderWhitelist(headerWhitelist ...string) TracingOption {
	return func(c *tracingOptions) {
		c.headerWhitelist = make(map[string]struct{}, len(headerWhitelist))
		c.headerBlacklist = nil

		for _, header := range headerWhitelist {
			c.headerWhitelist[strings.ToLower(header)] = struct{}{}
		}
	}
}

// TraceHeaderBlacklist sets a blacklist of headers to disallow.
// Automatically replaces the whitelist if used.
// The default is not set, which means "deny all".
func TraceHeaderBlacklist(headerBlacklist ...string) TracingOption {
	return func(c *tracingOptions) {
		c.headerWhitelist = nil
		c.headerBlacklist = make(map[string]struct{}, len(headerBlacklist))

		for _, header := range headerBlacklist {
			c.headerBlacklist[strings.ToLower(header)] = struct{}{}
		}
	}
}
