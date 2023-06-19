package tenant_test

import (
	"github.com/justinas/alice"
	"github.com/kernle32dll/turtleware"
	"github.com/kernle32dll/turtleware/tenant"
	"github.com/stretchr/testify/suite"

	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

type MiddlewareDataSuite struct {
	CommonSuite

	response *httptest.ResponseRecorder
	request  *http.Request
}

type TestDataModel struct {
	SomeString string
	SomeInt    int
}

// TestCloser is an io.ReadCloser abstraction for testing
// closing related functionality of turtleware.ResourceDataHandler.
type TestCloser struct {
	*bytes.Buffer
	closerErr error
	wasClosed bool
}

func (t *TestCloser) Close() error {
	t.wasClosed = true
	return t.closerErr
}

func TestMiddlewareDataSuite(t *testing.T) {
	suite.Run(t, &MiddlewareDataSuite{})
}

func (s *MiddlewareDataSuite) SetupTest() {
	s.CommonSuite.SetupTest()

	s.response = httptest.NewRecorder()
	s.request = httptest.NewRequest(http.MethodGet, "https://example.com/foo", http.NoBody)
}

func (s *MiddlewareDataSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *MiddlewareDataSuite) Test_StaticListDataHandler_Head() {
	// given
	errorCapture := &ErrorHandlerCapture{}

	s.request.Method = http.MethodHead

	testChain := tenant.StaticListDataHandler[TestDataModel](nil, errorCapture.Capture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Empty(s.response.Body.String())
	s.NoError(errorCapture.CapturedError)
}

func (s *MiddlewareDataSuite) Test_StaticListDataHandler_ErrContextMissingTenantUUID() {
	// given
	errorCapture := &ErrorHandlerCapture{}

	testChain := tenant.StaticListDataHandler[TestDataModel](nil, errorCapture.Capture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Empty(s.response.Body.String())
	s.ErrorIs(errorCapture.CapturedError, tenant.ErrContextMissingTenantUUID)
}

func (s *MiddlewareDataSuite) Test_StaticListDataHandler_ErrContextMissingPaging() {
	// given
	errorCapture := &ErrorHandlerCapture{}

	testChain := alice.New(
		s.buildAuthChain,
	).Then(tenant.StaticListDataHandler[TestDataModel](nil, errorCapture.Capture))

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Empty(s.response.Body.String())
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrContextMissingPaging)
}

func (s *MiddlewareDataSuite) Test_StaticListDataHandler_Handle_Err() {
	// given
	errorCapture := &ErrorHandlerCapture{}

	targetError := errors.New("some-error")

	dataFetcherFunc := func(ctx context.Context, tenantUUID string, paging turtleware.Paging) ([]TestDataModel, error) {
		return nil, targetError
	}

	testChain := alice.New(
		s.buildAuthChain,
		turtleware.PagingMiddleware,
	).Then(tenant.StaticListDataHandler(dataFetcherFunc, errorCapture.Capture))

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Empty(s.response.Body.String())
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrReceivingResults)
}

func (s *MiddlewareDataSuite) Test_StaticListDataHandler_Success() {
	// given
	errorCapture := &ErrorHandlerCapture{}

	s.request.URL.RawQuery = "limit=23&offset=5"

	dataFetcherFuncWasCalled := false
	dataFetcherFunc := func(ctx context.Context, tenantUUID string, paging turtleware.Paging) ([]TestDataModel, error) {
		dataFetcherFuncWasCalled = true
		s.Equal(s.tenantUUID, tenantUUID)
		s.Equal(turtleware.Paging{
			Limit:  23,
			Offset: 5,
		}, paging)

		return []TestDataModel{
			{
				SomeString: "test1",
				SomeInt:    42,
			},
			{
				SomeString: "test2",
				SomeInt:    1337,
			},
		}, nil
	}

	testChain := alice.New(
		s.buildAuthChain,
		turtleware.PagingMiddleware,
	).Then(tenant.StaticListDataHandler(dataFetcherFunc, errorCapture.Capture))

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.JSONEq(s.loadTestDataString("data/list_success.json"), s.response.Body.String())
	s.NoError(errorCapture.CapturedError)
	s.True(dataFetcherFuncWasCalled)
}

// Test_StaticListDataHandler_NilResult is an important test, that
// verifies that tenant.StaticListDataHandler writes out an empty result
// array, if the returned array is nil.
//
// This is important, as the serializers in Go actually differentiate between
// nil and an empty array.
func (s *MiddlewareDataSuite) Test_StaticListDataHandler_NilResult() {
	// given
	errorCapture := &ErrorHandlerCapture{}

	dataFetcherFuncWasCalled := false
	dataFetcherFunc := func(ctx context.Context, tenantUUID string, paging turtleware.Paging) ([]TestDataModel, error) {
		dataFetcherFuncWasCalled = true
		s.Equal(s.tenantUUID, tenantUUID)
		s.Equal(turtleware.Paging{
			Limit:  100,
			Offset: 0,
		}, paging)

		return nil, nil
	}

	testChain := alice.New(
		s.buildAuthChain,
		turtleware.PagingMiddleware,
	).Then(tenant.StaticListDataHandler(dataFetcherFunc, errorCapture.Capture))

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.JSONEq(s.loadTestDataString("data/list_empty.json"), s.response.Body.String())
	s.NoError(errorCapture.CapturedError)
	s.True(dataFetcherFuncWasCalled)
}

func (s *MiddlewareDataSuite) Test_ResourceDataHandler_Head() {
	// given
	errorCapture := &ErrorHandlerCapture{}

	s.request.Method = http.MethodHead

	testChain := tenant.ResourceDataHandler[TestDataModel](nil, errorCapture.Capture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Empty(s.response.Body.String())
	s.NoError(errorCapture.CapturedError)
}

func (s *MiddlewareDataSuite) Test_ResourceDataHandler_ErrContextMissingTenantUUID() {
	// given
	errorCapture := &ErrorHandlerCapture{}

	testChain := tenant.ResourceDataHandler[TestDataModel](nil, errorCapture.Capture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Empty(s.response.Body.String())
	s.ErrorIs(errorCapture.CapturedError, tenant.ErrContextMissingTenantUUID)
}

func (s *MiddlewareDataSuite) Test_ResourceDataHandler_ErrContextMissingEntityUUID() {
	// given
	errorCapture := &ErrorHandlerCapture{}

	testChain := alice.New(
		s.buildAuthChain,
	).Then(tenant.ResourceDataHandler[TestDataModel](nil, errorCapture.Capture))

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Empty(s.response.Body.String())
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrContextMissingEntityUUID)
}

func (s *MiddlewareDataSuite) Test_ResourceDataHandler_ErrResourceNotFound() {
	// given
	cases := map[string]error{
		"ErrNoRows":   sql.ErrNoRows,
		"ErrNotExist": os.ErrNotExist,
	}

	for testName, target := range cases {
		s.Run("Via_"+testName, func() {
			// given
			errorCapture := &ErrorHandlerCapture{}

			dataFetcherFunc := func(ctx context.Context, tenantUUID string, entityUUID string) (TestDataModel, error) {
				return TestDataModel{}, target
			}

			testChain := alice.New(
				s.buildAuthChain,
				s.buildEntityUUIDChain,
			).Then(tenant.ResourceDataHandler(dataFetcherFunc, errorCapture.Capture))

			// when
			testChain.ServeHTTP(s.response, s.request)

			// then
			s.Empty(s.response.Body.String())
			s.ErrorIs(errorCapture.CapturedError, turtleware.ErrResourceNotFound)
		})
	}
}

func (s *MiddlewareDataSuite) Test_ResourceDataHandler_ErrReceivingResults() {
	// given
	errorCapture := &ErrorHandlerCapture{}

	targetError := errors.New("some-error")

	dataFetcherFunc := func(ctx context.Context, tenantUUID string, entityUUID string) (TestDataModel, error) {
		return TestDataModel{}, targetError
	}

	testChain := alice.New(
		s.buildAuthChain,
		s.buildEntityUUIDChain,
	).Then(tenant.ResourceDataHandler(dataFetcherFunc, errorCapture.Capture))

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Empty(s.response.Body.String())
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrReceivingResults)
}

func (s *MiddlewareDataSuite) Test_ResourceDataHandler_Success() {
	// given
	errorCapture := &ErrorHandlerCapture{}

	dataFetcherFuncWasCalled := false
	dataFetcherFunc := func(ctx context.Context, tenantUUID string, entityUUID string) (TestDataModel, error) {
		dataFetcherFuncWasCalled = true
		s.Equal(s.tenantUUID, tenantUUID)
		s.Equal(s.entityUUID, entityUUID)

		return TestDataModel{
			SomeString: "test1",
			SomeInt:    42,
		}, nil
	}

	testChain := alice.New(
		s.buildAuthChain,
		s.buildEntityUUIDChain,
	).Then(tenant.ResourceDataHandler(dataFetcherFunc, errorCapture.Capture))

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.JSONEq(s.loadTestDataString("data/entity_success.json"), s.response.Body.String())
	s.NoError(errorCapture.CapturedError)
	s.True(dataFetcherFuncWasCalled)
}

func (s *MiddlewareDataSuite) Test_ResourceDataHandler_Success_Reader() {
	// given
	errorCapture := &ErrorHandlerCapture{}

	testResponse := bytes.NewBufferString("test")

	dataFetcherFuncWasCalled := false
	dataFetcherFunc := func(ctx context.Context, tenantUUID string, entityUUID string) (io.Reader, error) {
		dataFetcherFuncWasCalled = true
		s.Equal(s.tenantUUID, tenantUUID)
		s.Equal(s.entityUUID, entityUUID)

		return testResponse, nil
	}

	testChain := alice.New(
		s.buildAuthChain,
		s.buildEntityUUIDChain,
	).Then(tenant.ResourceDataHandler(dataFetcherFunc, errorCapture.Capture))

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Equal("test", s.response.Body.String())
	s.NoError(errorCapture.CapturedError)
	s.True(dataFetcherFuncWasCalled)
}

func (s *MiddlewareDataSuite) Test_ResourceDataHandler_Success_ReadCloser() {
	// given
	errorCapture := &ErrorHandlerCapture{}

	testResponse := &TestCloser{
		Buffer: bytes.NewBufferString("test"),
	}

	dataFetcherFunc := func(ctx context.Context, tenantUUID string, entityUUID string) (io.ReadCloser, error) {
		return testResponse, nil
	}

	testChain := alice.New(
		s.buildAuthChain,
		s.buildEntityUUIDChain,
	).Then(tenant.ResourceDataHandler(dataFetcherFunc, errorCapture.Capture))

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Equal("test", s.response.Body.String())
	s.NoError(errorCapture.CapturedError)
	s.True(testResponse.wasClosed)
}

func (s *MiddlewareDataSuite) Test_ResourceDataHandler_Success_ReadCloser_CloseError() {
	// given
	errorCapture := &ErrorHandlerCapture{}

	testResponse := &TestCloser{
		Buffer:    bytes.NewBufferString("test"),
		closerErr: errors.New("some error"),
	}

	dataFetcherFunc := func(ctx context.Context, tenantUUID string, entityUUID string) (io.ReadCloser, error) {
		return testResponse, nil
	}

	testChain := alice.New(
		s.buildAuthChain,
		s.buildEntityUUIDChain,
	).Then(tenant.ResourceDataHandler(dataFetcherFunc, errorCapture.Capture))

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Equal("test", s.response.Body.String())
	s.NoError(errorCapture.CapturedError)
	s.True(testResponse.wasClosed)
}
