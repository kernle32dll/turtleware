package tenant

import (
	"github.com/kernle32dll/turtleware"

	"context"
	"mime/multipart"
	"net/http"
)

type MultipartHandleFunc func(ctx context.Context, tenantUUID, entityUUID, userUUID string, part *multipart.Part) error

func MultipartMiddleware(partHandlerFunc MultipartHandleFunc, errorHandler turtleware.ErrorHandlerFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uploadContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			tenantUUID, err := UUIDFromRequestContext(uploadContext)
			if err != nil {
				errorHandler(uploadContext, w, r, err)
				return
			}

			if err := turtleware.HandleMultipartUpload(uploadContext, r, func(ctx context.Context, entityUUID, userUUID string, part *multipart.Part) error {
				return partHandlerFunc(ctx, tenantUUID, entityUUID, userUUID, part)
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
