package server

import (
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

var (
	// CtxTenantUUID is the context key used to pass down the tenant UUID.
	CtxTenantUUID = "tenantUUID"

	// ErrContextMissingTenantUUID is an internal error indicating a missing
	// tenant UUID in the request context, whereas one was expected.
	ErrContextMissingTenantUUID = errors.New("missing tenant UUID in context")

	// ErrTokenMissingTenantUUID indicates that a requested was
	// missing the tenant uuid.
	ErrTokenMissingTenantUUID = errors.New("token does not include tenant uuid")
)

type TenantListHashFunc func(ctx context.Context, tenantUUID string, paging Paging) (string, error)
type TenantListCountFunc func(ctx context.Context, tenantUUID string, paging Paging) (uint, uint, error)
type TenantListStaticDataFunc func(ctx context.Context, tenantUUID string, paging Paging) ([]map[string]interface{}, error)
type TenantListSQLDataFunc func(ctx context.Context, tenantUUID string, paging Paging) (*sql.Rows, error)

type TenantResourceLastModFunc func(ctx context.Context, tenantUUID string, entityUUID string) (time.Time, error)
type TenantResourceDataFunc func(ctx context.Context, tenantUUID string, entityUUID string) (interface{}, error)

func TenantStaticListDataHandler(dataFetcher TenantListStaticDataFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logrus.Trace("Bailing out of tenant based list request because of HEAD method")
			return
		}

		tenantUUID, err := TenantUUIDFromRequestContext(r.Context())
		if err != nil {
			WriteError(w, r, err, http.StatusInternalServerError)
			return
		}

		paging, err := PagingFromRequestContext(r.Context())
		if err != nil {
			WriteError(w, r, err, http.StatusInternalServerError)
			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		logrus.Trace("Handling request for tenant based resource list request")
		rows, err := dataFetcher(dataContext, tenantUUID, paging)
		if err != nil {
			logrus.Errorf("Error while receiving rows: %s", err)
			WriteError(w, r, ErrReceivingResults, http.StatusInternalServerError)
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
					WriteError(w, r, err, http.StatusInternalServerError)
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

func TenantSQLListDataHandler(dataFetcher TenantListSQLDataFunc, dataTransformer SQLResourceFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logrus.Trace("Bailing out of tenant list request because of HEAD method")
			return
		}

		tenantUUID, err := TenantUUIDFromRequestContext(r.Context())
		if err != nil {
			WriteError(w, r, err, http.StatusInternalServerError)
			return
		}

		paging, err := PagingFromRequestContext(r.Context())
		if err != nil {
			WriteError(w, r, err, http.StatusInternalServerError)
			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		rows, err := dataFetcher(dataContext, tenantUUID, paging)
		if err != nil {
			logrus.Errorf("Error while receiving rows: %s", err)
			WriteError(w, r, ErrReceivingResults, http.StatusInternalServerError)
			return
		}

		// Ensure row close, even on error
		defer func() {
			if err := rows.Close(); err != nil {
				logrus.Warnf("Failed to close row scanner: %s", err)
			}
		}()

		buffer := bytes.Buffer{}

		// Array open
		if _, err := buffer.WriteString("[\n"); err != nil {
			logrus.Warnf(bufferErrorMessage, err)
			WriteError(w, r, ErrMarshalling, http.StatusInternalServerError)
			return
		}

		if rows.Next() {
			for {
				tempEntity, err := dataTransformer(dataContext, rows)
				if err != nil {
					logrus.Errorf("Error while receiving results: %s", err)
					WriteError(w, r, ErrReceivingResults, http.StatusInternalServerError)
					return
				}

				// Element indent
				if _, err := buffer.WriteString("  "); err != nil {
					logrus.Warnf(bufferErrorMessage, err)
					WriteError(w, r, ErrMarshalling, http.StatusInternalServerError)
					return
				}

				// Marshal entity
				pagesJSON, err := json.MarshalIndent(tempEntity, "  ", "  ")
				if err != nil {
					logrus.Warnf(bufferErrorMessage, err)
					WriteError(w, r, ErrMarshalling, http.StatusInternalServerError)
					return
				}

				// Element
				if _, err := buffer.Write(pagesJSON); err != nil {
					logrus.Warnf(bufferErrorMessage, err)
					WriteError(w, r, ErrMarshalling, http.StatusInternalServerError)
					return
				}

				if rows.Next() {
					// Element separator
					if _, err := buffer.WriteString(",\n"); err != nil {
						logrus.Warnf(bufferErrorMessage, err)
						WriteError(w, r, ErrMarshalling, http.StatusInternalServerError)
						return
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
			WriteError(w, r, ErrMarshalling, http.StatusInternalServerError)
			return
		}

		if _, err := buffer.WriteTo(w); err != nil {
			logrus.Errorf("Error while writing marshaled response: %s", err)
		}
	})
}

func TenantResourceDataHandler(dataFetcher TenantResourceDataFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logrus.Trace("Bailing out of tenant based resource request because of HEAD method")
			return
		}

		tenantUUID, err := TenantUUIDFromRequestContext(r.Context())
		if err != nil {
			WriteError(w, r, err, http.StatusInternalServerError)
			return
		}

		entityUUID, err := EntityUUIDFromRequestContext(r.Context())
		if err != nil {
			WriteError(w, r, err, http.StatusInternalServerError)
			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		tempEntity, err := dataFetcher(dataContext, tenantUUID, entityUUID)
		if err == sql.ErrNoRows {
			WriteError(w, r, ErrResourceNotFound, http.StatusNotFound)
			return
		}

		if err != nil {
			logrus.Errorf("Error while receiving rows: %s", err)
			WriteError(w, r, ErrReceivingResults, http.StatusInternalServerError)
			return
		}

		logrus.Trace("Assembling response for tenant based resource request")
		pagesJSON, err := json.MarshalIndent(tempEntity, "", "  ")
		if err != nil {
			WriteError(w, r, ErrMarshalling, http.StatusInternalServerError)
			return
		}

		if _, err := w.Write(pagesJSON); err != nil {
			logrus.Errorf("Error while writing marshaled response: %s", err)
		}
	})
}

func TenantCountHeaderMiddleware(countFetcher TenantListCountFunc) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantUUID, err := TenantUUIDFromRequestContext(r.Context())
			if err != nil {
				WriteError(w, r, err, http.StatusInternalServerError)
				return
			}

			paging, err := PagingFromRequestContext(r.Context())
			if err != nil {
				WriteError(w, r, err, http.StatusInternalServerError)
				return
			}

			countContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			totalCount, count, err := countFetcher(countContext, tenantUUID, paging)
			if err != nil {
				logrus.Errorf("Failed to receive count: %s", err)
				WriteError(w, r, ErrReceivingMeta, http.StatusInternalServerError)
				return
			}

			w.Header().Set("X-Count", fmt.Sprintf("%d", count))
			w.Header().Set("X-Total-Count", fmt.Sprintf("%d", totalCount))

			h.ServeHTTP(w, r)
		})
	}
}

func TenantListCacheMiddleware(hashFetcher TenantListHashFunc) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logrus.Trace("Handling preflight for tenant based resource list request")

			etag, _ := ExtractCacheHeader(r)

			if etag != "" {
				logrus.Debugf("Received If-None-Match tag %s", etag)
			}

			tenantUUID, err := TenantUUIDFromRequestContext(r.Context())
			if err != nil {
				WriteError(w, r, err, http.StatusInternalServerError)
				return
			}

			paging, err := PagingFromRequestContext(r.Context())
			if err != nil {
				WriteError(w, r, err, http.StatusInternalServerError)
				return
			}

			hashContext, cancel := context.WithCancel(r.Context())
			defer cancel()
			hash, err := hashFetcher(hashContext, tenantUUID, paging)
			if err != nil {
				logrus.Errorf("Failed to receive hash: %s", err)
				WriteError(w, r, ErrReceivingMeta, http.StatusInternalServerError)
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

func TenantResourceCacheMiddleware(lastModFetcher TenantResourceLastModFunc) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logrus.Trace("Handling preflight for tenant based resource request")

			_, lastModified := ExtractCacheHeader(r)

			if lastModified.Valid {
				logrus.Debugf("Received If-Modified-Since date %s", lastModified.Time)
			}

			tenantUUID, err := TenantUUIDFromRequestContext(r.Context())
			if err != nil {
				WriteError(w, r, err, http.StatusInternalServerError)
				return
			}

			entityUUID, err := EntityUUIDFromRequestContext(r.Context())
			if err != nil {
				WriteError(w, r, err, http.StatusInternalServerError)
				return
			}

			hashContext, cancel := context.WithCancel(r.Context())
			defer cancel()
			maxModDate, err := lastModFetcher(hashContext, tenantUUID, entityUUID)
			if err == sql.ErrNoRows {
				WriteError(w, r, ErrResourceNotFound, http.StatusNotFound)
				return
			}

			if err != nil {
				logrus.Errorf("Failed to receive last-modification date: %s", err)
				WriteError(w, r, ErrReceivingMeta, http.StatusInternalServerError)
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

// TenantAuthMiddleware is a http middleware for checking tenant authentication details, and
// passing down the tenant UUID if existing, or bailing out otherwise.
func TenantAuthMiddleware(keys []interface{}) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := AuthTokenFromRequestContext(r.Context())
			if err != nil {
				WriteError(w, r, err, http.StatusInternalServerError)
				return
			}

			claims, err := ValidateToken(token, keys)
			if err != nil {
				WriteError(w, r, err, http.StatusBadRequest)
				return
			}

			tenantUUID, ok := claims["tenant_uuid"].(string)
			if !ok || tenantUUID == "" {
				WriteError(w, r, ErrTokenMissingTenantUUID, http.StatusBadRequest)
				return
			}

			h.ServeHTTP(
				w,
				r.WithContext(context.WithValue(r.Context(), CtxTenantUUID, tenantUUID)),
			)
		})
	}
}

func TenantUUIDFromRequestContext(ctx context.Context) (string, error) {
	tenantUUID, ok := ctx.Value(CtxTenantUUID).(string)
	if !ok {
		return "", ErrContextMissingTenantUUID
	}

	return tenantUUID, nil
}
