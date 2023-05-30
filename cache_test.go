package turtleware_test

import (
	"github.com/kernle32dll/turtleware"
	"github.com/stretchr/testify/suite"
	"net/http/httptest"

	"net/http"
	"testing"
	"time"
)

type CacheSuite struct {
	CommonSuite

	request *http.Request
}

func TestCacheSuite(t *testing.T) {
	suite.Run(t, &CacheSuite{})
}

func (s *CacheSuite) SetupTest() {
	s.request = httptest.NewRequest(http.MethodGet, "https://example.com/foo", http.NoBody)
}

func (s *CacheSuite) Test_ETag_And_Valid_ModDate() {
	// given
	compDate := time.Date(2017, 6, 14, 12, 5, 3, 0, time.UTC)
	s.request.Header.Add("If-Modified-Since", compDate.Format(time.RFC1123))
	s.request.Header.Add("If-None-Match", "123")

	// when
	etag, lastModifiedDate := turtleware.ExtractCacheHeader(s.request)

	// then
	s.Equal("123", etag)
	s.Equal(compDate, lastModifiedDate)
}

func (s *CacheSuite) Test_Only_ETag() {
	// given
	s.request.Header.Add("If-None-Match", "123")

	// when
	etag, lastModifiedDate := turtleware.ExtractCacheHeader(s.request)

	// then
	s.Equal("123", etag)
	s.True(lastModifiedDate.IsZero())
}

func (s *CacheSuite) Test_ETag_And_Empty_ModDate() {
	// given
	s.request.Header.Add("If-Modified-Since", "")
	s.request.Header.Add("If-None-Match", "123")

	// when
	etag, lastModifiedDate := turtleware.ExtractCacheHeader(s.request)

	// then
	s.Equal("123", etag)
	s.True(lastModifiedDate.IsZero())
}

func (s *CacheSuite) Test_ETag_And_Invalid_ModDate() {
	// given
	s.request.Header.Add("If-Modified-Since", "Käsekuchen")
	s.request.Header.Add("If-None-Match", "123")

	// when
	etag, lastModifiedDate := turtleware.ExtractCacheHeader(s.request)

	// then
	s.Empty(etag)
	s.True(lastModifiedDate.IsZero())
}
