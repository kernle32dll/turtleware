package tenant_test

import (
	"context"
	"net/http"
)

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
