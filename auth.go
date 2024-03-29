package turtleware

import (
	"github.com/go-logr/logr"
	"github.com/kernle32dll/keybox-go"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"

	"context"
	"crypto"
	"errors"
	"fmt"
	"log/slog"
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

	// ErrFailedToParsePrivateKey indicates a problem parsing a given private key as a JWK.
	ErrFailedToParsePrivateKey = errors.New("failed to parse private key as JWK")

	// ErrFailedToSetKID indicates a problem setting the KID field of a JWK.
	ErrFailedToSetKID = errors.New("failed to set 'kid' field")

	// ErrFailedToSetAlgorithm indicates a problem setting the alg field of a JWK.
	ErrFailedToSetAlgorithm = errors.New("failed to set 'alg' field")
)

// ReadKeySetFromFolder recursively reads a folder for public keys
// to assemble a JWK set from.
func ReadKeySetFromFolder(ctx context.Context, path string) (jwk.Set, error) {
	set := jwk.NewSet()

	logger := slog.New(logr.ToSlogHandler(logr.FromContextOrDiscard(ctx)))

	if err := filepath.Walk(path,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if err := ctx.Err(); err != nil {
				return err
			}

			if !info.IsDir() {
				logger.DebugContext(ctx, fmt.Sprintf("Reading %q for public key", path))

				parseResult, err := keybox.LoadPublicKey(path)
				if err != nil {
					logger.ErrorContext(
						ctx,
						fmt.Sprintf("Failed to load %q as public key", path),
						slog.Any("error", err),
					)
					return nil
				}

				kid := strings.TrimRight(info.Name(), filepath.Ext(info.Name()))
				key, err := JWKFromPublicKey(parseResult, kid)
				if err != nil {
					logger.ErrorContext(
						ctx,
						fmt.Sprintf("Failed to parse %q as JWK", path),
						slog.Any("error", err),
					)
					return nil
				}

				if err := set.AddKey(key); err != nil {
					logger.ErrorContext(
						ctx,
						fmt.Sprintf("Failed to add %q to key set", path),
						slog.Any("error", err),
					)
					return nil
				}
			}

			return nil
		}); err != nil {
		return nil, err
	}

	return set, nil
}

// JWKFromPrivateKey parses a given crypto.PrivateKey as a JWK, and tries
// to set the KID field of it.
// It also tries to guess the algorithm for signing with the JWK.
func JWKFromPrivateKey(privateKey crypto.PrivateKey, kid string) (jwk.Key, error) {
	key, err := jwk.FromRaw(privateKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrFailedToParsePrivateKey, err)
	}

	if err := key.Set(jwk.KeyIDKey, kid); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrFailedToSetKID, err)
	}

	var algo jwa.SignatureAlgorithm

	kt := key.KeyType()
	switch kt {
	case jwa.RSA:
		algo = jwa.RS512
	case jwa.EC:
		algo = jwa.ES512
	case jwa.OKP:
		algo = jwa.EdDSA
	case jwa.OctetSeq:
		algo = jwa.HS512
	default:
		return nil, fmt.Errorf("%w: unknown key type %s", ErrFailedToSetAlgorithm, kt)
	}

	if err := key.Set(jwk.AlgorithmKey, algo); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrFailedToSetAlgorithm, err)
	}

	return key, nil
}

// JWKFromPublicKey parses a given crypto.PublicKey as a JWK, and tries
// to set the KID field of it.
func JWKFromPublicKey(publicKey crypto.PublicKey, kid string) (jwk.Key, error) {
	key, err := jwk.FromRaw(publicKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrFailedToParsePrivateKey, err)
	}

	if err := key.Set(jwk.KeyIDKey, kid); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrFailedToSetKID, err)
	}

	var algo jwa.SignatureAlgorithm

	kt := key.KeyType()
	switch kt {
	case jwa.RSA:
		algo = jwa.RS512
	case jwa.EC:
		algo = jwa.ES512
	case jwa.OKP:
		algo = jwa.EdDSA
	case jwa.OctetSeq:
		algo = jwa.HS512
	default:
		return nil, fmt.Errorf("%w: unknown key type %s", ErrFailedToSetAlgorithm, kt)
	}

	if err := key.Set(jwk.AlgorithmKey, algo); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrFailedToSetAlgorithm, err)
	}

	return key, nil
}

// ValidateTokenBySet validates the given token with the given key set. If a key matches,
// the containing claims are returned.
func ValidateTokenBySet(
	tokenString string, keySet jwk.Set,
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
