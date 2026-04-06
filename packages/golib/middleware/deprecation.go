package middleware

import (
	"net/http"
	"time"
)

// Deprecated returns middleware that adds deprecation headers per RFC 8594.
// sunset is the date when the API will be removed.
// link is the URL to the replacement API documentation.
//
// Sets the following headers on every response:
//
//	Deprecation: true
//	Sunset: <HTTP-date format per RFC 7231>
//	Link: <url>; rel="successor-version"
//
// The handler still executes normally; this only adds informational headers.
func Deprecated(sunset time.Time, link string) func(http.Handler) http.Handler {
	// Pre-format the sunset date in HTTP date format (RFC 7231 / RFC 1123).
	sunsetStr := sunset.UTC().Format(http.TimeFormat)

	// Pre-format the Link header value.
	linkHeader := "<" + link + `>; rel="successor-version"`

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Deprecation", "true")
			w.Header().Set("Sunset", sunsetStr)
			w.Header().Set("Link", linkHeader)

			next.ServeHTTP(w, r)
		})
	}
}
