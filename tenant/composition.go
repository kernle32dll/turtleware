package tenant

import (
	"github.com/jmoiron/sqlx"
	"github.com/justinas/alice"
	"github.com/kernle32dll/turtleware"
	"github.com/lestrrat-go/jwx/v2/jwk"

	"context"
	"database/sql"
	"net/http"
	"time"
)

type GetEndpoint[T any] interface {
	EntityUUID(r *http.Request) (string, error)
	LastModification(ctx context.Context, tenantUUID string, entityUUID string) (time.Time, error)
	FetchEntity(ctx context.Context, tenantUUID string, entityUUID string) (T, error)
	HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
}

func ResourceHandler[T any](
	keySet jwk.Set,
	getEndpoint GetEndpoint[T],
) http.Handler {
	entityMiddleware := turtleware.EntityUUIDMiddleware(getEndpoint.EntityUUID)
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
	ListHash(ctx context.Context, tenantUUID string, paging turtleware.Paging) (string, error)
	TotalCount(ctx context.Context, tenantUUID string) (uint, error)
	FetchRows(ctx context.Context, tenantUUID string, paging turtleware.Paging) (*sql.Rows, error)
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
	ListHash(ctx context.Context, tenantUUID string, paging turtleware.Paging) (string, error)
	TotalCount(ctx context.Context, tenantUUID string) (uint, error)
	FetchRows(ctx context.Context, tenantUUID string, paging turtleware.Paging) (*sqlx.Rows, error)
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
	ListHash(ctx context.Context, tenantUUID string, paging turtleware.Paging) (string, error)
	TotalCount(ctx context.Context, tenantUUID string) (uint, error)
	FetchEntities(ctx context.Context, tenantUUID string, paging turtleware.Paging) ([]T, error)
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

type CreateEndpoint[T turtleware.CreateDTO] interface {
	EntityUUID(r *http.Request) (string, error)
	CreateEntity(ctx context.Context, tenantUUID, entityUUID, userUUID string, create T) error
	HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
}

func ResourceCreateHandler[T turtleware.CreateDTO](
	keySet jwk.Set,
	createEndpoint CreateEndpoint[T],
	nextHandler http.Handler,
) http.Handler {
	entityMiddleware := turtleware.EntityUUIDMiddleware(createEndpoint.EntityUUID)
	createMiddleware := ResourceCreateMiddleware(createEndpoint.CreateEntity, createEndpoint.HandleError)

	return resourcePreHandler(keySet).Append(
		entityMiddleware,
		createMiddleware,
	).Then(
		nextHandler,
	)
}

// --------------------------

type PatchEndpoint[T turtleware.PatchDTO] interface {
	EntityUUID(r *http.Request) (string, error)
	UpdateEntity(ctx context.Context, tenantUUID, entityUUID, userUUID string, patch T, ifUnmodifiedSince time.Time) error
	HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
}

func ResourcePatchHandler[T turtleware.PatchDTO](
	keySet jwk.Set,
	patchEndpoint PatchEndpoint[T],
	nextHandler http.Handler,
) http.Handler {
	entityMiddleware := turtleware.EntityUUIDMiddleware(patchEndpoint.EntityUUID)
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
	pagingMiddleware := turtleware.PagingMiddleware

	return resourcePreHandler(keySet).Append(pagingMiddleware)
}

func resourcePreHandler(
	keySet jwk.Set,
) alice.Chain {
	authHeaderMiddleware := turtleware.AuthBearerHeaderMiddleware
	authMiddleware := turtleware.AuthClaimsMiddleware(keySet)
	tenantUUIDMiddleware := UUIDMiddleware

	return alice.New(
		authHeaderMiddleware,
		authMiddleware,
		tenantUUIDMiddleware,
	)
}
