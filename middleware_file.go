package turtleware

import (
	"github.com/sirupsen/logrus"

	"context"
	"mime/multipart"
	"net/http"
)

type FileHandleFunc func(ctx context.Context, entityUUID, userUUID string, fileName string, file multipart.File) error

func DefaultFileUploadErrorHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	if err == http.ErrNotMultipart ||
		err == http.ErrMissingBoundary ||
		err == multipart.ErrMessageTooLarge {
		TagContextSpanWithError(ctx, err)
		WriteError(w, r, http.StatusBadRequest, err)
	} else {
		DefaultErrorHandler(ctx, w, r, err)
	}
}

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

func HandleFileUpload(ctx context.Context, r *http.Request, fileHandleFunc FileHandleFunc) error {
	logger := logrus.WithContext(ctx)

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

			logEntry := logger.WithFields(map[string]interface{}{
				"fieldName": fieldName,
				"fileName":  fileName,
				"index":     i,
			})

			f, err := file.Open()
			if err != nil {
				return err
			}

			if err := fileHandleFunc(ctx, entityUUID, userUUID, fileName, f); err != nil {
				logEntry.Errorf("Multipart handling failed: %s", err)

				if err := f.Close(); err != nil {
					logEntry.Errorf("Failed to close file handle")
				}

				return err
			}

			if err := f.Close(); err != nil {
				logEntry.Errorf("Failed to close file handle")
			}
		}
	}

	return nil
}
