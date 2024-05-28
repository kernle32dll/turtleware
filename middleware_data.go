package turtleware

import (
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"

	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

// ListStaticDataFunc is a function for retrieving a slice of data, scoped to the provided paging.
type ListStaticDataFunc[T any] func(ctx context.Context, paging Paging) ([]T, error)

// ListSQLDataFunc is a function for retrieving a sql.Rows iterator, scoped to the provided paging.
type ListSQLDataFunc func(ctx context.Context, paging Paging) (*sql.Rows, error)

// ListSQLxDataFunc is a function for retrieving a sqlx.Rows iterator, scoped to the provided paging.
type ListSQLxDataFunc func(ctx context.Context, paging Paging) (*sqlx.Rows, error)

// ResourceDataFunc is a function for retrieving a single resource via its UUID.
type ResourceDataFunc[T any] func(ctx context.Context, entityUUID string) (T, error)

// SQLResourceFunc is a function for scanning a single row from a sql.Rows iterator, and transforming it into a struct type.
type SQLResourceFunc[T any] func(ctx context.Context, r *sql.Rows) (T, error)

// SQLxResourceFunc is a function for scanning a single row from a sqlx.Rows iterator, and transforming it into a struct type.
type SQLxResourceFunc[T any] func(ctx context.Context, r *sqlx.Rows) (T, error)

// StaticListDataHandler is a handler for serving a list of resources from a static list.
// Data is retrieved from the given ListStaticDataFunc, and then serialized to the http.ResponseWriter.
// Errors encountered during the process are passed to the provided ErrorHandlerFunc.
func StaticListDataHandler[T any](dataFetcher ListStaticDataFunc[T], errorHandler ErrorHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := zerolog.Ctx(r.Context())

		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logger.Trace().Msgf("Bailing out of list request because of HEAD method")

			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		paging, err := PagingFromRequestContext(dataContext)
		if err != nil {
			errorHandler(dataContext, w, r, err)

			return
		}

		logger.Trace().Msgf("Handling request for resource list request")
		rows, err := dataFetcher(dataContext, paging)
		if err != nil {
			logger.Error().Err(err).Msg("Error while receiving rows")
			errorHandler(dataContext, w, r, ErrReceivingResults)

			return
		}

		if rows == nil {
			rows = make([]T, 0)
		}

		logger.Trace().Msg("Assembling response for resource list request")
		EmissioneWriter.Write(w, r, http.StatusOK, rows)
	})
}

// SQLListDataHandler is a handler for serving a list of resources from a SQL source.
// Data is retrieved via a sql.Rows iterator retrieved from the given ListSQLDataFunc,
// scanned into a struct via the SQLResourceFunc, and then serialized to the http.ResponseWriter.
// Serialization is buffered, so the entire result set is read before writing the response.
// Errors encountered during the process are passed to the provided ErrorHandlerFunc.
func SQLListDataHandler[T any](dataFetcher ListSQLDataFunc, dataTransformer SQLResourceFunc[T], errorHandler ErrorHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := zerolog.Ctx(r.Context())

		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logger.Trace().Msg("Bailing out of list request because of HEAD method")

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
			logger.Error().Err(err).Msg("Error while receiving rows")
			errorHandler(dataContext, w, r, ErrReceivingResults)

			return
		}

		// Ensure row close, even on error
		defer func() {
			if err := rows.Close(); err != nil {
				logger.Warn().Err(err).Msg("Failed to close row scanner")
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

func bufferSQLResults[T any](ctx context.Context, rows *sql.Rows, dataTransformer SQLResourceFunc[T]) ([]T, error) {
	dataContext, cancel := context.WithCancel(ctx)
	defer cancel()

	logger := zerolog.Ctx(dataContext)

	results := make([]T, 0)

	for rows.Next() {
		tempEntity, err := dataTransformer(dataContext, rows)
		if err != nil {
			logger.Error().Err(err).Msg("Error while receiving results")

			return nil, ErrReceivingResults
		}

		results = append(results, tempEntity)
	}

	// Log, but don't act on the error
	if err := rows.Err(); err != nil {
		logger.Error().Err(err).Msg("Error while receiving results")
	}

	return results, nil
}

// SQLxListDataHandler is a handler for serving a list of resources from a SQL source via sqlx.
// Data is retrieved via a sqlx.Rows iterator retrieved from the given ListSQLxDataFunc,
// scanned into a struct via the SQLxResourceFunc, and then serialized to the http.ResponseWriter.
// Serialization is buffered, so the entire result set is read before writing the response.
// Errors encountered during the process are passed to the provided ErrorHandlerFunc.
func SQLxListDataHandler[T any](dataFetcher ListSQLxDataFunc, dataTransformer SQLxResourceFunc[T], errorHandler ErrorHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := zerolog.Ctx(r.Context())

		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logger.Trace().Msg("Bailing out of list request because of HEAD method")

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
			logger.Error().Err(err).Msg("Error while receiving rows")
			errorHandler(dataContext, w, r, ErrReceivingResults)

			return
		}

		// Ensure row close, even on error
		defer func() {
			if err := rows.Close(); err != nil {
				logger.Warn().Err(err).Msg("Failed to close row scanner")
			}
		}()

		results, err := bufferSQLxResults(dataContext, rows, dataTransformer)
		if err != nil {
			errorHandler(dataContext, w, r, ErrReceivingResults)

			return
		}

		EmissioneWriter.Write(w, r, http.StatusOK, results)
	})
}

func bufferSQLxResults[T any](ctx context.Context, rows *sqlx.Rows, dataTransformer SQLxResourceFunc[T]) ([]T, error) {
	dataContext, cancel := context.WithCancel(ctx)
	defer cancel()

	logger := zerolog.Ctx(dataContext)

	results := make([]T, 0)

	for rows.Next() {
		tempEntity, err := dataTransformer(dataContext, rows)
		if err != nil {
			logger.Error().Err(err).Msg("Error while receiving results")

			return nil, ErrReceivingResults
		}

		results = append(results, tempEntity)
	}

	// Log, but don't act on the error
	if err := rows.Err(); err != nil {
		logger.Error().Err(err).Msg("Error while receiving results")
	}

	return results, nil
}

// ResourceDataHandler is a handler for serving a single resource. Data is retrieved from the
// given ResourceDataFunc, and then serialized to the http.ResponseWriter.
// If the response is an io.Reader, the response is streamed to the client via StreamResponse.
// Otherwise, the entire result set is read before writing the response.
// Errors encountered during the process are passed to the provided ErrorHandlerFunc.
func ResourceDataHandler[T any](dataFetcher ResourceDataFunc[T], errorHandler ErrorHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := zerolog.Ctx(r.Context())

		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logger.Trace().Msg("Bailing out of resource request because of HEAD method")

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
			logger.Error().Err(err).Msg("Error while receiving results")
			errorHandler(dataContext, w, r, ErrReceivingResults)

			return
		}

		if reader, ok := any(tempEntity).(io.Reader); ok {
			logger.Trace().Msg("Streaming response for resource request")
			StreamResponse(reader, w, r, errorHandler)
		} else {
			logger.Trace().Msg("Assembling response for resource request")
			EmissioneWriter.Write(w, r, http.StatusOK, tempEntity)
		}
	})
}

// StreamResponse streams the provided io.Reader to the http.ResponseWriter. The function
// tries to determine the content type of the stream by reading the first 512 bytes, and sets
// the content-type HTTP header accordingly.
// Errors encountered during the process are passed to the provided ErrorHandlerFunc.
func StreamResponse(reader io.Reader, w http.ResponseWriter, r *http.Request, errorHandler ErrorHandlerFunc) {
	logger := zerolog.Ctx(r.Context())

	if readCloser, ok := reader.(io.ReadCloser); ok {
		defer func() {
			if err := readCloser.Close(); err != nil {
				logger.Error().Err(err).Msg("Error closing reader")
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
		logger.Error().Err(err).Msg("Fatal error while streaming data")

		return
	}

	// Copy all that is left in the pipe
	if _, err := io.Copy(w, reader); err != nil {
		// Worst-case - we already send the header and potentially
		// some content, but something went wrong in between.
		logger.Error().Err(err).Msg("Fatal error while streaming data")

		return
	}
}
