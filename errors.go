package turtleware

import (
	"github.com/sirupsen/logrus"

	"encoding/json"
	"net/http"
)

// WriteError sets the given status code, and writes a nicely formatted json
// error to the response body - if the request type is not HEAD.
func WriteError(w http.ResponseWriter, r *http.Request, error error, code int) {
	w.WriteHeader(code)

	if r.Method != http.MethodHead {
		fields := logrus.Fields{
			"error":      error,
			"error_code": code,
		}
		logrus.WithFields(fields).Warnf("Writing error: %s", error)

		// Set content type, if not already set
		if len(w.Header().Get("Content-Type")) == 0 {
			w.Header().Set("Content-Type", "application/json")
		}

		errorMap := make(map[string]interface{}, 3)
		errorMap["status"] = code
		errorMap["text"] = http.StatusText(code)
		errorMap["error"] = error.Error()

		pagesJSON, err := json.MarshalIndent(errorMap, "", "  ")
		if err != nil {
			fields := logrus.Fields{
				"error":      error,
				"error_code": code,
				"next_error": err,
			}

			logrus.WithFields(fields).Errorf("Error while marshalling error message: %s", err)
			return
		}

		if _, err := w.Write(pagesJSON); err != nil {
			fields := logrus.Fields{
				"error":      error,
				"error_code": code,
				"next_error": err,
			}
			logrus.WithFields(fields).Errorf("Error while writing marshaled error message: %s", err)
		}
	}
}
