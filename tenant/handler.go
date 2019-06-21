package tenant

import (
	"github.com/kernle32dll/turtleware"
	"net/http"
)

func StaticListHandler(
	name string,
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

	return listPreHandler(name, keys)(
		cacheMiddleware(
			countMiddleware(
				dataMiddleware,
			),
		),
	)
}

func ListSQLHandler(
	name string,
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

	return listPreHandler(name, keys)(
		cacheMiddleware(
			countMiddleware(
				dataMiddleware,
			),
		),
	)
}

func ResourceHandler(
	name string,
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

	return resourcePreHandler(name, keys)(
		entityMiddleware(
			cacheMiddleware(
				dataMiddleware,
			),
		),
	)
}

func ResourcePatchHandler(
	name string,
	keys []interface{},
	entityFetcher turtleware.ResourceEntityFunc,
	patchDTOProviderFunc PatchDTOProviderFunc,
	patchFunc PatchFunc,
	errorHandler turtleware.ErrorHandlerFunc,
	nextHandler http.Handler,
) http.Handler {
	if errorHandler == nil {
		errorHandler = DefaultPatchErrorHandler
	}

	entityMiddleware := turtleware.EntityUUIDMiddleware(entityFetcher)
	patchMiddleware := ResourcePatchMiddleware(patchDTOProviderFunc, patchFunc, errorHandler)

	return resourcePreHandler(name, keys)(
		entityMiddleware(
			patchMiddleware(
				nextHandler,
			),
		),
	)
}

func listPreHandler(
	name string,
	keys []interface{},
) func(h http.Handler) http.Handler {
	pagingMiddleware := turtleware.PagingMiddleware

	return func(h http.Handler) http.Handler {
		return resourcePreHandler(name, keys)(
			pagingMiddleware(
				h,
			),
		)
	}
}

func resourcePreHandler(
	name string,
	keys []interface{},
) func(h http.Handler) http.Handler {
	tracingMiddleware := turtleware.TracingMiddleware(name, nil)
	jsonMiddleware := turtleware.ContentTypeJSONMiddleware
	authHeaderMiddleware := turtleware.AuthBearerHeaderMiddleware
	authMiddleware := turtleware.AuthClaimsMiddleware(keys)
	tenantUuidMiddleware := UUIDMiddleware()

	return func(h http.Handler) http.Handler {
		return tracingMiddleware(
			jsonMiddleware(
				authHeaderMiddleware(
					authMiddleware(
						tenantUuidMiddleware(
							h,
						),
					),
				),
			),
		)
	}
}
