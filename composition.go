package turtleware

import (
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/justinas/alice"
	"github.com/lestrrat-go/jwx/v2/jwk"

	"context"
	"database/sql"
	"net/http"
	"time"
)

type GetEndpoint[T any] interface {
	EntityUUID(r *http.Request) (uuid.UUID, error)
	LastModification(ctx context.Context, entityUUID uuid.UUID) (time.Time, error)
	FetchEntity(ctx context.Context, entityUUID uuid.UUID) (T, error)
	HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
}

func ResourceHandler[T any](
	keySet jwk.Set,
	getEndpoint GetEndpoint[T],
) http.Handler {
	entityMiddleware := EntityUUIDMiddleware(getEndpoint.EntityUUID)
	cacheMiddleware := ResourceCacheMiddleware(getEndpoint.LastModification, getEndpoint.HandleError)
	dataMiddleware := ResourceDataHandler(getEndpoint.FetchEntity, getEndpoint.HandleError)

	return resourcePreHandler(keySet).Append(
		entityMiddleware,
		cacheMiddleware,
	).Then(
		dataMiddleware,
	)
}

// --------------------------

type GetSQLListEndpoint[T any] interface {
	ListHash(ctx context.Context, paging Paging) (string, error)
	TotalCount(ctx context.Context) (uint, error)
	FetchRows(ctx context.Context, paging Paging) (*sql.Rows, error)
	TransformEntity(ctx context.Context, r *sql.Rows) (T, error)
	HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
}

func ListSQLHandler[T any](
	keySet jwk.Set,
	listEndpoint GetSQLListEndpoint[T],
) http.Handler {
	cacheMiddleware := ListCacheMiddleware(listEndpoint.ListHash, listEndpoint.HandleError)
	countMiddleware := CountHeaderMiddleware(listEndpoint.TotalCount, listEndpoint.HandleError)
	dataMiddleware := SQLListDataHandler(listEndpoint.FetchRows, listEndpoint.TransformEntity, listEndpoint.HandleError)

	return listPreHandler(keySet).Append(
		cacheMiddleware,
		countMiddleware,
	).Then(
		dataMiddleware,
	)
}

// --------------------------

type GetSQLxListEndpoint[T any] interface {
	ListHash(ctx context.Context, paging Paging) (string, error)
	TotalCount(ctx context.Context) (uint, error)
	FetchRows(ctx context.Context, paging Paging) (*sqlx.Rows, error)
	TransformEntity(ctx context.Context, r *sqlx.Rows) (T, error)
	HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
}

func ListSQLxHandler[T any](
	keySet jwk.Set,
	listEndpoint GetSQLxListEndpoint[T],
) http.Handler {
	cacheMiddleware := ListCacheMiddleware(listEndpoint.ListHash, listEndpoint.HandleError)
	countMiddleware := CountHeaderMiddleware(listEndpoint.TotalCount, listEndpoint.HandleError)
	dataMiddleware := SQLxListDataHandler(listEndpoint.FetchRows, listEndpoint.TransformEntity, listEndpoint.HandleError)

	return listPreHandler(keySet).Append(
		cacheMiddleware,
		countMiddleware,
	).Then(
		dataMiddleware,
	)
}

// --------------------------

type GetStaticListEndpoint[T any] interface {
	ListHash(ctx context.Context, paging Paging) (string, error)
	TotalCount(ctx context.Context) (uint, error)
	FetchEntities(ctx context.Context, paging Paging) ([]T, error)
	HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
}

func StaticListHandler[T any](
	keySet jwk.Set,
	listEndpoint GetStaticListEndpoint[T],
) http.Handler {
	cacheMiddleware := ListCacheMiddleware(listEndpoint.ListHash, listEndpoint.HandleError)
	countMiddleware := CountHeaderMiddleware(listEndpoint.TotalCount, listEndpoint.HandleError)
	dataMiddleware := StaticListDataHandler(listEndpoint.FetchEntities, listEndpoint.HandleError)

	return listPreHandler(keySet).Append(
		cacheMiddleware,
		countMiddleware,
	).Then(
		dataMiddleware,
	)
}

// --------------------------

type CreateEndpoint[T CreateDTO] interface {
	EntityUUID(r *http.Request) (uuid.UUID, error)
	CreateEntity(ctx context.Context, entityUUID, userUUID uuid.UUID, create T) error
	HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
}

func ResourceCreateHandler[T CreateDTO](
	keySet jwk.Set,
	createEndpoint CreateEndpoint[T],
	nextHandler http.Handler,
) http.Handler {
	entityMiddleware := EntityUUIDMiddleware(createEndpoint.EntityUUID)
	createMiddleware := ResourceCreateMiddleware(createEndpoint.CreateEntity, createEndpoint.HandleError)

	return resourcePreHandler(keySet).Append(
		entityMiddleware,
		createMiddleware,
	).Then(
		nextHandler,
	)
}

// --------------------------

type PatchEndpoint[T PatchDTO] interface {
	EntityUUID(r *http.Request) (uuid.UUID, error)
	UpdateEntity(ctx context.Context, entityUUID, userUUID uuid.UUID, patch T, ifUnmodifiedSince time.Time) error
	HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
}

func ResourcePatchHandler[T PatchDTO](
	keySet jwk.Set,
	patchEndpoint PatchEndpoint[T],
	nextHandler http.Handler,
) http.Handler {
	entityMiddleware := EntityUUIDMiddleware(patchEndpoint.EntityUUID)
	patchMiddleware := ResourcePatchMiddleware(patchEndpoint.UpdateEntity, patchEndpoint.HandleError)

	return resourcePreHandler(keySet).Append(
		entityMiddleware,
		patchMiddleware,
	).Then(
		nextHandler,
	)
}

// --------------------------

func listPreHandler(
	keySet jwk.Set,
) alice.Chain {
	pagingMiddleware := PagingMiddleware

	return resourcePreHandler(keySet).Append(pagingMiddleware)
}

func resourcePreHandler(
	keySet jwk.Set,
) alice.Chain {
	authHeaderMiddleware := AuthBearerHeaderMiddleware
	authMiddleware := AuthClaimsMiddleware(keySet)

	return alice.New(
		authHeaderMiddleware,
		authMiddleware,
	)
}
