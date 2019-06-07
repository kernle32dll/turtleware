package server

import (
	"net/http"
)

func TenantStaticListHandler(
	hashFetcher TenantListHashFunc,
	countFetcher TenantListCountFunc,
	dataFetcher TenantListStaticDataFunc,
) http.Handler {
	cacheMiddleware := TenantListCacheMiddleware(hashFetcher)
	countMiddleware := TenantCountHeaderMiddleware(countFetcher)
	dataMiddleware := TenantStaticListDataHandler(dataFetcher)

	return cacheMiddleware(
		countMiddleware(
			dataMiddleware,
		),
	)
}

func TenantListSQLHandler(
	hashFetcher TenantListHashFunc,
	countFetcher TenantListCountFunc,
	dataFetcher TenantListSQLDataFunc,
	dataTransformer SQLResourceFunc,
) http.Handler {
	cacheMiddleware := TenantListCacheMiddleware(hashFetcher)
	countMiddleware := TenantCountHeaderMiddleware(countFetcher)
	dataMiddleware := TenantSQLListDataHandler(dataFetcher, dataTransformer)

	return cacheMiddleware(
		countMiddleware(
			dataMiddleware,
		),
	)
}

func TenantRessourceHandler(
	entityFetcher ResourceEntityFunc,
	lastModFetcher TenantResourceLastModFunc,
	dataFetcher TenantResourceDataFunc,
) http.Handler {
	entityMiddleware := EntityUUIDMiddleware(entityFetcher)
	cacheMiddleware := TenantResourceCacheMiddleware(lastModFetcher)
	dataMiddleware := TenantResourceDataHandler(dataFetcher)

	return entityMiddleware(
		cacheMiddleware(
			dataMiddleware,
		),
	)
}

func TenantListPreHandler(
	keys []interface{},
) func(h http.Handler) http.Handler {
	pagingMiddleware := PagingMiddleware

	return func(h http.Handler) http.Handler {
		return TenantResourcePreHandler(keys)(
			pagingMiddleware(
				h,
			),
		)
	}
}

func TenantResourcePreHandler(
	keys []interface{},
) func(h http.Handler) http.Handler {
	jsonMiddleware := ContentTypeJSONMiddleware
	authHeaderMiddleware := AuthBearerHeaderMiddleware
	authMiddleware := TenantAuthMiddleware(keys)

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
