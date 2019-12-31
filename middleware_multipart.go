package turtleware

import (
	"github.com/sirupsen/logrus"

	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
)

var (
	ErrParsingContentTypeHeader       = errors.New("error parsing Content-Type header")
	ErrInvalidContentTypeForMultipart = errors.New("only Content-Type multipart/form-data is allowed")
	ErrMultipartBoundaryMissing       = errors.New("boundary parameter in Content-Type missing")
)

type MultipartHandleFunc func(ctx context.Context, entityUUID, userUUID string, part *multipart.Part) error

func DefaultMultipartErrorHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	if err == ErrParsingContentTypeHeader ||
		err == ErrInvalidContentTypeForMultipart ||
		err == ErrMultipartBoundaryMissing {
		WriteError(w, r, http.StatusBadRequest, err)
	} else {
		DefaultErrorHandler(ctx, w, r, err)
	}
}

func MultipartMiddleware(partHandlerFunc MultipartHandleFunc, errorHandler ErrorHandlerFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uploadContext, cancel := context.WithCancel(r.Context())
			defer cancel()

			if err := HandleMultipartUpload(uploadContext, r, partHandlerFunc); err != nil {
				errorHandler(uploadContext, w, r, err)
				return
			}

			if next != nil {
				next.ServeHTTP(w, r)
			}
		})
	}
}

func HandleMultipartUpload(ctx context.Context, r *http.Request, partHandlerFunc MultipartHandleFunc) error {
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

	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		logger.Errorf("%s: %s", ErrParsingContentTypeHeader, err)
		return ErrParsingContentTypeHeader
	}

	if !strings.HasPrefix(mediaType, "multipart/form-data") {
		return ErrInvalidContentTypeForMultipart
	}

	boundary := params["boundary"]
	if boundary == "" {
		return ErrMultipartBoundaryMissing
	}

	mr := multipart.NewReader(r.Body, boundary)
	i := 0
	for {
		part, err := mr.NextPart()
		if err != nil {
			if err == io.EOF {
				break
			}

			return err
		}

		if err := partHandlerFunc(ctx, entityUUID, userUUID, part); err != nil {
			logger.Errorf("Multipart %d handling failed: %s", i, err)
			return err
		}

		i++
	}

	return nil
}
