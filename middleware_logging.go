package server

import (
	"github.com/sirupsen/logrus"

	"errors"
	"net/http"
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
func RequestLoggerMiddleware(next http.Handler, opts ...LoggingOption) http.Handler {
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

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if config.logHeaders && logrus.IsLevelEnabled(logrus.DebugLevel) {
			var filteredHeaders http.Header

			if config.headerWhitelist != nil {
				filteredHeaders := http.Header{}
				for key, values := range r.Header {
					if _, allowed := config.headerWhitelist[key]; allowed {
						filteredHeaders[key] = values
					}
				}
			} else if config.headerBlacklist != nil {
				filteredHeaders := http.Header{}
				for key, values := range r.Header {
					if _, denied := config.headerBlacklist[key]; !denied {
						filteredHeaders[key] = values
					}
				}
			} else {
				filteredHeaders = r.Header
			}

			logrus.WithField("headers", filteredHeaders).Infof("Received %s request for %s", r.Method, r.URL)
		} else {
			logrus.Infof("Received %s request for %s", r.Method, r.URL)
		}

		next.ServeHTTP(w, r)
	})
}

// RequestTimingMiddleware is a http middleware for timing the response time of a request.
func RequestTimingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)

		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			duration := time.Since(start)

			// Double division, so we get appropriate precision
			micros := duration / time.Microsecond
			millis := float64(micros) / float64(time.Microsecond)

			logrus.WithFields(logrus.Fields{
				"timemillis": millis,
				"status":     sw.status,
				"length":     sw.length,
			}).Infof("Request took %s", duration)
		}
	})
}

// RequestNotFoundHandler is a http handler for logging requests which were not matched.
// This is mostly useful for gorilla/mux with its NotFoundHandler.
func RequestNotFoundHandler(opts ...LoggingOption) http.Handler {
	return RequestTimingMiddleware(
		RequestLoggerMiddleware(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				logrus.WithField("reason", "url unmatched").Warnf("%s request for %s was not matched", r.Method, r.URL)
				WriteError(w, r, errors.New("request url and method was not matched"), http.StatusNotFound)
			}),
			opts...,
		),
	)
}

// RequestNotAllowedHandler is a http handler for logging requests which were url matched, but
// using an invalid method.
// This is mostly useful for gorilla/mux with its MethodNotAllowedHandler.
func RequestNotAllowedHandler(opts ...LoggingOption) http.Handler {
	return RequestTimingMiddleware(
		RequestLoggerMiddleware(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				logrus.WithField("reason", "url method not allowed").Warnf("%s request for %s was not matched", r.Method, r.URL)
				WriteError(w, r, errors.New("request url was matched, but method was not allowed"), http.StatusMethodNotAllowed)
			}),
			opts...,
		),
	)
}