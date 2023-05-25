package turtleware_test

import (
	"github.com/justinas/alice"
	"github.com/kernle32dll/turtleware"
	"github.com/stretchr/testify/suite"

	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type MiddlewareCreateSuite struct {
	CommonSuite

	response *httptest.ResponseRecorder
	request  *http.Request
}

var ErrTestCreateModelTest = errors.New("validation test")

type TestCreateModel struct {
	SomeString      string
	ValidationError bool
}

func (t TestCreateModel) Validate() []error {
	if !t.ValidationError {
		return nil
	}

	return []error{ErrTestCreateModelTest}
}

func TestMiddlewareCreateSuite(t *testing.T) {
	suite.Run(t, &MiddlewareCreateSuite{})
}

func (s *MiddlewareCreateSuite) SetupTest() {
	s.CommonSuite.SetupTest()

	s.response = httptest.NewRecorder()
	s.request = httptest.NewRequest(http.MethodGet, "https://example.com/foo", http.NoBody)
}

func (s *MiddlewareCreateSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *MiddlewareCreateSuite) Test_DefaultCreateErrorHandler_Handled() {
	// given
	cases := map[string]struct {
		err        error
		goldenFile string
		statusCode int
	}{
		"ErrMarshalling": {
			// handled via DefaultErrorHandler
			err:        turtleware.ErrMarshalling,
			goldenFile: "error_errmarshalling.json",
			statusCode: http.StatusBadRequest,
		},
	}

	for testName, target := range cases {
		s.Run(testName, func() {
			// given
			targetError := target.err

			// when
			turtleware.DefaultCreateErrorHandler(context.Background(), s.response, s.request, targetError)

			// then
			s.Equal(target.statusCode, s.response.Code)
			s.JSONEq(s.loadTestDataString("errorhandler/create/"+target.goldenFile), s.response.Body.String())
			s.True(turtleware.IsHandledByDefaultCreateErrorHandler(targetError))
		})
	}
}

func (s *MiddlewareCreateSuite) Test_DefaultCreateErrorHandler_NotHandled() {
	// given
	targetError := errors.New("some-error")

	// when
	turtleware.DefaultCreateErrorHandler(context.Background(), s.response, s.request, targetError)

	// then
	s.JSONEq(s.loadTestDataString("errors/some_error.json"), s.response.Body.String())
	s.False(turtleware.IsHandledByDefaultCreateErrorHandler(targetError))
}

func (s *MiddlewareCreateSuite) Test_ResourceCreateMiddleware_ErrContextMissingAuthClaims() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	testChain := alice.New(
		turtleware.ResourceCreateMiddleware[TestCreateModel](nil, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.False(nextCapture.Called)
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrContextMissingAuthClaims)
}

func (s *MiddlewareCreateSuite) Test_ResourceCreateMiddleware_ErrContextMissingEntityUUID() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	testChain := alice.New(
		s.buildAuthChain,
		turtleware.ResourceCreateMiddleware[TestCreateModel](nil, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.False(nextCapture.Called)
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrContextMissingEntityUUID)
}

func (s *MiddlewareCreateSuite) Test_ResourceCreateMiddleware_TrashBody() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	s.request.Body = io.NopCloser(bytes.NewBufferString("trash"))

	testChain := alice.New(
		s.buildAuthChain,
		s.buildEntityUUIDChain,
		turtleware.ResourceCreateMiddleware[TestCreateModel](nil, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.False(nextCapture.Called)
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrMarshalling)
}

func (s *MiddlewareCreateSuite) Test_ResourceCreateMiddleware_ValidationError() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	model := TestCreateModel{
		SomeString:      "test",
		ValidationError: true,
	}

	s.request.Body = s.createModelBodyReader(model)

	testChain := alice.New(
		s.buildAuthChain,
		s.buildEntityUUIDChain,
		turtleware.ResourceCreateMiddleware[TestCreateModel](nil, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.False(nextCapture.Called)

	s.ErrorAs(errorCapture.CapturedError, &turtleware.ValidationWrapperError{})
	s.ErrorIs(errorCapture.CapturedError, ErrTestCreateModelTest)
}

func (s *MiddlewareCreateSuite) Test_ResourceCreateMiddleware_Handle_Err() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}
	model := TestCreateModel{
		SomeString: "test",
	}

	s.request.Body = s.createModelBodyReader(model)

	targetError := errors.New("some-error")

	createHandlerFunc := func(
		ctx context.Context,
		entityUUID,
		userUUID string,
		create TestCreateModel,
	) error {
		return targetError
	}

	testChain := alice.New(
		s.buildAuthChain,
		s.buildEntityUUIDChain,
		turtleware.ResourceCreateMiddleware(createHandlerFunc, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.False(nextCapture.Called)
	s.ErrorIs(errorCapture.CapturedError, targetError)
}

func (s *MiddlewareCreateSuite) Test_ResourceCreateMiddleware_Success() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}
	model := TestCreateModel{
		SomeString: "test",
	}

	s.request.Body = s.createModelBodyReader(model)

	createHandlerFunc := func(
		ctx context.Context,
		entityUUID,
		userUUID string,
		create TestCreateModel,
	) error {
		s.Equal(s.entityUUID, entityUUID)
		s.Equal(s.userUUID, userUUID)
		s.Equal(model, create)
		return nil
	}

	testChain := alice.New(
		s.buildAuthChain,
		s.buildEntityUUIDChain,
		turtleware.ResourceCreateMiddleware(createHandlerFunc, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.True(nextCapture.Called)
	s.NoError(errorCapture.CapturedError)
}

func (s *MiddlewareCreateSuite) createModelBodyReader(model turtleware.CreateDTO) io.ReadCloser {
	pr, pw := io.Pipe()
	encoder := json.NewEncoder(pw)
	go func(encoder *json.Encoder) {
		s.Require().NoError(encoder.Encode(model))
	}(encoder)
	return pr
}
