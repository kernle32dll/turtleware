package turtleware

import (
	"github.com/justinas/alice"
	"github.com/lestrrat-go/jwx/jwk"

	"net/http"
)

func StaticListHandler(
	keySet jwk.Set,
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

	return listPreHandler(keySet).Append(
		cacheMiddleware,
		countMiddleware,
	).Then(
		dataMiddleware,
	)
}

func ListSQLHandler(
	keySet jwk.Set,
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

	return listPreHandler(keySet).Append(
		cacheMiddleware,
		countMiddleware,
	).Then(
		dataMiddleware,
	)
}

func ResourceHandler(
	keySet jwk.Set,
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

	return resourcePreHandler(keySet).Append(
		entityMiddleware,
		cacheMiddleware,
	).Then(
		dataMiddleware,
	)
}

func ResourcePatchHandler(
	keySet jwk.Set,
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

	return resourcePreHandler(keySet).Append(
		entityMiddleware,
		patchMiddleware,
	).Then(
		nextHandler,
	)
}

func ResourceCreateHandler(
	keySet jwk.Set,
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

	return resourcePreHandler(keySet).Append(
		entityMiddleware,
		createMiddleware,
	).Then(
		nextHandler,
	)
}

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
