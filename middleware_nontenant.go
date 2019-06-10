package turtleware

import (
	"github.com/sirupsen/logrus"

	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ListHashFunc func(ctx context.Context, paging Paging) (string, error)
type ListCountFunc func(ctx context.Context, paging Paging) (uint, uint, error)
type ListStaticDataFunc func(ctx context.Context, paging Paging) ([]map[string]interface{}, error)
type ListSQLDataFunc func(ctx context.Context, paging Paging) (*sql.Rows, error)

type ResourceLastModFunc func(ctx context.Context, entityUUID string) (time.Time, error)
type ResourceDataFunc func(ctx context.Context, entityUUID string) (interface{}, error)

func StaticListDataHandler(dataFetcher ListStaticDataFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logrus.Trace("Bailing out of list request because of HEAD method")
			return
		}

		paging, err := PagingFromRequestContext(r.Context())
		if err != nil {
			WriteError(w, r, err, http.StatusInternalServerError)
			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		logrus.Trace("Handling request for resource list request")
		rows, err := dataFetcher(dataContext, paging)
		if err != nil {
			logrus.Errorf("Error while receiving rows: %s", err)
			WriteError(w, r, ErrReceivingResults, http.StatusInternalServerError)
			return
		}

		logrus.Trace("Assembling response for resource list request")
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

func SQLListDataHandler(dataFetcher ListSQLDataFunc, dataTransformer SQLResourceFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logrus.Trace("Bailing out of tenant list request because of HEAD method")
			return
		}

		paging, err := PagingFromRequestContext(r.Context())
		if err != nil {
			WriteError(w, r, err, http.StatusInternalServerError)
			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		rows, err := dataFetcher(dataContext, paging)
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

func ResourceDataHandler(dataFetcher ResourceDataFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logrus.Trace("Bailing out of resource request because of HEAD method")
			return
		}

		entityUUID, err := EntityUUIDFromRequestContext(r.Context())
		if err != nil {
			WriteError(w, r, err, http.StatusInternalServerError)
			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		tempEntity, err := dataFetcher(dataContext, entityUUID)
		if err == sql.ErrNoRows {
			WriteError(w, r, ErrResourceNotFound, http.StatusNotFound)
			return
		}

		if err != nil {
			logrus.Errorf("Error while receiving rows: %s", err)
			WriteError(w, r, ErrReceivingResults, http.StatusInternalServerError)
			return
		}

		logrus.Trace("Assembling response for resource request")
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

func CountHeaderMiddleware(countFetcher ListCountFunc) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			paging, err := PagingFromRequestContext(r.Context())
			if err != nil {
				WriteError(w, r, err, http.StatusInternalServerError)
				return
			}

			countContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			totalCount, count, err := countFetcher(countContext, paging)
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

func ListCacheMiddleware(hashFetcher ListHashFunc) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logrus.Trace("Handling preflight for resource list request")

			etag, _ := ExtractCacheHeader(r)

			if etag != "" {
				logrus.Debugf("Received If-None-Match tag %s", etag)
			}

			paging, err := PagingFromRequestContext(r.Context())
			if err != nil {
				WriteError(w, r, err, http.StatusInternalServerError)
				return
			}

			hashContext, cancel := context.WithCancel(r.Context())
			defer cancel()
			hash, err := hashFetcher(hashContext, paging)
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

func ResourceCacheMiddleware(lastModFetcher ResourceLastModFunc) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logrus.Trace("Handling preflight for resource request")

			_, lastModified := ExtractCacheHeader(r)

			if lastModified.Valid {
				logrus.Debugf("Received If-Modified-Since date %s", lastModified.Time)
			}

			entityUUID, err := EntityUUIDFromRequestContext(r.Context())
			if err != nil {
				WriteError(w, r, err, http.StatusInternalServerError)
				return
			}

			hashContext, cancel := context.WithCancel(r.Context())
			defer cancel()
			maxModDate, err := lastModFetcher(hashContext, entityUUID)
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

// AuthMiddleware is a http middleware for checking authentication details, and
// bailing out if it cant be validated.
func AuthMiddleware(keys []interface{}) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := AuthTokenFromRequestContext(r.Context())
			if err != nil {
				WriteError(w, r, err, http.StatusInternalServerError)
				return
			}

			if _, err := ValidateToken(token, keys); err != nil {
				WriteError(w, r, err, http.StatusBadRequest)
				return
			}

			h.ServeHTTP(w, r)
		})
	}
}
