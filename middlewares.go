package turtleware

import (
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"github.com/sirupsen/logrus"

	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type ListHashFunc func(ctx context.Context, paging Paging) (string, error)
type ListCountFunc func(ctx context.Context, paging Paging) (uint, uint, error)
type ListStaticDataFunc func(ctx context.Context, paging Paging) ([]interface{}, error)
type ListSQLDataFunc func(ctx context.Context, paging Paging) (*sql.Rows, error)

type ResourceLastModFunc func(ctx context.Context, entityUUID string) (time.Time, error)
type ResourceDataFunc func(ctx context.Context, entityUUID string) (interface{}, error)

type ErrorHandlerFunc func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)

func DefaultErrorHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	if span := opentracing.SpanFromContext(ctx); span != nil {
		ext.Error.Set(span, true)
		span.LogFields(
			log.Object("event", "error"),
			log.Object("error.object", err),
		)
	}

	if err == ErrResourceNotFound {
		WriteError(w, r, http.StatusNotFound, err)
	} else {
		WriteError(w, r, http.StatusInternalServerError, err)
	}
}

func StaticListDataHandler(dataFetcher ListStaticDataFunc, errorHandler ErrorHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := logrus.WithContext(r.Context())

		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logger.Trace("Bailing out of list request because of HEAD method")
			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		paging, err := PagingFromRequestContext(dataContext)
		if err != nil {
			errorHandler(dataContext, w, r, err)
			return
		}

		logger.Trace("Handling request for resource list request")
		rows, err := dataFetcher(dataContext, paging)
		if err != nil {
			logger.Errorf("Error while receiving rows: %s", err)
			errorHandler(dataContext, w, r, ErrReceivingResults)
			return
		}

		if rows == nil {
			rows = make([]interface{}, 0)
		}

		logger.Trace("Assembling response for resource list request")
		EmissioneWriter.Write(w, r, http.StatusOK, rows)
	})
}

func SQLListDataHandler(dataFetcher ListSQLDataFunc, dataTransformer SQLResourceFunc, errorHandler ErrorHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := logrus.WithContext(r.Context())

		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logger.Trace("Bailing out of list request because of HEAD method")
			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		paging, err := PagingFromRequestContext(dataContext)
		if err != nil {
			errorHandler(dataContext, w, r, err)
			return
		}

		rows, err := dataFetcher(dataContext, paging)
		if err != nil {
			logger.Errorf("Error while receiving rows: %s", err)
			errorHandler(dataContext, w, r, ErrReceivingResults)
			return
		}

		// Ensure row close, even on error
		defer func() {
			if err := rows.Close(); err != nil {
				logger.Warnf("Failed to close row scanner: %s", err)
			}
		}()

		results, err := bufferSQLResults(dataContext, rows, dataTransformer)
		if err != nil {
			errorHandler(dataContext, w, r, ErrReceivingResults)
			return
		}

		EmissioneWriter.Write(w, r, http.StatusOK, results)
	})
}

func bufferSQLResults(ctx context.Context, rows *sql.Rows, dataTransformer SQLResourceFunc) ([]interface{}, error) {
	dataContext, cancel := context.WithCancel(ctx)
	defer cancel()

	logger := logrus.WithContext(dataContext)

	results := make([]interface{}, 0)
	for rows.Next() {
		tempEntity, err := dataTransformer(dataContext, rows)
		if err != nil {
			logger.Errorf("Error while receiving results: %s", err)
			return nil, ErrReceivingResults
		}

		results = append(results, tempEntity)
	}

	// Log, but don't act on the error
	if err := rows.Err(); err != nil {
		logger.Errorf("Error while receiving results: %s", err)
	}

	return results, nil
}

func ResourceDataHandler(dataFetcher ResourceDataFunc, errorHandler ErrorHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := logrus.WithContext(r.Context())

		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logger.Trace("Bailing out of resource request because of HEAD method")
			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		entityUUID, err := EntityUUIDFromRequestContext(dataContext)
		if err != nil {
			errorHandler(dataContext, w, r, err)
			return
		}

		tempEntity, err := dataFetcher(dataContext, entityUUID)
		if err == sql.ErrNoRows || err == os.ErrNotExist {
			errorHandler(dataContext, w, r, ErrResourceNotFound)
			return
		}

		if err != nil {
			logger.Errorf("Error while receiving results: %s", err)
			errorHandler(dataContext, w, r, ErrReceivingResults)
			return
		}

		if reader, ok := tempEntity.(io.ReadCloser); ok {
			logger.Trace("Streaming response for resource request")
			StreamResponse(reader, w, r, errorHandler)
		} else {
			logger.Trace("Assembling response for resource request")
			EmissioneWriter.Write(w, r, http.StatusOK, tempEntity)
		}
	})
}

func CountHeaderMiddleware(countFetcher ListCountFunc, errorHandler ErrorHandlerFunc) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			countContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			logger := logrus.WithContext(countContext)

			paging, err := PagingFromRequestContext(countContext)
			if err != nil {
				errorHandler(countContext, w, r, err)
				return
			}

			totalCount, count, err := countFetcher(countContext, paging)
			if err != nil {
				logger.Errorf("Failed to receive count: %s", err)
				errorHandler(countContext, w, r, ErrReceivingMeta)
				return
			}

			w.Header().Set("X-Count", fmt.Sprintf("%d", count))
			w.Header().Set("X-Total-Count", fmt.Sprintf("%d", totalCount))

			h.ServeHTTP(w, r)
		})
	}
}

func ListCacheMiddleware(hashFetcher ListHashFunc, errorHandler ErrorHandlerFunc) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := logrus.WithContext(r.Context())

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
				logger.Errorf("Failed to receive hash: %s", err)
				errorHandler(hashContext, w, r, ErrReceivingMeta)
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
			if err == sql.ErrNoRows {
				errorHandler(hashContext, w, r, ErrResourceNotFound)
				return
			}

			if err != nil {
				logger.Errorf("Failed to receive last-modification date: %s", err)
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

func StreamResponse(reader io.ReadCloser, w http.ResponseWriter, r *http.Request, errorHandler ErrorHandlerFunc) {
	logger := logrus.WithContext(r.Context())
	defer func() {
		if err := reader.Close(); err != nil {
			logger.Errorf("Error closing reader: %s", err)
		}
	}()

	tee := io.TeeReader(reader, w)

	// Only the first 512 bytes are used to sniff the content type.
	buffer := make([]byte, 512)
	if _, err := tee.Read(buffer); err != nil {
		errorHandler(
			r.Context(),
			w, r,
			fmt.Errorf("error while trying to read content type: %w", err),
		)
		return
	}

	w.Header().Set("Content-Type", http.DetectContentType(buffer))
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, tee); err != nil {
		// Worst-case - we already send the header and potentially
		// some content, but something went wrong in between.
		logger.Errorf("Fatal error while streaming data: %s", err)
	}
}
