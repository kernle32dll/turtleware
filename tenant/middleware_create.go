package tenant

import (
	"github.com/kernle32dll/turtleware"
	"github.com/rs/zerolog"

	"context"
	"encoding/json"
	"net/http"
)

type CreateFunc[T turtleware.CreateDTO] func(ctx context.Context, tenantUUID, entityUUID, userUUID string, create T) error

func ResourceCreateMiddleware[T turtleware.CreateDTO](createFunc CreateFunc[T], errorHandler turtleware.ErrorHandlerFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			createContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			logger := zerolog.Ctx(createContext)

			tenantUUID, err := UUIDFromRequestContext(createContext)
			if err != nil {
				errorHandler(createContext, w, r, err)
				return
			}

			userUUID, err := turtleware.UserUUIDFromRequestContext(createContext)
			if err != nil {
				errorHandler(createContext, w, r, err)
				return
			}

			entityUUID, err := turtleware.EntityUUIDFromRequestContext(createContext)
			if err != nil {
				errorHandler(createContext, w, r, err)
				return
			}

			// ----------------

			var create T
			if err := json.NewDecoder(r.Body).Decode(&create); err != nil {
				errorHandler(createContext, w, r, turtleware.ErrMarshalling)
				return
			}

			if validationErrors := create.Validate(); len(validationErrors) > 0 {
				errorHandler(createContext, w, r, &turtleware.ValidationWrapperError{Errors: validationErrors})
				return
			}

			if err := createFunc(createContext, tenantUUID, entityUUID, userUUID, create); err != nil {
				logger.Error().Err(err).Msg("Create failed")
				errorHandler(createContext, w, r, err)
				return
			}

			if next != nil {
				next.ServeHTTP(w, r)
			}
		})
	}
}
