package turtleware

import (
	"github.com/sirupsen/logrus"

	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

var (
	ErrUnmodifiedSinceHeaderMissing = errors.New("If-Unmodified-Since header missing")
	ErrUnmodifiedSinceHeaderInvalid = errors.New("received If-Unmodified-Since header in invalid format")
	ErrNoChanges                    = errors.New("patch request did not contain any changes")
)

type PatchFunc func(ctx context.Context, entityUUID, userUUID string, patch PatchDTO, ifUnmodifiedSince time.Time) error

type PatchDTOProviderFunc func() PatchDTO

type ValidationWrapperError struct {
	Errors []error
}

func (validationWrapperError *ValidationWrapperError) Error() string {
	errorStrings := make([]string, len(validationWrapperError.Errors))

	for i, err := range validationWrapperError.Errors {
		errorStrings[i] = err.Error()
	}

	return strings.Join(errorStrings, ", ")
}

type PatchDTO interface {
	HasChanges() bool
	Validate() []error
}

func DefaultPatchErrorHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	if err == ErrUnmodifiedSinceHeaderInvalid ||
		err == ErrNoChanges {
		TagContextSpanWithError(ctx, err)
		WriteError(w, r, http.StatusBadRequest, err)
	} else if err == ErrUnmodifiedSinceHeaderMissing {
		TagContextSpanWithError(ctx, err)
		WriteError(w, r, http.StatusPreconditionRequired, err)
	} else if validationError, ok := err.(*ValidationWrapperError); ok {
		TagContextSpanWithError(ctx, err)
		WriteError(w, r, http.StatusBadRequest, validationError.Errors...)
	} else {
		DefaultErrorHandler(ctx, w, r, err)
	}
}

func ResourcePatchMiddleware(patchDTOProviderFunc PatchDTOProviderFunc, patchFunc PatchFunc, errorHandler ErrorHandlerFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			patchContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			logger := logrus.WithContext(patchContext)

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

			patch := patchDTOProviderFunc()
			if err := json.NewDecoder(r.Body).Decode(patch); err != nil {
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
				logger.Errorf("Patch failed: %s", err)
				errorHandler(patchContext, w, r, err)
				return
			}

			if next != nil {
				next.ServeHTTP(w, r)
			}
		})
	}
}

func GetIfUnmodifiedSince(r *http.Request) (time.Time, error) {
	ifUnmodifiedSinceHeader := r.Header.Get("If-Unmodified-Since")
	if ifUnmodifiedSinceHeader == "" {
		return time.Time{}, ErrUnmodifiedSinceHeaderMissing
	}

	ifUnmodifiedSince, err := parseTimeByFormats(ifUnmodifiedSinceHeader, time.RFC1123, time.RFC3339)
	if err != nil {
		return time.Time{}, ErrUnmodifiedSinceHeaderInvalid
	}

	return ifUnmodifiedSince, nil
}

func parseTimeByFormats(value string, layouts ...string) (time.Time, error) {
	for _, layout := range layouts {
		if parsedValue, err := time.Parse(layout, value); err == nil {
			return parsedValue, nil
		}
	}

	return time.Time{}, errors.New("no layout matched")
}
