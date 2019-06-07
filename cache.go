package server

import (
	"gopkg.in/guregu/null.v3"

	"github.com/sirupsen/logrus"

	"net/http"
	"time"
)

// ExtractCacheHeader extracts the Etag (If-None-Match) and last modification (If-Modified-Since)
// headers from a given request.
func ExtractCacheHeader(r *http.Request) (string, null.Time) {
	etag := r.Header.Get("If-None-Match")
	lastModifiedHeader := r.Header.Get("If-Modified-Since")

	lastModifiedHeaderTime := null.Time{Valid: false}
	if lastModifiedHeader != "" {
		parsedTime, err := time.Parse(time.RFC1123, lastModifiedHeader)
		if err != nil {
			logrus.Warnf("Received If-Modified-Since header in invalid format: %s ; %s", lastModifiedHeader, err)
			return "", lastModifiedHeaderTime
		}

		lastModifiedHeaderTime.SetValid(parsedTime)
	}

	return etag, lastModifiedHeaderTime
}
