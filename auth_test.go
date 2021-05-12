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
	"errors"
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
			request, err = http.NewRequest(http.MethodGet, "https://example.com/foo", nil)
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

	Describe("ValidateTokenBySet", func() {
		var (
			expectedClaims = map[string]interface{}{
				jwt.JwtIDKey: "deadbeef",
			}

			token string
			keys  jwk.Set

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
			rsaPublicKey, genErr := turtleware.JWKFromPublicKey(rsaPrivateKey.Public(), "rsa-key")
			if genErr != nil {
				panic(genErr.Error())
			}

			ecdsaPublicKey, genErr := turtleware.JWKFromPublicKey(ecdsaPrivateKey.Public(), "ecdsa-key")
			if genErr != nil {
				panic(genErr.Error())
			}

			ed25519PublicKey, genErr := turtleware.JWKFromPublicKey(ed25519PrivateKey.Public(), "ed25519-key")
			if genErr != nil {
				panic(genErr.Error())
			}

			hmacPublicKey, genErr := turtleware.JWKFromPublicKey(hmacKey, "hmac-key")
			if genErr != nil {
				panic(genErr.Error())
			}

			keys = jwk.NewSet()
			keys.Add(rsaPublicKey)
			keys.Add(ecdsaPublicKey)
			keys.Add(ed25519PublicKey)
			keys.Add(hmacPublicKey)
		})

		Describe("RSA", func() {
			Context("when a valid RSA256 token is provided", func() {
				BeforeEach(func() {
					key, _ := keys.LookupKeyID("rsa-key")
					if genErr := key.Set(jwk.AlgorithmKey, jwa.RS256); genErr != nil {
						panic(genErr.Error())
					}

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
					key, _ := keys.LookupKeyID("rsa-key")
					if genErr := key.Set(jwk.AlgorithmKey, jwa.RS384); genErr != nil {
						panic(genErr.Error())
					}

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
					key, _ := keys.LookupKeyID("ecdsa-key")
					if genErr := key.Set(jwk.AlgorithmKey, jwa.ES256); genErr != nil {
						panic(genErr.Error())
					}

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
					key, _ := keys.LookupKeyID("ecdsa-key")
					if genErr := key.Set(jwk.AlgorithmKey, jwa.ES384); genErr != nil {
						panic(genErr.Error())
					}

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

		Describe("ed25519", func() {
			Context("when a valid EdDSA token is provided", func() {
				BeforeEach(func() {
					token = generateToken(jwa.EdDSA, ed25519PrivateKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: "ed25519-key"})
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
					key, _ := keys.LookupKeyID("hmac-key")
					if genErr := key.Set(jwk.AlgorithmKey, jwa.HS256); genErr != nil {
						panic(genErr.Error())
					}

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
					key, _ := keys.LookupKeyID("hmac-key")
					if genErr := key.Set(jwk.AlgorithmKey, jwa.HS384); genErr != nil {
						panic(genErr.Error())
					}

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

	Describe("JWKFromPrivateKey", func() {
		var (
			privateKey crypto.PrivateKey
			kid        string

			key jwk.Key
			err error
		)

		BeforeEach(func() {
			kid = "some-key-id"
		})

		// Actual method call
		JustBeforeEach(func() {
			key, err = turtleware.JWKFromPrivateKey(privateKey, kid)
		})

		Context("With a valid RSA key", func() {
			BeforeEach(func() {
				var (
					genErr error
				)
				privateKey, genErr = rsa.GenerateKey(rand.Reader, 2048)
				if genErr != nil {
					panic(genErr.Error())
				}
			})

			It("should return a key with the expected attributes", func() {
				Expect(key.KeyID()).To(Equal(kid))
				Expect(key.Algorithm()).To(Equal(jwa.RS512.String()))
				Expect(key.KeyType()).To(Equal(jwa.RSA))
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("With a valid ECDSA key", func() {
			BeforeEach(func() {
				var (
					genErr error
				)

				privateKey, genErr = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
				if genErr != nil {
					panic(genErr.Error())
				}
			})

			It("should return a key with the expected attributes", func() {
				Expect(key.KeyID()).To(Equal(kid))
				Expect(key.Algorithm()).To(Equal(jwa.ES512.String()))
				Expect(key.KeyType()).To(Equal(jwa.EC))
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("With a valid ed25519 key", func() {
			BeforeEach(func() {
				var (
					genErr error
				)

				_, privateKey, genErr = ed25519.GenerateKey(rand.Reader)
				if genErr != nil {
					panic(genErr.Error())
				}
			})

			It("should return a key with the expected attributes", func() {
				Expect(key.KeyID()).To(Equal(kid))
				Expect(key.Algorithm()).To(Equal(jwa.EdDSA.String()))
				Expect(key.KeyType()).To(Equal(jwa.OKP))
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("With a valid HMAC key", func() {
			BeforeEach(func() {
				privateKey = []byte("supersecretpassphrase")
			})

			It("should return a key with the expected attributes", func() {
				Expect(key.KeyID()).To(Equal(kid))
				Expect(key.Algorithm()).To(Equal(jwa.HS512.String()))
				Expect(key.KeyType()).To(Equal(jwa.OctetSeq))
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("With no key", func() {
			BeforeEach(func() {
				privateKey = nil
			})

			It("should error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should error with ErrFailedToParsePrivateKey", func() {
				Expect(errors.Is(err, turtleware.ErrFailedToParsePrivateKey)).To(BeTrue())
			})
		})
	})

	Describe("JWKFromPublicKey", func() {
		var (
			publicKey crypto.PublicKey
			kid       string

			key jwk.Key
			err error
		)

		BeforeEach(func() {
			kid = "some-key-id"
		})

		// Actual method call
		JustBeforeEach(func() {
			key, err = turtleware.JWKFromPublicKey(publicKey, kid)
		})

		Context("With a valid RSA key", func() {
			BeforeEach(func() {
				privateKey, genErr := rsa.GenerateKey(rand.Reader, 2048)
				if genErr != nil {
					panic(genErr.Error())
				}

				publicKey = privateKey.Public()
			})

			It("should return a key with the expected attributes", func() {
				Expect(key.KeyID()).To(Equal(kid))
				Expect(key.Algorithm()).To(Equal(jwa.RS512.String()))
				Expect(key.KeyType()).To(Equal(jwa.RSA))
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("With a valid ECDSA key", func() {
			BeforeEach(func() {
				privateKey, genErr := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
				if genErr != nil {
					panic(genErr.Error())
				}

				publicKey = privateKey.Public()
			})

			It("should return a key with the expected attributes", func() {
				Expect(key.KeyID()).To(Equal(kid))
				Expect(key.Algorithm()).To(Equal(jwa.ES512.String()))
				Expect(key.KeyType()).To(Equal(jwa.EC))
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("With a valid ed25519 key", func() {
			BeforeEach(func() {
				_, privateKey, genErr := ed25519.GenerateKey(rand.Reader)
				if genErr != nil {
					panic(genErr.Error())
				}

				publicKey = privateKey.Public()
			})

			It("should return a key with the expected attributes", func() {
				Expect(key.KeyID()).To(Equal(kid))
				Expect(key.Algorithm()).To(Equal(jwa.EdDSA.String()))
				Expect(key.KeyType()).To(Equal(jwa.OKP))
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("With a valid HMAC key", func() {
			BeforeEach(func() {
				publicKey = []byte("supersecretpassphrase")
			})

			It("should return a key with the expected attributes", func() {
				Expect(key.KeyID()).To(Equal(kid))
				Expect(key.Algorithm()).To(Equal(jwa.HS512.String()))
				Expect(key.KeyType()).To(Equal(jwa.OctetSeq))
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("With no key", func() {
			BeforeEach(func() {
				publicKey = nil
			})

			It("should error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should error with ErrFailedToParsePrivateKey", func() {
				Expect(errors.Is(err, turtleware.ErrFailedToParsePrivateKey)).To(BeTrue())
			})
		})
	})

	Describe("ReadKeySetFromFolder", func() {
		var (
			keyFolder string

			keySet jwk.Set
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
			Expect(keySet.Len()).To(Equal(3))
		})

		It("should contain the RSA key", func() {
			for i := 0; i < keySet.Len(); i++ {
				key, _ := keySet.Get(i)
				kid, exists := key.Get(jwk.KeyIDKey)
				if exists && kid == "rsa-key" {
					return
				}
			}

			Fail("RSA key not loaded")
		})

		It("should contain the ecdsa key", func() {
			for i := 0; i < keySet.Len(); i++ {
				key, _ := keySet.Get(i)
				kid, exists := key.Get(jwk.KeyIDKey)
				if exists && kid == "ecdsa-key" {
					return
				}
			}

			Fail("ecdsa key not loaded")
		})

		It("should contain the ed25519 key", func() {
			for i := 0; i < keySet.Len(); i++ {
				key, _ := keySet.Get(i)
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
