package turtleware_test

import (
	"github.com/kernle32dll/turtleware"

	"context"
	"net/http"
	"net/http/httptest"
)

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

// ErrorHandlerCapture is a helper struct to capture errors for a turtleware.ErrorHandlerFunc.
type ErrorHandlerCapture struct {
	CapturedError error
}

func (e *ErrorHandlerCapture) Capture(_ context.Context, _ http.ResponseWriter, _ *http.Request, err error) {
	e.CapturedError = err
}

// MiddlewareCapture is a helper struct to capture a middleware calling the next handler.
type MiddlewareCapture struct {
	Called bool
}

func (m *MiddlewareCapture) ServeHTTP(http.ResponseWriter, *http.Request) {
	m.Called = true
}
