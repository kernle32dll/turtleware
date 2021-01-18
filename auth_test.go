package turtleware_test

import (
	"github.com/kernle32dll/turtleware"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jws"
	"github.com/lestrrat-go/jwx/jwt"

	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

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

				rsa256JWT = generateToken(jwa.RS256, rsaPrivateKey, expectedClaims, nil)
				rsa384JWT = generateToken(jwa.RS384, rsaPrivateKey, expectedClaims, nil)
				rsa512JWT = generateToken(jwa.RS512, rsaPrivateKey, expectedClaims, nil)
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

				ecdsa256JWT = generateToken(jwa.ES256, ecdsa256PrivateKey, expectedClaims, nil)

				// ------

				ecdsa384PrivateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
				Expect(err).ToNot(HaveOccurred())
				ecdsa384Key = ecdsa384PrivateKey.Public()

				ecdsa384JWT = generateToken(jwa.ES384, ecdsa384PrivateKey, expectedClaims, nil)

				// ------

				ecdsa512PrivateKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
				Expect(err).ToNot(HaveOccurred())
				ecdsa512Key = ecdsa512PrivateKey.Public()

				ecdsa512JWT = generateToken(jwa.ES512, ecdsa512PrivateKey, expectedClaims, nil)
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

				hmac256JWT = generateToken(jwa.HS256, hmacKey, expectedClaims, nil)
				hmac384JWT = generateToken(jwa.HS384, hmacKey, expectedClaims, nil)
				hmac512JWT = generateToken(jwa.HS512, hmacKey, expectedClaims, nil)
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

	Describe("ValidateTokenBySet", func() {
		var (
			expectedClaims = map[string]interface{}{
				jwt.JwtIDKey: "deadbeef",
			}

			token string
			keys  *jwk.Set

			claims map[string]interface{}
			err    error
		)

		// Actual method call
		JustBeforeEach(func() {
			claims, err = turtleware.ValidateTokenBySet(token, keys)
		})

		// Prepare key set
		var (
			rsaPrivateKey     *rsa.PrivateKey
			ecdsaPrivateKey   *ecdsa.PrivateKey
			ed25519PrivateKey ed25519.PrivateKey
			hmacKey           []byte
		)

		BeforeEach(func() {
			// private keys
			var (
				genErr error
			)
			rsaPrivateKey, genErr = rsa.GenerateKey(rand.Reader, 2048)
			if genErr != nil {
				panic(genErr.Error())
			}

			ecdsaPrivateKey, genErr = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
			if genErr != nil {
				panic(genErr.Error())
			}

			_, ed25519PrivateKey, genErr = ed25519.GenerateKey(rand.Reader)
			if genErr != nil {
				panic(genErr.Error())
			}

			hmacKey = []byte("supersecretpassphrase")

			// public keys
			rsaPublicKey, genErr := jwk.New(rsaPrivateKey.Public())
			if genErr != nil {
				panic(genErr.Error())
			}
			if genErr := rsaPublicKey.Set(jwk.KeyIDKey, "rsa-key"); genErr != nil {
				panic(genErr.Error())
			}

			ecdsaPublicKey, genErr := jwk.New(ecdsaPrivateKey.Public())
			if genErr != nil {
				panic(genErr.Error())
			}
			if genErr := ecdsaPublicKey.Set(jwk.KeyIDKey, "ecdsa-key"); genErr != nil {
				panic(genErr.Error())
			}

			ed25519PublicKey, genErr := jwk.New(ed25519PrivateKey.Public())
			if genErr != nil {
				panic(genErr.Error())
			}
			if genErr := ed25519PublicKey.Set(jwk.KeyIDKey, "ed25519-key"); genErr != nil {
				panic(genErr.Error())
			}

			hmacPublicKey, genErr := jwk.New(hmacKey)
			if genErr != nil {
				panic(genErr.Error())
			}
			if genErr := hmacPublicKey.Set(jwk.KeyIDKey, "hmac-key"); genErr != nil {
				panic(genErr.Error())
			}

			keys = &jwk.Set{Keys: []jwk.Key{rsaPublicKey, ecdsaPublicKey, ed25519PublicKey, hmacPublicKey}}
		})

		Describe("RSA", func() {
			Context("when a valid RSA256 token is provided", func() {
				BeforeEach(func() {
					token = generateToken(jwa.RS256, rsaPrivateKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: "rsa-key"})
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims[jwt.JwtIDKey]).To(BeEquivalentTo(expectedClaims[jwt.JwtIDKey]))
				})
			})

			Context("when a valid RSA384 token is provided", func() {
				BeforeEach(func() {
					token = generateToken(jwa.RS384, rsaPrivateKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: "rsa-key"})
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims[jwt.JwtIDKey]).To(BeEquivalentTo(expectedClaims[jwt.JwtIDKey]))
				})
			})

			Context("when a valid RSA512 token is provided", func() {
				BeforeEach(func() {
					token = generateToken(jwa.RS512, rsaPrivateKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: "rsa-key"})
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims[jwt.JwtIDKey]).To(BeEquivalentTo(expectedClaims[jwt.JwtIDKey]))
				})
			})
		})

		Describe("ECDSA", func() {
			Context("when a valid ES256 token is provided", func() {
				BeforeEach(func() {
					token = generateToken(jwa.ES256, ecdsaPrivateKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: "ecdsa-key"})
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims[jwt.JwtIDKey]).To(BeEquivalentTo(expectedClaims[jwt.JwtIDKey]))
				})
			})

			Context("when a valid ES384 token is provided", func() {
				BeforeEach(func() {
					token = generateToken(jwa.ES384, ecdsaPrivateKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: "ecdsa-key"})
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims[jwt.JwtIDKey]).To(BeEquivalentTo(expectedClaims[jwt.JwtIDKey]))
				})
			})

			Context("when a valid ES512 token is provided", func() {
				BeforeEach(func() {
					token = generateToken(jwa.ES512, ecdsaPrivateKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: "ecdsa-key"})
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims[jwt.JwtIDKey]).To(BeEquivalentTo(expectedClaims[jwt.JwtIDKey]))
				})
			})
		})

		Describe("HMAC", func() {
			Context("when a valid HS256 token is provided", func() {
				BeforeEach(func() {
					token = generateToken(jwa.HS256, hmacKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: "hmac-key"})
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims[jwt.JwtIDKey]).To(BeEquivalentTo(expectedClaims[jwt.JwtIDKey]))
				})
			})

			Context("when a valid HS384 token is provided", func() {
				BeforeEach(func() {
					token = generateToken(jwa.HS384, hmacKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: "hmac-key"})
				})

				It("should not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return the expected claims", func() {
					Expect(claims[jwt.JwtIDKey]).To(BeEquivalentTo(expectedClaims[jwt.JwtIDKey]))
				})
			})

			Context("when a valid HS512 token is provided", func() {
				BeforeEach(func() {
					token = generateToken(jwa.HS512, hmacKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: "hmac-key"})
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

	Describe("ReadKeySetFromFolder", func() {
		var (
			keyFolder string

			keySet *jwk.Set
			err    error
		)

		// Actual method call
		JustBeforeEach(func() {
			keySet, err = turtleware.ReadKeySetFromFolder(keyFolder)
		})

		// Create temp folder
		BeforeEach(func() {
			var tErr error
			keyFolder, tErr = createTempFolder()
			if tErr != nil {
				Fail(tErr.Error())
			}
		})

		// Clean up temp folder
		AfterEach(func() {
			if err := os.RemoveAll(keyFolder); err != nil {
				Fail(err.Error())
			}
		})

		// Create keys
		BeforeEach(func() {
			// RSA
			rsaKey, err := rsa.GenerateKey(rand.Reader, 512)
			if err != nil {
				panic(err.Error())
			}

			if err := createValidPublicKey(keyFolder, "rsa-key.puba", rsaKey.Public()); err != nil {
				panic(err.Error())
			}

			// ecdsa
			ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				panic(err.Error())
			}

			if err := createValidPublicKey(keyFolder, "ecdsa-key.pubn", ecdsaKey.Public()); err != nil {
				panic(err.Error())
			}

			// ed25519
			ed25519PubKey, _, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				panic(err.Error())
			}

			if err := createValidPublicKey(keyFolder, "ed25519-key.pubc", ed25519PubKey); err != nil {
				panic(err.Error())
			}

			// garbage
			if err := ioutil.WriteFile(filepath.Join(keyFolder, "garbage.pubd"), []byte("garbage"), 0644); err != nil {
				panic(err.Error())
			}
		})

		It("should return the correct number of keys", func() {
			Expect(len(keySet.Keys)).To(Equal(3))
		})

		It("should contain the RSA key", func() {
			keys := keySet.Keys

			for _, key := range keys {
				kid, exists := key.Get(jwk.KeyIDKey)
				if exists && kid == "rsa-key" {
					return
				}
			}

			Fail("RSA key not loaded")
		})

		It("should contain the ecdsa key", func() {
			keys := keySet.Keys

			for _, key := range keys {
				kid, exists := key.Get(jwk.KeyIDKey)
				if exists && kid == "ecdsa-key" {
					return
				}
			}

			Fail("ecdsa key not loaded")
		})

		It("should contain the ed25519 key", func() {
			keys := keySet.Keys

			for _, key := range keys {
				kid, exists := key.Get(jwk.KeyIDKey)
				if exists && kid == "ed25519-key" {
					return
				}
			}

			Fail("ed25519 key not loaded")
		})

		It("should contain the ed25519 key", func() {
			keys := keySet.Keys

			for _, key := range keys {
				kid, exists := key.Get(jwk.KeyIDKey)
				if exists && kid == "ed25519-key" {
					return
				}
			}

			Fail("ed25519 key not loaded")
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

func generateToken(algo jwa.SignatureAlgorithm, key interface{}, claims map[string]interface{}, headers map[string]interface{}) string {
	t := jwt.New()

	for k, v := range claims {
		if err := t.Set(k, v); err != nil {
			Fail(err.Error())
		}
	}

	hdr := jws.NewHeaders()
	for k, v := range headers {
		if err := hdr.Set(k, v); err != nil {
			Fail(err.Error())
		}
	}

	signedT, err := jwt.Sign(t, algo, key, jwt.WithHeaders(hdr))
	if err != nil {
		Fail(err.Error())
	}

	return string(signedT)
}

func createTempFolder() (string, error) {
	keyFolder, err := ioutil.TempDir("", "")
	if err != nil {
		return "", err
	}

	return keyFolder, nil
}

func createValidPublicKey(keyFolder string, filename string, key crypto.PublicKey) error {
	bytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return err
	}

	pemBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: bytes,
	}

	return ioutil.WriteFile(filepath.Join(keyFolder, filename), pem.EncodeToMemory(pemBlock), 0644)
}
