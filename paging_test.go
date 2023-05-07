package turtleware_test

import (
	"github.com/kernle32dll/turtleware"
	"github.com/stretchr/testify/suite"

	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type PagingSuite struct {
	CommonSuite
}

func TestPagingSuite(t *testing.T) {
	suite.Run(t, &PagingSuite{})
}

func buildTestRequest(values map[string]string) *http.Request {
	urlValues := make(url.Values, len(values))
	for k, v := range values {
		urlValues[k] = []string{v}
	}

	testURL := "https://example.com"

	params := urlValues.Encode()
	if params != "" {
		testURL = testURL + "?" + params
	}

	return httptest.NewRequest(http.MethodGet, testURL, http.NoBody)
}

func (s *PagingSuite) Test_ParsePagingFromRequest() {
	s.Run("Limit_And_Offset", func() {
		// given
		r := buildTestRequest(map[string]string{
			"offset": "30",
			"limit":  "10",
		})

		// when
		paging, err := turtleware.ParsePagingFromRequest(r)

		// then
		s.NoError(err)
		s.Equal(turtleware.Paging{
			Offset: 30,
			Limit:  10,
		}, paging)
	})

	s.Run("Only_Limit", func() {
		s.Run("Success", func() {
			// given
			r := buildTestRequest(map[string]string{
				"limit": "10",
			})

			// when
			paging, err := turtleware.ParsePagingFromRequest(r)

			// then
			s.NoError(err)
			s.Equal(turtleware.Paging{
				Offset: 0,
				Limit:  10,
			}, paging)
		})

		s.Run("TooBig", func() {
			// given
			r := buildTestRequest(map[string]string{
				"limit": "9001",
			})

			// when
			paging, err := turtleware.ParsePagingFromRequest(r)

			// then
			s.NoError(err)
			s.Equal(turtleware.Paging{
				Offset: 0,
				Limit:  500,
			}, paging)
		})

		s.Run("Negative", func() {
			// given
			r := buildTestRequest(map[string]string{
				"limit": "-30",
			})

			// when
			paging, err := turtleware.ParsePagingFromRequest(r)

			// then
			s.Error(err)
			s.ErrorIs(err, turtleware.ErrInvalidLimit)
			s.Equal(turtleware.Paging{}, paging)
		})

		s.Run("Trash", func() {
			// given
			r := buildTestRequest(map[string]string{
				"limit": "schnitzel",
			})

			// when
			paging, err := turtleware.ParsePagingFromRequest(r)

			// then
			s.Error(err)
			s.ErrorIs(err, turtleware.ErrInvalidLimit)
			s.Equal(turtleware.Paging{}, paging)
		})
	})

	s.Run("Only_Offset", func() {
		s.Run("Success", func() {
			// given
			r := buildTestRequest(map[string]string{
				"offset": "30",
			})

			// when
			paging, err := turtleware.ParsePagingFromRequest(r)

			// then
			s.NoError(err)
			s.Equal(turtleware.Paging{
				Offset: 30,
				Limit:  100,
			}, paging)
		})

		s.Run("Negative", func() {
			// given
			r := buildTestRequest(map[string]string{
				"offset": "-30",
			})

			// when
			paging, err := turtleware.ParsePagingFromRequest(r)

			// then
			s.Error(err)
			s.ErrorIs(err, turtleware.ErrInvalidOffset)
			s.Equal(turtleware.Paging{}, paging)
		})

		s.Run("Trash", func() {
			// given
			r := buildTestRequest(map[string]string{
				"offset": "schnitzel",
			})

			// when
			paging, err := turtleware.ParsePagingFromRequest(r)

			// then
			s.Error(err)
			s.ErrorIs(err, turtleware.ErrInvalidOffset)
			s.Equal(turtleware.Paging{}, paging)
		})
	})

	s.Run("No_Parameters", func() {
		// given
		r := buildTestRequest(nil)

		// when
		paging, err := turtleware.ParsePagingFromRequest(r)

		// then
		s.NoError(err)
		s.Equal(turtleware.Paging{
			Offset: 0,
			Limit:  100,
		}, paging)
	})
}

func (s *PagingSuite) Test_ParsePagingFromRequest_String() {
	s.Run("Only_Limit", func() {
		// given
		paging := turtleware.Paging{Offset: 0, Limit: 10}

		// when
		stringVal := paging.String()

		// then
		s.Equal("limit=10", stringVal)
	})

	s.Run("Only_Offset", func() {
		// given
		paging := turtleware.Paging{Offset: 30, Limit: 0}

		// when
		stringVal := paging.String()

		// then
		s.Equal("offset=30&limit=0", stringVal)
	})

	s.Run("Limit_And_Offset", func() {
		// given
		paging := turtleware.Paging{Offset: 30, Limit: 10}

		// when
		stringVal := paging.String()

		// then
		s.Equal("offset=30&limit=10", stringVal)

	})

	s.Run("EmptyPaging", func() {
		// given
		paging := turtleware.Paging{}

		// when
		stringVal := paging.String()

		// then
		s.Equal("limit=0", stringVal)
	})
}
