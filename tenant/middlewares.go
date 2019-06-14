package tenant

import (
	"github.com/kernle32dll/turtleware"
	"github.com/sirupsen/logrus"

	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type ctxKey int

const (
	// CtxTenantUUID is the context key used to pass down the tenant UUID.
	CtxTenantUUID ctxKey = iota
)

var (
	// ErrContextMissingTenantUUID is an internal error indicating a missing
	// tenant UUID in the request context, whereas one was expected.
	ErrContextMissingTenantUUID = errors.New("missing tenant UUID in context")

	// ErrTokenMissingTenantUUID indicates that a requested was
	// missing the tenant uuid.
	ErrTokenMissingTenantUUID = errors.New("token does not include tenant uuid")
)

type ListHashFunc func(ctx context.Context, r *http.Request, tenantUUID string, paging turtleware.Paging) (string, error)
type ListCountFunc func(ctx context.Context, r *http.Request, tenantUUID string, paging turtleware.Paging) (uint, uint, error)
type ListStaticDataFunc func(ctx context.Context, r *http.Request, tenantUUID string, paging turtleware.Paging) ([]map[string]interface{}, error)
type ListSQLDataFunc func(ctx context.Context, r *http.Request, tenantUUID string, paging turtleware.Paging) (*sql.Rows, error)

type ResourceLastModFunc func(ctx context.Context, r *http.Request, tenantUUID string, entityUUID string) (time.Time, error)
type ResourceDataFunc func(ctx context.Context, r *http.Request, tenantUUID string, entityUUID string) (interface{}, error)

const bufferErrorMessage = "Error while buffering response output: %s"

func StaticListDataHandler(dataFetcher ListStaticDataFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logrus.Trace("Bailing out of tenant based list request because of HEAD method")
			return
		}

		tenantUUID, err := UUIDFromRequestContext(r.Context())
		if err != nil {
			turtleware.WriteError(w, r, http.StatusInternalServerError, err)
			return
		}

		paging, err := turtleware.PagingFromRequestContext(r.Context())
		if err != nil {
			turtleware.WriteError(w, r, http.StatusInternalServerError, err)
			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		logrus.Trace("Handling request for tenant based resource list request")
		rows, err := dataFetcher(dataContext, r, tenantUUID, paging)
		if err != nil {
			logrus.Errorf("Error while receiving rows: %s", err)
			turtleware.WriteError(w, r, http.StatusInternalServerError, turtleware.ErrReceivingResults)
			return
		}

		logrus.Trace("Assembling response for tenant based resource list request")
		buffer := bytes.Buffer{}
		buffer.WriteString("[\n")

		if len(rows) > 0 {
			for i := 0; i < len(rows); i++ {
				buffer.WriteString("  ")
				pagesJSON, err := json.MarshalIndent(rows[i], "  ", "  ")
				if err != nil {
					turtleware.WriteError(w, r, http.StatusInternalServerError, err)
					return
				}

				buffer.Write(pagesJSON)

				if i < (len(rows) - 1) {
					buffer.WriteString(",\n")
				} else {
					break
				}
			}
		}

		buffer.WriteString("\n]")
		buffer.WriteTo(w)
	})
}

func SQLListDataHandler(dataFetcher ListSQLDataFunc, dataTransformer turtleware.SQLResourceFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logrus.Trace("Bailing out of tenant list request because of HEAD method")
			return
		}

		tenantUUID, err := UUIDFromRequestContext(r.Context())
		if err != nil {
			turtleware.WriteError(w, r, http.StatusInternalServerError, err)
			return
		}

		paging, err := turtleware.PagingFromRequestContext(r.Context())
		if err != nil {
			turtleware.WriteError(w, r, http.StatusInternalServerError, err)
			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		rows, err := dataFetcher(dataContext, r, tenantUUID, paging)
		if err != nil {
			logrus.Errorf("Error while receiving rows: %s", err)
			turtleware.WriteError(w, r, http.StatusInternalServerError, turtleware.ErrReceivingResults)
			return
		}

		// Ensure row close, even on error
		defer func() {
			if err := rows.Close(); err != nil {
				logrus.Warnf("Failed to close row scanner: %s", err)
			}
		}()

		buffer, err := bufferSQLResults(r.Context(), rows, dataTransformer)
		if err != nil {
			turtleware.WriteError(w, r, http.StatusInternalServerError, turtleware.ErrReceivingResults)
			return
		}

		if _, err := buffer.WriteTo(w); err != nil {
			logrus.Errorf("Error while writing marshaled response: %s", err)
		}
	})
}

func bufferSQLResults(ctx context.Context, rows *sql.Rows, dataTransformer turtleware.SQLResourceFunc) (bytes.Buffer, error) {
	dataContext, cancel := context.WithCancel(ctx)
	defer cancel()

	buffer := bytes.Buffer{}

	// Array open
	if _, err := buffer.WriteString("[\n"); err != nil {
		logrus.Warnf(bufferErrorMessage, err)
		return bytes.Buffer{}, turtleware.ErrMarshalling
	}

	if rows.Next() {
		for {
			tempEntity, err := dataTransformer(dataContext, rows)
			if err != nil {
				logrus.Errorf("Error while receiving results: %s", err)
				return bytes.Buffer{}, turtleware.ErrReceivingResults
			}

			// Element indent
			if _, err := buffer.WriteString("  "); err != nil {
				logrus.Warnf(bufferErrorMessage, err)
				return bytes.Buffer{}, turtleware.ErrMarshalling
			}

			// Marshal entity
			pagesJSON, err := json.MarshalIndent(tempEntity, "  ", "  ")
			if err != nil {
				logrus.Warnf(bufferErrorMessage, err)
				return bytes.Buffer{}, turtleware.ErrMarshalling
			}

			// Element
			if _, err := buffer.Write(pagesJSON); err != nil {
				logrus.Warnf(bufferErrorMessage, err)
				return bytes.Buffer{}, turtleware.ErrMarshalling
			}

			if rows.Next() {
				// Element separator
				if _, err := buffer.WriteString(",\n"); err != nil {
					logrus.Warnf(bufferErrorMessage, err)
					return bytes.Buffer{}, turtleware.ErrMarshalling
				}
			} else {
				break
			}
		}
	}

	// Log, but don't act on the error
	if err := rows.Err(); err != nil {
		logrus.Errorf("Error while receiving results: %s", err)
	}

	// Array close
	if _, err := buffer.WriteString("\n]"); err != nil {
		logrus.Warnf(bufferErrorMessage, err)
		return bytes.Buffer{}, turtleware.ErrMarshalling
	}

	return buffer, nil
}

func ResourceDataHandler(dataFetcher ResourceDataFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logrus.Trace("Bailing out of tenant based resource request because of HEAD method")
			return
		}

		tenantUUID, err := UUIDFromRequestContext(r.Context())
		if err != nil {
			turtleware.WriteError(w, r, http.StatusInternalServerError, err)
			return
		}

		entityUUID, err := turtleware.EntityUUIDFromRequestContext(r.Context())
		if err != nil {
			turtleware.WriteError(w, r, http.StatusInternalServerError, err)
			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		tempEntity, err := dataFetcher(dataContext, r, tenantUUID, entityUUID)
		if err == sql.ErrNoRows {
			turtleware.WriteError(w, r, http.StatusNotFound, turtleware.ErrResourceNotFound)
			return
		}

		if err != nil {
			logrus.Errorf("Error while receiving rows: %s", err)
			turtleware.WriteError(w, r, http.StatusInternalServerError, turtleware.ErrReceivingResults)
			return
		}

		logrus.Trace("Assembling response for tenant based resource request")
		pagesJSON, err := json.MarshalIndent(tempEntity, "", "  ")
		if err != nil {
			turtleware.WriteError(w, r, http.StatusInternalServerError, turtleware.ErrMarshalling)
			return
		}

		if _, err := w.Write(pagesJSON); err != nil {
			logrus.Errorf("Error while writing marshaled response: %s", err)
		}
	})
}

func CountHeaderMiddleware(countFetcher ListCountFunc) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantUUID, err := UUIDFromRequestContext(r.Context())
			if err != nil {
				turtleware.WriteError(w, r, http.StatusInternalServerError, err)
				return
			}

			paging, err := turtleware.PagingFromRequestContext(r.Context())
			if err != nil {
				turtleware.WriteError(w, r, http.StatusInternalServerError, err)
				return
			}

			countContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			totalCount, count, err := countFetcher(countContext, r, tenantUUID, paging)
			if err != nil {
				logrus.Errorf("Failed to receive count: %s", err)
				turtleware.WriteError(w, r, http.StatusInternalServerError, turtleware.ErrReceivingMeta)
				return
			}

			w.Header().Set("X-Count", fmt.Sprintf("%d", count))
			w.Header().Set("X-Total-Count", fmt.Sprintf("%d", totalCount))

			h.ServeHTTP(w, r)
		})
	}
}

func ListCacheMiddleware(hashFetcher ListHashFunc) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logrus.Trace("Handling preflight for tenant based resource list request")

			etag, _ := turtleware.ExtractCacheHeader(r)

			if etag != "" {
				logrus.Debugf("Received If-None-Match tag %s", etag)
			}

			tenantUUID, err := UUIDFromRequestContext(r.Context())
			if err != nil {
				turtleware.WriteError(w, r, http.StatusInternalServerError, err)
				return
			}

			paging, err := turtleware.PagingFromRequestContext(r.Context())
			if err != nil {
				turtleware.WriteError(w, r, http.StatusInternalServerError, err)
				return
			}

			hashContext, cancel := context.WithCancel(r.Context())
			defer cancel()
			hash, err := hashFetcher(hashContext, r, tenantUUID, paging)
			if err != nil {
				logrus.Errorf("Failed to receive hash: %s", err)
				turtleware.WriteError(w, r, http.StatusInternalServerError, turtleware.ErrReceivingMeta)
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

func ResourceCacheMiddleware(lastModFetcher ResourceLastModFunc) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logrus.Trace("Handling preflight for tenant based resource request")

			_, lastModified := turtleware.ExtractCacheHeader(r)

			if lastModified.Valid {
				logrus.Debugf("Received If-Modified-Since date %s", lastModified.Time)
			}

			tenantUUID, err := UUIDFromRequestContext(r.Context())
			if err != nil {
				turtleware.WriteError(w, r, http.StatusInternalServerError, err)
				return
			}

			entityUUID, err := turtleware.EntityUUIDFromRequestContext(r.Context())
			if err != nil {
				turtleware.WriteError(w, r, http.StatusInternalServerError, err)
				return
			}

			hashContext, cancel := context.WithCancel(r.Context())
			defer cancel()
			maxModDate, err := lastModFetcher(hashContext, r, tenantUUID, entityUUID)
			if err == sql.ErrNoRows {
				turtleware.WriteError(w, r, http.StatusNotFound, turtleware.ErrResourceNotFound)
				return
			}

			if err != nil {
				logrus.Errorf("Failed to receive last-modification date: %s", err)
				turtleware.WriteError(w, r, http.StatusInternalServerError, turtleware.ErrReceivingMeta)
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

// AuthMiddleware is a http middleware for checking tenant authentication details, and
// passing down the tenant UUID if existing, or bailing out otherwise.
func AuthMiddleware(keys []interface{}) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := turtleware.AuthTokenFromRequestContext(r.Context())
			if err != nil {
				turtleware.WriteError(w, r, http.StatusInternalServerError, err)
				return
			}

			claims, err := turtleware.ValidateToken(token, keys)
			if err != nil {
				turtleware.WriteError(w, r, http.StatusBadRequest, err)
				return
			}

			tenantUUID, ok := claims["tenant_uuid"].(string)
			if !ok || tenantUUID == "" {
				turtleware.WriteError(w, r, http.StatusBadRequest, ErrTokenMissingTenantUUID)
				return
			}

			h.ServeHTTP(
				w,
				r.WithContext(context.WithValue(r.Context(), CtxTenantUUID, tenantUUID)),
			)
		})
	}
}

func UUIDFromRequestContext(ctx context.Context) (string, error) {
	tenantUUID, ok := ctx.Value(CtxTenantUUID).(string)
	if !ok {
		return "", ErrContextMissingTenantUUID
	}

	return tenantUUID, nil
}
