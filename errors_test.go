package turtleware_test

import (
	"github.com/kernle32dll/turtleware"

	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteError(t *testing.T) {
	err1, err2 := errors.New("error1"), errors.New("error2")

	tests := []struct {
		name    string
		accepts string
		errors  []error
		want    string
	}{
		{"json", "application/json", []error{err1, err2}, `{
  "status": 418,
  "text": "I'm a teapot",
  "errors": [
    "error1",
    "error2"
  ]
}`},
		{"json-empty", "application/json", []error{}, `{
  "status": 418,
  "text": "I'm a teapot",
  "errors": []
}`},
		{"json-nil", "application/json", nil, `{
  "status": 418,
  "text": "I'm a teapot",
  "errors": []
}`},
		{"xml", "application/xml", []error{err1, err2}, `<ErrorResponse>
  <Status>418</Status>
  <Text>I&#39;m a teapot</Text>
  <ErrorList>
    <Error>error1</Error>
    <Error>error2</Error>
  </ErrorList>
</ErrorResponse>`},
		{"xml-empty", "application/xml", []error{}, `<ErrorResponse>
  <Status>418</Status>
  <Text>I&#39;m a teapot</Text>
  <ErrorList></ErrorList>
</ErrorResponse>`},
		{"xml-nil", "application/xml", nil, `<ErrorResponse>
  <Status>418</Status>
  <Text>I&#39;m a teapot</Text>
  <ErrorList></ErrorList>
</ErrorResponse>`},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()

			turtleware.WriteError(
				w,
				&http.Request{Header: map[string][]string{"Accept": {tt.accepts}}},
				http.StatusTeapot,
				tt.errors...,
			)

			if w.Code != http.StatusTeapot {
				t.Errorf("Write() = %v, want %v", w.Code, http.StatusTeapot)
			}

			if got := w.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Write() = %v, want %v", got, "no-store")
			}

			if got := w.Body.String(); got != tt.want {
				t.Errorf("Write() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWriteError_head(t *testing.T) {
	w := httptest.NewRecorder()

	turtleware.WriteError(
		w,
		&http.Request{
			Method: http.MethodHead,
			Header: map[string][]string{"Accept": {"*/*"}},
		},
		http.StatusTeapot,
		errors.New("error1"), errors.New("error2"),
	)

	if w.Code != http.StatusTeapot {
		t.Errorf("Write() = %v, want %v", w.Code, http.StatusTeapot)
	}

	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Write() = %v, want %v", got, "no-store")
	}

	if got := w.Body.String(); got != "" {
		t.Errorf("Write() = %v, want %v", got, "")
	}
}
