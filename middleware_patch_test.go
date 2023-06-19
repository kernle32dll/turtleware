package turtleware_test

import (
	"github.com/google/uuid"
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
	"time"
)

type MiddlewarePatchSuite struct {
	CommonSuite

	response *httptest.ResponseRecorder
	request  *http.Request
}

var ErrTestPatchModelTest = errors.New("validation test")

type TestPatchModel struct {
	SomeString      string
	HasSomeChanges  bool
	ValidationError bool
}

func (t TestPatchModel) HasChanges() bool {
	return t.HasSomeChanges
}

func (t TestPatchModel) Validate() []error {
	if !t.ValidationError {
		return nil
	}

	return []error{ErrTestPatchModelTest}
}

func TestMiddlewarePatchSuite(t *testing.T) {
	suite.Run(t, &MiddlewarePatchSuite{})
}

func (s *MiddlewarePatchSuite) SetupTest() {
	s.CommonSuite.SetupTest()

	s.response = httptest.NewRecorder()
	s.request = httptest.NewRequest(http.MethodGet, "https://example.com/foo", http.NoBody)
}

func (s *MiddlewarePatchSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *MiddlewarePatchSuite) Test_DefaultPatchErrorHandler_Handled() {
	// given
	cases := map[string]struct {
		err        error
		goldenFile string
		statusCode int
	}{
		"ErrUnmodifiedSinceHeaderInvalid": {
			err:        turtleware.ErrUnmodifiedSinceHeaderInvalid,
			goldenFile: "error_errunmodifiedsinceheaderinvalid.json",
			statusCode: http.StatusBadRequest,
		},
		"ErrNoChanges": {
			err:        turtleware.ErrNoChanges,
			goldenFile: "error_errnochanges.json",
			statusCode: http.StatusBadRequest,
		},
		"ErrUnmodifiedSinceHeaderMissing": {
			err:        turtleware.ErrUnmodifiedSinceHeaderMissing,
			goldenFile: "error_errunmodifiedsinceheadermissing.json",
			statusCode: http.StatusPreconditionRequired,
		},
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
			turtleware.DefaultPatchErrorHandler(context.Background(), s.response, s.request, targetError)

			// then
			s.Equal(target.statusCode, s.response.Code)
			s.JSONEq(s.loadTestDataString("errorhandler/patch/"+target.goldenFile), s.response.Body.String())
			s.True(turtleware.IsHandledByDefaultPatchErrorHandler(targetError))
		})
	}
}

func (s *MiddlewarePatchSuite) Test_DefaultPatchErrorHandler_NotHandled() {
	// given
	targetError := errors.New("some-error")

	// when
	turtleware.DefaultPatchErrorHandler(context.Background(), s.response, s.request, targetError)

	// then
	s.JSONEq(s.loadTestDataString("errors/some_error.json"), s.response.Body.String())
	s.False(turtleware.IsHandledByDefaultPatchErrorHandler(targetError))
}

func (s *MiddlewarePatchSuite) Test_ResourcePatchMiddleware_ErrContextMissingAuthClaims() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	testChain := alice.New(
		turtleware.ResourcePatchMiddleware[TestPatchModel](nil, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.False(nextCapture.Called)
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrContextMissingAuthClaims)
}

func (s *MiddlewarePatchSuite) Test_ResourcePatchMiddleware_ErrContextMissingEntityUUID() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	testChain := alice.New(
		s.buildAuthChain,
		turtleware.ResourcePatchMiddleware[TestPatchModel](nil, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.False(nextCapture.Called)
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrContextMissingEntityUUID)
}

func (s *MiddlewarePatchSuite) Test_ResourcePatchMiddleware_TrashBody() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	s.request.Body = io.NopCloser(bytes.NewBufferString("trash"))

	testChain := alice.New(
		s.buildAuthChain,
		s.buildEntityUUIDChain,
		turtleware.ResourcePatchMiddleware[TestPatchModel](nil, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.False(nextCapture.Called)
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrMarshalling)
}

func (s *MiddlewarePatchSuite) Test_ResourcePatchMiddleware_NoChanges() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}
	model := TestPatchModel{
		SomeString:     "test",
		HasSomeChanges: false,
	}

	s.request.Body = s.patchModelBodyReader(model)

	testChain := alice.New(
		s.buildAuthChain,
		s.buildEntityUUIDChain,
		turtleware.ResourcePatchMiddleware[TestPatchModel](nil, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.False(nextCapture.Called)
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrNoChanges)
}

func (s *MiddlewarePatchSuite) Test_ResourcePatchMiddleware_ValidationError() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}
	model := TestPatchModel{
		SomeString:      "test",
		ValidationError: true,
		HasSomeChanges:  true,
	}

	s.request.Body = s.patchModelBodyReader(model)

	testChain := alice.New(
		s.buildAuthChain,
		s.buildEntityUUIDChain,
		turtleware.ResourcePatchMiddleware[TestPatchModel](nil, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.False(nextCapture.Called)
	s.ErrorAs(errorCapture.CapturedError, &turtleware.ValidationWrapperError{})
	s.ErrorIs(errorCapture.CapturedError, ErrTestPatchModelTest)
}

func (s *MiddlewarePatchSuite) Test_ResourcePatchMiddleware_UnmodifiedSinceError() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}
	model := TestPatchModel{
		SomeString:      "test",
		ValidationError: false,
		HasSomeChanges:  true,
	}

	s.request.Body = s.patchModelBodyReader(model)
	s.request.Header.Set("If-Unmodified-Since", "trash")

	testChain := alice.New(
		s.buildAuthChain,
		s.buildEntityUUIDChain,
		turtleware.ResourcePatchMiddleware[TestPatchModel](nil, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.False(nextCapture.Called)
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrUnmodifiedSinceHeaderInvalid)
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrNoDateTimeLayoutMatched)
}

func (s *MiddlewarePatchSuite) Test_ResourcePatchMiddleware_Handle_Err() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}
	testTime := time.Now().UTC()
	model := TestPatchModel{
		SomeString:     "test",
		HasSomeChanges: true,
	}

	s.request.Body = s.patchModelBodyReader(model)
	s.request.Header.Set("If-Unmodified-Since", testTime.Format(time.RFC3339Nano))

	targetError := errors.New("some-error")

	patchHandlerFunc := func(context.Context, uuid.UUID, uuid.UUID, TestPatchModel, time.Time) error {
		return targetError
	}

	testChain := alice.New(
		s.buildAuthChain,
		s.buildEntityUUIDChain,
		turtleware.ResourcePatchMiddleware(patchHandlerFunc, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.False(nextCapture.Called)
	s.ErrorIs(errorCapture.CapturedError, targetError)
}

func (s *MiddlewarePatchSuite) Test_ResourcePatchMiddleware_Success() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}
	testTime := time.Now().UTC()
	model := TestPatchModel{
		SomeString:     "test",
		HasSomeChanges: true,
	}

	s.request.Body = s.patchModelBodyReader(model)
	s.request.Header.Set("If-Unmodified-Since", testTime.Format(time.RFC3339Nano))

	patchHandlerFuncWasCalled := false
	patchHandlerFunc := func(ctx context.Context, entityUUID, userUUID uuid.UUID, create TestPatchModel, ifUnmodifiedSince time.Time) error {
		patchHandlerFuncWasCalled = true
		s.Equal(s.entityUUID, entityUUID)
		s.Equal(s.userUUID, userUUID)
		s.Equal(model, create)
		s.Equal(testTime, ifUnmodifiedSince)
		return nil
	}

	testChain := alice.New(
		s.buildAuthChain,
		s.buildEntityUUIDChain,
		turtleware.ResourcePatchMiddleware(patchHandlerFunc, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.True(nextCapture.Called)
	s.NoError(errorCapture.CapturedError)
	s.True(patchHandlerFuncWasCalled)
}

func (s *MiddlewarePatchSuite) patchModelBodyReader(model turtleware.PatchDTO) io.ReadCloser {
	pr, pw := io.Pipe()
	encoder := json.NewEncoder(pw)
	go func(encoder *json.Encoder) {
		s.Require().NoError(encoder.Encode(model))
	}(encoder)
	return pr
}

func (s *MiddlewarePatchSuite) Test_GetIfUnmodifiedSince_NoHeader() {
	// given
	// -

	// when
	_, err := turtleware.GetIfUnmodifiedSince(s.request)

	// then
	s.ErrorIs(err, turtleware.ErrUnmodifiedSinceHeaderMissing)
}

func (s *MiddlewarePatchSuite) Test_GetIfUnmodifiedSince_NothingMatches() {
	// given
	s.request.Header.Set("If-Unmodified-Since", "trash")

	// when
	_, err := turtleware.GetIfUnmodifiedSince(s.request)

	// then
	s.ErrorIs(err, turtleware.ErrUnmodifiedSinceHeaderInvalid)
	s.ErrorIs(err, turtleware.ErrNoDateTimeLayoutMatched)
}

func (s *MiddlewarePatchSuite) Test_GetIfUnmodifiedSince_Matches() {
	testTime := time.Now().UTC()
	cases := map[string]struct {
		format      string
		compareTime time.Time
	}{
		"RFC1123": {
			format:      time.RFC1123,
			compareTime: testTime.Truncate(time.Second),
		},
		"RFC3339": {
			format:      time.RFC3339,
			compareTime: testTime.Truncate(time.Second),
		},
		"RFC3339Nano": {
			format:      time.RFC3339Nano,
			compareTime: testTime,
		},
	}

	for testName, target := range cases {
		s.Run(testName, func() {
			// given
			s.request.Header.Set("If-Unmodified-Since", testTime.Format(target.format))

			// when
			parsedTime, err := turtleware.GetIfUnmodifiedSince(s.request)

			// then
			s.Equal(target.compareTime, parsedTime)
			s.NoError(err)
		})
	}
}
