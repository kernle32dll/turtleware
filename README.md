[![Build Status](https://travis-ci.com/kernle32dll/turtleware.svg?branch=master)](https://travis-ci.com/kernle32dll/turtleware)
[![GoDoc](https://godoc.org/github.com/kernle32dll/turtleware?status.svg)](http://godoc.org/github.com/kernle32dll/turtleware)
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

Detailed documentation can be found on [GoDoc](https://godoc.org/github.com/kernle32dll/turtleware).

## Getting started

turtleware is build on a chain of middlewares, and provides simple handler methods for direct usage.
See handler.go for all existing handlers, and how they chain the existing middlewares together.

For the impatient, here is a simple example on how to provide a (static) REST resource with turtleware:


```go
package examples

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/kernle32dll/turtleware"
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

	// -----------------------------

	// A static entity UUID - in the real world, provide some kind of routing
	staticUUID := "15f8ea60-9b0f-493c-8c5b-8de4022a0e4b"

	// Provides authentication via the provided key
	preflight := turtleware.TenantResourcePreHandler([]interface{}{privateKey.PublicKey})

	// Provides caching, and the data itself
	data := turtleware.TenantRessourceHandler(
		// Provide the entity UUID
		func(r *http.Request) (string, error) {
			return staticUUID, nil
		}, func(ctx context.Context, tenantUUID string, entityUUID string) (time.Time, error) {
			log.Printf("last-modification check for %s of tenant %s", entityUUID, tenantUUID)

			if tenantUUID == staticTenantUUID {
				return time.Date(1991, 5, 23, 1, 2, 3, 4, time.UTC), nil
			} else {
				return time.Time{}, sql.ErrNoRows
			}
		}, func(ctx context.Context, tenantUUID string, entityUUID string) (interface{}, error) {
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

	http.Handle(fmt.Sprintf("/entities/%s", staticUUID), preflight(data))

	if err := http.ListenAndServe(":80", nil); err != nil {
		log.Fatal(err)
	}
}
```