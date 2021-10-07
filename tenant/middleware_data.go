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

type ListStaticDataFunc func(ctx context.Context, tenantUUID string, paging turtleware.Paging) ([]interface{}, error)
type ListSQLDataFunc func(ctx context.Context, tenantUUID string, paging turtleware.Paging) (*sql.Rows, error)
type ListSQLxDataFunc func(ctx context.Context, tenantUUID string, paging turtleware.Paging) (*sqlx.Rows, error)
type ResourceDataFunc func(ctx context.Context, tenantUUID string, entityUUID string) (interface{}, error)

func StaticListDataHandler(dataFetcher ListStaticDataFunc, errorHandler turtleware.ErrorHandlerFunc) http.Handler {
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
			rows = make([]interface{}, 0)
		}

		logger.Trace().Msg("Assembling response for tenant based resource list request")
		turtleware.EmissioneWriter.Write(w, r, http.StatusOK, rows)
	})
}

func SQLListDataHandler(dataFetcher ListSQLDataFunc, dataTransformer turtleware.SQLResourceFunc, errorHandler turtleware.ErrorHandlerFunc) http.Handler {
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

func bufferSQLResults(ctx context.Context, rows *sql.Rows, dataTransformer turtleware.SQLResourceFunc) ([]interface{}, error) {
	dataContext, cancel := context.WithCancel(ctx)
	defer cancel()

	logger := zerolog.Ctx(dataContext)

	results := make([]interface{}, 0)

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

func SQLxListDataHandler(dataFetcher ListSQLxDataFunc, dataTransformer turtleware.SQLxResourceFunc, errorHandler turtleware.ErrorHandlerFunc) http.Handler {
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

func bufferSQLxResults(ctx context.Context, rows *sqlx.Rows, dataTransformer turtleware.SQLxResourceFunc) ([]interface{}, error) {
	dataContext, cancel := context.WithCancel(ctx)
	defer cancel()

	logger := zerolog.Ctx(dataContext)

	results := make([]interface{}, 0)

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

func ResourceDataHandler(dataFetcher ResourceDataFunc, errorHandler turtleware.ErrorHandlerFunc) http.Handler {
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

		if reader, ok := tempEntity.(io.Reader); ok {
			logger.Trace().Msg("Streaming response for tenant based resource request")
			turtleware.StreamResponse(reader, w, r, errorHandler)
		} else {
			logger.Trace().Msg("Assembling response for tenant based resource request")
			turtleware.EmissioneWriter.Write(w, r, http.StatusOK, tempEntity)
		}
	})
}
