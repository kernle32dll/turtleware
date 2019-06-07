package turtleware_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/kernle32dll/turtleware"

	"gopkg.in/guregu/null.v3"

	"net/http"
	"time"
)

var _ = Describe("ExtractCacheHeader", func() {
	var (
		request *http.Request

		etag             string
		lastModifiedDate null.Time

		err error
	)

	// Prepare sample request for each test
	BeforeEach(func() {
		request, err = http.NewRequest(http.MethodGet, "http://example.com/foo", nil)
		Expect(err).NotTo(HaveOccurred())
	})

	// Actual method call
	JustBeforeEach(func() {
		etag, lastModifiedDate = turtleware.ExtractCacheHeader(request)
	})

	Context("when both a valid etag and mod date are provided", func() {
		var (
			compDate = time.Date(2017, 6, 14, 12, 5, 3, 0, time.UTC)
		)

		BeforeEach(func() {
			request.Header.Add("If-Modified-Since", compDate.Format(time.RFC1123))
			request.Header.Add("If-None-Match", "123")
		})

		It("should return the etag", func() {
			Expect(etag).To(BeEquivalentTo("123"))
		})

		It("should return the last modification date", func() {
			Expect(lastModifiedDate.Valid).To(BeTrue())
			Expect(lastModifiedDate.Time).To(BeEquivalentTo(compDate))
		})
	})

	Context("when only a valid etag is provided", func() {
		BeforeEach(func() {
			request.Header.Add("If-None-Match", "123")
		})

		It("should return the etag", func() {
			Expect(etag).To(BeEquivalentTo("123"))
		})

		It("should return a null last modification date", func() {
			Expect(lastModifiedDate.Valid).To(BeFalse())
		})
	})

	Context("when a valid etag and an empty last modification date are provided", func() {
		BeforeEach(func() {
			request.Header.Add("If-Modified-Since", "")
			request.Header.Add("If-None-Match", "123")
		})

		It("should return the etag", func() {
			Expect(etag).To(BeEquivalentTo("123"))
		})

		It("should return a null last modification date", func() {
			Expect(lastModifiedDate.Valid).To(BeFalse())
		})
	})

	Context("when a valid etag and an invalid last modification date are provided", func() {
		BeforeEach(func() {
			request.Header.Add("If-Modified-Since", "Käsekuchen")
			request.Header.Add("If-None-Match", "123")
		})

		It("should not return the etag, but an empty string", func() {
			Expect(etag).To(BeEquivalentTo(""))
		})

		It("should return a null last modification date", func() {
			Expect(lastModifiedDate.Valid).To(BeFalse())
		})
	})
})
