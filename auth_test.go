package turtleware_test

import (
	"github.com/kernle32dll/turtleware"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwt"

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
			expectedClaims = map[string]interface{}{
				jwt.JwtIDKey: "deadbeef",
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
				rsaPublicKey crypto.PublicKey
				rsa256JWT    string
				rsa384JWT    string
				rsa512JWT    string
			)

			// Create keys for testing
			BeforeEach(func() {
				rsaPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
				Expect(err).ToNot(HaveOccurred())
				rsaPublicKey = rsaPrivateKey.Public()

				rsa256JWT = generateToken(jwa.RS256, rsaPrivateKey, expectedClaims)
				rsa384JWT = generateToken(jwa.RS384, rsaPrivateKey, expectedClaims)
				rsa512JWT = generateToken(jwa.RS512, rsaPrivateKey, expectedClaims)
			})

			Context("when a valid RSA256 token and key are provided", func() {
				BeforeEach(func() {
					token = rsa256JWT
					keys = []interface{}{rsaPublicKey}
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims[jwt.JwtIDKey]).To(BeEquivalentTo(expectedClaims[jwt.JwtIDKey]))
				})
			})

			Context("when a valid RSA384 token and key are provided", func() {
				BeforeEach(func() {
					token = rsa384JWT
					keys = []interface{}{rsaPublicKey}
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims[jwt.JwtIDKey]).To(BeEquivalentTo(expectedClaims[jwt.JwtIDKey]))
				})
			})

			Context("when a valid RSA512 token and key are provided", func() {
				BeforeEach(func() {
					token = rsa512JWT
					keys = []interface{}{rsaPublicKey}
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims[jwt.JwtIDKey]).To(BeEquivalentTo(expectedClaims[jwt.JwtIDKey]))
				})
			})

			Context("when a valid RSA512 token is mixed with a HMAC key", func() {
				BeforeEach(func() {
					token = rsa512JWT
					keys = []interface{}{[]byte("supersecretpassphrase")}
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
				ecdsa256PrivateKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
				Expect(err).ToNot(HaveOccurred())
				ecdsa256Key = ecdsa256PrivateKey.Public()

				ecdsa256JWT = generateToken(jwa.ES256, ecdsa256PrivateKey, expectedClaims)

				// ------

				ecdsa384PrivateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
				Expect(err).ToNot(HaveOccurred())
				ecdsa384Key = ecdsa384PrivateKey.Public()

				ecdsa384JWT = generateToken(jwa.ES384, ecdsa384PrivateKey, expectedClaims)

				// ------

				ecdsa512PrivateKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
				Expect(err).ToNot(HaveOccurred())
				ecdsa512Key = ecdsa512PrivateKey.Public()

				ecdsa512JWT = generateToken(jwa.ES512, ecdsa512PrivateKey, expectedClaims)
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
					Expect(claims[jwt.JwtIDKey]).To(BeEquivalentTo(expectedClaims[jwt.JwtIDKey]))
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
					Expect(claims[jwt.JwtIDKey]).To(BeEquivalentTo(expectedClaims[jwt.JwtIDKey]))
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
					Expect(claims[jwt.JwtIDKey]).To(BeEquivalentTo(expectedClaims[jwt.JwtIDKey]))
				})
			})

			Context("when a valid ES512 token is mixed with a HMAC key", func() {
				BeforeEach(func() {
					token = ecdsa512JWT
					keys = []interface{}{[]byte("supersecretpassphrase")}
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

				hmac256JWT = generateToken(jwa.HS256, hmacKey, expectedClaims)
				hmac384JWT = generateToken(jwa.HS384, hmacKey, expectedClaims)
				hmac512JWT = generateToken(jwa.HS512, hmacKey, expectedClaims)
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
					Expect(claims[jwt.JwtIDKey]).To(BeEquivalentTo(expectedClaims[jwt.JwtIDKey]))
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
					Expect(claims[jwt.JwtIDKey]).To(BeEquivalentTo(expectedClaims[jwt.JwtIDKey]))
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
					Expect(claims[jwt.JwtIDKey]).To(BeEquivalentTo(expectedClaims[jwt.JwtIDKey]))
				})
			})
		})
	})
})

func generateToken(algo jwa.SignatureAlgorithm, key interface{}, expectedClaims map[string]interface{}) string {
	t := jwt.New()

	for k, v := range expectedClaims {
		if err := t.Set(k, v); err != nil {
			Fail(err.Error())
		}
	}

	signedT, err := jwt.Sign(t, algo, key)
	if err != nil {
		Fail(err.Error())
	}

	return string(signedT)
}
