// Package mwxframe provides X-Frame-Options middleware.
// Prevents clickjacking by controlling iframe embedding.
// Supports DENY and SAMEORIGIN directives.
package mwxframe

import "net/http"

// Deny returns middleware that sets X-Frame-Options: DENY.
func Deny(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// SameOrigin returns middleware that sets X-Frame-Options: SAMEORIGIN.
func SameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		next.ServeHTTP(w, r)
	})
}
