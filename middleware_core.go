package turtleware

import (
	"github.com/sirupsen/logrus"

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

type ListHashFunc func(ctx context.Context, paging Paging) (string, error)
type ListCountFunc func(ctx context.Context) (uint, error)

type ResourceLastModFunc func(ctx context.Context, entityUUID string) (time.Time, error)

type ErrorHandlerFunc func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)

func CountHeaderMiddleware(countFetcher ListCountFunc, errorHandler ErrorHandlerFunc) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			countContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			logger := logrus.WithContext(countContext)

			totalCount, err := countFetcher(countContext)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) || errors.Is(err, os.ErrNotExist) {
					totalCount = 0
				} else {
					logger.WithError(err).Errorf("Failed to receive count")
					errorHandler(countContext, w, r, ErrReceivingMeta)

					return
				}
			}

			w.Header().Set("X-Total-Count", fmt.Sprintf("%d", totalCount))

			h.ServeHTTP(w, r)
		})
	}
}

func ListCacheMiddleware(hashFetcher ListHashFunc, errorHandler ErrorHandlerFunc) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := logrus.WithContext(r.Context())
			w.Header().Set("Cache-Control", "must-revalidate")
			w.Header().Add("Cache-Control", "max-age=0")

			logger.Trace("Handling preflight for resource list request")

			etag, _ := ExtractCacheHeader(r)

			if etag != "" {
				logger.Debugf("Received If-None-Match tag %s", etag)
			}

			hashContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			paging, err := PagingFromRequestContext(hashContext)
			if err != nil {
				errorHandler(hashContext, w, r, err)

				return
			}

			hash, err := hashFetcher(hashContext, paging)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					hash = emptyListHash
				} else {
					logger.WithError(err).Error("Failed to receive hash")
					errorHandler(hashContext, w, r, ErrReceivingMeta)

					return
				}
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

func ResourceCacheMiddleware(lastModFetcher ResourceLastModFunc, errorHandler ErrorHandlerFunc) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := logrus.WithContext(r.Context())

			logger.Trace("Handling preflight for resource request")

			_, lastModified := ExtractCacheHeader(r)

			if lastModified.Valid {
				logger.Debugf("Received If-Modified-Since date %s", lastModified.Time)
			}

			hashContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			entityUUID, err := EntityUUIDFromRequestContext(hashContext)
			if err != nil {
				errorHandler(hashContext, w, r, err)

				return
			}

			maxModDate, err := lastModFetcher(hashContext, entityUUID)
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, os.ErrNotExist) {
				// Skip cache check
				h.ServeHTTP(w, r)

				return
			}

			if err != nil {
				logger.WithError(err).Error("Failed to receive last-modification date")
				errorHandler(hashContext, w, r, ErrReceivingMeta)

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
