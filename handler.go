package turtleware

import (
	"github.com/justinas/alice"

	"net/http"
)

func StaticListHandler(
	keys []interface{},
	hashFetcher ListHashFunc,
	countFetcher ListCountFunc,
	dataFetcher ListStaticDataFunc,
	errorHandler ErrorHandlerFunc,
) http.Handler {
	if errorHandler == nil {
		errorHandler = DefaultErrorHandler
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
	dataTransformer SQLResourceFunc,
	errorHandler ErrorHandlerFunc,
) http.Handler {
	if errorHandler == nil {
		errorHandler = DefaultErrorHandler
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
	entityFetcher ResourceEntityFunc,
	lastModFetcher ResourceLastModFunc,
	dataFetcher ResourceDataFunc,
	errorHandler ErrorHandlerFunc,
) http.Handler {
	if errorHandler == nil {
		errorHandler = DefaultErrorHandler
	}

	entityMiddleware := EntityUUIDMiddleware(entityFetcher)
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
	entityFetcher ResourceEntityFunc,
	patchDTOProviderFunc PatchDTOProviderFunc,
	patchFunc PatchFunc,
	errorHandler ErrorHandlerFunc,
	nextHandler http.Handler,
) http.Handler {
	if errorHandler == nil {
		errorHandler = DefaultPatchErrorHandler
	}

	entityMiddleware := EntityUUIDMiddleware(entityFetcher)
	patchMiddleware := ResourcePatchMiddleware(patchDTOProviderFunc, patchFunc, errorHandler)

	return resourcePreHandler(keys).Append(
		entityMiddleware,
		patchMiddleware,
	).Then(
		nextHandler,
	)
}

func ResourceCreateHandler(
	keys []interface{},
	entityFetcher ResourceEntityFunc,
	createDTOProviderFunc CreateDTOProviderFunc,
	createFunc CreateFunc,
	errorHandler ErrorHandlerFunc,
	nextHandler http.Handler,
) http.Handler {
	if errorHandler == nil {
		errorHandler = DefaultCreateErrorHandler
	}

	entityMiddleware := EntityUUIDMiddleware(entityFetcher)
	createMiddleware := ResourceCreateMiddleware(createDTOProviderFunc, createFunc, errorHandler)

	return resourcePreHandler(keys).Append(
		entityMiddleware,
		createMiddleware,
	).Then(
		nextHandler,
	)
}

func listPreHandler(
	keys []interface{},
) alice.Chain {
	pagingMiddleware := PagingMiddleware

	return resourcePreHandler(keys).Append(pagingMiddleware)
}

func resourcePreHandler(
	keys []interface{},
) alice.Chain {
	authHeaderMiddleware := AuthBearerHeaderMiddleware
	authMiddleware := AuthClaimsMiddleware(keys)

	return alice.New(
		authHeaderMiddleware,
		authMiddleware,
	)
}
