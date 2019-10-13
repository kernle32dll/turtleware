package turtleware

import (
	"github.com/sirupsen/logrus"

	"net/http"
)

// WriteError sets the given status code, and writes a nicely formatted json
// errors to the response body - if the request type is not HEAD.
func WriteError(w http.ResponseWriter, r *http.Request, code int, errors ...error) {
	w.WriteHeader(code)

	if r.Method != http.MethodHead {
		fields := logrus.Fields{
			"errors":     errors,
			"error_code": code,
		}
		logrus.WithFields(fields).Warnf("Writing errors: %s", errors)

		errorList := make([]string, len(errors))
		for i, err := range errors {
			errorList[i] = err.Error()
		}

		errorMap := make(map[string]interface{}, 3)
		errorMap["status"] = code
		errorMap["text"] = http.StatusText(code)
		errorMap["errors"] = errorList

		defer func() {
			if r := recover(); r != nil {
				logrus.WithFields(fields).Errorf("Error while marshalling error message: %s", r)
			}
		}()
		EmissioneWriter.Write(w, r, code, errorMap)
	}
}
