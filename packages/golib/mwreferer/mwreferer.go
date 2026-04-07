// Package mwreferer provides Referrer-Policy header middleware.
// Sets browser referrer policy to control how much referrer
// information is shared with cross-origin requests.
package mwreferer

import "net/http"

// Standard Referrer-Policy values per W3C.
const (
	NoReferrer                  = "no-referrer"
	NoReferrerWhenDowngrade     = "no-referrer-when-downgrade"
	Origin                      = "origin"
	OriginWhenCrossOrigin       = "origin-when-cross-origin"
	SameOrigin                  = "same-origin"
	StrictOrigin                = "strict-origin"
	StrictOriginWhenCrossOrigin = "strict-origin-when-cross-origin"
	UnsafeURL                   = "unsafe-url"
)

// Middleware returns a middleware that sets Referrer-Policy.
func Middleware(policy string) func(http.Handler) http.Handler {
	if policy == "" {
		policy = StrictOriginWhenCrossOrigin
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Referrer-Policy", policy)
			next.ServeHTTP(w, r)
		})
	}
}

// Strict returns middleware with the strict-origin-when-cross-origin policy.
func Strict(next http.Handler) http.Handler {
	return Middleware(StrictOriginWhenCrossOrigin)(next)
}

// None returns middleware with the no-referrer policy.
func None(next http.Handler) http.Handler {
	return Middleware(NoReferrer)(next)
}
