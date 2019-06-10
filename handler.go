package turtleware

import (
	"net/http"
)

func StaticListHandler(
	keys []interface{},
	hashFetcher ListHashFunc,
	countFetcher ListCountFunc,
	dataFetcher ListStaticDataFunc,
) http.Handler {
	cacheMiddleware := ListCacheMiddleware(hashFetcher)
	countMiddleware := CountHeaderMiddleware(countFetcher)
	dataMiddleware := StaticListDataHandler(dataFetcher)

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
) http.Handler {
	cacheMiddleware := ListCacheMiddleware(hashFetcher)
	countMiddleware := CountHeaderMiddleware(countFetcher)
	dataMiddleware := SQLListDataHandler(dataFetcher, dataTransformer)

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
	entityFetcher ResourceEntityFunc,
	lastModFetcher ResourceLastModFunc,
	dataFetcher ResourceDataFunc,
) http.Handler {
	entityMiddleware := EntityUUIDMiddleware(entityFetcher)
	cacheMiddleware := ResourceCacheMiddleware(lastModFetcher)
	dataMiddleware := ResourceDataHandler(dataFetcher)

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
	authMiddleware := AuthMiddleware(keys)

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
