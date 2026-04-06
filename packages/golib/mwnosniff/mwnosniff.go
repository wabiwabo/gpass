// Package mwnosniff provides X-Content-Type-Options middleware.
// Sets nosniff header to prevent MIME type sniffing attacks.
// Part of OWASP recommended security headers.
package mwnosniff

import "net/http"

// Middleware sets X-Content-Type-Options: nosniff on all responses.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
