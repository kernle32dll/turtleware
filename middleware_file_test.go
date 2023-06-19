package turtleware_test

import (
	"github.com/google/uuid"
	"github.com/justinas/alice"
	"github.com/kernle32dll/turtleware"
	"github.com/stretchr/testify/suite"

	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

type MiddlewareFileSuite struct {
	CommonSuite

	response *httptest.ResponseRecorder
	request  *http.Request
}

func TestMiddlewareFileSuite(t *testing.T) {
	suite.Run(t, &MiddlewareFileSuite{})
}

func (s *MiddlewareFileSuite) SetupTest() {
	s.CommonSuite.SetupTest()

	s.response = httptest.NewRecorder()
	s.request = httptest.NewRequest(http.MethodGet, "https://example.com/foo", http.NoBody)
}

func (s *MiddlewareFileSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *MiddlewareFileSuite) Test_DefaultFileUploadErrorHandler_Handled() {
	// given
	cases := map[string]struct {
		err        error
		goldenFile string
	}{
		"ErrNotMultipart": {
			err:        http.ErrNotMultipart,
			goldenFile: "error_errnotmultipart.json",
		},
		"ErrMissingBoundary": {
			err:        http.ErrMissingBoundary,
			goldenFile: "error_errmissingboundary.json",
		},
		"ErrMessageTooLarge": {
			err:        multipart.ErrMessageTooLarge,
			goldenFile: "error_errmessagetoolarge.json",
		},
		"ErrMarshalling": {
			// handled via DefaultErrorHandler
			err:        turtleware.ErrMarshalling,
			goldenFile: "error_errmarshalling.json",
		},
	}

	for testName, target := range cases {
		s.Run(testName, func() {
			// given
			targetError := target.err

			// when
			turtleware.DefaultFileUploadErrorHandler(context.Background(), s.response, s.request, targetError)

			// then
			s.Equal(http.StatusBadRequest, s.response.Code)
			s.JSONEq(s.loadTestDataString("errorhandler/fileupload/"+target.goldenFile), s.response.Body.String())
			s.True(turtleware.IsHandledByDefaultFileUploadErrorHandler(targetError))
		})
	}
}

func (s *MiddlewareFileSuite) Test_DefaultFileUploadErrorHandler_NotHandled() {
	// given
	targetError := errors.New("some-error")

	// when
	turtleware.DefaultFileUploadErrorHandler(context.Background(), s.response, s.request, targetError)

	// then
	s.JSONEq(s.loadTestDataString("errors/some_error.json"), s.response.Body.String())
	s.False(turtleware.IsHandledByDefaultFileUploadErrorHandler(targetError))
}

func (s *MiddlewareFileSuite) Test_FileUploadMiddleware_ErrContextMissingAuthClaims() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	testChain := alice.New(
		turtleware.FileUploadMiddleware(nil, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.False(nextCapture.Called)
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrContextMissingAuthClaims)
}

func (s *MiddlewareFileSuite) Test_FileUploadMiddleware_ErrContextMissingEntityUUID() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	testChain := alice.New(
		s.buildAuthChain,
		turtleware.FileUploadMiddleware(nil, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.False(nextCapture.Called)
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrContextMissingEntityUUID)
}

func (s *MiddlewareFileSuite) Test_FileUploadMiddleware_ErrNotMultipart() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	testChain := alice.New(
		s.buildAuthChain,
		s.buildEntityUUIDChain,
		turtleware.FileUploadMiddleware(nil, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.False(nextCapture.Called)
	s.ErrorIs(errorCapture.CapturedError, http.ErrNotMultipart)
}

func (s *MiddlewareFileSuite) Test_FileUploadMiddleware_FileHandle_Err() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	part, contentType := s.CreateMultipart()
	s.request.Body = io.NopCloser(bytes.NewBuffer(part))
	s.request.Header.Set("Content-Type", contentType)

	targetError := errors.New("some-error")

	fileHandlerFunc := func(
		ctx context.Context,
		entityUUID, userUUID uuid.UUID,
		fileName string,
		file multipart.File,
	) error {
		return targetError
	}

	testChain := alice.New(
		s.buildAuthChain,
		s.buildEntityUUIDChain,
		turtleware.FileUploadMiddleware(fileHandlerFunc, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.False(nextCapture.Called)
	s.ErrorIs(errorCapture.CapturedError, targetError)
}

func (s *MiddlewareFileSuite) Test_FileUploadMiddleware_Success() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	part, contentType := s.CreateMultipart()
	s.request.Body = io.NopCloser(bytes.NewBuffer(part))
	s.request.Header.Set("Content-Type", contentType)

	fileCounter := 1
	fileHandlerFunc := func(
		ctx context.Context,
		entityUUID, userUUID uuid.UUID,
		fileName string,
		file multipart.File,
	) error {
		content, err := io.ReadAll(file)
		s.Require().NoError(err)

		s.Equal(s.entityUUID, entityUUID)
		s.Equal(s.userUUID, userUUID)
		s.Equal(fmt.Sprintf("test%d.txt", fileCounter), fileName)
		s.Equal(fmt.Sprintf("works%d", fileCounter), string(content))
		fileCounter++

		return nil
	}

	testChain := alice.New(
		s.buildAuthChain,
		s.buildEntityUUIDChain,
		turtleware.FileUploadMiddleware(fileHandlerFunc, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.True(nextCapture.Called)
	s.NoError(errorCapture.CapturedError)
}

func (s *MiddlewareFileSuite) CreateMultipart() ([]byte, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	s.attachMultipartFile(writer, "test1.txt", []byte("works1"))
	s.attachMultipartFile(writer, "test2.txt", []byte("works2"))

	s.Require().NoError(writer.Close())

	return body.Bytes(), writer.FormDataContentType()
}

func (s *MiddlewareFileSuite) attachMultipartFile(w *multipart.Writer, fileName string, data []byte) {
	formFile, err := w.CreateFormFile("file", fileName)
	s.Require().NoError(err)

	_, err = formFile.Write(data)
	s.Require().NoError(err)
}
