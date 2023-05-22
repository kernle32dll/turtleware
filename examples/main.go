package main

import (
	"github.com/kernle32dll/turtleware"
	"github.com/kernle32dll/turtleware/tenant"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jws"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/rs/zerolog"

	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"
	"time"
)

const (
	// Static tenant UUIDs - in the real world, this should be part of your tenant authentication.
	staticTenantUUID1 = "b1af9e1b-b073-474c-99d7-4a9dcc83c4eb"
	staticTenantUUID2 = "e04c2fed-25b3-4c5f-85f0-d3bed3d02e4a"

	// A static entity UUID - in the real world, provide some kind of routing
	staticEntityUUID = "15f8ea60-9b0f-493c-8c5b-8de4022a0e4b"

	keyID = "some-key"
)

// -----------------------------

// Entity is an example entity.
type Entity struct {
	UUID       string `json:"uuid"`
	TenantUUID string `json:"tenant_uuid"`
	Text       string `json:"text"`
}

// Endpoint is a holder definition for a single endpoint. In reality,
// this struct would hold additional data to e.g. interact with your
// persistence layer.
type Endpoint struct{}

func (e Endpoint) EntityUUID(*http.Request) (string, error) {
	// Here you would parse the entity UUID from the request,
	return staticEntityUUID, nil
}

func (e Endpoint) LastModification(ctx context.Context, tenantUUID string, entityUUID string) (time.Time, error) {
	// Fetch the logger from context
	logger := zerolog.Ctx(ctx)
	logger.Info().Msgf("last-modification check for %s of tenant %s", entityUUID, tenantUUID)

	// Here you must return the last modification date for a given entity,
	// so we can leverage caching efficiently.
	// You max return sql.ErrNoRows or os.ErrNotExist to indicate that the entity does not exist.
	return time.Date(1991, 5, 23, 1, 2, 3, 4, time.UTC), nil
}

func (e Endpoint) FetchEntity(ctx context.Context, tenantUUID string, entityUUID string) (interface{}, error) {
	// Fetch the logger from context
	logger := zerolog.Ctx(ctx)
	logger.Info().Msgf("fetch for %s of tenant %s", entityUUID, tenantUUID)

	return Entity{
		UUID:       entityUUID,
		TenantUUID: tenantUUID,
		Text:       "It works!",
	}, nil
}

func (e Endpoint) HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	// Here you can implement your own error handling, or delegate to the default error handler.
	turtleware.DefaultErrorHandler(ctx, w, r, err)
}

// -----------------------------

func main() {
	// Initialize some exemplary logger
	writer := zerolog.NewConsoleWriter()
	writer.TimeFormat = time.RFC3339
	logger := zerolog.New(writer).Level(zerolog.DebugLevel).With().Timestamp().Logger()
	zerolog.DefaultContextLogger = &logger

	// -----------------------------

	// Prepare valid private and public keys to provide tokens.
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to generate RSA key")
	}
	jwkPublicKey, err := turtleware.JWKFromPublicKey(privateKey.Public(), keyID)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to generate JWK public key from RSA private key")
	}

	keySet := jwk.NewSet()
	keySet.Add(jwkPublicKey)

	// -----------------------------

	// Generate an exemplary token. This is just an example - in your
	// application some other service should provide valid tokens.
	tokenString, err := generateToken(
		jwa.RS512,
		privateKey,
		map[string]interface{}{
			// If you want to use multi tenancy functionality, the token
			// MUST provide a "tenant_uuid" claim, that is passed down.
			"tenant_uuid": staticTenantUUID1,
		},
		map[string]interface{}{
			"exp": time.Now().Add(time.Minute * 30).Unix(),
		},
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to generate staticTenantUUID1 token")
	}

	logger.Info().Msg("Use this curl command for requests:")
	logger.Info().Msgf(
		`curl -H "Authorization: Bearer %s" http://127.0.0.1:8000/entities/%s`,
		tokenString, staticEntityUUID,
	)

	// This is a valid token, for another tenant.
	otherTenantTokenString, err := generateToken(
		jwa.RS512,
		privateKey,
		map[string]interface{}{
			"tenant_uuid": staticTenantUUID2,
		},
		map[string]interface{}{
			"exp": time.Now().Add(time.Minute * 30).Unix(),
		},
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to generate staticTenantUUID2 token")
	}

	logger.Info().Msgf(
		`Use this curl command to showcase multi-tenancy:`)
	logger.Info().Msgf(
		`curl -H "Authorization: Bearer %s" http://127.0.0.1:8000/entities/%s`,
		otherTenantTokenString, staticEntityUUID,
	)

	// -----------------------------

	handler := tenant.ResourceHandler(keySet, &Endpoint{})
	http.Handle(fmt.Sprintf("/entities/%s", staticEntityUUID), handler)

	if err := http.ListenAndServe(":8000", nil); err != nil {
		logger.Fatal().Err(err).Msg("failed to shutdown server")
	}
}

func generateToken(
	algo jwa.SignatureAlgorithm,
	key interface{},
	claims map[string]interface{},
	headers map[string]interface{},
) (string, error) {
	t := jwt.New()

	hdr := jws.NewHeaders()
	for k, v := range headers {
		if err := hdr.Set(k, v); err != nil {
			return "", err
		}
	}

	for k, v := range claims {
		if err := t.Set(k, v); err != nil {
			return "", err
		}
	}

	if err := hdr.Set(jwk.KeyIDKey, keyID); err != nil {
		return "", err
	}

	signedT, err := jwt.Sign(t, algo, key, jwt.WithHeaders(hdr))
	if err != nil {
		return "", err
	}

	return string(signedT), nil
}
