package tenant

import (
	"github.com/kernle32dll/turtleware"
	"github.com/rs/zerolog"

	"context"
	"encoding/json"
	"net/http"
	"time"
)

type PatchFunc[T turtleware.PatchDTO] func(ctx context.Context, tenantUUID, entityUUID, userUUID string, patch T, ifUnmodifiedSince time.Time) error

func ResourcePatchMiddleware[T turtleware.PatchDTO](patchFunc PatchFunc[T], errorHandler turtleware.ErrorHandlerFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			patchContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			logger := zerolog.Ctx(patchContext)

			tenantUUID, err := UUIDFromRequestContext(patchContext)
			if err != nil {
				errorHandler(patchContext, w, r, err)
				return
			}

			userUUID, err := turtleware.UserUUIDFromRequestContext(patchContext)
			if err != nil {
				errorHandler(patchContext, w, r, err)
				return
			}

			entityUUID, err := turtleware.EntityUUIDFromRequestContext(patchContext)
			if err != nil {
				errorHandler(patchContext, w, r, err)
				return
			}

			// ----------------

			var patch T
			if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
				errorHandler(patchContext, w, r, turtleware.ErrMarshalling)
				return
			}

			if !patch.HasChanges() {
				errorHandler(patchContext, w, r, turtleware.ErrNoChanges)
				return
			}

			if validationErrors := patch.Validate(); len(validationErrors) > 0 {
				errorHandler(patchContext, w, r, &turtleware.ValidationWrapperError{Errors: validationErrors})
				return
			}

			ifUnmodifiedSince, err := turtleware.GetIfUnmodifiedSince(r)
			if err != nil {
				errorHandler(patchContext, w, r, err)
				return
			}

			if err := patchFunc(patchContext, tenantUUID, entityUUID, userUUID, patch, ifUnmodifiedSince); err != nil {
				logger.Error().Err(err).Msg("Patch failed")
				errorHandler(patchContext, w, r, err)
				return
			}

			if next != nil {
				next.ServeHTTP(w, r)
			}
		})
	}
}
