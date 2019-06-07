package turtleware_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/kernle32dll/turtleware"

	"net/http"
	"net/http/httptest"
)

var _ = Describe("Paging", func() {
	Describe("ParsePagingFromRequest", func() {
		var (
			r *http.Request

			paging turtleware.Paging
			err    error
		)

		// Actual method call
		JustBeforeEach(func() {
			paging, err = turtleware.ParsePagingFromRequest(r)
		})

		Context("when only a limit is provided", func() {
			BeforeEach(func() {
				r = httptest.NewRequest(http.MethodGet, "http://example.com/foo?limit=10", nil)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the correct limit", func() {
				Expect(paging.Limit).To(BeEquivalentTo(10))
			})

			It("should return zero for the offset", func() {
				Expect(paging.Offset).To(BeEquivalentTo(0))
			})
		})

		Context("when only a offset is provided", func() {
			BeforeEach(func() {
				r = httptest.NewRequest(http.MethodGet, "http://example.com/foo?offset=30", nil)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return 100 for the limit", func() {
				Expect(paging.Limit).To(BeEquivalentTo(100))
			})

			It("should return the correct offset", func() {
				Expect(paging.Offset).To(BeEquivalentTo(30))
			})
		})

		Context("when both a limit and an offset are provided", func() {
			BeforeEach(func() {
				r = httptest.NewRequest(http.MethodGet, "http://example.com/foo?limit=10&offset=30", nil)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the correct limit", func() {
				Expect(paging.Limit).To(BeEquivalentTo(10))
			})

			It("should return the correct offset", func() {
				Expect(paging.Offset).To(BeEquivalentTo(30))
			})
		})

		Context("when a limit which surpasses the maximum is is provided", func() {
			BeforeEach(func() {
				r = httptest.NewRequest(http.MethodGet, "http://example.com/foo?limit=1000", nil)
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return 500 for the limit", func() {
				Expect(paging.Limit).To(BeEquivalentTo(500))
			})

			It("should return zero for the offset", func() {
				Expect(paging.Offset).To(BeEquivalentTo(0))
			})
		})

		Context("when an invalid limit is provided", func() {
			BeforeEach(func() {
				r = httptest.NewRequest(http.MethodGet, "http://example.com/foo?limit=foo", nil)
			})

			It("should error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should return an empty paging object", func() {
				Expect(paging).To(BeEquivalentTo(turtleware.Paging{}))
			})
		})

		Context("when an invalid offset is provided", func() {
			BeforeEach(func() {
				r = httptest.NewRequest(http.MethodGet, "http://example.com/foo?offset=bar", nil)
			})

			It("should error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should return an empty paging object", func() {
				Expect(paging).To(BeEquivalentTo(turtleware.Paging{}))
			})
		})

		Context("when a negative limit is provided", func() {
			BeforeEach(func() {
				r = httptest.NewRequest(http.MethodGet, "http://example.com/foo?limit=-10", nil)
			})

			It("should error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should return an empty paging object", func() {
				Expect(paging).To(BeEquivalentTo(turtleware.Paging{}))
			})
		})

		Context("when a negative offset is provided", func() {
			BeforeEach(func() {
				r = httptest.NewRequest(http.MethodGet, "http://example.com/foo?offset=-10", nil)
			})

			It("should error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should return an empty paging object", func() {
				Expect(paging).To(BeEquivalentTo(turtleware.Paging{}))
			})
		})
	})

	Describe("String", func() {
		var (
			paging turtleware.Paging
			result string
		)

		// Actual method call
		JustBeforeEach(func() {
			result = paging.String()
		})

		Context("when only a limit is provided", func() {
			BeforeEach(func() {
				paging = turtleware.Paging{Offset: 0, Limit: 10}
			})

			It("should return a string with the limit", func() {
				Expect(result).To(BeEquivalentTo("limit=10"))
			})
		})

		Context("when both a limit and an offset are provided", func() {
			BeforeEach(func() {
				paging = turtleware.Paging{Offset: 20, Limit: 10}
			})

			It("should return a string with both limit and offset", func() {
				Expect(result).To(BeEquivalentTo("offset=20&limit=10"))
			})
		})
	})
})
