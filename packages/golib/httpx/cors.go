package httpx

import (
	"net/http"
	"strings"
)

// CORSOptions configures the CORS middleware. The zero value denies all
// cross-origin requests, which is correct for backend services that should
// only be reached from within the cluster or via the API gateway.
type CORSOptions struct {
	// AllowedOrigins is the list of origins permitted to make cross-origin
	// requests. Use ["*"] only for fully-public APIs that don't read cookies
	// or auth headers — wildcards are incompatible with credentials.
	AllowedOrigins []string

	// AllowedMethods defaults to GET, POST, PUT, PATCH, DELETE, OPTIONS.
	AllowedMethods []string

	// AllowedHeaders defaults to Content-Type, Authorization, X-Request-Id.
	AllowedHeaders []string

	// AllowCredentials adds Access-Control-Allow-Credentials: true. Forbidden
	// when AllowedOrigins contains "*" — the browser ignores it anyway.
	AllowCredentials bool

	// MaxAgeSeconds caches the preflight response. Defaults to 600 (10 min).
	MaxAgeSeconds int
}

// CORS wraps h with cross-origin request handling. Reflects the inbound
// Origin header only if it appears in AllowedOrigins; otherwise the headers
// are not set and the browser will reject the cross-origin response.
//
// Preflight (OPTIONS) requests for an allowed origin terminate here with
// 204 No Content; non-OPTIONS requests pass through to h with the
// Access-Control-Allow-Origin header set on the response.
func CORS(h http.Handler, opts CORSOptions) http.Handler {
	allowed := make(map[string]bool, len(opts.AllowedOrigins))
	wildcard := false
	for _, o := range opts.AllowedOrigins {
		if o == "*" {
			wildcard = true
		}
		allowed[o] = true
	}

	methods := strings.Join(defaultStr(opts.AllowedMethods,
		[]string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}), ", ")
	headers := strings.Join(defaultStr(opts.AllowedHeaders,
		[]string{"Content-Type", "Authorization", "X-Request-Id"}), ", ")
	maxAge := opts.MaxAgeSeconds
	if maxAge <= 0 {
		maxAge = 600
	}
	maxAgeStr := itoa(maxAge)
	credentials := opts.AllowCredentials && !wildcard

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (wildcard || allowed[origin]) {
			if wildcard && !credentials {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}
			if credentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}

		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			// Preflight
			if origin != "" && (wildcard || allowed[origin]) {
				w.Header().Set("Access-Control-Allow-Methods", methods)
				w.Header().Set("Access-Control-Allow-Headers", headers)
				w.Header().Set("Access-Control-Max-Age", maxAgeStr)
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		h.ServeHTTP(w, r)
	})
}

func defaultStr(v, def []string) []string {
	if len(v) == 0 {
		return def
	}
	return v
}
