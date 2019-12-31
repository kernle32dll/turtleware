package turtleware_test

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/kernle32dll/turtleware"

	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Auth", func() {
	Describe("FromAuthHeader", func() {
		var (
			request *http.Request

			token string
			err   error
		)

		// Prepare sample request for each test
		BeforeEach(func() {
			request, err = http.NewRequest(http.MethodGet, "http://example.com/foo", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		// Actual method call
		JustBeforeEach(func() {
			token, err = turtleware.FromAuthHeader(request)
		})

		Context("when a valid bearer token is provided", func() {
			BeforeEach(func() {
				request.Header.Add("authorization", "Bearer 123")
			})

			It("should return the token", func() {
				Expect(token).To(BeEquivalentTo("123"))
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when no auth header is provided", func() {
			BeforeEach(func() {
				request.Header.Add("authorization", "")
			})

			It("should return an empty string for the token", func() {
				Expect(token).To(BeEquivalentTo(""))
			})

			It("should error with ErrMissingAuthHeader", func() {
				Expect(err).To(BeEquivalentTo(turtleware.ErrMissingAuthHeader))
			})
		})

		Context("when something different than a bearer token is provided", func() {
			BeforeEach(func() {
				request.Header.Add("authorization", "cucumber 123")
			})

			It("should return an empty string for the token", func() {
				Expect(token).To(BeEquivalentTo(""))
			})

			It("should error with ErrMissingAuthHeader", func() {
				Expect(err).To(BeEquivalentTo(turtleware.ErrAuthHeaderWrongFormat))
			})
		})

		Context("when more than two parts for the auth header are provided", func() {
			BeforeEach(func() {
				request.Header.Add("authorization", "bearer 123 456")
			})

			It("should return an empty string for the token", func() {
				Expect(token).To(BeEquivalentTo(""))
			})

			It("should error with ErrMissingAuthHeader", func() {
				Expect(err).To(BeEquivalentTo(turtleware.ErrAuthHeaderWrongFormat))
			})
		})
	})

	Describe("ValidateToken", func() {
		var (
			expectedClaims = jwt.StandardClaims{
				Id: "deadbeef",
			}

			token string
			keys  []interface{}

			claims map[string]interface{}
			err    error
		)

		// Actual method call
		JustBeforeEach(func() {
			claims, err = turtleware.ValidateToken(token, keys)
		})

		Describe("RSA", func() {
			var (
				rsa256Key crypto.PublicKey
				rsa256JWT string

				rsa384Key crypto.PublicKey
				rsa384JWT string

				rsa512Key crypto.PublicKey
				rsa512JWT string
			)

			// Create keys for testing
			BeforeEach(func() {
				rsa256PrivateKey, err := rsa.GenerateKey(rand.Reader, 1024)
				Expect(err).ToNot(HaveOccurred())
				rsa256Key = rsa256PrivateKey.Public()

				rsa256JWT, err = jwt.NewWithClaims(jwt.SigningMethodRS256, expectedClaims).SignedString(rsa256PrivateKey)
				Expect(err).ToNot(HaveOccurred())

				// ------

				rsa384PrivateKey, err := rsa.GenerateKey(rand.Reader, 1024)
				Expect(err).ToNot(HaveOccurred())
				rsa384Key = rsa384PrivateKey.Public()

				rsa384JWT, err = jwt.NewWithClaims(jwt.SigningMethodRS384, expectedClaims).SignedString(rsa384PrivateKey)
				Expect(err).ToNot(HaveOccurred())

				// ------

				rsa512PrivateKey, err := rsa.GenerateKey(rand.Reader, 1024)
				Expect(err).ToNot(HaveOccurred())
				rsa512Key = rsa512PrivateKey.Public()

				rsa512JWT, err = jwt.NewWithClaims(jwt.SigningMethodRS512, expectedClaims).SignedString(rsa512PrivateKey)
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when a valid RSA256 token and key are provided", func() {
				BeforeEach(func() {
					token = rsa256JWT
					keys = []interface{}{rsa256Key}
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims["jti"]).To(BeEquivalentTo(expectedClaims.Id))
				})
			})

			Context("when a valid RSA384 token and key are provided", func() {
				BeforeEach(func() {
					token = rsa384JWT
					keys = []interface{}{rsa384Key}
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims["jti"]).To(BeEquivalentTo(expectedClaims.Id))
				})
			})

			Context("when a valid RSA512 token and key are provided", func() {
				BeforeEach(func() {
					token = rsa512JWT
					keys = []interface{}{rsa512Key}
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims["jti"]).To(BeEquivalentTo(expectedClaims.Id))
				})
			})

			Context("when a valid RSA512 token is mixed with an RSA256 key", func() {
				BeforeEach(func() {
					token = rsa512JWT
					keys = []interface{}{rsa256Key}
				})

				It("should error", func() {
					Expect(err).To(BeEquivalentTo(turtleware.ErrTokenValidationFailed))
				})

				It("should return no claims", func() {
					Expect(claims).To(BeNil())
				})
			})
		})

		Describe("ECDSA", func() {
			var (
				ecdsa256Key crypto.PublicKey
				ecdsa256JWT string

				ecdsa384Key crypto.PublicKey
				ecdsa384JWT string

				ecdsa512Key crypto.PublicKey
				ecdsa512JWT string
			)

			// Create keys for testing
			BeforeEach(func() {
				ecdsa256PrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				Expect(err).ToNot(HaveOccurred())
				ecdsa256Key = ecdsa256PrivateKey.Public()

				ecdsa256JWT, err = jwt.NewWithClaims(jwt.SigningMethodES256, expectedClaims).SignedString(ecdsa256PrivateKey)
				Expect(err).ToNot(HaveOccurred())

				// ------

				ecdsa384PrivateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
				Expect(err).ToNot(HaveOccurred())
				ecdsa384Key = ecdsa384PrivateKey.Public()

				ecdsa384JWT, err = jwt.NewWithClaims(jwt.SigningMethodES384, expectedClaims).SignedString(ecdsa384PrivateKey)
				Expect(err).ToNot(HaveOccurred())

				// ------

				ecdsa512PrivateKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
				Expect(err).ToNot(HaveOccurred())
				ecdsa512Key = ecdsa512PrivateKey.Public()

				ecdsa512JWT, err = jwt.NewWithClaims(jwt.SigningMethodES512, expectedClaims).SignedString(ecdsa512PrivateKey)
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when a valid ES256 token and key are provided", func() {
				BeforeEach(func() {
					token = ecdsa256JWT
					keys = []interface{}{ecdsa256Key}
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims["jti"]).To(BeEquivalentTo(expectedClaims.Id))
				})
			})

			Context("when a valid ES384 token and key are provided", func() {
				BeforeEach(func() {
					token = ecdsa384JWT
					keys = []interface{}{ecdsa384Key}
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims["jti"]).To(BeEquivalentTo(expectedClaims.Id))
				})
			})

			Context("when a valid ES512 token and key are provided", func() {
				BeforeEach(func() {
					token = ecdsa512JWT
					keys = []interface{}{ecdsa512Key}
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims["jti"]).To(BeEquivalentTo(expectedClaims.Id))
				})
			})

			Context("when a valid ES512 token is mixed with an ES256 key", func() {
				BeforeEach(func() {
					token = ecdsa512JWT
					keys = []interface{}{ecdsa256Key}
				})

				It("should error", func() {
					Expect(err).To(BeEquivalentTo(turtleware.ErrTokenValidationFailed))
				})

				It("should return no claims", func() {
					Expect(claims).To(BeNil())
				})
			})
		})

		Describe("HMAC", func() {
			var (
				hmacKey []byte

				hmac256JWT string
				hmac384JWT string
				hmac512JWT string
			)

			// Create keys for testing
			BeforeEach(func() {
				hmacKey = make([]byte, 128)

				_, err = rand.Read(hmacKey)
				Expect(err).ToNot(HaveOccurred())

				hmac256JWT, err = jwt.NewWithClaims(jwt.SigningMethodHS256, expectedClaims).SignedString(hmacKey)
				Expect(err).ToNot(HaveOccurred())
				hmac384JWT, err = jwt.NewWithClaims(jwt.SigningMethodHS384, expectedClaims).SignedString(hmacKey)
				Expect(err).ToNot(HaveOccurred())
				hmac512JWT, err = jwt.NewWithClaims(jwt.SigningMethodHS512, expectedClaims).SignedString(hmacKey)
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when a valid HS256 token and key are provided", func() {
				BeforeEach(func() {
					token = hmac256JWT
					keys = []interface{}{hmacKey}
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims["jti"]).To(BeEquivalentTo(expectedClaims.Id))
				})
			})

			Context("when a valid HS384 token and key are provided", func() {
				BeforeEach(func() {
					token = hmac384JWT
					keys = []interface{}{hmacKey}
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims["jti"]).To(BeEquivalentTo(expectedClaims.Id))
				})
			})

			Context("when a valid HS512 token and key are provided", func() {
				BeforeEach(func() {
					token = hmac512JWT
					keys = []interface{}{hmacKey}
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims["jti"]).To(BeEquivalentTo(expectedClaims.Id))
				})
			})
		})
	})
})
