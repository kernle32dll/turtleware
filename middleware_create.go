package turtleware

import (
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"context"
	"encoding/json"
	"net/http"
)

type CreateFunc[T CreateDTO] func(ctx context.Context, entityUUID, userUUID uuid.UUID, create T) error

type CreateDTO interface {
	Validate() []error
}

// IsHandledByDefaultCreateErrorHandler indicates if the DefaultCreateErrorHandler has any special
// handling for the given error, or if it defaults to handing it out as-is.
func IsHandledByDefaultCreateErrorHandler(err error) bool {
	return IsHandledByDefaultErrorHandler(err)
}

func DefaultCreateErrorHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	DefaultErrorHandler(ctx, w, r, err)
}

func ResourceCreateMiddleware[T CreateDTO](createFunc CreateFunc[T], errorHandler ErrorHandlerFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			createContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			logger := zerolog.Ctx(createContext)

			userUUID, err := UserUUIDFromRequestContext(createContext)
			if err != nil {
				errorHandler(createContext, w, r, err)

				return
			}

			entityUUID, err := EntityUUIDFromRequestContext(createContext)
			if err != nil {
				errorHandler(createContext, w, r, err)

				return
			}

			// ----------------

			var create T
			if err := json.NewDecoder(r.Body).Decode(&create); err != nil {
				errorHandler(createContext, w, r, ErrMarshalling)

				return
			}

			if validationErrors := create.Validate(); len(validationErrors) > 0 {
				errorHandler(createContext, w, r, &ValidationWrapperError{validationErrors})

				return
			}

			if err := createFunc(createContext, entityUUID, userUUID, create); err != nil {
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
