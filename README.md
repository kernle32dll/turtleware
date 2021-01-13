![test](https://github.com/kernle32dll/turtleware/workflows/test/badge.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/kernle32dll/turtleware.svg)](https://pkg.go.dev/github.com/kernle32dll/turtleware)
[![Go Report Card](https://goreportcard.com/badge/github.com/kernle32dll/turtleware)](https://goreportcard.com/report/github.com/kernle32dll/turtleware)
[![codecov](https://codecov.io/gh/kernle32dll/turtleware/branch/master/graph/badge.svg)](https://codecov.io/gh/kernle32dll/turtleware)

# turtleware

turtleware is an opinionated framework for creating REST services. It provides pluggable middlewares and some utility methods
to simplify life. Its uses JWT bearer authentication, and relies heavily on caching. Logging is hardwired to the Logrus
logging lib and its default logger.

Download:

```
go get github.com/kernle32dll/turtleware
```

Detailed documentation can be found on [pkg.go.dev](https://pkg.go.dev/github.com/kernle32dll/turtleware).

## State of the project

turtleware is actively used in productive projects by the author.

Still, this project is still pretty much work-in-progress. Bugs happen, and breaking-changes might occur at **any** time.
Also, only Go 1.13 is supported for now. Code coverage is low, and documentation slim.

It is currently targeted for the course of 2020 to slowly stabilize the framework.

## Getting started

turtleware is build on a chain of middlewares, and provides simple handler methods for direct usage.
See handler.go for all existing handlers, and how they chain the existing middlewares together.

For the impatient, here is a simple example on how to provide a (static) REST resource with turtleware:


```go
package examples

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/kernle32dll/turtleware"

	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Entity struct {
	UUID string `json:"uuid"`
	Text string `json:"text"`
}

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
	handler := turtleware.RessourceHandler(
		// Provides authentication via the provided key
		[]interface{}{privateKey.PublicKey},
		// Provide the entity UUID
		func(r *http.Request) (string, error) {
			return staticUUID, nil
		},
		// Provide the date of last modification
		func(ctx context.Context, entityUUID string) (time.Time, error) {
			log.Printf("last-modification check for %s", entityUUID)

			// return sql.ErrNoRows to signal an entity not existing
			return time.Date(1991, 5, 23, 1, 2, 3, 4, time.UTC), nil
		},
		// Provide the actual data
		func(ctx context.Context, entityUUID string) (interface{}, error) {
			return Entity{
				UUID: entityUUID,
				Text: "It works!",
			}, nil
		},
	)

	http.Handle(fmt.Sprintf("/entities/%s", staticUUID), handler)

	if err := http.ListenAndServe(":80", nil); err != nil {
		log.Fatal(err)
	}
}
```