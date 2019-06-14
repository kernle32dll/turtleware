package tenant

import (
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

	return listPreHandler(keys)(
		cacheMiddleware(
			countMiddleware(
				dataMiddleware,
			),
		),
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

	return listPreHandler(keys)(
		cacheMiddleware(
			countMiddleware(
				dataMiddleware,
			),
		),
	)
}

func RessourceHandler(
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

	return resourcePreHandler(keys)(
		entityMiddleware(
			cacheMiddleware(
				dataMiddleware,
			),
		),
	)
}

func RessourcePatchHandler(
	keys []interface{},
	entityFetcher turtleware.ResourceEntityFunc,
	patchFunc PatchFunc,
	patchDTOProviderFunc PatchDTOProviderFunc,
	errorHandler turtleware.ErrorHandlerFunc,
	nextHandler http.Handler,
) http.Handler {
	if errorHandler == nil {
		errorHandler = DefaultPatchErrorHandler
	}

	entityMiddleware := turtleware.EntityUUIDMiddleware(entityFetcher)
	patchMiddleware := ResourcePatchMiddleware(patchFunc, patchDTOProviderFunc, errorHandler)

	return resourcePreHandler(keys)(
		entityMiddleware(
			patchMiddleware(
				nextHandler,
			),
		),
	)
}

func listPreHandler(
	keys []interface{},
) func(h http.Handler) http.Handler {
	pagingMiddleware := turtleware.PagingMiddleware

	return func(h http.Handler) http.Handler {
		return resourcePreHandler(keys)(
			pagingMiddleware(
				h,
			),
		)
	}
}

func resourcePreHandler(
	keys []interface{},
) func(h http.Handler) http.Handler {
	jsonMiddleware := turtleware.ContentTypeJSONMiddleware
	authHeaderMiddleware := turtleware.AuthBearerHeaderMiddleware
	authMiddleware := turtleware.AuthClaimsMiddleware(keys)
	tenantUuidMiddleware := UUIDMiddleware()

	return func(h http.Handler) http.Handler {
		return jsonMiddleware(
			authHeaderMiddleware(
				authMiddleware(
					tenantUuidMiddleware(
						h,
					),
				),
			),
		)
	}
}
