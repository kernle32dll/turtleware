package turtleware

import (
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

var (
	ErrUnmodifiedSinceHeaderMissing = errors.New("If-Unmodified-Since header missing")
	ErrUnmodifiedSinceHeaderInvalid = errors.New("received If-Unmodified-Since header in invalid format")
	ErrNoChanges                    = errors.New("patch request did not contain any changes")
	ErrNoDateTimeLayoutMatched      = errors.New("no date time layout matched")
)

type PatchFunc[T PatchDTO] func(ctx context.Context, entityUUID, userUUID uuid.UUID, patch T, ifUnmodifiedSince time.Time) error

type PatchDTO interface {
	HasChanges() bool
	Validate() []error
}

// IsHandledByDefaultPatchErrorHandler indicates if the DefaultPatchErrorHandler has any special
// handling for the given error, or if it defaults to handing it out as-is.
func IsHandledByDefaultPatchErrorHandler(err error) bool {
	return errors.Is(err, ErrUnmodifiedSinceHeaderInvalid) ||
		errors.Is(err, ErrNoChanges) ||
		errors.Is(err, ErrUnmodifiedSinceHeaderMissing) ||
		IsHandledByDefaultErrorHandler(err)
}

func DefaultPatchErrorHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ErrUnmodifiedSinceHeaderInvalid) || errors.Is(err, ErrNoChanges) {
		WriteError(ctx, w, r, http.StatusBadRequest, err)
		return
	}

	if errors.Is(err, ErrUnmodifiedSinceHeaderMissing) {
		WriteError(ctx, w, r, http.StatusPreconditionRequired, err)
		return
	}

	DefaultErrorHandler(ctx, w, r, err)
}

func ResourcePatchMiddleware[T PatchDTO](patchFunc PatchFunc[T], errorHandler ErrorHandlerFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			patchContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			logger := zerolog.Ctx(patchContext)

			userUUID, err := UserUUIDFromRequestContext(patchContext)
			if err != nil {
				errorHandler(patchContext, w, r, err)
				return
			}

			entityUUID, err := EntityUUIDFromRequestContext(patchContext)
			if err != nil {
				errorHandler(patchContext, w, r, err)
				return
			}

			// ----------------

			var patch T
			if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
				errorHandler(patchContext, w, r, ErrMarshalling)
				return
			}

			if !patch.HasChanges() {
				errorHandler(patchContext, w, r, ErrNoChanges)
				return
			}

			if validationErrors := patch.Validate(); len(validationErrors) > 0 {
				errorHandler(patchContext, w, r, &ValidationWrapperError{validationErrors})
				return
			}

			ifUnmodifiedSince, err := GetIfUnmodifiedSince(r)
			if err != nil {
				errorHandler(patchContext, w, r, err)
				return
			}

			if err := patchFunc(patchContext, entityUUID, userUUID, patch, ifUnmodifiedSince); err != nil {
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

// GetIfUnmodifiedSince tries to parse the last modification (If-Modified-Since) header from
// a given request. It tries the following formats (in that order):
//
// - time.RFC1123
// - time.RFC3339Nano
// - time.RFC3339
func GetIfUnmodifiedSince(r *http.Request) (time.Time, error) {
	ifUnmodifiedSinceHeader := r.Header.Get("If-Unmodified-Since")
	if ifUnmodifiedSinceHeader == "" {
		return time.Time{}, ErrUnmodifiedSinceHeaderMissing
	}

	ifUnmodifiedSince, err := parseTimeByFormats(ifUnmodifiedSinceHeader, time.RFC1123, time.RFC3339Nano, time.RFC3339)
	if err != nil {
		return time.Time{}, errors.Join(ErrUnmodifiedSinceHeaderInvalid, err)
	}

	return ifUnmodifiedSince, nil
}

func parseTimeByFormats(value string, layouts ...string) (time.Time, error) {
	for _, layout := range layouts {
		if parsedValue, err := time.Parse(layout, value); err == nil {
			return parsedValue, nil
		}
	}

	return time.Time{}, ErrNoDateTimeLayoutMatched
}
