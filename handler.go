package turtleware

import (
	"net/http"
)

func StaticListHandler(
	hashFetcher ListHashFunc,
	countFetcher ListCountFunc,
	dataFetcher ListStaticDataFunc,
) http.Handler {
	cacheMiddleware := ListCacheMiddleware(hashFetcher)
	countMiddleware := CountHeaderMiddleware(countFetcher)
	dataMiddleware := StaticListDataHandler(dataFetcher)

	return cacheMiddleware(
		countMiddleware(
			dataMiddleware,
		),
	)
}

func ListSQLHandler(
	hashFetcher ListHashFunc,
	countFetcher ListCountFunc,
	dataFetcher ListSQLDataFunc,
	dataTransformer SQLResourceFunc,
) http.Handler {
	cacheMiddleware := ListCacheMiddleware(hashFetcher)
	countMiddleware := CountHeaderMiddleware(countFetcher)
	dataMiddleware := SQLListDataHandler(dataFetcher, dataTransformer)

	return cacheMiddleware(
		countMiddleware(
			dataMiddleware,
		),
	)
}

func RessourceHandler(
	entityFetcher ResourceEntityFunc,
	lastModFetcher ResourceLastModFunc,
	dataFetcher ResourceDataFunc,
) http.Handler {
	entityMiddleware := EntityUUIDMiddleware(entityFetcher)
	cacheMiddleware := ResourceCacheMiddleware(lastModFetcher)
	dataMiddleware := ResourceDataHandler(dataFetcher)

	return entityMiddleware(
		cacheMiddleware(
			dataMiddleware,
		),
	)
}

func ListPreHandler(
	keys []interface{},
) func(h http.Handler) http.Handler {
	pagingMiddleware := PagingMiddleware

	return func(h http.Handler) http.Handler {
		return ResourcePreHandler(keys)(
			pagingMiddleware(
				h,
			),
		)
	}
}

func ResourcePreHandler(
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
