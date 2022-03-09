package turtleware_test

import (
	"github.com/kernle32dll/turtleware"

	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Server Suite")
}

// ExpectedError creates an error output, as returned by turtleware.
func ExpectedError(status int, errors ...error) []byte {
	errorList := make([]string, len(errors))
	for i, err := range errors {
		errorList[i] = err.Error()
	}

	errObj := struct {
		Status int      `json:"status"`
		Text   string   `json:"text"`
		Errors []string `json:"errors"`
	}{
		Status: status,
		Text:   http.StatusText(status),
		Errors: errorList,
	}

	rec := httptest.NewRecorder()

	turtleware.EmissioneWriter.Write(rec, &http.Request{}, status, errObj)

	return rec.Body.Bytes()
}
