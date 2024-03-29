package turtleware

import (
	"github.com/go-logr/logr"

	"fmt"
	"log/slog"
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
			ctx := r.Context()
			log := slog.New(logr.ToSlogHandler(logr.FromContextOrDiscard(ctx)))
			log.WarnContext(
				ctx,
				fmt.Sprintf("Received If-Modified-Since header in invalid format: %s", lastModifiedHeader),
				slog.Any("error", err),
			)

			return "", lastModifiedHeaderTime
		}

		lastModifiedHeaderTime = parsedTime
	}

	return etag, lastModifiedHeaderTime
}
