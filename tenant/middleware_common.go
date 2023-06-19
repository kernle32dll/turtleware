package tenant

import (
	"github.com/google/uuid"
	"github.com/kernle32dll/turtleware"

	"context"
	"errors"
	"net/http"
)

type ctxKey int

const (
	// ctxTenantUUID is the context key used to pass down the tenant UUID.
	ctxTenantUUID ctxKey = iota
)

var (
	// ErrContextMissingTenantUUID is an internal error indicating a missing
	// tenant UUID in the request context, whereas one was expected.
	ErrContextMissingTenantUUID = errors.New("missing tenant UUID in context")

	// ErrTokenMissingTenantUUID indicates that a requested was
	// missing the tenant uuid.
	ErrTokenMissingTenantUUID = errors.New("token does not include tenant uuid")
)

// UUIDMiddleware is a http middleware for checking tenant authentication details, and
// passing down the tenant UUID if existing, or bailing out otherwise.
func UUIDMiddleware() func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := turtleware.AuthClaimsFromRequestContext(r.Context())
			if err != nil {
				turtleware.WriteError(r.Context(), w, r, http.StatusInternalServerError, err)
				return
			}

			tenantUUIDString, ok := claims["tenant_uuid"].(string)
			if !ok || tenantUUIDString == "" {
				turtleware.WriteError(r.Context(), w, r, http.StatusBadRequest, ErrTokenMissingTenantUUID)
				return
			}

			tenantUUID, err := uuid.Parse(tenantUUIDString)
			if err != nil {
				// TODO!
				turtleware.WriteError(r.Context(), w, r, http.StatusBadRequest, ErrTokenMissingTenantUUID)
				return
			}

			h.ServeHTTP(
				w,
				r.WithContext(context.WithValue(r.Context(), ctxTenantUUID, tenantUUID)),
			)
		})
	}
}

func UUIDFromRequestContext(ctx context.Context) (uuid.UUID, error) {
	tenantUUID, ok := ctx.Value(ctxTenantUUID).(uuid.UUID)
	if !ok {
		return uuid.Nil, ErrContextMissingTenantUUID
	}

	return tenantUUID, nil
}
