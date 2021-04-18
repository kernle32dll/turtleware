package tenant

import (
	"github.com/justinas/alice"
	"github.com/kernle32dll/turtleware"
	"github.com/lestrrat-go/jwx/jwk"

	"context"
	"database/sql"
	"net/http"
	"time"
)

type GetEndpoint interface {
	EntityUUID(r *http.Request) (string, error)
	LastModification(ctx context.Context, tenantUUID string, entityUUID string) (time.Time, error)
	FetchEntity(ctx context.Context, tenantUUID string, entityUUID string) (interface{}, error)
	HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
}

func ResourceHandler(
	keySet jwk.Set,
	getEndpoint GetEndpoint,
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

type GetSQLListEndpoint interface {
	ListHash(ctx context.Context, tenantUUID string, paging turtleware.Paging) (string, error)
	TotalCount(ctx context.Context, tenantUUID string) (uint, error)
	FetchRows(ctx context.Context, tenantUUID string, paging turtleware.Paging) (*sql.Rows, error)
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

type GetStaticListEndpoint interface {
	ListHash(ctx context.Context, tenantUUID string, paging turtleware.Paging) (string, error)
	TotalCount(ctx context.Context, tenantUUID string) (uint, error)
	FetchEntities(ctx context.Context, tenantUUID string, paging turtleware.Paging) ([]interface{}, error)
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
	ProvideDTO() turtleware.CreateDTO
	CreateEntity(ctx context.Context, tenantUUID, entityUUID, userUUID string, create turtleware.CreateDTO) error
	HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
}

func ResourceCreateHandler(
	keySet jwk.Set,
	createEndpoint CreateEndpoint,
	nextHandler http.Handler,
) http.Handler {
	entityMiddleware := turtleware.EntityUUIDMiddleware(createEndpoint.EntityUUID)
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
	ProvideDTO() turtleware.PatchDTO
	UpdateEntity(ctx context.Context, tenantUUID, entityUUID, userUUID string, patch turtleware.PatchDTO, ifUnmodifiedSince time.Time) error
	HandleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
}

func ResourcePatchHandler(
	keySet jwk.Set,
	patchEndpoint PatchEndpoint,
	nextHandler http.Handler,
) http.Handler {
	entityMiddleware := turtleware.EntityUUIDMiddleware(patchEndpoint.EntityUUID)
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
	pagingMiddleware := turtleware.PagingMiddleware

	return resourcePreHandler(keySet).Append(pagingMiddleware)
}

func resourcePreHandler(
	keySet jwk.Set,
) alice.Chain {
	authHeaderMiddleware := turtleware.AuthBearerHeaderMiddleware
	authMiddleware := turtleware.AuthClaimsMiddleware(keySet)
	tenantUUIDMiddleware := UUIDMiddleware()

	return alice.New(
		authHeaderMiddleware,
		authMiddleware,
		tenantUUIDMiddleware,
	)
}
