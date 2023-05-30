package turtleware

import (
	"github.com/rs/zerolog"

	"net/http"
	"time"
)

// ExtractCacheHeader extracts the Etag (If-None-Match) and last modification (If-Modified-Since)
// headers from a given request.
func ExtractCacheHeader(r *http.Request) (string, time.Time) {
	etag := r.Header.Get("If-None-Match")
	lastModifiedHeader := r.Header.Get("If-Modified-Since")

	lastModifiedHeaderTime := time.Time{}

	if lastModifiedHeader != "" {
		parsedTime, err := time.Parse(time.RFC1123, lastModifiedHeader)
		if err != nil {
			zerolog.Ctx(r.Context()).Warn().Err(err).Msgf("Received If-Modified-Since header in invalid format: %s", lastModifiedHeader)

			return "", lastModifiedHeaderTime
		}

		lastModifiedHeaderTime = parsedTime
	}

	return etag, lastModifiedHeaderTime
}
