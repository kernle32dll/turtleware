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

type ListStaticDataFunc func(ctx context.Context, paging Paging) ([]interface{}, error)
type ListSQLDataFunc func(ctx context.Context, paging Paging) (*sql.Rows, error)
type ListSQLxDataFunc func(ctx context.Context, paging Paging) (*sqlx.Rows, error)

type ResourceDataFunc func(ctx context.Context, entityUUID string) (interface{}, error)
type SQLResourceFunc func(ctx context.Context, r *sql.Rows) (interface{}, error)
type SQLxResourceFunc func(ctx context.Context, r *sqlx.Rows) (interface{}, error)

func StaticListDataHandler(dataFetcher ListStaticDataFunc, errorHandler ErrorHandlerFunc) http.Handler {
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
			rows = make([]interface{}, 0)
		}

		logger.Trace().Msg("Assembling response for resource list request")
		EmissioneWriter.Write(w, r, http.StatusOK, rows)
	})
}

func SQLListDataHandler(dataFetcher ListSQLDataFunc, dataTransformer SQLResourceFunc, errorHandler ErrorHandlerFunc) http.Handler {
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

func bufferSQLResults(ctx context.Context, rows *sql.Rows, dataTransformer SQLResourceFunc) ([]interface{}, error) {
	dataContext, cancel := context.WithCancel(ctx)
	defer cancel()

	logger := zerolog.Ctx(dataContext)

	results := make([]interface{}, 0)

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

func SQLxListDataHandler(dataFetcher ListSQLxDataFunc, dataTransformer SQLxResourceFunc, errorHandler ErrorHandlerFunc) http.Handler {
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

func bufferSQLxResults(ctx context.Context, rows *sqlx.Rows, dataTransformer SQLxResourceFunc) ([]interface{}, error) {
	dataContext, cancel := context.WithCancel(ctx)
	defer cancel()

	logger := zerolog.Ctx(dataContext)

	results := make([]interface{}, 0)

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

func ResourceDataHandler(dataFetcher ResourceDataFunc, errorHandler ErrorHandlerFunc) http.Handler {
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

		if reader, ok := tempEntity.(io.Reader); ok {
			logger.Trace().Msg("Streaming response for resource request")
			StreamResponse(reader, w, r, errorHandler)
		} else {
			logger.Trace().Msg("Assembling response for resource request")
			EmissioneWriter.Write(w, r, http.StatusOK, tempEntity)
		}
	})
}

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
