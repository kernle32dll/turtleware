package turtleware

import (
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
	dataTransformer SQLResourceFunc,
	errorHandler ErrorHandlerFunc,
) http.Handler {
	if errorHandler == nil {
		errorHandler = DefaultErrorHandler
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

	return resourcePreHandler(keys)(
		entityMiddleware(
			cacheMiddleware(
				dataMiddleware,
			),
		),
	)
}

func listPreHandler(
	keys []interface{},
) func(h http.Handler) http.Handler {
	pagingMiddleware := PagingMiddleware

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
	jsonMiddleware := ContentTypeJSONMiddleware
	authHeaderMiddleware := AuthBearerHeaderMiddleware
	authMiddleware := AuthClaimsMiddleware(keys)

	return func(h http.Handler) http.Handler {
		return jsonMiddleware(
			authHeaderMiddleware(
				authMiddleware(
					h,
				),
			),
		)
	}
}
