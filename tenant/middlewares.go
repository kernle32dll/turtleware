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
type ListCountFunc func(ctx context.Context, tenantUUID string, paging turtleware.Paging) (uint, uint, error)
type ListStaticDataFunc func(ctx context.Context, tenantUUID string, paging turtleware.Paging) ([]map[string]interface{}, error)
type ListSQLDataFunc func(ctx context.Context, tenantUUID string, paging turtleware.Paging) (*sql.Rows, error)

type ResourceLastModFunc func(ctx context.Context, tenantUUID string, entityUUID string) (time.Time, error)
type ResourceDataFunc func(ctx context.Context, tenantUUID string, entityUUID string) (interface{}, error)

func StaticListDataHandler(dataFetcher ListStaticDataFunc, errorHandler turtleware.ErrorHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logrus.Trace("Bailing out of tenant based list request because of HEAD method")
			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		tenantUUID, err := UUIDFromRequestContext(r.Context())
		if err != nil {
			errorHandler(dataContext, w, r, err)
			return
		}

		paging, err := turtleware.PagingFromRequestContext(r.Context())
		if err != nil {
			errorHandler(dataContext, w, r, err)
			return
		}

		logrus.Trace("Handling request for tenant based resource list request")
		rows, err := dataFetcher(dataContext, tenantUUID, paging)
		if err != nil {
			logrus.Errorf("Error while receiving rows: %s", err)
			errorHandler(dataContext, w, r, turtleware.ErrReceivingResults)
			return
		}

		logrus.Trace("Assembling response for tenant based resource list request")
		turtleware.EmissioneWriter.Write(w, r, http.StatusOK, rows)
	})
}

func SQLListDataHandler(dataFetcher ListSQLDataFunc, dataTransformer turtleware.SQLResourceFunc, errorHandler turtleware.ErrorHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logrus.Trace("Bailing out of tenant list request because of HEAD method")
			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		tenantUUID, err := UUIDFromRequestContext(r.Context())
		if err != nil {
			errorHandler(dataContext, w, r, err)
			return
		}

		paging, err := turtleware.PagingFromRequestContext(r.Context())
		if err != nil {
			errorHandler(dataContext, w, r, err)
			return
		}

		rows, err := dataFetcher(dataContext, tenantUUID, paging)
		if err != nil {
			logrus.Errorf("Error while receiving rows: %s", err)
			errorHandler(dataContext, w, r, turtleware.ErrReceivingResults)
			return
		}

		// Ensure row close, even on error
		defer func() {
			if err := rows.Close(); err != nil {
				logrus.Warnf("Failed to close row scanner: %s", err)
			}
		}()

		results, err := bufferSQLResults(r.Context(), rows, dataTransformer)
		if err != nil {
			errorHandler(dataContext, w, r, turtleware.ErrReceivingResults)
			return
		}

		turtleware.EmissioneWriter.Write(w, r, http.StatusOK, results)
	})
}

func bufferSQLResults(ctx context.Context, rows *sql.Rows, dataTransformer turtleware.SQLResourceFunc) ([]interface{}, error) {
	dataContext, cancel := context.WithCancel(ctx)
	defer cancel()

	var results []interface{}
	for rows.Next() {
		tempEntity, err := dataTransformer(dataContext, rows)
		if err != nil {
			logrus.Errorf("Error while receiving results: %s", err)
			return nil, turtleware.ErrReceivingResults
		}

		results = append(results, tempEntity)
	}

	// Log, but don't act on the error
	if err := rows.Err(); err != nil {
		logrus.Errorf("Error while receiving results: %s", err)
	}

	return results, nil
}

func ResourceDataHandler(dataFetcher ResourceDataFunc, errorHandler turtleware.ErrorHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logrus.Trace("Bailing out of tenant based resource request because of HEAD method")
			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		tenantUUID, err := UUIDFromRequestContext(r.Context())
		if err != nil {
			errorHandler(dataContext, w, r, err)
			return
		}

		entityUUID, err := turtleware.EntityUUIDFromRequestContext(r.Context())
		if err != nil {
			errorHandler(dataContext, w, r, err)
			return
		}

		tempEntity, err := dataFetcher(dataContext, tenantUUID, entityUUID)
		if err == sql.ErrNoRows {
			errorHandler(dataContext, w, r, turtleware.ErrResourceNotFound)
			return
		}

		if err != nil {
			logrus.Errorf("Error while receiving rows: %s", err)
			errorHandler(dataContext, w, r, turtleware.ErrReceivingResults)
			return
		}

		logrus.Trace("Assembling response for tenant based resource request")
		turtleware.EmissioneWriter.Write(w, r, http.StatusOK, tempEntity)
	})
}

func CountHeaderMiddleware(countFetcher ListCountFunc, errorHandler turtleware.ErrorHandlerFunc) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			countContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			tenantUUID, err := UUIDFromRequestContext(r.Context())
			if err != nil {
				errorHandler(countContext, w, r, err)
				return
			}

			paging, err := turtleware.PagingFromRequestContext(r.Context())
			if err != nil {
				errorHandler(countContext, w, r, err)
				return
			}

			totalCount, count, err := countFetcher(countContext, tenantUUID, paging)
			if err != nil {
				logrus.Errorf("Failed to receive count: %s", err)
				errorHandler(countContext, w, r, turtleware.ErrReceivingMeta)
				return
			}

			w.Header().Set("X-Count", fmt.Sprintf("%d", count))
			w.Header().Set("X-Total-Count", fmt.Sprintf("%d", totalCount))

			h.ServeHTTP(w, r)
		})
	}
}

func ListCacheMiddleware(hashFetcher ListHashFunc, errorHandler turtleware.ErrorHandlerFunc) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logrus.Trace("Handling preflight for tenant based resource list request")

			etag, _ := turtleware.ExtractCacheHeader(r)

			if etag != "" {
				logrus.Debugf("Received If-None-Match tag %s", etag)
			}

			hashContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			tenantUUID, err := UUIDFromRequestContext(r.Context())
			if err != nil {
				errorHandler(hashContext, w, r, err)
				return
			}

			paging, err := turtleware.PagingFromRequestContext(r.Context())
			if err != nil {
				errorHandler(hashContext, w, r, err)
				return
			}

			hash, err := hashFetcher(hashContext, tenantUUID, paging)
			if err != nil {
				logrus.Errorf("Failed to receive hash: %s", err)
				errorHandler(hashContext, w, r, turtleware.ErrReceivingMeta)
				return
			}

			w.Header().Set("Etag", hash)

			cacheHit := etag == hash
			if cacheHit {
				logrus.Debug("Successful cache hit")
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
			logrus.Trace("Handling preflight for tenant based resource request")

			_, lastModified := turtleware.ExtractCacheHeader(r)

			if lastModified.Valid {
				logrus.Debugf("Received If-Modified-Since date %s", lastModified.Time)
			}

			hashContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			tenantUUID, err := UUIDFromRequestContext(r.Context())
			if err != nil {
				errorHandler(hashContext, w, r, err)
				return
			}

			entityUUID, err := turtleware.EntityUUIDFromRequestContext(r.Context())
			if err != nil {
				errorHandler(hashContext, w, r, err)
				return
			}

			maxModDate, err := lastModFetcher(hashContext, tenantUUID, entityUUID)
			if err == sql.ErrNoRows {
				errorHandler(hashContext, w, r, turtleware.ErrResourceNotFound)
				return
			}

			if err != nil {
				logrus.Errorf("Failed to receive last-modification date: %s", err)
				errorHandler(hashContext, w, r, turtleware.ErrReceivingMeta)
				return
			}

			w.Header().Set("Last-Modified", maxModDate.Format(time.RFC1123))

			cacheHit := lastModified.Valid && maxModDate.Truncate(time.Second).Equal(lastModified.Time.Truncate(time.Second))
			if cacheHit {
				logrus.Debug("Successful cache hit")
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
