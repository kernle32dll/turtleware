package tenant

import (
	"github.com/kernle32dll/turtleware"
	"github.com/sirupsen/logrus"

	"context"
	"database/sql"
	"io"
	"net/http"
	"os"
)

type ListStaticDataFunc func(ctx context.Context, tenantUUID string, paging turtleware.Paging) ([]interface{}, error)
type ListSQLDataFunc func(ctx context.Context, tenantUUID string, paging turtleware.Paging) (*sql.Rows, error)
type ResourceDataFunc func(ctx context.Context, tenantUUID string, entityUUID string) (interface{}, error)

func StaticListDataHandler(dataFetcher ListStaticDataFunc, errorHandler turtleware.ErrorHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := logrus.WithContext(r.Context())

		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logger.Trace("Bailing out of tenant based list request because of HEAD method")
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

		logger.Trace("Handling request for tenant based resource list request")
		rows, err := dataFetcher(dataContext, tenantUUID, paging)
		if err != nil {
			logger.Errorf("Error while receiving rows: %s", err)
			errorHandler(dataContext, w, r, turtleware.ErrReceivingResults)
			return
		}

		if rows == nil {
			rows = make([]interface{}, 0)
		}

		logger.Trace("Assembling response for tenant based resource list request")
		turtleware.EmissioneWriter.Write(w, r, http.StatusOK, rows)
	})
}

func SQLListDataHandler(dataFetcher ListSQLDataFunc, dataTransformer turtleware.SQLResourceFunc, errorHandler turtleware.ErrorHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := logrus.WithContext(r.Context())

		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logger.Trace("Bailing out of tenant list request because of HEAD method")
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
			logger.Errorf("Error while receiving rows: %s", err)
			errorHandler(dataContext, w, r, turtleware.ErrReceivingResults)
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
			errorHandler(dataContext, w, r, turtleware.ErrReceivingResults)
			return
		}

		turtleware.EmissioneWriter.Write(w, r, http.StatusOK, results)
	})
}

func bufferSQLResults(ctx context.Context, rows *sql.Rows, dataTransformer turtleware.SQLResourceFunc) ([]interface{}, error) {
	dataContext, cancel := context.WithCancel(ctx)
	defer cancel()

	logger := logrus.WithContext(dataContext)

	results := make([]interface{}, 0)
	for rows.Next() {
		tempEntity, err := dataTransformer(dataContext, rows)
		if err != nil {
			logger.Errorf("Error while receiving results: %s", err)
			return nil, turtleware.ErrReceivingResults
		}

		results = append(results, tempEntity)
	}

	// Log, but don't act on the error
	if err := rows.Err(); err != nil {
		logger.Errorf("Error while receiving results: %s", err)
	}

	return results, nil
}

func ResourceDataHandler(dataFetcher ResourceDataFunc, errorHandler turtleware.ErrorHandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := logrus.WithContext(r.Context())

		// Only proceed if we are working with an actual request
		if r.Method == http.MethodHead {
			logger.Trace("Bailing out of tenant based resource request because of HEAD method")
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
		if err == sql.ErrNoRows || err == os.ErrNotExist {
			errorHandler(dataContext, w, r, turtleware.ErrResourceNotFound)
			return
		}

		if err != nil {
			logger.Errorf("Error while receiving results: %s", err)
			errorHandler(dataContext, w, r, turtleware.ErrReceivingResults)
			return
		}

		if reader, ok := tempEntity.(io.Reader); ok {
			logger.Trace("Streaming response for tenant based resource request")
			turtleware.StreamResponse(reader, w, r, errorHandler)
		} else {
			logger.Trace("Assembling response for tenant based resource request")
			turtleware.EmissioneWriter.Write(w, r, http.StatusOK, tempEntity)
		}
	})
}
