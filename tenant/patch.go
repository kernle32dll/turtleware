package tenant

import (
	"github.com/kernle32dll/turtleware"
	"github.com/sirupsen/logrus"

	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

var (
	// ErrMissingUserUUID signals that an received JWT did not contain an user UUID.
	ErrMissingUserUUID = errors.New("token does not include user uuid")

	ErrUnmodifiedSinceHeaderMissing = errors.New("If-Unmodified-Since header missing")
	ErrUnmodifiedSinceHeaderInvalid = errors.New("received If-Unmodified-Since header in invalid format")
	ErrNoChanges                    = errors.New("patch request did not contain any changes")
)

type PatchFunc func(ctx context.Context, tenantUUID, entityUUID, userUUID string, patch PatchDTO, ifUnmodifiedSince time.Time) error

type PatchDTOProviderFunc func() PatchDTO

type ValidationWrapperError struct {
	Errors []error
}

func (ValidationWrapperError ValidationWrapperError) Error() string {
	errorStrings := make([]string, len(ValidationWrapperError.Errors))

	for i, err := range ValidationWrapperError.Errors {
		errorStrings[i] = err.Error()
	}

	return strings.Join(errorStrings, ", ")
}

type PatchDTO interface {
	HasChanges() bool
	Validate() []error
}

func DefaultPatchErrorHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	if err == ErrMissingUserUUID ||
		err == ErrUnmodifiedSinceHeaderInvalid ||
		err == ErrNoChanges ||
		err == turtleware.ErrMarshalling {
		turtleware.WriteError(w, r, http.StatusBadRequest, err)
	} else if err == ErrUnmodifiedSinceHeaderMissing {
		turtleware.WriteError(w, r, http.StatusPreconditionRequired, err)
	} else if validationError, ok := err.(ValidationWrapperError); ok {
		turtleware.WriteError(w, r, http.StatusBadRequest, validationError.Errors...)
	} else {
		turtleware.DefaultErrorHandler(ctx, w, r, err)
	}
}

func ResourcePatchMiddleware(patchDTOProviderFunc PatchDTOProviderFunc, patchFunc PatchFunc, errorHandler turtleware.ErrorHandlerFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			patchContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			tenantUUID, err := UUIDFromRequestContext(patchContext)
			if err != nil {
				errorHandler(patchContext, w, r, err)
				return
			}

			claims, err := turtleware.AuthClaimsFromRequestContext(patchContext)
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

			userUUID := claims["uuid"].(string)
			if userUUID == "" {
				errorHandler(patchContext, w, r, ErrMissingUserUUID)
				return
			}

			patch := patchDTOProviderFunc()
			if err := json.NewDecoder(r.Body).Decode(patch); err != nil {
				errorHandler(patchContext, w, r, turtleware.ErrMarshalling)
				return
			}

			if !patch.HasChanges() {
				errorHandler(patchContext, w, r, ErrNoChanges)
				return
			}

			if validationErrors := patch.Validate(); len(validationErrors) > 0 {
				errorHandler(patchContext, w, r, ValidationWrapperError{validationErrors})
				return
			}

			ifUnmodifiedSinceHeader := r.Header.Get("If-Unmodified-Since")
			if ifUnmodifiedSinceHeader == "" {
				errorHandler(patchContext, w, r, ErrUnmodifiedSinceHeaderMissing)
				return
			}

			ifUnmodifiedSince, err := parseTimeByFormats(ifUnmodifiedSinceHeader, time.RFC1123, time.RFC3339)
			if err != nil {
				errorHandler(patchContext, w, r, ErrUnmodifiedSinceHeaderInvalid)
				return
			}

			if err := patchFunc(patchContext, tenantUUID, entityUUID, userUUID, patch, ifUnmodifiedSince); err != nil {
				logrus.Errorf("Patch failed: %s", err)
				errorHandler(patchContext, w, r, err)
				return
			}

			if next != nil {
				next.ServeHTTP(w, r)
			}
		})
	}
}

func parseTimeByFormats(value string, layouts ...string) (time.Time, error) {
	for _, layout := range layouts {
		if parsedValue, err := time.Parse(layout, value); err == nil {
			return parsedValue, nil
		}
	}

	return time.Time{}, errors.New("no layout matched")
}
