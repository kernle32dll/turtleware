package turtleware

import (
	"github.com/jmoiron/sqlx"
	"github.com/justinas/alice"
	"github.com/lestrrat-go/jwx/v2/jwk"

	"context"
	"database/sql"
	"net/http"
	"time"
)

type GetEndpoint interface {
	EntityUUID(r *http.Request) (string, error)
	LastModification(ctx context.Context, entityUUID string) (time.Time, error)
	FetchEntity(ctx context.Context, entityUUID string) (interface{}, error)
	HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
}

func ResourceHandler(
	keySet jwk.Set,
	getEndpoint GetEndpoint,
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

type GetSQLListEndpoint interface {
	ListHash(ctx context.Context, paging Paging) (string, error)
	TotalCount(ctx context.Context) (uint, error)
	FetchRows(ctx context.Context, paging Paging) (*sql.Rows, error)
	TransformEntity(ctx context.Context, r *sql.Rows) (interface{}, error)
	HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
}

func ListSQLHandler(
	keySet jwk.Set,
	listEndpoint GetSQLListEndpoint,
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

type GetSQLxListEndpoint interface {
	ListHash(ctx context.Context, paging Paging) (string, error)
	TotalCount(ctx context.Context) (uint, error)
	FetchRows(ctx context.Context, paging Paging) (*sqlx.Rows, error)
	TransformEntity(ctx context.Context, r *sqlx.Rows) (interface{}, error)
	HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
}

func ListSQLxHandler(
	keySet jwk.Set,
	listEndpoint GetSQLxListEndpoint,
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

type GetStaticListEndpoint interface {
	ListHash(ctx context.Context, paging Paging) (string, error)
	TotalCount(ctx context.Context) (uint, error)
	FetchEntities(ctx context.Context, paging Paging) ([]interface{}, error)
	HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
}

func StaticListHandler(
	keySet jwk.Set,
	listEndpoint GetStaticListEndpoint,
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

type CreateEndpoint interface {
	EntityUUID(r *http.Request) (string, error)
	ProvideDTO() CreateDTO
	CreateEntity(ctx context.Context, entityUUID, userUUID string, create CreateDTO) error
	HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
}

func ResourceCreateHandler(
	keySet jwk.Set,
	createEndpoint CreateEndpoint,
	nextHandler http.Handler,
) http.Handler {
	entityMiddleware := EntityUUIDMiddleware(createEndpoint.EntityUUID)
	createMiddleware := ResourceCreateMiddleware(createEndpoint.ProvideDTO, createEndpoint.CreateEntity, createEndpoint.HandleError)

	return resourcePreHandler(keySet).Append(
		entityMiddleware,
		createMiddleware,
	).Then(
		nextHandler,
	)
}

// --------------------------

type PatchEndpoint interface {
	EntityUUID(r *http.Request) (string, error)
	ProvideDTO() PatchDTO
	UpdateEntity(ctx context.Context, entityUUID, userUUID string, patch PatchDTO, ifUnmodifiedSince time.Time) error
	HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
}

func ResourcePatchHandler(
	keySet jwk.Set,
	patchEndpoint PatchEndpoint,
	nextHandler http.Handler,
) http.Handler {
	entityMiddleware := EntityUUIDMiddleware(patchEndpoint.EntityUUID)
	patchMiddleware := ResourcePatchMiddleware(patchEndpoint.ProvideDTO, patchEndpoint.UpdateEntity, patchEndpoint.HandleError)

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
