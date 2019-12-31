package turtleware

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/sirupsen/logrus"

	"crypto/rsa"
	"errors"
	"fmt"
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

// ValidateToken validates the given token with the given keys. If a key matches,
// the containing claims are returned. Otherwise ErrTokenValidationFailed is returned.
func ValidateToken(
	token string, keys []interface{},
) (map[string]interface{}, error) {
	var (
		claims map[string]interface{}
		err    error
	)

	for _, key := range keys {
		if rsaPublicKey, ok := key.(*rsa.PublicKey); ok {
			claims, err = ValidateRSAJwt(token, rsaPublicKey, jwt.SigningMethodRS256, jwt.SigningMethodRS384, jwt.SigningMethodRS512)
		} else if secretPassphrase, ok := key.([]byte); ok {
			claims, err = ValidateHMACJwt(token, secretPassphrase, jwt.SigningMethodHS256, jwt.SigningMethodHS384, jwt.SigningMethodHS512)
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

// ValidateRSAJwt validates a given token string against a RSA public key, and returns the claims when
// the signature is valid.
func ValidateRSAJwt(
	tokenString string, publicKey *rsa.PublicKey, methods ...*jwt.SigningMethodRSA,
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
func ValidateHMACJwt(
	tokenString string, secret []byte, methods ...*jwt.SigningMethodHMAC,
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

// ValidateRSAJwt validates a given token string against an RSA public key, and returns the claims when
// the signature is valid.
func validateJwt(tokenString string, secret interface{}, method jwt.SigningMethod) (map[string]interface{}, error) {
	token, err := jwt.ParseWithClaims(tokenString, jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if token.Method != method {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})

	if err != nil {
		return nil, err
	}

	claims, _ := token.Claims.(jwt.MapClaims)
	return claims, nil
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
