package turtleware_test

import (
	"github.com/justinas/alice"
	"github.com/kernle32dll/turtleware"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"

	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	staticUserUUID   = "123"
	staticEntityUUID = "456"
)

var _ = Describe("Multipart Middleware", func() {
	var (
		preChain  alice.Chain
		jwtString string
	)

	// Testing middlewares requires a sophisticated setup, so
	// the request context is correctly setup.
	BeforeSuite(func() {
		ecdsaPrivateKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
		Expect(err).ToNot(HaveOccurred())

		jwtString = generateToken(jwa.ES512, ecdsaPrivateKey, map[string]interface{}{
			"uuid": staticUserUUID,
		}, map[string]interface{}{
			jwk.KeyIDKey:     "some-kid",
			jwk.AlgorithmKey: jwa.ES512,
		})

		ecdsaPublicKey, genErr := jwk.New(ecdsaPrivateKey.Public())
		if genErr != nil {
			panic(genErr.Error())
		}
		if genErr := ecdsaPublicKey.Set(jwk.KeyIDKey, "some-kid"); genErr != nil {
			panic(genErr.Error())
		}
		if genErr := ecdsaPublicKey.Set(jwk.AlgorithmKey, jwa.ES512); genErr != nil {
			panic(genErr.Error())
		}

		authHeaderMiddleware := turtleware.AuthBearerHeaderMiddleware

		keySet := jwk.NewSet()
		keySet.Add(ecdsaPublicKey)

		authMiddleware := turtleware.AuthClaimsMiddleware(keySet)
		tenantUUIDMiddleware := turtleware.EntityUUIDMiddleware(func(r *http.Request) (string, error) {
			return staticEntityUUID, nil
		})

		preChain = alice.New(
			authHeaderMiddleware,
			authMiddleware,
			tenantUUIDMiddleware,
		)
	})

	var (
		// input
		header http.Header
		body   io.ReadWriter

		// output
		nextCalled    bool
		responseBytes []byte
		fileNames     []string
		fileBytes     [][]byte
	)

	BeforeEach(func() {
		header = http.Header{}
		body = &bytes.Buffer{}

		nextCalled = false
		responseBytes = nil
		fileBytes = nil
	})

	JustBeforeEach(func() {
		request, err := http.NewRequest(http.MethodGet, "http://example.com/foo", body)
		Expect(err).NotTo(HaveOccurred())

		request.Header = header
		request.Header.Set("authorization", "Bearer "+jwtString)

		// ----------

		middleware := turtleware.FileUploadMiddleware(func(ctx context.Context, entityUUID, userUUID string, fileName string, file multipart.File) error {
			if entityUUID != staticEntityUUID {
				panic("wrong entity UUID")
			}

			if userUUID != staticUserUUID {
				panic("wrong entity UUID")
			}

			content, err := ioutil.ReadAll(file)
			if err != nil {
				return err
			}

			fileNames = append(fileNames, fileName)
			fileBytes = append(fileBytes, content)

			return nil
		}, turtleware.DefaultFileUploadErrorHandler)

		recorder := httptest.NewRecorder()

		preChain.Then(
			middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				nextCalled = true
			})),
		).ServeHTTP(recorder, request)

		responseBytes = recorder.Body.Bytes()
	})

	Describe("WithBody", func() {
		var part, contentType = CreateMultipart()

		Context("when body and content-type are set", func() {
			BeforeEach(func() {
				if _, err := body.Write(part); err != nil {
					panic(err)
				}

				header.Set("Content-Type", contentType)
			})

			It("should have called the next handler", func() {
				Expect(nextCalled).To(BeTrue())
			})

			It("should call the file handler 1 time with expected filename and bytes", func() {
				Expect(len(fileBytes)).To(BeEquivalentTo(1))
				Expect(fileNames[0]).To(BeEquivalentTo("test.txt"))
				Expect(fileBytes[0]).To(BeEquivalentTo([]byte("works")))
			})

			It("should write nothing to the output stream", func() {
				Expect(string(responseBytes)).To(BeEquivalentTo(""))
			})
		})

		Context("when the body is set, but the content-type is missing", func() {
			BeforeEach(func() {
				if _, err := body.Write(part); err != nil {
					panic(err)
				}
			})

			It("should not have called the next handler", func() {
				Expect(nextCalled).To(BeFalse())
			})

			It("should call the file handler 0 times", func() {
				Expect(len(fileBytes)).To(BeEquivalentTo(0))
			})

			It("should write an error to the output stream", func() {
				Expect(string(responseBytes)).To(BeEquivalentTo(string(ExpectedError(http.StatusBadRequest, http.ErrNotMultipart))))
			})
		})
	})
})

func CreateMultipart() ([]byte, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	formFile, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		panic(err)
	}

	_, err = formFile.Write([]byte("works"))
	if err != nil {
		panic(err)
	}

	if err := writer.Close(); err != nil {
		panic(err)
	}

	return body.Bytes(), writer.FormDataContentType()
}
