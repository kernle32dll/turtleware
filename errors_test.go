package turtleware_test

import (
	"errors"
	"github.com/kernle32dll/turtleware"
	"github.com/stretchr/testify/suite"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode"
)

type ErrorsSuite struct {
	CommonSuite

	w *httptest.ResponseRecorder

	err1 error
	err2 error
}

func TestErrorsSuite(t *testing.T) {
	suite.Run(t, &ErrorsSuite{})
}

func (s *ErrorsSuite) SetupSuite() {
	s.err1, s.err2 = errors.New("error1"), errors.New("error2")
}

func (s *ErrorsSuite) SetupTest() {
	s.w = httptest.NewRecorder()
}

func (s *ErrorsSuite) Test_Head() {
	// given
	r := &http.Request{
		Method: http.MethodHead,
		Header: map[string][]string{"Accept": {"*/*"}},
	}

	// when
	turtleware.WriteError(s.w, r, http.StatusTeapot)

	// then
	s.Equal(http.StatusTeapot, s.w.Code)
	s.Equal("no-store", s.w.Header().Get("Cache-Control"))
	s.Empty(s.w.Body.String())
}

func (s *ErrorsSuite) Test_Json_MultipleErrors() {
	// given
	r := &http.Request{
		Method: http.MethodGet,
		Header: map[string][]string{"Accept": {"application/json"}},
	}

	// when
	turtleware.WriteError(s.w, r, http.StatusTeapot, s.err1, s.err2)

	// then
	s.Equal(http.StatusTeapot, s.w.Code)
	s.Equal("no-store", s.w.Header().Get("Cache-Control"))
	s.JSONEq(
		s.loadTestDataString("errors/multiple_errors.json"),
		s.w.Body.String(),
	)
}

func (s *ErrorsSuite) Test_Json_EmptyErrors() {
	// given
	r := &http.Request{
		Method: http.MethodGet,
		Header: map[string][]string{"Accept": {"application/json"}},
	}

	// when
	turtleware.WriteError(s.w, r, http.StatusTeapot)

	// then
	s.Equal(http.StatusTeapot, s.w.Code)
	s.Equal("no-store", s.w.Header().Get("Cache-Control"))
	s.JSONEq(
		s.loadTestDataString("errors/empty_errors.json"),
		s.w.Body.String(),
	)
}

func (s *ErrorsSuite) Test_xml_MultipleErrors() {
	// given
	r := &http.Request{
		Method: http.MethodGet,
		Header: map[string][]string{"Accept": {"application/xml"}},
	}

	// when
	turtleware.WriteError(s.w, r, http.StatusTeapot, s.err1, s.err2)

	// then
	s.Equal(http.StatusTeapot, s.w.Code)
	s.Equal("no-store", s.w.Header().Get("Cache-Control"))
	s.Equal(
		// Note: We need to replace spaces here, since whitespaces
		// loaded on different OSes differ (\r vs \r\n)
		stripSpaces(s.loadTestDataString("errors/multiple_errors.xml")),
		stripSpaces(s.w.Body.String()),
	)
}

func stripSpaces(str string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, str)
}

func (s *ErrorsSuite) Test_xml_EmptyErrors() {
	// given
	r := &http.Request{
		Method: http.MethodGet,
		Header: map[string][]string{"Accept": {"application/xml"}},
	}

	// when
	turtleware.WriteError(s.w, r, http.StatusTeapot)

	// then
	s.Equal(http.StatusTeapot, s.w.Code)
	s.Equal("no-store", s.w.Header().Get("Cache-Control"))
	s.Equal(
		// Note: We need to replace spaces here, since whitespaces
		// loaded on different OSes differ (\r vs \r\n)
		stripSpaces(s.loadTestDataString("errors/empty_errors.xml")),
		stripSpaces(s.w.Body.String()),
	)
}
