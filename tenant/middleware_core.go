package tenant

import (
	"github.com/kernle32dll/turtleware"
	"github.com/sirupsen/logrus"

	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"
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

type ListHashFunc func(ctx context.Context, tenantUUID string, paging turtleware.Paging) (string, error)
type ListCountFunc func(ctx context.Context, tenantUUID string) (uint, error)

type ResourceLastModFunc func(ctx context.Context, tenantUUID string, entityUUID string) (time.Time, error)

func CountHeaderMiddleware(countFetcher ListCountFunc, errorHandler turtleware.ErrorHandlerFunc) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			countContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			logger := logrus.WithContext(countContext)

			tenantUUID, err := UUIDFromRequestContext(countContext)
			if err != nil {
				errorHandler(countContext, w, r, err)
				return
			}

			totalCount, err := countFetcher(countContext, tenantUUID)
			if err != nil {
				logger.WithError(err).Error("Failed to receive count")
				errorHandler(countContext, w, r, turtleware.ErrReceivingMeta)
				return
			}

			w.Header().Set("X-Total-Count", fmt.Sprintf("%d", totalCount))

			h.ServeHTTP(w, r)
		})
	}
}

func ListCacheMiddleware(hashFetcher ListHashFunc, errorHandler turtleware.ErrorHandlerFunc) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := logrus.WithContext(r.Context())

			logger.Trace("Handling preflight for tenant based resource list request")

			etag, _ := turtleware.ExtractCacheHeader(r)

			if etag != "" {
				logger.Debugf("Received If-None-Match tag %s", etag)
			}

			hashContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			tenantUUID, err := UUIDFromRequestContext(hashContext)
			if err != nil {
				errorHandler(hashContext, w, r, err)
				return
			}

			paging, err := turtleware.PagingFromRequestContext(hashContext)
			if err != nil {
				errorHandler(hashContext, w, r, err)
				return
			}

			hash, err := hashFetcher(hashContext, tenantUUID, paging)
			if err != nil {
				logger.WithError(err).Error("Failed to receive hash")
				errorHandler(hashContext, w, r, turtleware.ErrReceivingMeta)
				return
			}

			w.Header().Set("Etag", hash)

			cacheHit := etag == hash
			if cacheHit {
				logger.Debug("Successful cache hit")
				w.WriteHeader(http.StatusNotModified)
				return
			}

			h.ServeHTTP(w, r)
		})
	}
}

func ResourceCacheMiddleware(lastModFetcher ResourceLastModFunc, errorHandler turtleware.ErrorHandlerFunc) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := logrus.WithContext(r.Context())

			logger.Trace("Handling preflight for tenant based resource request")

			_, lastModified := turtleware.ExtractCacheHeader(r)

			if lastModified.Valid {
				logger.Debugf("Received If-Modified-Since date %s", lastModified.Time)
			}

			hashContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			tenantUUID, err := UUIDFromRequestContext(hashContext)
			if err != nil {
				errorHandler(hashContext, w, r, err)
				return
			}

			entityUUID, err := turtleware.EntityUUIDFromRequestContext(hashContext)
			if err != nil {
				errorHandler(hashContext, w, r, err)
				return
			}

			maxModDate, err := lastModFetcher(hashContext, tenantUUID, entityUUID)
			if errors.Is(err, sql.ErrNoRows) {
				errorHandler(hashContext, w, r, turtleware.ErrResourceNotFound)
				return
			}

			if err != nil {
				logger.WithError(err).Error("Failed to receive last-modification date")
				errorHandler(hashContext, w, r, turtleware.ErrReceivingMeta)
				return
			}

			w.Header().Set("Last-Modified", maxModDate.Format(time.RFC1123))

			cacheHit := lastModified.Valid && maxModDate.Truncate(time.Second).Equal(lastModified.Time.Truncate(time.Second))
			if cacheHit {
				logger.Debug("Successful cache hit")
				w.WriteHeader(http.StatusNotModified)
				return
			}

			h.ServeHTTP(w, r)
		})
	}
}

// UUIDMiddleware is a http middleware for checking tenant authentication details, and
// passing down the tenant UUID if existing, or bailing out otherwise.
func UUIDMiddleware() func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := turtleware.AuthClaimsFromRequestContext(r.Context())
			if err != nil {
				turtleware.WriteError(w, r, http.StatusInternalServerError, err)
				return
			}

			tenantUUID, ok := claims["tenant_uuid"].(string)
			if !ok || tenantUUID == "" {
				turtleware.WriteError(w, r, http.StatusBadRequest, ErrTokenMissingTenantUUID)
				return
			}

			h.ServeHTTP(
				w,
				r.WithContext(context.WithValue(r.Context(), ctxTenantUUID, tenantUUID)),
			)
		})
	}
}

func UUIDFromRequestContext(ctx context.Context) (string, error) {
	tenantUUID, ok := ctx.Value(ctxTenantUUID).(string)
	if !ok {
		return "", ErrContextMissingTenantUUID
	}

	return tenantUUID, nil
}
