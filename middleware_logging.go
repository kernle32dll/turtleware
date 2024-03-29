package turtleware

import (
	"github.com/rs/zerolog"

	"errors"
	"net/http"
	"strings"
	"time"
)

type loggingOptions struct {
	logHeaders      bool
	headerWhitelist map[string]struct{}
	headerBlacklist map[string]struct{}
}

// LoggingOption represents an option for the logging parameters.
type LoggingOption func(*loggingOptions)

// LogHeaders sets whether headers should be logged.
// The default is false.
func LogHeaders(logHeaders bool) LoggingOption {
	return func(c *loggingOptions) {
		c.logHeaders = logHeaders
	}
}

// LogHeaderWhitelist sets a whitelist of headers to allow.
// Automatically replaces the blacklist if used.
// The default is not set, which means "allow all".
func LogHeaderWhitelist(headerWhitelist ...string) LoggingOption {
	return func(c *loggingOptions) {
		c.headerWhitelist = make(map[string]struct{}, len(headerWhitelist))
		c.headerBlacklist = nil

		for _, header := range headerWhitelist {
			c.headerWhitelist[strings.ToLower(header)] = struct{}{}
		}
	}
}

// LogHeaderBlacklist sets a blacklist of headers to disallow.
// Automatically replaces the whitelist if used.
// The default is not set, which means "allow all".
func LogHeaderBlacklist(headerBlacklist ...string) LoggingOption {
	return func(c *loggingOptions) {
		c.headerWhitelist = nil
		c.headerBlacklist = make(map[string]struct{}, len(headerBlacklist))

		for _, header := range headerBlacklist {
			c.headerBlacklist[strings.ToLower(header)] = struct{}{}
		}
	}
}

// statusWriter is a wrapper for a http.ResponseWriter for capturing
// the http status code and content length
// Source: https://www.reddit.com/r/golang/comments/7p35s4/how_do_i_get_the_response_status_for_my_middleware/dse5y4g
type statusWriter struct {
	http.ResponseWriter
	status int
	length int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}

	n, err := w.ResponseWriter.Write(b)
	w.length += n

	return n, err
}

// RequestLoggerMiddleware is a http middleware for logging non-sensitive properties about the request.
func RequestLoggerMiddleware(opts ...LoggingOption) func(next http.Handler) http.Handler {
	// default
	config := &loggingOptions{
		logHeaders:      false,
		headerWhitelist: nil,
		headerBlacklist: nil,
	}

	// apply opts
	for _, opt := range opts {
		opt(config)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := zerolog.Ctx(r.Context())

			logEvent := logger.Info()
			if debugLevel := logger.Debug(); config.logHeaders && debugLevel.Enabled() {
				if filteredHeaders := filterHeaders(r, config.headerWhitelist, config.headerBlacklist); len(filteredHeaders) > 0 {
					logEvent = logEvent.Interface("headers", filteredHeaders)
				}
			}

			logEvent.Msgf("Received %s request for %s", r.Method, r.URL)

			next.ServeHTTP(w, r)
		})
	}
}

// RequestTimingMiddleware is a http middleware for timing the response time of a request.
func RequestTimingMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			sw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(sw, r)

			logger := zerolog.Ctx(r.Context())
			if debugLevel := logger.Debug(); debugLevel.Enabled() {
				duration := time.Since(start)

				// Double division, so we get appropriate precision
				micros := duration / time.Microsecond
				millis := float64(micros) / float64(time.Microsecond)

				logger.Info().
					Float64("timemillis", millis).
					Int("status", sw.status).
					Int("length", sw.length).
					Msgf("Request took %s", duration)
			}
		})
	}
}

// RequestNotFoundHandler is a http handler for logging requests which were not matched.
// This is mostly useful for gorilla/mux with its NotFoundHandler.
func RequestNotFoundHandler(opts ...LoggingOption) http.Handler {
	return RequestTimingMiddleware()(
		RequestLoggerMiddleware(
			opts...,
		)(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				logger := zerolog.Ctx(r.Context())
				logger.Warn().Str("reason", "url unmatched").Msgf("%s request for %s was not matched", r.Method, r.URL)

				WriteError(r.Context(), w, r, http.StatusNotFound, errors.New("request url and method was not matched"))
			}),
		),
	)
}

// RequestNotAllowedHandler is a http handler for logging requests which were url matched, but
// using an invalid method.
// This is mostly useful for gorilla/mux with its MethodNotAllowedHandler.
func RequestNotAllowedHandler(opts ...LoggingOption) http.Handler {
	return RequestTimingMiddleware()(
		RequestLoggerMiddleware(
			opts...,
		)(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				logger := zerolog.Ctx(r.Context())
				logger.Warn().Str("reason", "url method not allowed").Msgf("%s request for %s was not matched", r.Method, r.URL)

				WriteError(r.Context(), w, r, http.StatusMethodNotAllowed, errors.New("request url was matched, but method was not allowed"))
			}),
		),
	)
}

func filterHeaders(r *http.Request, headerWhitelist map[string]struct{}, headerBlacklist map[string]struct{}) http.Header {
	if headerWhitelist != nil {
		filteredHeaders := http.Header{}

		for key, values := range r.Header {
			if _, allowed := headerWhitelist[strings.ToLower(key)]; allowed {
				filteredHeaders[key] = values
			}
		}

		return filteredHeaders
	} else if headerBlacklist != nil {
		filteredHeaders := http.Header{}

		for key, values := range r.Header {
			if _, denied := headerBlacklist[strings.ToLower(key)]; !denied {
				filteredHeaders[key] = values
			}
		}

		return filteredHeaders
	}

	// If we neither explicitly allow or deny any header, we don't log anything.
	// This is intentional, so we don't accidentally expose confidential headers per default.
	return nil
}
