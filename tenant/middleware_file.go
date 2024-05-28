package tenant

import (
	"github.com/kernle32dll/turtleware"

	"context"
	"mime/multipart"
	"net/http"
)

// FileHandleFunc is a function that handles a single tenant scoped file upload.
type FileHandleFunc func(ctx context.Context, tenantUUID, entityUUID, userUUID string, fileName string, file multipart.File) error

// FileUploadMiddleware is a middleware that handles uploads of one or multiple files.
// Uploads are parsed from the request via turtleware.HandleFileUpload, and then passed to the provided FileHandleFunc.
// Errors encountered during the process are passed to the provided turtleware.ErrorHandlerFunc.
func FileUploadMiddleware(partHandlerFunc FileHandleFunc, errorHandler turtleware.ErrorHandlerFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uploadContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			tenantUUID, err := UUIDFromRequestContext(uploadContext)
			if err != nil {
				errorHandler(uploadContext, w, r, err)
				return
			}

			if err := turtleware.HandleFileUpload(uploadContext, r, func(ctx context.Context, entityUUID, userUUID string, fileName string, file multipart.File) error {
				return partHandlerFunc(ctx, tenantUUID, entityUUID, userUUID, fileName, file)
			}); err != nil {
				errorHandler(uploadContext, w, r, err)
				return
			}

			if next != nil {
				next.ServeHTTP(w, r)
			}
		})
	}
}
