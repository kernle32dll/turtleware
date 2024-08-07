package tenant

import (
	"github.com/jmoiron/sqlx"
	"github.com/kernle32dll/turtleware"
	"github.com/rs/zerolog"

	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"os"
)

// ListStaticDataFunc is a function for retrieving a slice of data, scoped to the provided tenant and paging.
type ListStaticDataFunc[T any] func(ctx context.Context, tenantUUID string, paging turtleware.Paging) ([]T, error)

// ListSQLDataFunc is a function for retrieving a sql.Rows iterator, scoped to the provided tenant and paging.
type ListSQLDataFunc func(ctx context.Context, tenantUUID string, paging turtleware.Paging) (*sql.Rows, error)

// ListSQLxDataFunc is a function for retrieving a sqlx.Rows iterator, scoped to the provided tenant and paging.
type ListSQLxDataFunc func(ctx context.Context, tenantUUID string, paging turtleware.Paging) (*sqlx.Rows, error)

// ResourceDataFunc is a function for retrieving a single tenant scoped resource via its UUID.
type ResourceDataFunc[T any] func(ctx context.Context, tenantUUID string, entityUUID string) (T, error)

// StaticListDataHandler is a handler for serving a list of tenant scoped resources from a static list.
// Data is retrieved from the given ListStaticDataFunc, and then serialized to the http.ResponseWriter.
// Errors encountered during the process are passed to the provided turtleware.ErrorHandlerFunc.
func StaticListDataHandler[T any](dataFetcher ListStaticDataFunc[T], errorHandler turtleware.ErrorHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := zerolog.Ctx(r.Context())

		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logger.Trace().Msg("Bailing out of tenant based list request because of HEAD method")
			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		tenantUUID, err := UUIDFromRequestContext(dataContext)
		if err != nil {
			errorHandler(dataContext, w, r, err)
			return
		}

		paging, err := turtleware.PagingFromRequestContext(dataContext)
		if err != nil {
			errorHandler(dataContext, w, r, err)
			return
		}

		logger.Trace().Msg("Handling request for tenant based resource list request")
		rows, err := dataFetcher(dataContext, tenantUUID, paging)
		if err != nil {
			logger.Error().Err(err).Msg("Error while receiving rows")
			errorHandler(dataContext, w, r, turtleware.ErrReceivingResults)
			return
		}

		if rows == nil {
			rows = make([]T, 0)
		}

		logger.Trace().Msg("Assembling response for tenant based resource list request")
		turtleware.EmissioneWriter.Write(w, r, http.StatusOK, rows)
	})
}

// SQLListDataHandler is a handler for serving a list of tenant scoped resources from a SQL source.
// Data is retrieved via a sql.Rows iterator retrieved from the given ListSQLDataFunc,
// scanned into a struct via the turtleware.SQLxResourceFunc, and then serialized to the http.ResponseWriter.
// Serialization is buffered, so the entire result set is read before writing the response.
// Errors encountered during the process are passed to the provided turtleware.ErrorHandlerFunc.
func SQLListDataHandler[T any](dataFetcher ListSQLDataFunc, dataTransformer turtleware.SQLResourceFunc[T], errorHandler turtleware.ErrorHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := zerolog.Ctx(r.Context())

		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logger.Trace().Msg("Bailing out of tenant list request because of HEAD method")
			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		tenantUUID, err := UUIDFromRequestContext(dataContext)
		if err != nil {
			errorHandler(dataContext, w, r, err)
			return
		}

		paging, err := turtleware.PagingFromRequestContext(dataContext)
		if err != nil {
			errorHandler(dataContext, w, r, err)
			return
		}

		rows, err := dataFetcher(dataContext, tenantUUID, paging)
		if err != nil {
			logger.Error().Err(err).Msg("Error while receiving rows")
			errorHandler(dataContext, w, r, turtleware.ErrReceivingResults)
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
			errorHandler(dataContext, w, r, turtleware.ErrReceivingResults)
			return
		}

		turtleware.EmissioneWriter.Write(w, r, http.StatusOK, results)
	})
}

func bufferSQLResults[T any](ctx context.Context, rows *sql.Rows, dataTransformer turtleware.SQLResourceFunc[T]) ([]T, error) {
	dataContext, cancel := context.WithCancel(ctx)
	defer cancel()

	logger := zerolog.Ctx(dataContext)

	results := make([]T, 0)

	for rows.Next() {
		tempEntity, err := dataTransformer(dataContext, rows)
		if err != nil {
			logger.Error().Err(err).Msg("Error while receiving results")
			return nil, turtleware.ErrReceivingResults
		}

		results = append(results, tempEntity)
	}

	// Log, but don't act on the error
	if err := rows.Err(); err != nil {
		logger.Error().Err(err).Msg("Error while receiving results")
	}

	return results, nil
}

// SQLxListDataHandler is a handler for serving a list of tenant scoped resources from a SQL source via sqlx.
// Data is retrieved via a sqlx.Rows iterator retrieved from the given ListSQLxDataFunc,
// scanned into a struct via the turtleware.SQLxResourceFunc, and then serialized to the http.ResponseWriter.
// Serialization is buffered, so the entire result set is read before writing the response.
// Errors encountered during the process are passed to the provided turtleware.ErrorHandlerFunc.
func SQLxListDataHandler[T any](dataFetcher ListSQLxDataFunc, dataTransformer turtleware.SQLxResourceFunc[T], errorHandler turtleware.ErrorHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := zerolog.Ctx(r.Context())

		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logger.Trace().Msg("Bailing out of tenant list request because of HEAD method")
			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		tenantUUID, err := UUIDFromRequestContext(dataContext)
		if err != nil {
			errorHandler(dataContext, w, r, err)
			return
		}

		paging, err := turtleware.PagingFromRequestContext(dataContext)
		if err != nil {
			errorHandler(dataContext, w, r, err)
			return
		}

		rows, err := dataFetcher(dataContext, tenantUUID, paging)
		if err != nil {
			logger.Error().Err(err).Msg("Error while receiving rows")
			errorHandler(dataContext, w, r, turtleware.ErrReceivingResults)
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
			errorHandler(dataContext, w, r, turtleware.ErrReceivingResults)
			return
		}

		turtleware.EmissioneWriter.Write(w, r, http.StatusOK, results)
	})
}

func bufferSQLxResults[T any](ctx context.Context, rows *sqlx.Rows, dataTransformer turtleware.SQLxResourceFunc[T]) ([]T, error) {
	dataContext, cancel := context.WithCancel(ctx)
	defer cancel()

	logger := zerolog.Ctx(dataContext)

	results := make([]T, 0)

	for rows.Next() {
		tempEntity, err := dataTransformer(dataContext, rows)
		if err != nil {
			logger.Error().Err(err).Msg("Error while receiving results")
			return nil, turtleware.ErrReceivingResults
		}

		results = append(results, tempEntity)
	}

	// Log, but don't act on the error
	if err := rows.Err(); err != nil {
		logger.Error().Err(err).Msg("Error while receiving results")
	}

	return results, nil
}

// ResourceDataHandler is a handler for serving a single tenant scoped resource. Data is retrieved from the
// given ResourceDataFunc, and then serialized to the http.ResponseWriter.
// If the response is an io.Reader, the response is streamed to the client via turtleware.StreamResponse.
// Otherwise, the entire result set is read before writing the response.
// Errors encountered during the process are passed to the provided turtleware.ErrorHandlerFunc.
func ResourceDataHandler[T any](dataFetcher ResourceDataFunc[T], errorHandler turtleware.ErrorHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := zerolog.Ctx(r.Context())

		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logger.Trace().Msg("Bailing out of tenant based resource request because of HEAD method")
			return
		}

		dataContext, cancel := context.WithCancel(r.Context())
		defer cancel()

		tenantUUID, err := UUIDFromRequestContext(dataContext)
		if err != nil {
			errorHandler(dataContext, w, r, err)
			return
		}

		entityUUID, err := turtleware.EntityUUIDFromRequestContext(dataContext)
		if err != nil {
			errorHandler(dataContext, w, r, err)
			return
		}

		tempEntity, err := dataFetcher(dataContext, tenantUUID, entityUUID)
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, os.ErrNotExist) {
			errorHandler(dataContext, w, r, turtleware.ErrResourceNotFound)
			return
		}

		if err != nil {
			logger.Error().Err(err).Msg("Error while receiving results")
			errorHandler(dataContext, w, r, turtleware.ErrReceivingResults)
			return
		}

		if reader, ok := any(tempEntity).(io.Reader); ok {
			logger.Trace().Msg("Streaming response for tenant based resource request")
			turtleware.StreamResponse(reader, w, r, errorHandler)
		} else {
			logger.Trace().Msg("Assembling response for tenant based resource request")
			turtleware.EmissioneWriter.Write(w, r, http.StatusOK, tempEntity)
		}
	})
}
