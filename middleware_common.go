package turtleware

import (
	"github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"

	"context"
	"database/sql"
	"errors"
	"net/http"
)

type ctxKey int

const (
	// ctxAuthToken is the context key used to pass down the bearer token.
	ctxAuthToken ctxKey = iota

	// ctxEntityUUID is the context key used to pass down the entity UUID.
	ctxEntityUUID

	// ctxPaging is the context key used to pass down paging information.
	ctxPaging

	// ctxAuthClaims is the context key used to pass down jwt claims.
	ctxAuthClaims
)

var (
	// ErrContextMissingAuthToken is an internal error indicating a missing
	// auth token in the request context, whereas one was expected.
	ErrContextMissingAuthToken = errors.New("missing auth token in context")

	// ErrContextMissingEntityUUID is an internal error indicating a missing
	// entity UUID in the request context, whereas one was expected.
	ErrContextMissingEntityUUID = errors.New("missing entity UUID in context")

	// ErrContextMissingPaging is an internal error indicating missing paging
	// in the request context, whereas one was expected.
	ErrContextMissingPaging = errors.New("missing paging in context")

	// ErrContextMissingAuthClaims is an internal error indicating missing auth
	// claims in the request context, whereas they were expected.
	ErrContextMissingAuthClaims = errors.New("missing auth claims in context")

	// ErrMarshalling signals that an error occurred while marshalling.
	ErrMarshalling = errors.New("error marshalling")

	// ErrReceivingResults signals that an error occurred while receiving the results
	// from the database or similar.
	ErrReceivingResults = errors.New("error while receiving results from database")

	// ErrResourceNotFound indicates that a requested resource was not found.
	ErrResourceNotFound = errors.New("resource not found")

	// ErrReceivingMeta signals that an error occurred while receiving the metadata
	// from the database or remotes.
	ErrReceivingMeta = errors.New("error while receiving metadata")
)

type ResourceEntityFunc func(r *http.Request) (string, error)

type SQLResourceFunc func(ctx context.Context, r *sql.Rows) (interface{}, error)

// EntityUUIDMiddleware is a http middleware for extracting the uuid of the resource requested,
// and passing it down.
func EntityUUIDMiddleware(entityFunc ResourceEntityFunc) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			entityUUID, err := entityFunc(r)
			if err != nil {
				WriteError(w, r, http.StatusInternalServerError, err)
				return
			}

			h.ServeHTTP(
				w,
				r.WithContext(context.WithValue(r.Context(), ctxEntityUUID, entityUUID)),
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
				WriteError(w, r, http.StatusUnauthorized, err)
			} else {
				WriteError(w, r, http.StatusBadRequest, err)
			}

			return
		}

		h.ServeHTTP(
			w,
			r.WithContext(context.WithValue(r.Context(), ctxAuthToken, token)),
		)
	})
}

// AuthClaimsMiddleware is a http middleware for extracting authentication claims, and
// passing them down.
func AuthClaimsMiddleware(keys []interface{}) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := AuthTokenFromRequestContext(r.Context())
			if err != nil {
				WriteError(w, r, http.StatusInternalServerError, err)
				return
			}

			claims, err := ValidateToken(token, keys)
			if err != nil {
				WriteError(w, r, http.StatusBadRequest, err)
				return
			}

			h.ServeHTTP(
				w,
				r.WithContext(context.WithValue(r.Context(), ctxAuthClaims, claims)),
			)
		})
	}
}

// PagingMiddleware is a http middleware for extracting paging information, and passing
// it down.
func PagingMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paging, err := ParsePagingFromRequest(r)
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, err)
			return
		}

		h.ServeHTTP(
			w,
			r.WithContext(context.WithValue(r.Context(), ctxPaging, paging)),
		)
	})
}

// TracingMiddleware is a http middleware for injecting a new named opentracing
// span into the request context. If tracer is nil, opentracing.GlobalTracer()
// is used.
func TracingMiddleware(name string, tracer opentracing.Tracer) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			locTracer := tracer
			if locTracer == nil {
				locTracer = opentracing.GlobalTracer()
			}

			wireCtx, err := locTracer.Extract(
				opentracing.HTTPHeaders,
				opentracing.HTTPHeadersCarrier(r.Header),
			)
			if err != nil {
				// ErrSpanContextNotFound is just a trace, otherwise its an error
				if err == opentracing.ErrSpanContextNotFound {
					logrus.Trace(err)
				} else {
					logrus.Error(err)
				}
			}

			span, spanCtx := opentracing.StartSpanFromContextWithTracer(r.Context(), locTracer, name, opentracing.ChildOf(wireCtx))
			defer span.Finish()

			logger := logrus.WithContext(spanCtx)

			// ---------------------

			// Write tracing headers back into response, to enable clients to debug calls without
			// sending a valid trace in the first place.

			carrier := opentracing.HTTPHeadersCarrier{}
			if err := locTracer.Inject(
				span.Context(),
				opentracing.HTTPHeaders,
				carrier); err != nil {
				logger.Warnf("Failed to re-purpose trace headers: %s", err)
			} else {
				// Ignore error, as it can never happen
				_ = carrier.ForeachKey(func(key, val string) error {
					w.Header().Add(key, val)
					return nil
				})
			}

			h.ServeHTTP(
				w,
				r.WithContext(spanCtx),
			)
		})
	}
}

func EntityUUIDFromRequestContext(ctx context.Context) (string, error) {
	entityUUID, ok := ctx.Value(ctxEntityUUID).(string)
	if !ok {
		return "", ErrContextMissingEntityUUID
	}

	return entityUUID, nil
}

func AuthTokenFromRequestContext(ctx context.Context) (string, error) {
	token, ok := ctx.Value(ctxAuthToken).(string)
	if !ok {
		return "", ErrContextMissingAuthToken
	}

	return token, nil
}

func PagingFromRequestContext(ctx context.Context) (Paging, error) {
	paging, ok := ctx.Value(ctxPaging).(Paging)
	if !ok {
		return Paging{}, ErrContextMissingPaging
	}

	return paging, nil
}

func AuthClaimsFromRequestContext(ctx context.Context) (map[string]interface{}, error) {
	claims, ok := ctx.Value(ctxAuthClaims).(map[string]interface{})
	if !ok {
		return nil, ErrContextMissingAuthClaims
	}

	return claims, nil
}
