package turtleware

import (
	"github.com/sirupsen/logrus"

	"errors"
	"net/http"
	"strings"
	"time"
)

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
	//default
	config := &loggingOptions{
		logHeaders:      false,
		headerWhitelist: nil,
		headerBlacklist: nil,
	}

	//apply opts
	for _, opt := range opts {
		opt(config)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := logrus.WithContext(r.Context())

			if config.logHeaders && logrus.IsLevelEnabled(logrus.DebugLevel) {
				var filteredHeaders http.Header

				if config.headerWhitelist != nil {
					filteredHeaders = http.Header{}
					for key, values := range r.Header {
						if _, allowed := config.headerWhitelist[strings.ToLower(key)]; allowed {
							filteredHeaders[key] = values
						}
					}
				} else if config.headerBlacklist != nil {
					filteredHeaders = http.Header{}
					for key, values := range r.Header {
						if _, denied := config.headerBlacklist[strings.ToLower(key)]; !denied {
							filteredHeaders[key] = values
						}
					}
				} else {
					filteredHeaders = r.Header
				}

				logger.WithField("headers", filteredHeaders).Infof("Received %s request for %s", r.Method, r.URL)
			} else {
				logger.Infof("Received %s request for %s", r.Method, r.URL)
			}

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

			if logrus.IsLevelEnabled(logrus.DebugLevel) {
				duration := time.Since(start)

				// Double division, so we get appropriate precision
				micros := duration / time.Microsecond
				millis := float64(micros) / float64(time.Microsecond)

				logrus.WithContext(r.Context()).WithFields(logrus.Fields{
					"timemillis": millis,
					"status":     sw.status,
					"length":     sw.length,
				}).Infof("Request took %s", duration)
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
				logrus.
					WithContext(r.Context()).
					WithField("reason", "url unmatched").
					Warnf("%s request for %s was not matched", r.Method, r.URL)

				WriteError(w, r, http.StatusNotFound, errors.New("request url and method was not matched"))
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
				logrus.
					WithContext(r.Context()).
					WithField("reason", "url method not allowed").
					Warnf("%s request for %s was not matched", r.Method, r.URL)

				WriteError(w, r, http.StatusMethodNotAllowed, errors.New("request url was matched, but method was not allowed"))
			}),
		),
	)
}
