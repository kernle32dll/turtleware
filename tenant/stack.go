package tenant

import (
	"github.com/justinas/alice"
	"github.com/kernle32dll/turtleware"

	"net/http"
)

func StaticListHandler(
	keys []interface{},
	hashFetcher ListHashFunc,
	countFetcher ListCountFunc,
	dataFetcher ListStaticDataFunc,
	errorHandler turtleware.ErrorHandlerFunc,
) http.Handler {
	if errorHandler == nil {
		errorHandler = turtleware.DefaultErrorHandler
	}

	cacheMiddleware := ListCacheMiddleware(hashFetcher, errorHandler)
	countMiddleware := CountHeaderMiddleware(countFetcher, errorHandler)
	dataMiddleware := StaticListDataHandler(dataFetcher, errorHandler)

	return listPreHandler(keys).Append(
		cacheMiddleware,
		countMiddleware,
	).Then(
		dataMiddleware,
	)
}

func ListSQLHandler(
	keys []interface{},
	hashFetcher ListHashFunc,
	countFetcher ListCountFunc,
	dataFetcher ListSQLDataFunc,
	dataTransformer turtleware.SQLResourceFunc,
	errorHandler turtleware.ErrorHandlerFunc,
) http.Handler {
	if errorHandler == nil {
		errorHandler = turtleware.DefaultErrorHandler
	}

	cacheMiddleware := ListCacheMiddleware(hashFetcher, errorHandler)
	countMiddleware := CountHeaderMiddleware(countFetcher, errorHandler)
	dataMiddleware := SQLListDataHandler(dataFetcher, dataTransformer, errorHandler)

	return listPreHandler(keys).Append(
		cacheMiddleware,
		countMiddleware,
	).Then(
		dataMiddleware,
	)
}

func ResourceHandler(
	keys []interface{},
	entityFetcher turtleware.ResourceEntityFunc,
	lastModFetcher ResourceLastModFunc,
	dataFetcher ResourceDataFunc,
	errorHandler turtleware.ErrorHandlerFunc,
) http.Handler {
	if errorHandler == nil {
		errorHandler = turtleware.DefaultErrorHandler
	}

	entityMiddleware := turtleware.EntityUUIDMiddleware(entityFetcher)
	cacheMiddleware := ResourceCacheMiddleware(lastModFetcher, errorHandler)
	dataMiddleware := ResourceDataHandler(dataFetcher, errorHandler)

	return resourcePreHandler(keys).Append(
		entityMiddleware,
		cacheMiddleware,
	).Then(
		dataMiddleware,
	)
}

func ResourcePatchHandler(
	keys []interface{},
	entityFetcher turtleware.ResourceEntityFunc,
	patchDTOProviderFunc turtleware.PatchDTOProviderFunc,
	patchFunc PatchFunc,
	errorHandler turtleware.ErrorHandlerFunc,
	nextHandler http.Handler,
) http.Handler {
	if errorHandler == nil {
		errorHandler = turtleware.DefaultPatchErrorHandler
	}

	entityMiddleware := turtleware.EntityUUIDMiddleware(entityFetcher)
	patchMiddleware := ResourcePatchMiddleware(patchDTOProviderFunc, patchFunc, errorHandler)

	return resourcePreHandler(keys).Append(
		entityMiddleware,
		patchMiddleware,
	).Then(
		nextHandler,
	)
}

func listPreHandler(
	keys []interface{},
) alice.Chain {
	pagingMiddleware := turtleware.PagingMiddleware

	return resourcePreHandler(keys).Append(pagingMiddleware)
}

func resourcePreHandler(
	keys []interface{},
) alice.Chain {
	authHeaderMiddleware := turtleware.AuthBearerHeaderMiddleware
	authMiddleware := turtleware.AuthClaimsMiddleware(keys)
	tenantUUIDMiddleware := UUIDMiddleware()

	return alice.New(
		authHeaderMiddleware,
		authMiddleware,
		tenantUUIDMiddleware,
	)
}
