package tenant

import (
	"github.com/kernle32dll/turtleware"
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
	dataTransformer turtleware.SQLResourceFunc,
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
	entityFetcher turtleware.ResourceEntityFunc,
	lastModFetcher ResourceLastModFunc,
	dataFetcher ResourceDataFunc,
) http.Handler {
	entityMiddleware := turtleware.EntityUUIDMiddleware(entityFetcher)
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
	pagingMiddleware := turtleware.PagingMiddleware

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
	jsonMiddleware := turtleware.ContentTypeJSONMiddleware
	authHeaderMiddleware := turtleware.AuthBearerHeaderMiddleware
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
