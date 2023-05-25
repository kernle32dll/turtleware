package turtleware_test

import (
	"github.com/justinas/alice"
	"github.com/kernle32dll/turtleware"
	"github.com/stretchr/testify/suite"

	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

type MiddlewareCoreSuite struct {
	CommonSuite

	response *httptest.ResponseRecorder
	request  *http.Request
}

func TestMiddlewareCoreSuite(t *testing.T) {
	suite.Run(t, &MiddlewareCoreSuite{})
}

func (s *MiddlewareCoreSuite) SetupTest() {
	s.CommonSuite.SetupTest()

	s.response = httptest.NewRecorder()
	s.request = httptest.NewRequest(http.MethodGet, "https://example.com/foo", http.NoBody)
}

func (s *MiddlewareCoreSuite) SetupSubTest() {
	s.SetupTest()
}

var validErrors = map[string]error{
	"sql.ErrNoRows":  sql.ErrNoRows,
	"os.ErrNotExist": os.ErrNotExist,
}

func (s *MiddlewareCoreSuite) Test_CountHeaderMiddleware_Success() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	countFetcher := func(
		ctx context.Context,
	) (uint, error) {
		return uint(1337), nil
	}

	testChain := alice.New(
		turtleware.CountHeaderMiddleware(countFetcher, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Equal("1337", s.response.Header().Get("X-Total-Count"))
	s.True(nextCapture.Called)
	s.NoError(errorCapture.CapturedError)
}

func (s *MiddlewareCoreSuite) Test_CountHeaderMiddleware_Error() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	countFetcher := func(
		ctx context.Context,
	) (uint, error) {
		return 0, errors.New("some-error")
	}

	testChain := alice.New(
		turtleware.CountHeaderMiddleware(countFetcher, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Empty(s.response.Header().Get("X-Total-Count"))
	s.False(nextCapture.Called)
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrReceivingMeta)
}

func (s *MiddlewareCoreSuite) Test_CountHeaderMiddleware_Valid_Errors() {
	for testName, targetErr := range validErrors {
		s.Run(testName, func() {
			// given
			nextCapture := &MiddlewareCapture{}
			errorCapture := &ErrorHandlerCapture{}

			countFetcher := func(
				ctx context.Context,
			) (uint, error) {
				return 0, targetErr
			}

			testChain := alice.New(
				turtleware.CountHeaderMiddleware(countFetcher, errorCapture.Capture),
			).Then(nextCapture)

			// when
			testChain.ServeHTTP(s.response, s.request)

			// then
			s.Equal("0", s.response.Header().Get("X-Total-Count"))
			s.True(nextCapture.Called)
			s.NoError(errorCapture.CapturedError)
		})
	}
}

func (s *MiddlewareCoreSuite) Test_ListCacheMiddleware_Success_CacheMiss() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	hashFetcher := func(
		ctx context.Context,
		paging turtleware.Paging,
	) (string, error) {
		return "some-hash", nil
	}

	testChain := alice.New(
		turtleware.PagingMiddleware,
		turtleware.ListCacheMiddleware(hashFetcher, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Contains(
		s.response.Header().Values("Cache-Control"),
		"must-revalidate",
	)
	s.Contains(
		s.response.Header().Values("Cache-Control"),
		"max-age=0",
	)
	s.Equal(
		"some-hash",
		s.response.Header().Get("Etag"),
	)
	s.Equal(http.StatusOK, s.response.Code)
	s.True(nextCapture.Called)
	s.NoError(errorCapture.CapturedError)
}

func (s *MiddlewareCoreSuite) Test_ListCacheMiddleware_Success_CacheHit() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	s.request.Header.Set("If-None-Match", "some-hash")

	hashFetcher := func(
		ctx context.Context,
		paging turtleware.Paging,
	) (string, error) {
		return "some-hash", nil
	}

	testChain := alice.New(
		turtleware.PagingMiddleware,
		turtleware.ListCacheMiddleware(hashFetcher, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Contains(
		s.response.Header().Values("Cache-Control"),
		"must-revalidate",
	)
	s.Contains(
		s.response.Header().Values("Cache-Control"),
		"max-age=0",
	)
	s.Equal(
		"some-hash",
		s.response.Header().Get("Etag"),
	)
	s.Equal(http.StatusNotModified, s.response.Code)
	s.False(nextCapture.Called)
	s.NoError(errorCapture.CapturedError)
}

func (s *MiddlewareCoreSuite) Test_ListCacheMiddleware_Error() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	hashFetcher := func(
		ctx context.Context,
		paging turtleware.Paging,
	) (string, error) {
		return "", errors.New("some-error")
	}

	testChain := alice.New(
		turtleware.PagingMiddleware,
		turtleware.ListCacheMiddleware(hashFetcher, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Contains(
		s.response.Header().Values("Cache-Control"),
		"must-revalidate",
	)
	s.Contains(
		s.response.Header().Values("Cache-Control"),
		"max-age=0",
	)
	s.Equal(
		"",
		s.response.Header().Get("Etag"),
	)
	s.False(nextCapture.Called)
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrReceivingMeta)
}

func (s *MiddlewareCoreSuite) Test_ListCacheMiddleware_ErrContextMissingPaging() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	testChain := alice.New(
		turtleware.ListCacheMiddleware(nil, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Contains(
		s.response.Header().Values("Cache-Control"),
		"must-revalidate",
	)
	s.Contains(
		s.response.Header().Values("Cache-Control"),
		"max-age=0",
	)
	s.Equal(
		"",
		s.response.Header().Get("Etag"),
	)
	s.False(nextCapture.Called)
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrContextMissingPaging)
}

func (s *MiddlewareCoreSuite) Test_ListCacheMiddleware_Valid_Errors() {
	for testName, targetErr := range validErrors {
		s.Run(testName, func() {
			// given
			nextCapture := &MiddlewareCapture{}
			errorCapture := &ErrorHandlerCapture{}

			hashFetcher := func(
				ctx context.Context,
				paging turtleware.Paging,
			) (string, error) {
				return "", targetErr
			}

			testChain := alice.New(
				turtleware.PagingMiddleware,
				turtleware.ListCacheMiddleware(hashFetcher, errorCapture.Capture),
			).Then(nextCapture)

			// when
			testChain.ServeHTTP(s.response, s.request)

			// then
			s.Contains(
				s.response.Header().Values("Cache-Control"),
				"must-revalidate",
			)
			s.Contains(
				s.response.Header().Values("Cache-Control"),
				"max-age=0",
			)
			s.Equal(
				"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				s.response.Header().Get("Etag"),
			)
			s.True(nextCapture.Called)
			s.NoError(errorCapture.CapturedError)
		})
	}
}

func (s *MiddlewareCoreSuite) Test_ResourceCacheMiddleware_Success_CacheMiss() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	lastModTime := time.Date(1991, 5, 23, 1, 2, 3, 4, time.UTC)

	lastModFetcher := func(
		ctx context.Context,
		entityUUID string,
	) (time.Time, error) {
		s.Equal(s.entityUUID, entityUUID)
		return lastModTime, nil
	}

	testChain := alice.New(
		s.buildEntityUUIDChain,
		turtleware.ResourceCacheMiddleware(lastModFetcher, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Equal("Thu, 23 May 1991 01:02:03 UTC", s.response.Header().Get("Last-Modified"))
	s.Equal(http.StatusOK, s.response.Code)
	s.True(nextCapture.Called)
	s.NoError(errorCapture.CapturedError)
}

func (s *MiddlewareCoreSuite) Test_ResourceCacheMiddleware_Success_CacheHit() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	lastModTime := time.Date(1991, 5, 23, 1, 2, 3, 4, time.UTC)
	s.request.Header.Set("If-Modified-Since", lastModTime.Format(time.RFC1123))

	lastModFetcher := func(
		ctx context.Context,
		entityUUID string,
	) (time.Time, error) {
		s.Equal(s.entityUUID, entityUUID)
		return lastModTime, nil
	}

	testChain := alice.New(
		s.buildEntityUUIDChain,
		turtleware.ResourceCacheMiddleware(lastModFetcher, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Equal("Thu, 23 May 1991 01:02:03 UTC", s.response.Header().Get("Last-Modified"))
	s.Equal(http.StatusNotModified, s.response.Code)
	s.False(nextCapture.Called)
	s.NoError(errorCapture.CapturedError)
}

func (s *MiddlewareCoreSuite) Test_ResourceCacheMiddleware_Error() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	lastModFetcher := func(
		ctx context.Context,
		entityUUID string,
	) (time.Time, error) {
		return time.Time{}, errors.New("some-error")
	}

	testChain := alice.New(
		s.buildEntityUUIDChain,
		turtleware.ResourceCacheMiddleware(lastModFetcher, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Empty(s.response.Header().Get("Last-Modified"))
	s.False(nextCapture.Called)
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrReceivingMeta)
}

func (s *MiddlewareCoreSuite) Test_ResourceCacheMiddleware_ErrContextMissingEntityUUID() {
	// given
	nextCapture := &MiddlewareCapture{}
	errorCapture := &ErrorHandlerCapture{}

	testChain := alice.New(
		turtleware.ResourceCacheMiddleware(nil, errorCapture.Capture),
	).Then(nextCapture)

	// when
	testChain.ServeHTTP(s.response, s.request)

	// then
	s.Empty(s.response.Header().Get("Last-Modified"))
	s.False(nextCapture.Called)
	s.ErrorIs(errorCapture.CapturedError, turtleware.ErrContextMissingEntityUUID)
}

func (s *MiddlewareCoreSuite) Test_ResourceCacheMiddleware_Valid_Errors() {
	for testName, targetErr := range validErrors {
		s.Run(testName, func() {
			// given
			nextCapture := &MiddlewareCapture{}
			errorCapture := &ErrorHandlerCapture{}

			lastModFetcher := func(
				ctx context.Context,
				entityUUID string,
			) (time.Time, error) {
				return time.Time{}, targetErr
			}

			testChain := alice.New(
				s.buildEntityUUIDChain,
				turtleware.ResourceCacheMiddleware(lastModFetcher, errorCapture.Capture),
			).Then(nextCapture)

			// when
			testChain.ServeHTTP(s.response, s.request)

			// then
			s.Empty(s.response.Header().Get("Last-Modified"))
			s.True(nextCapture.Called)
			s.NoError(errorCapture.CapturedError)
		})
	}
}
