package turtleware

import (
	"github.com/sirupsen/logrus"

	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

type ListStaticDataFunc func(ctx context.Context, paging Paging) ([]interface{}, error)
type ListSQLDataFunc func(ctx context.Context, paging Paging) (*sql.Rows, error)

type ResourceDataFunc func(ctx context.Context, entityUUID string) (interface{}, error)
type SQLResourceFunc func(ctx context.Context, r *sql.Rows) (interface{}, error)

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
			logger.WithError(err).Error("Error while receiving rows")
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
			logger.WithError(err).Error("Error while receiving rows")
			errorHandler(dataContext, w, r, ErrReceivingResults)

			return
		}

		// Ensure row close, even on error
		defer func() {
			if err := rows.Close(); err != nil {
				logger.WithError(err).Warn("Failed to close row scanner")
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
			logger.WithError(err).Error("Error while receiving results")

			return nil, ErrReceivingResults
		}

		results = append(results, tempEntity)
	}

	// Log, but don't act on the error
	if err := rows.Err(); err != nil {
		logger.WithError(err).Error("Error while receiving results")
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
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, os.ErrNotExist) {
			errorHandler(dataContext, w, r, ErrResourceNotFound)

			return
		}

		if err != nil {
			logger.WithError(err).Error("Error while receiving results")
			errorHandler(dataContext, w, r, ErrReceivingResults)

			return
		}

		if reader, ok := tempEntity.(io.Reader); ok {
			logger.Trace("Streaming response for resource request")
			StreamResponse(reader, w, r, errorHandler)
		} else {
			logger.Trace("Assembling response for resource request")
			EmissioneWriter.Write(w, r, http.StatusOK, tempEntity)
		}
	})
}

func StreamResponse(reader io.Reader, w http.ResponseWriter, r *http.Request, errorHandler ErrorHandlerFunc) {
	logger := logrus.WithContext(r.Context())

	if readCloser, ok := reader.(io.ReadCloser); ok {
		defer func() {
			if err := readCloser.Close(); err != nil {
				logger.WithError(err).Error("Error closing reader")
			}
		}()
	}

	// Only the first 512 bytes are used to sniff the content type.
	buffer := make([]byte, 512)
	headerRead, err := reader.Read(buffer)

	if err != nil {
		errorHandler(
			r.Context(),
			w, r,
			fmt.Errorf("error while trying to read content type: %w", err),
		)

		return
	}

	w.Header().Set("Content-Type", http.DetectContentType(buffer))
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(buffer[:headerRead]); err != nil {
		// Worst-case - we already send the header and potentially
		// some content, but something went wrong in between.
		logger.WithError(err).Error("Fatal error while streaming data")

		return
	}

	// Copy all that is left in the pipe
	if _, err := io.Copy(w, reader); err != nil {
		// Worst-case - we already send the header and potentially
		// some content, but something went wrong in between.
		logger.WithError(err).Error("Fatal error while streaming data")

		return
	}
}
