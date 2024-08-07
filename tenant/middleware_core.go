package tenant

import (
	"github.com/kernle32dll/turtleware"
	"github.com/rs/zerolog"

	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"
)

var emptyListHash = hex.EncodeToString(sha256.New().Sum(nil))

// ListHashFunc is a function for returning a calculated hash for a given subset of entities
// of a given tenant, via the given paging, for a list endpoint.
// The function may return sql.ErrNoRows or os.ErrNotExist to indicate that there are not
// elements, for easier handling.
type ListHashFunc func(ctx context.Context, tenantUUID string, paging turtleware.Paging) (string, error)

// ListCountFunc is a function for returning the total amount of entities of a given tenant
// for a list endpoint.
// The function may return sql.ErrNoRows or os.ErrNotExist to indicate that there are not
// elements, for easier handling.
type ListCountFunc func(ctx context.Context, tenantUUID string) (uint, error)

// ResourceLastModFunc is a function for returning the last modification data for a specific
// entity of a given tenant.
// The function may return sql.ErrNoRows or os.ErrNotExist to indicate that there are not
// elements, for easier handling.
type ResourceLastModFunc func(ctx context.Context, tenantUUID string, entityUUID string) (time.Time, error)

// CountHeaderMiddleware is a middleware for injecting an X-Total-Count header into the response,
// by the provided ListCountFunc. If an error is encountered, the provided turtleware.ErrorHandlerFunc is called.
func CountHeaderMiddleware(
	countFetcher ListCountFunc,
	errorHandler turtleware.ErrorHandlerFunc,
) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			countContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			logger := zerolog.Ctx(countContext)

			tenantUUID, err := UUIDFromRequestContext(countContext)
			if err != nil {
				errorHandler(countContext, w, r, err)
				return
			}

			totalCount, err := countFetcher(countContext, tenantUUID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) || errors.Is(err, os.ErrNotExist) {
					totalCount = 0
				} else {
					logger.Error().Err(err).Msg("Failed to receive count")
					errorHandler(countContext, w, r, turtleware.ErrReceivingMeta)

					return
				}
			}

			w.Header().Set("X-Total-Count", fmt.Sprintf("%d", totalCount))

			h.ServeHTTP(w, r)
		})
	}
}

// ListCacheMiddleware is a middleware for transparently handling caching via the provided
// ListHashFunc. The next handler of the middleware is only called on a cache miss. That is,
// if the If-None-Match header and the fetched hash differ.
// If the ListHashFunc returns either sql.ErrNoRows or os.ErrNotExist, the sha256 hash of an
// empty string is assumed as the hash.
// If an error is encountered, the provided turtleware.ErrorHandlerFunc is called.
func ListCacheMiddleware(
	hashFetcher ListHashFunc,
	errorHandler turtleware.ErrorHandlerFunc,
) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := zerolog.Ctx(r.Context())
			w.Header().Set("Cache-Control", "must-revalidate")
			w.Header().Add("Cache-Control", "max-age=0")

			logger.Trace().Msg("Handling preflight for tenant based resource list request")

			etag, _ := turtleware.ExtractCacheHeader(r)

			if etag != "" {
				logger.Debug().Msgf("Received If-None-Match tag %s", etag)
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
				if errors.Is(err, sql.ErrNoRows) || errors.Is(err, os.ErrNotExist) {
					hash = emptyListHash
				} else {
					logger.Error().Err(err).Msg("Failed to receive hash")
					errorHandler(hashContext, w, r, turtleware.ErrReceivingMeta)

					return
				}
			}

			w.Header().Set("Etag", hash)

			cacheHit := etag == hash
			if cacheHit {
				logger.Debug().Msg("Successful cache hit")
				w.WriteHeader(http.StatusNotModified)

				return
			}

			h.ServeHTTP(w, r)
		})
	}
}

// ResourceCacheMiddleware is a middleware for transparently handling caching of a single entity
// (or resource) of a tenant via the provided ResourceLastModFunc. The next handler of the middleware
// is only called when the If-Modified-Since header and the fetched last modification date differ.
// If an error is encountered, the provided turtleware.ErrorHandlerFunc is called.
func ResourceCacheMiddleware(
	lastModFetcher ResourceLastModFunc,
	errorHandler turtleware.ErrorHandlerFunc,
) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := zerolog.Ctx(r.Context())

			logger.Trace().Msg("Handling preflight for tenant based resource request")

			_, lastModified := turtleware.ExtractCacheHeader(r)

			if !lastModified.IsZero() {
				logger.Debug().Msgf("Received If-Modified-Since date %s", lastModified)
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
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, os.ErrNotExist) {
				// Skip cache check
				h.ServeHTTP(w, r)

				return
			}

			if err != nil {
				logger.Error().Err(err).Msg("Failed to receive last-modification date")
				errorHandler(hashContext, w, r, turtleware.ErrReceivingMeta)

				return
			}

			w.Header().Set("Last-Modified", maxModDate.Format(time.RFC1123))

			cacheHit := !lastModified.IsZero() && maxModDate.Truncate(time.Second).Equal(lastModified.Truncate(time.Second))
			if cacheHit {
				logger.Debug().Msg("Successful cache hit")
				w.WriteHeader(http.StatusNotModified)

				return
			}

			h.ServeHTTP(w, r)
		})
	}
}
