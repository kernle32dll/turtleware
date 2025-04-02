package turtleware_test

import (
	"github.com/kernle32dll/turtleware"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/stretchr/testify/suite"

	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

type AuthSuite struct {
	CommonSuite
}

func TestAuthSuite(t *testing.T) {
	suite.Run(t, &AuthSuite{})
}

func (s *AuthSuite) Test_FromAuthHeader() {
	request, err := http.NewRequest(http.MethodGet, "https://example.com/foo", http.NoBody)
	s.Require().NoError(err)

	s.Run("No_Auth_Header", func() {
		// given
		// -

		// when
		token, err := turtleware.FromAuthHeader(request)

		// then
		s.ErrorIs(err, turtleware.ErrMissingAuthHeader)
		s.Empty(token)

	})

	s.Run("Empty_Auth_Header", func() {
		// given
		request.Header.Set("authorization", "")

		// when
		token, err := turtleware.FromAuthHeader(request)

		// then
		s.ErrorIs(err, turtleware.ErrMissingAuthHeader)
		s.Empty(token)
	})

	s.Run("Valid_Bearer", func() {
		// given
		request.Header.Set("authorization", "Bearer 123")

		// when
		token, err := turtleware.FromAuthHeader(request)

		// then
		s.NoError(err)
		s.Equal("123", token)
	})

	s.Run("Wrong_Type", func() {
		// given
		request.Header.Set("authorization", "cucumber 123")

		// when
		token, err := turtleware.FromAuthHeader(request)

		// then
		s.ErrorIs(err, turtleware.ErrAuthHeaderWrongFormat)
		s.Empty(token)
	})

	s.Run("Wrong_Parts", func() {
		// given
		request.Header.Set("authorization", "bearer 123 456")

		// when
		token, err := turtleware.FromAuthHeader(request)

		// then
		s.ErrorIs(err, turtleware.ErrAuthHeaderWrongFormat)
		s.Empty(token)
	})
}

func (s *AuthSuite) Test_ValidateTokenBySet() {
	// private keys
	rsaPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	s.Require().NoError(err)

	ecdsaPrivateKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	s.Require().NoError(err)

	_, ed25519PrivateKey, err := ed25519.GenerateKey(rand.Reader)
	s.Require().NoError(err)

	hmacKey := []byte("supersecretpassphrase")

	// public keys
	rsaPublicKey, err := turtleware.JWKFromPublicKey(rsaPrivateKey.Public(), "rsa-key")
	s.Require().NoError(err)

	ecdsaPublicKey, err := turtleware.JWKFromPublicKey(ecdsaPrivateKey.Public(), "ecdsa-key")
	s.Require().NoError(err)

	ed25519PublicKey, err := turtleware.JWKFromPublicKey(ed25519PrivateKey.Public(), "ed25519-key")
	s.Require().NoError(err)

	hmacPublicKey, err := turtleware.JWKFromPublicKey(hmacKey, "hmac-key")
	s.Require().NoError(err)

	// build keyset
	keys := jwk.NewSet()
	s.Require().NoError(keys.AddKey(rsaPublicKey))
	s.Require().NoError(keys.AddKey(ecdsaPublicKey))
	s.Require().NoError(keys.AddKey(ed25519PublicKey))
	s.Require().NoError(keys.AddKey(hmacPublicKey))

	expectedClaims := map[string]interface{}{
		jwt.JwtIDKey: "deadbeef",
	}

	s.Run("Garbage", func() {
		claims, err := turtleware.ValidateTokenBySet("trash", keys)

		// then
		s.Error(err)
		s.Nil(claims)
	})

	s.Run("RSA", func() {
		key, _ := keys.LookupKeyID("rsa-key")

		s.Run("Valid_Token_RSA256", func() {
			// given
			s.Require().NoError(key.Set(jwk.AlgorithmKey, jwa.RS256))
			token := s.generateToken(jwa.RS256, rsaPrivateKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: key.KeyID()})

			// when
			claims, err := turtleware.ValidateTokenBySet(token, keys)

			// then
			s.NoError(err)
			s.Equal(expectedClaims, claims)
		})

		s.Run("Valid_Token_RSA384", func() {
			// given
			s.Require().NoError(key.Set(jwk.AlgorithmKey, jwa.RS384))
			token := s.generateToken(jwa.RS384, rsaPrivateKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: key.KeyID()})

			// when
			claims, err := turtleware.ValidateTokenBySet(token, keys)

			// then
			s.NoError(err)
			s.Equal(expectedClaims, claims)
		})

		s.Run("Valid_Token_RSA512", func() {
			// given
			s.Require().NoError(key.Set(jwk.AlgorithmKey, jwa.RS512))
			token := s.generateToken(jwa.RS512, rsaPrivateKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: key.KeyID()})

			// when
			claims, err := turtleware.ValidateTokenBySet(token, keys)

			// then
			s.NoError(err)
			s.Equal(expectedClaims, claims)
		})
	})

	s.Run("ECDSA", func() {
		key, _ := keys.LookupKeyID("ecdsa-key")

		s.Run("Valid_Token_ES256", func() {
			// given
			s.Require().NoError(key.Set(jwk.AlgorithmKey, jwa.ES256))
			token := s.generateToken(jwa.ES256, ecdsaPrivateKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: key.KeyID()})

			// when
			claims, err := turtleware.ValidateTokenBySet(token, keys)

			// then
			s.NoError(err)
			s.Equal(expectedClaims, claims)
		})

		s.Run("Valid_Token_ES384", func() {
			// given
			s.Require().NoError(key.Set(jwk.AlgorithmKey, jwa.ES384))
			token := s.generateToken(jwa.ES384, ecdsaPrivateKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: key.KeyID()})

			// when
			claims, err := turtleware.ValidateTokenBySet(token, keys)

			// then
			s.NoError(err)
			s.Equal(expectedClaims, claims)
		})

		s.Run("Valid_Token_ES512", func() {
			// given
			s.Require().NoError(key.Set(jwk.AlgorithmKey, jwa.ES512))
			token := s.generateToken(jwa.ES512, ecdsaPrivateKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: key.KeyID()})

			// when
			claims, err := turtleware.ValidateTokenBySet(token, keys)

			// then
			s.NoError(err)
			s.Equal(expectedClaims, claims)
		})
	})

	s.Run("ed25519", func() {
		key, _ := keys.LookupKeyID("ed25519-key")

		s.Run("Valid_Token", func() {
			// given
			s.Require().NoError(key.Set(jwk.AlgorithmKey, jwa.EdDSA))
			token := s.generateToken(jwa.EdDSA, ed25519PrivateKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: key.KeyID()})

			// when
			claims, err := turtleware.ValidateTokenBySet(token, keys)

			// then
			s.NoError(err)
			s.Equal(expectedClaims, claims)
		})
	})

	s.Run("HMAC", func() {
		key, _ := keys.LookupKeyID("hmac-key")

		s.Run("Valid_Token_HS256", func() {
			// given
			s.Require().NoError(key.Set(jwk.AlgorithmKey, jwa.HS256))
			token := s.generateToken(jwa.HS256, hmacKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: key.KeyID()})

			// when
			claims, err := turtleware.ValidateTokenBySet(token, keys)

			// then
			s.NoError(err)
			s.Equal(expectedClaims, claims)
		})

		s.Run("Valid_Token_HS384", func() {
			// given
			s.Require().NoError(key.Set(jwk.AlgorithmKey, jwa.HS384))
			token := s.generateToken(jwa.HS384, hmacKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: key.KeyID()})

			// when
			claims, err := turtleware.ValidateTokenBySet(token, keys)

			// then
			s.NoError(err)
			s.Equal(expectedClaims, claims)
		})

		s.Run("Valid_Token_HS512", func() {
			// given
			s.Require().NoError(key.Set(jwk.AlgorithmKey, jwa.HS512))
			token := s.generateToken(jwa.HS512, hmacKey, expectedClaims, map[string]interface{}{jwk.KeyIDKey: key.KeyID()})

			// when
			claims, err := turtleware.ValidateTokenBySet(token, keys)

			// then
			s.NoError(err)
			s.Equal(expectedClaims, claims)
		})
	})
}

func (s *AuthSuite) Test_JWKFromPrivateKey() {
	// given
	kid := "some-key-id"

	s.Run("RSA", func() {
		// given
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		s.Require().NoError(err)

		// when
		key, err := turtleware.JWKFromPrivateKey(privateKey, kid)

		// then
		s.NoError(err)
		s.Equal(kid, key.KeyID())
		s.Equal(jwa.RS512, key.Algorithm())
		s.Equal(jwa.RSA, key.KeyType())
	})

	s.Run("ECDSA", func() {
		// given
		privateKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
		s.Require().NoError(err)

		// when
		key, err := turtleware.JWKFromPrivateKey(privateKey, kid)

		// then
		s.NoError(err)
		s.Equal(kid, key.KeyID())
		s.Equal(jwa.ES512, key.Algorithm())
		s.Equal(jwa.EC, key.KeyType())
	})

	s.Run("ed25519", func() {
		// given
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		s.Require().NoError(err)

		// when
		key, err := turtleware.JWKFromPrivateKey(privateKey, kid)

		// then
		s.NoError(err)
		s.Equal(kid, key.KeyID())
		s.Equal(jwa.EdDSA, key.Algorithm())
		s.Equal(jwa.OKP, key.KeyType())
	})

	s.Run("HMAC", func() {
		// given
		privateKey := []byte("supersecretpassphrase")

		// when
		key, err := turtleware.JWKFromPrivateKey(privateKey, kid)

		// then
		s.NoError(err)
		s.Equal(kid, key.KeyID())
		s.Equal(jwa.HS512, key.Algorithm())
		s.Equal(jwa.OctetSeq, key.KeyType())
	})

	s.Run("No_Key", func() {
		// given
		// -

		// when
		key, err := turtleware.JWKFromPrivateKey(nil, kid)

		// then
		s.ErrorIs(err, turtleware.ErrFailedToParsePrivateKey)
		s.Nil(key)
	})
}

func (s *AuthSuite) Test_JWKFromPublicKey() {
	// given
	kid := "some-key-id"

	s.Run("RSA", func() {
		// given
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		s.Require().NoError(err)

		// when
		key, err := turtleware.JWKFromPublicKey(privateKey.Public(), kid)

		// then
		s.NoError(err)
		s.Equal(kid, key.KeyID())
		s.Equal(jwa.RS512, key.Algorithm())
		s.Equal(jwa.RSA, key.KeyType())
	})

	s.Run("ECDSA", func() {
		// given
		privateKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
		s.Require().NoError(err)

		// when
		key, err := turtleware.JWKFromPublicKey(privateKey.Public(), kid)

		// then
		s.NoError(err)
		s.Equal(kid, key.KeyID())
		s.Equal(jwa.ES512, key.Algorithm())
		s.Equal(jwa.EC, key.KeyType())
	})

	s.Run("ed25519", func() {
		// given
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		s.Require().NoError(err)

		// when
		key, err := turtleware.JWKFromPublicKey(privateKey.Public(), kid)

		// then
		s.NoError(err)
		s.Equal(kid, key.KeyID())
		s.Equal(jwa.EdDSA, key.Algorithm())
		s.Equal(jwa.OKP, key.KeyType())
	})

	s.Run("HMAC", func() {
		// given
		privateKey := []byte("supersecretpassphrase")

		// when
		key, err := turtleware.JWKFromPublicKey(privateKey, kid)

		// then
		s.NoError(err)
		s.Equal(kid, key.KeyID())
		s.Equal(jwa.HS512, key.Algorithm())
		s.Equal(jwa.OctetSeq, key.KeyType())
	})

	s.Run("No_Key", func() {
		// given
		// -

		// when
		key, err := turtleware.JWKFromPublicKey(nil, kid)

		// then
		s.ErrorIs(err, turtleware.ErrFailedToParsePrivateKey)
		s.Nil(key)
	})

}

func (s *AuthSuite) Test_ReadKeySetFromFolder() {
	// given
	keyFolder := s.T().TempDir()

	s.Run("Success", func() {
		// RSA
		rsaKey, err := rsa.GenerateKey(rand.Reader, 1024)
		s.Require().NoError(err)
		s.Require().NoError(createValidPublicKey(keyFolder, "rsa-key.puba", rsaKey.Public()))

		// ecdsa
		ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		s.Require().NoError(err)
		s.Require().NoError(createValidPublicKey(keyFolder, "ecdsa-key.pubn", ecdsaKey.Public()))

		// ed25519
		ed25519PubKey, _, err := ed25519.GenerateKey(rand.Reader)
		s.Require().NoError(err)
		s.Require().NoError(createValidPublicKey(keyFolder, "ed25519-key.pubc", ed25519PubKey))

		// garbage
		s.Require().NoError(os.WriteFile(filepath.Join(keyFolder, "garbage.pubd"), []byte("garbage"), 0644))

		// when
		keySet, err := turtleware.ReadKeySetFromFolder(context.Background(), keyFolder)

		// then
		s.Equal(keySet.Len(), 3)
		s.NoError(err)

		s.True(containsKey(keySet, "rsa-key"), "RSA key not loaded")
		s.True(containsKey(keySet, "ecdsa-key"), "ecdsa key not loaded")
		s.True(containsKey(keySet, "ed25519-key"), "ed25519 key not loaded")
	})

	s.Run("Context_Error", func() {
		// given
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// when
		keySet, err := turtleware.ReadKeySetFromFolder(ctx, keyFolder)

		// then
		s.Nil(keySet)
		s.Error(err)
	})
}

func containsKey(keySet jwk.Set, keyID string) bool {
	for i := 0; i < keySet.Len(); i++ {
		key, _ := keySet.Key(i)
		kid, exists := key.Get(jwk.KeyIDKey)
		if exists && kid == keyID {
			return true
		}
	}

	return false
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

	return os.WriteFile(filepath.Join(keyFolder, filename), pem.EncodeToMemory(pemBlock), 0644)
}
