package turtleware_test

import (
	_ "embed"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kernle32dll/turtleware"
	"github.com/stretchr/testify/suite"
)

var (
	//go:embed testdata/router_routenotmached.json
	expectedRouteNotMatched string

	//go:embed testdata/router_methodnotmached.json
	expectedMethodNotMatched string

	//go:embed testdata/router_panic.json
	expectedPanicResponse string
)

type RouterSuite struct {
	suite.Suite
}

func TestRouterSuite(t *testing.T) {
	suite.Run(t, &RouterSuite{})
}

func (s *RouterSuite) Test_RouteNotMatched() {
	// given
	rtr := turtleware.NewRouter("")
	r := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/unknown", nil)

	// when
	recorder := httptest.NewRecorder()
	rtr.Build().ServeHTTP(recorder, r)

	// then
	s.Equal(http.StatusNotFound, recorder.Code)
	s.JSONEq(expectedRouteNotMatched, recorder.Body.String())

	s.Equal(http.Header{
		"Cache-Control": []string{"no-store"},
		"Content-Type":  []string{"application/json;charset=utf-8"},
		"Vary":          []string{"Accept-Encoding"},
	}, recorder.Header())
}

func (s *RouterSuite) Test_MethodNotMatched() {
	// given
	rtr := turtleware.NewRouter("")
	rtr.Entity("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	r := httptest.NewRequestWithContext(s.T().Context(), http.MethodDelete, "/test", nil)

	// when
	recorder := httptest.NewRecorder()
	rtr.Build().ServeHTTP(recorder, r)

	// then
	s.Equal(http.StatusMethodNotAllowed, recorder.Code)
	s.JSONEq(expectedMethodNotMatched, recorder.Body.String())

	s.Equal(http.Header{
		"Allow":         []string{"GET, HEAD, OPTIONS"},
		"Cache-Control": []string{"no-store"},
		"Content-Type":  []string{"application/json;charset=utf-8"},
		"Vary":          []string{"Accept-Encoding"},
	}, recorder.Header())
}

func (s *RouterSuite) Test_Endpoint_CORS() {
	// given
	rtr := turtleware.NewRouter("")

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(`some-content`))
	})

	rtr.Entity("/entity", dummyHandler)
	rtr.List("/list", dummyHandler)
	rtr.Update("/patch", dummyHandler)
	rtr.Create("/post", dummyHandler)
	rtr.Replace("/put", dummyHandler)

	testMatrix := []struct {
		method           string
		url              string
		exposedHeaders   []string
		requestedHeaders []string
	}{
		{method: http.MethodGet, url: "/entity", requestedHeaders: []string{"authorization"}},
		{method: http.MethodHead, url: "/entity", requestedHeaders: []string{"authorization"}},
		{method: http.MethodGet, url: "/list", requestedHeaders: []string{"authorization"}, exposedHeaders: []string{"X-Total-Count"}},
		{method: http.MethodHead, url: "/list", requestedHeaders: []string{"authorization"}, exposedHeaders: []string{"X-Total-Count"}},
		{method: http.MethodPatch, url: "/patch", requestedHeaders: []string{"content-type", "if-unmodified-since"}},
		{method: http.MethodPost, url: "/post", requestedHeaders: []string{"content-type"}},
		{method: http.MethodPut, url: "/put", requestedHeaders: []string{"content-type"}},
	}

	for _, tt := range testMatrix {
		s.Run(fmt.Sprintf("%s %s Preflight", tt.method, tt.url), func() {
			// when
			r := httptest.NewRequestWithContext(s.T().Context(), http.MethodOptions, tt.url, nil)
			r.Header.Set("Access-Control-Request-Method", strings.ToUpper(tt.method))
			r.Header.Set("Access-Control-Request-Headers", strings.Join(tt.requestedHeaders, ", "))
			r.Header.Set("Origin", "https://example.com")
			recorder := httptest.NewRecorder()
			rtr.Build().ServeHTTP(recorder, r)

			// then
			s.Equal(http.StatusNoContent, recorder.Code)
			s.Empty(recorder.Body.String())
			s.Equal(http.Header{
				"Access-Control-Allow-Credentials": []string{"true"},
				"Access-Control-Allow-Headers":     []string{strings.Join(tt.requestedHeaders, ", ")},
				"Access-Control-Allow-Methods":     []string{r.Header.Get("Access-Control-Request-Method")},
				"Access-Control-Allow-Origin":      []string{r.Header.Get("Origin")},
				"Access-Control-Max-Age":           []string{"86400"},
				"Vary":                             []string{"Accept-Encoding", "Origin, Access-Control-Request-Method, Access-Control-Request-Headers"},
			}, recorder.Header())
		})

		s.Run(fmt.Sprintf("%s %s Actual", tt.method, tt.url), func() {
			// when
			r := httptest.NewRequestWithContext(s.T().Context(), tt.method, tt.url, nil)
			r.Header.Set("Access-Control-Request-Method", strings.ToUpper(tt.method))
			r.Header.Set("Access-Control-Request-Headers", strings.Join(tt.requestedHeaders, ", "))
			r.Header.Set("Origin", "https://example.com")
			recorder := httptest.NewRecorder()
			rtr.Build().ServeHTTP(recorder, r)

			// then
			s.Equal(http.StatusTeapot, recorder.Code)
			s.Equal("some-content", recorder.Body.String())
			s.Equal(http.Header{
				"Access-Control-Allow-Credentials": []string{"true"},
				"Access-Control-Allow-Origin":      []string{r.Header.Get("Origin")},
				"Access-Control-Expose-Headers":    []string{strings.Join(append([]string{"Uber-Trace-Id", "Accept"}, tt.exposedHeaders...), ", ")},
				"Vary":                             []string{"Accept-Encoding", "Origin"},
			}, recorder.Header())
		})
	}

	s.Run("Unmatched", func() {
		// when
		r := httptest.NewRequestWithContext(s.T().Context(), http.MethodOptions, "/entity", nil)
		r.Header.Set("Access-Control-Request-Method", "POST")
		r.Header.Set("Origin", "https://example.com")
		recorder := httptest.NewRecorder()
		rtr.Build().ServeHTTP(recorder, r)

		// then
		s.Equal(http.StatusNoContent, recorder.Code)
		s.Empty(recorder.Body.String())
		s.Equal(http.Header{
			"Vary": []string{"Accept-Encoding", "Origin, Access-Control-Request-Method, Access-Control-Request-Headers"},
		}, recorder.Header())
	})
}

func (s *RouterSuite) Test_Endpoint_Entity() {
	// given
	rtr := turtleware.NewRouter("")
	rtr.Entity("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"ok"}`))
	}))

	// when
	r := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/test", nil)
	recorder := httptest.NewRecorder()
	rtr.Build().ServeHTTP(recorder, r)

	// then
	s.Equal(http.StatusOK, recorder.Code)
	s.JSONEq(`{"message":"ok"}`, recorder.Body.String())
	s.Equal(http.Header{
		"Vary": []string{"Accept-Encoding", "Origin"},
	}, recorder.Header())
}

func (s *RouterSuite) Test_Endpoint_List() {
	// given
	rtr := turtleware.NewRouter("")
	rtr.List("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"list ok"}`))
	}))

	// when
	r := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/test", nil)
	recorder := httptest.NewRecorder()
	rtr.Build().ServeHTTP(recorder, r)

	// then
	s.Equal(http.StatusOK, recorder.Code)
	s.JSONEq(`{"message":"list ok"}`, recorder.Body.String())
	s.Equal(http.Header{
		"Vary": []string{"Accept-Encoding", "Origin"},
	}, recorder.Header())
}

func (s *RouterSuite) Test_Endpoint_Patch() {
	// given
	rtr := turtleware.NewRouter("")
	rtr.Update("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"patch ok"}`))
	}))

	// when
	r := httptest.NewRequestWithContext(s.T().Context(), http.MethodPatch, "/test", nil)
	recorder := httptest.NewRecorder()
	rtr.Build().ServeHTTP(recorder, r)

	// then
	s.Equal(http.StatusOK, recorder.Code)
	s.JSONEq(`{"message":"patch ok"}`, recorder.Body.String())
	s.Equal(http.Header{
		"Vary": []string{"Accept-Encoding", "Origin"},
	}, recorder.Header())
}

func (s *RouterSuite) Test_Endpoint_Post() {
	// given
	rtr := turtleware.NewRouter("")
	rtr.Create("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"post ok"}`))
	}))

	// when
	r := httptest.NewRequestWithContext(s.T().Context(), http.MethodPost, "/test", nil)
	recorder := httptest.NewRecorder()
	rtr.Build().ServeHTTP(recorder, r)

	// then
	s.Equal(http.StatusOK, recorder.Code)
	s.JSONEq(`{"message":"post ok"}`, recorder.Body.String())
	s.Equal(http.Header{
		"Vary": []string{"Accept-Encoding", "Origin"},
	}, recorder.Header())
}

func (s *RouterSuite) Test_Endpoint_Put() {
	// given
	rtr := turtleware.NewRouter("")
	rtr.Replace("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"put ok"}`))
	}))

	// when
	r := httptest.NewRequestWithContext(s.T().Context(), http.MethodPut, "/test", nil)
	recorder := httptest.NewRecorder()
	rtr.Build().ServeHTTP(recorder, r)

	// then
	s.Equal(http.StatusOK, recorder.Code)
	s.JSONEq(`{"message":"put ok"}`, recorder.Body.String())
	s.Equal(http.Header{
		"Vary": []string{"Accept-Encoding", "Origin"},
	}, recorder.Header())
}

func (s *RouterSuite) Test_Panic() {
	// given
	rtr := turtleware.NewRouter("")
	rtr.Entity("/panic", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	}))

	// when
	r := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/panic", nil)
	recorder := httptest.NewRecorder()
	rtr.Build().ServeHTTP(recorder, r)

	// then
	s.Equal(http.StatusInternalServerError, recorder.Code)
	s.JSONEq(expectedPanicResponse, recorder.Body.String())
	s.Equal(http.Header{
		"Cache-Control": []string{"no-store"},
		"Content-Type":  []string{"application/json;charset=utf-8"},
		"Vary":          []string{"Accept-Encoding", "Origin"},
	}, recorder.Header())
}

func (s *RouterSuite) Test_Middleware() {
	// given
	rtr := turtleware.NewRouter("")
	rtr.AddMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Custom-Middleware", "active")
			next.ServeHTTP(w, r)
		})
	})

	rtr.Entity("/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"ok"}`))
	}))

	// when
	r := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/test", nil)
	recorder := httptest.NewRecorder()
	rtr.Build().ServeHTTP(recorder, r)

	// then
	s.Equal(http.StatusOK, recorder.Code)
	s.JSONEq(`{"message":"ok"}`, recorder.Body.String())
	s.Equal("active", recorder.Header().Get("X-Custom-Middleware"))
}
