package tenant

import (
	"github.com/justinas/alice"
	"github.com/kernle32dll/turtleware"
	"github.com/lestrrat-go/jwx/jwk"

	"net/http"
)

func StaticListHandler(
	keySet *jwk.Set,
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

	return listPreHandler(keySet).Append(
		cacheMiddleware,
		countMiddleware,
	).Then(
		dataMiddleware,
	)
}

func ListSQLHandler(
	keySet *jwk.Set,
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

	return listPreHandler(keySet).Append(
		cacheMiddleware,
		countMiddleware,
	).Then(
		dataMiddleware,
	)
}

func ResourceHandler(
	keySet *jwk.Set,
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

	return resourcePreHandler(keySet).Append(
		entityMiddleware,
		cacheMiddleware,
	).Then(
		dataMiddleware,
	)
}

func ResourcePatchHandler(
	keySet *jwk.Set,
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

	return resourcePreHandler(keySet).Append(
		entityMiddleware,
		patchMiddleware,
	).Then(
		nextHandler,
	)
}

func ResourceCreateHandler(
	keySet *jwk.Set,
	entityFetcher turtleware.ResourceEntityFunc,
	createDTOProviderFunc turtleware.CreateDTOProviderFunc,
	createFunc CreateFunc,
	errorHandler turtleware.ErrorHandlerFunc,
	nextHandler http.Handler,
) http.Handler {
	if errorHandler == nil {
		errorHandler = turtleware.DefaultCreateErrorHandler
	}

	entityMiddleware := turtleware.EntityUUIDMiddleware(entityFetcher)
	createMiddleware := ResourceCreateMiddleware(createDTOProviderFunc, createFunc, errorHandler)

	return resourcePreHandler(keySet).Append(
		entityMiddleware,
		createMiddleware,
	).Then(
		nextHandler,
	)
}

func listPreHandler(
	keySet *jwk.Set,
) alice.Chain {
	pagingMiddleware := turtleware.PagingMiddleware

	return resourcePreHandler(keySet).Append(pagingMiddleware)
}

func resourcePreHandler(
	keySet *jwk.Set,
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
