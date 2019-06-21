package turtleware

import (
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"github.com/sirupsen/logrus"

	"encoding/json"
	"net/http"
)

// WriteError sets the given status code, and writes a nicely formatted json
// errors to the response body - if the request type is not HEAD.
func WriteError(w http.ResponseWriter, r *http.Request, code int, errors ...error) {
	w.WriteHeader(code)

	// Log errors to the request span, if we have one
	if span := opentracing.SpanFromContext(r.Context()); span != nil {
		ext.Error.Set(span, true)
		span.LogFields(
			log.Object("event", "error"),
			log.Object("error.object", errors),
		)
	}

	if r.Method != http.MethodHead {
		fields := logrus.Fields{
			"errors":     errors,
			"error_code": code,
		}
		logrus.WithFields(fields).Warnf("Writing errors: %s", errors)

		// Set content type, if not already set
		if len(w.Header().Get("Content-Type")) == 0 {
			w.Header().Set("Content-Type", "application/json")
		}

		errorList := make([]string, len(errors))
		for i, err := range errors {
			errorList[i] = err.Error()
		}

		errorMap := make(map[string]interface{}, 3)
		errorMap["status"] = code
		errorMap["text"] = http.StatusText(code)
		errorMap["errors"] = errorList

		pagesJSON, err := json.MarshalIndent(errorMap, "", "  ")
		if err != nil {
			fields := logrus.Fields{
				"errors":     errors,
				"error_code": code,
				"next_error": err,
			}

			logrus.WithFields(fields).Errorf("Error while marshalling error message: %s", err)
			return
		}

		if _, err := w.Write(pagesJSON); err != nil {
			fields := logrus.Fields{
				"errors":     errors,
				"error_code": code,
				"next_error": err,
			}
			logrus.WithFields(fields).Errorf("Error while writing marshaled error message: %s", err)
		}
	}
}
