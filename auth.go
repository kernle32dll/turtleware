package turtleware

import (
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/sirupsen/logrus"

	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
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
