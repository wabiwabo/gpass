package middleware

import (
	"context"
	"net/http"
	"regexp"
	"slices"
)

const apiVersionKey contextKey = "api_version"

var versionRe = regexp.MustCompile(`application/vnd\.garudapass\.(v\d+)\+json`)

// APIVersion returns middleware that checks the Accept header for API versioning.
// Supports: Accept: application/vnd.garudapass.v1+json
// Default version used if no Accept header or standard application/json.
func APIVersion(defaultVersion string, supported []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			accept := r.Header.Get("Accept")
			version := ParseAcceptVersion(accept)

			if version == "" {
				version = defaultVersion
			} else if !slices.Contains(supported, version) {
				http.Error(w, "unsupported API version", http.StatusNotAcceptable)
				return
			}

			ctx := context.WithValue(r.Context(), apiVersionKey, version)
			w.Header().Set("X-API-Version", version)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetAPIVersion retrieves the negotiated API version from context.
func GetAPIVersion(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(apiVersionKey).(string); ok {
		return v
	}
	return ""
}

// ParseAcceptVersion extracts version from Accept header.
// Format: application/vnd.garudapass.v{N}+json
func ParseAcceptVersion(accept string) string {
	matches := versionRe.FindStringSubmatch(accept)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}
