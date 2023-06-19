package tenant

import (
	"github.com/google/uuid"
	"github.com/kernle32dll/turtleware"

	"context"
	"mime/multipart"
	"net/http"
)

type FileHandleFunc func(ctx context.Context, tenantUUID, entityUUID, userUUID uuid.UUID, fileName string, file multipart.File) error

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

			if err := turtleware.HandleFileUpload(uploadContext, r, func(ctx context.Context, entityUUID, userUUID uuid.UUID, fileName string, file multipart.File) error {
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
