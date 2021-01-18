package turtleware

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path/filepath"

	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"errors"
	"net/http"
	"strings"
)

var (
	// ErrTokenValidationFailed indicates that the token provided
	// could not be validated.
	ErrTokenValidationFailed = errors.New("failed to validate token signature")

	// ErrMissingAuthHeader indicates that a requested was
	// missing an authentication header.
	ErrMissingAuthHeader = errors.New("authentication header missing")

	// ErrAuthHeaderWrongFormat indicates that a requested contained an a authorization
	// header, but it was in the wrong format.
	ErrAuthHeaderWrongFormat = errors.New("authorization header format must be Bearer {token}")
)

// ReadKeySetFromFolder recursively reads a folder for public keys
// to assemble a JWK set from.
func ReadKeySetFromFolder(path string) (*jwk.Set, error) {
	var jwtPublicKeys []jwk.Key

	if err := filepath.Walk(path,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				logrus.Debugf("Reading %s for public key", path)

				parseResult, err := tryToLoadPublicKey(path)
				if err != nil {
					logrus.WithError(err).Errorf("Failed to load %s as public key", path)
					return nil
				}

				key, err := jwk.New(parseResult)
				if err != nil {
					logrus.WithError(err).Errorf("Failed to parse %s as JWK", path)
					return nil
				}

				ext := filepath.Ext(info.Name())
				if err := key.Set(jwk.KeyIDKey, strings.TrimRight(info.Name(), ext)); err != nil {
					logrus.WithError(err).Errorf("Failed to set 'kid' for %s", path)
					return nil
				}

				jwtPublicKeys = append(jwtPublicKeys, key)
			}

			return nil
		}); err != nil {
		return nil, err
	}

	return &jwk.Set{Keys: jwtPublicKeys}, nil
}

func tryToLoadPublicKey(path string) (crypto.PublicKey, error) {
	pemBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read content of %q: %w", path, err)
	}

	// Parse PEM block
	var block *pem.Block
	if block, _ = pem.Decode(pemBytes); block == nil {
		return nil, fmt.Errorf("failed to parse PEM of %q: %w", path, err)
	}

	var (
		parsedKey interface{}
	)
	if parsedKey, err = x509.ParsePKIXPublicKey(block.Bytes); err != nil {
		if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
			parsedKey = cert.PublicKey
		} else {
			return nil, fmt.Errorf("failed to parse x509 of %q: %w", path, err)
		}
	}

	return parsedKey.(crypto.PublicKey), nil
}

// ValidateToken validates the given token with the given keys. If a key matches,
// the containing claims are returned. Otherwise ErrTokenValidationFailed is returned.
// Deprecated: use ValidateTokenBySet
func ValidateToken(
	token string, keys []interface{},
) (map[string]interface{}, error) {
	var (
		claims map[string]interface{}
		err    error
	)

	for _, key := range keys {
		if rsaPublicKey, ok := key.(*rsa.PublicKey); ok {
			claims, err = ValidateRSAJwt(token, rsaPublicKey, jwa.RS256, jwa.RS384, jwa.RS512)
		} else if ecdsaPublicKey, ok := key.(*ecdsa.PublicKey); ok {
			claims, err = ValidateECDSAJwt(token, ecdsaPublicKey, jwa.ES256, jwa.ES384, jwa.ES512)
		} else if secretPassphrase, ok := key.([]byte); ok {
			claims, err = ValidateHMACJwt(token, secretPassphrase, jwa.HS256, jwa.HS384, jwa.HS512)
		} else {
			// Unknown key type - ignore
			continue
		}

		// Key type recognized, but an error occurred
		// See if we have other keys which might work
		if err != nil {
			continue
		}

		return claims, nil
	}

	logrus.WithFields(logrus.Fields{
		"token": token,
		"error": err,
	}).Infof("Received invalid token: %s", token)

	// Hide exact validation error cause
	return nil, ErrTokenValidationFailed
}

// ValidateTokenBySet validates the given token with the given key set. If a key matches,
// the containing claims are returned.
func ValidateTokenBySet(
	tokenString string, keySet *jwk.Set,
) (map[string]interface{}, error) {
	token, err := jwt.ParseString(tokenString, jwt.WithKeySet(keySet))
	if err != nil {
		return nil, err
	}

	return token.AsMap(context.Background())
}

// ValidateRSAJwt validates a given token string against a RSA public key, and returns the claims when
// the signature is valid.
// Deprecated: will be removed with ValidateToken
func ValidateRSAJwt(
	tokenString string, publicKey *rsa.PublicKey, methods ...jwa.SignatureAlgorithm,
) (map[string]interface{}, error) {
	if len(methods) == 0 {
		return nil, errors.New("you must provide at least one RSA signing method")
	}

	var lastErr error

	for _, method := range methods {
		claims, err := validateJwt(tokenString, publicKey, method)
		if err == nil {
			return claims, nil
		}

		lastErr = err
	}

	return nil, lastErr
}

// ValidateHMACJwt validates a given token string against a HMAC secret, and returns the claims when
// the signature is valid.
// Deprecated: will be removed with ValidateToken
func ValidateHMACJwt(
	tokenString string, secret []byte, methods ...jwa.SignatureAlgorithm,
) (map[string]interface{}, error) {
	if len(methods) == 0 {
		return nil, errors.New("you must provide at least one HMAC signing method")
	}

	var lastErr error

	for _, method := range methods {
		claims, err := validateJwt(tokenString, secret, method)
		if err == nil {
			return claims, nil
		}

		lastErr = err
	}

	return nil, lastErr
}

// ValidateECDSAJwt validates a given token string against a ECDSA public key, and returns the claims when
// the signature is valid.
// Deprecated: will be removed with ValidateToken
func ValidateECDSAJwt(
	tokenString string, publicKey *ecdsa.PublicKey, methods ...jwa.SignatureAlgorithm,
) (map[string]interface{}, error) {
	if len(methods) == 0 {
		return nil, errors.New("you must provide at least one ECDSA signing method")
	}

	var lastErr error

	for _, method := range methods {
		claims, err := validateJwt(tokenString, publicKey, method)
		if err == nil {
			return claims, nil
		}

		lastErr = err
	}

	return nil, lastErr
}

// ValidateRSAJwt validates a given token string against an RSA public key, and returns the claims when
// the signature is valid.
// Deprecated: will be removed with ValidateToken
func validateJwt(tokenString string, secret interface{}, method jwa.SignatureAlgorithm) (map[string]interface{}, error) {
	token, err := jwt.ParseString(tokenString, jwt.WithVerify(method, secret))
	if err != nil {
		return nil, err
	}

	return token.AsMap(context.Background())
}

// FromAuthHeader is a "TokenExtractor" that takes a give request and extracts
// the JWT token from the Authorization header.
//
// Copied from https://github.com/auth0/go-jwt-middleware/blob/master/jwtmiddleware.go
func FromAuthHeader(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", ErrMissingAuthHeader
	}

	// TODO: Make this a bit more robust, parsing-wise
	authHeaderParts := strings.Split(authHeader, " ")
	if len(authHeaderParts) != 2 || strings.ToLower(authHeaderParts[0]) != "bearer" {
		return "", ErrAuthHeaderWrongFormat
	}

	return authHeaderParts[1], nil
}
