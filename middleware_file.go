package turtleware

import (
	"github.com/rs/zerolog"

	"context"
	"errors"
	"mime/multipart"
	"net/http"
)

// FileHandleFunc is a function that handles a single file upload.
type FileHandleFunc func(ctx context.Context, entityUUID, userUUID string, fileName string, file multipart.File) error

// IsHandledByDefaultFileUploadErrorHandler indicates if the DefaultFileUploadErrorHandler has any special
// handling for the given error, or if it defaults to handing it out as-is.
func IsHandledByDefaultFileUploadErrorHandler(err error) bool {
	return errors.Is(err, http.ErrNotMultipart) ||
		errors.Is(err, http.ErrMissingBoundary) ||
		errors.Is(err, multipart.ErrMessageTooLarge) ||
		IsHandledByDefaultErrorHandler(err)
}

// DefaultFileUploadErrorHandler is a default error handler, which sensibly handles errors known by turtleware.
func DefaultFileUploadErrorHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, http.ErrNotMultipart) ||
		errors.Is(err, http.ErrMissingBoundary) ||
		errors.Is(err, multipart.ErrMessageTooLarge) {
		WriteError(ctx, w, r, http.StatusBadRequest, err)
		return
	}

	DefaultErrorHandler(ctx, w, r, err)
}

// FileUploadMiddleware is a middleware that handles uploads of one or multiple files.
// Uploads are parsed from the request via HandleFileUpload, and then passed to the provided FileHandleFunc.
// Errors encountered during the process are passed to the provided ErrorHandlerFunc.
func FileUploadMiddleware(fileHandleFunc FileHandleFunc, errorHandler ErrorHandlerFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uploadContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			if err := HandleFileUpload(uploadContext, r, fileHandleFunc); err != nil {
				errorHandler(uploadContext, w, r, err)

				return
			}

			if next != nil {
				next.ServeHTTP(w, r)
			}
		})
	}
}

// HandleFileUpload is a helper function for handling file uploads.
// It parses upload metadata from the request, and then calls the provided FileHandleFunc for each file part.
// Errors encountered during the process are passed to the caller.
func HandleFileUpload(ctx context.Context, r *http.Request, fileHandleFunc FileHandleFunc) error {
	logger := zerolog.Ctx(ctx)

	userUUID, err := UserUUIDFromRequestContext(ctx)
	if err != nil {
		return err
	}

	entityUUID, err := EntityUUIDFromRequestContext(ctx)
	if err != nil {
		return err
	}

	// ----------------

	mr, err := r.MultipartReader()
	if err != nil {
		return err
	}

	form, err := mr.ReadForm(int64(5 << 20))
	if err != nil {
		return err
	}

	for fieldName, files := range form.File {
		for i, file := range files {
			fileName := file.Filename

			logEntry := logger.With().
				Str("fieldName", fieldName).
				Str("fileName", fileName).
				Int("index", i).
				Logger()

			f, err := file.Open()
			if err != nil {
				return err
			}

			if err := fileHandleFunc(ctx, entityUUID, userUUID, fileName, f); err != nil {
				logEntry.Error().Err(err).Msg("Multipart handling failed")

				if err := f.Close(); err != nil {
					logEntry.Error().Err(err).Msg("Failed to close file handle")
				}

				return err
			}

			if err := f.Close(); err != nil {
				logEntry.Error().Err(err).Msg("Failed to close file handle")
			}
		}
	}

	return nil
}
