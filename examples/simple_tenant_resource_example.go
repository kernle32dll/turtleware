package examples

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/kernle32dll/turtleware/tenant"

	"context"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	// Prepare valid private and public keys to provide tokens.
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	// A static tenant UUID - in the real world, this should be part of
	// your tenant authentication.
	staticTenantUUID := "b1af9e1b-b073-474c-99d7-4a9dcc83c4eb"

	// Generate an exemplary token. This is just an example - in your
	// application some other service should provide valid tokens.
	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"tenant_uuid": staticTenantUUID,
		"exp":         time.Now().Add(time.Minute * 30).Unix(),
	}).SignedString(privateKey)
	if err != nil {
		panic(err)
	}

	log.Printf("Use this token for requests:\n%s", tokenString)

	// This is a valid token, for another tenant.
	otherTenantTokenString, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"tenant_uuid": "e04c2fed-25b3-4c5f-85f0-d3bed3d02e4a",
		"exp":         time.Now().Add(time.Minute * 30).Unix(),
	}).SignedString(privateKey)
	if err != nil {
		panic(err)
	}

	log.Printf("Use this token to showcase multi-tenancy:\n%s", otherTenantTokenString)

	// -----------------------------

	// A static entity UUID - in the real world, provide some kind of routing
	staticUUID := "15f8ea60-9b0f-493c-8c5b-8de4022a0e4b"

	// Provides caching, and the data itself
	handler := tenant.RessourceHandler(
		// Provides authentication via the provided key
		[]interface{}{privateKey.PublicKey},
		// Provide the entity UUID
		func(r *http.Request) (string, error) {
			return staticUUID, nil
		},
		// Provide the date of last modification
		func(ctx context.Context, tenantUUID string, entityUUID string) (time.Time, error) {
			log.Printf("last-modification check for %s of tenant %s", entityUUID, tenantUUID)

			if tenantUUID == staticTenantUUID {
				return time.Date(1991, 5, 23, 1, 2, 3, 4, time.UTC), nil
			} else {
				return time.Time{}, sql.ErrNoRows
			}
		},
		// Provide the actual data
		func(ctx context.Context, tenantUUID string, entityUUID string) (interface{}, error) {
			if tenantUUID == staticTenantUUID {
				return Entity{
					UUID: entityUUID,
					Text: "It works!",
				}, nil
			} else {
				return nil, sql.ErrNoRows
			}
		},
	)

	http.Handle(fmt.Sprintf("/entities/%s", staticUUID), handler)

	if err := http.ListenAndServe(":80", nil); err != nil {
		log.Fatal(err)
	}
}
