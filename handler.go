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
