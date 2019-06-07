package turtleware

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
)

var (
	// CtxAuthToken is the context key used to pass down the bearer token.
	CtxAuthToken = "authToken"

	// CtxEntityUUID is the context key used to pass down the entity UUID.
	CtxEntityUUID = "entityUUID"

	// CtxPaging is the context key used to pass down paging information.
	CtxPaging = "paging"

	// ErrContextMissingAuthToken is an internal error indicating a missing
	// auth token in the request context, whereas one was expected.
	ErrContextMissingAuthToken = errors.New("missing auth token in context")

	// ErrContextMissingEntityUUID is an internal error indicating a missing
	// entity UUID in the request context, whereas one was expected.
	ErrContextMissingEntityUUID = errors.New("missing entity UUID in context")

	// ErrContextMissingPaging is an internal error indicating missing paging
	// in the request context, whereas one was expected.
	ErrContextMissingPaging = errors.New("missing paging in context")

	// ErrMarshalling signals that an error occurred while marshalling the response
	ErrMarshalling = errors.New("error marshalling response")

	// ErrReceivingResults signals that an error occurred while receiving the results
	// from the database or similar
	ErrReceivingResults = errors.New("error while receiving results from database")

	// ErrResourceNotFound indicates that a requested resource was not found
	ErrResourceNotFound = errors.New("resource not found")

	// ErrReceivingMeta signals that an error occurred while receiving the metadata
	// from the database or remotes
	ErrReceivingMeta = errors.New("error while receiving metadata")
)

const bufferErrorMessage = "Error while buffering response output: %s"

type ResourceEntityFunc func(r *http.Request) (string, error)

type SQLResourceFunc func(ctx context.Context, r *sql.Rows) (interface{}, error)

// EntityUUIDMiddleware is a http middleware for extracting the uuid of the resource requested,
// and passing it down.
func EntityUUIDMiddleware(entityFunc ResourceEntityFunc) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			entityUUID, err := entityFunc(r)
			if err != nil {
				WriteError(w, r, err, http.StatusInternalServerError)
				return
			}

			h.ServeHTTP(
				w,
				r.WithContext(context.WithValue(r.Context(), CtxEntityUUID, entityUUID)),
			)
		})
	}
}

// AuthBearerHeaderMiddleware is a http middleware for extracting the bearer token from
// the authorization header, and passing it down. If the header is not existing, the
// WWW-Authenticate header is set and the handler bails out.
func AuthBearerHeaderMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := FromAuthHeader(r)
		if err != nil {
			if err == ErrMissingAuthHeader {
				// If it was a browser request, give it a chance to authenticate
				w.Header().Add("WWW-Authenticate", `bearer`)
			} else {
				WriteError(w, r, err, http.StatusBadRequest)
			}

			return
		}

		h.ServeHTTP(
			w,
			r.WithContext(context.WithValue(r.Context(), CtxAuthToken, token)),
		)
	})
}

// PagingMiddleware is a http middleware for extracting paging information, and passing
// it down.
func PagingMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paging, err := ParsePagingFromRequest(r)
		if err != nil {
			WriteError(w, r, err, http.StatusInternalServerError)
			return
		}

		h.ServeHTTP(
			w,
			r.WithContext(context.WithValue(r.Context(), CtxPaging, paging)),
		)
	})
}

// ContentTypeJSONMiddleware is a http middleware for setting the content type to
// application/json, if the request method was not HEAD.
func ContentTypeJSONMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only set content type if we are not sending a HEAD request
		if r.Method != http.MethodHead {
			w.Header().Set("Content-Type", "application/json")
		}

		h.ServeHTTP(w, r)
	})
}

func EntityUUIDFromRequestContext(ctx context.Context) (string, error) {
	entityUUID, ok := ctx.Value(CtxEntityUUID).(string)
	if !ok {
		return "", ErrContextMissingEntityUUID
	}

	return entityUUID, nil
}

func AuthTokenFromRequestContext(ctx context.Context) (string, error) {
	token, ok := ctx.Value(CtxAuthToken).(string)
	if !ok {
		return "", ErrContextMissingAuthToken
	}

	return token, nil
}

func PagingFromRequestContext(ctx context.Context) (Paging, error) {
	paging, ok := ctx.Value(CtxPaging).(Paging)
	if !ok {
		return Paging{}, ErrContextMissingPaging
	}

	return paging, nil
}
