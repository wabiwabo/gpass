// Package mwtrailing provides trailing slash normalization middleware.
// Redirects or strips trailing slashes from URLs for consistent
// routing behavior across all API endpoints.
package mwtrailing

import (
	"net/http"
	"strings"
)

// Strip returns middleware that removes trailing slashes via redirect.
func Strip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && strings.HasSuffix(r.URL.Path, "/") {
			target := strings.TrimRight(r.URL.Path, "/")
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Add returns middleware that adds trailing slashes via redirect.
func Add(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && !strings.HasSuffix(r.URL.Path, "/") {
			// Don't add slash to paths with file extensions
			lastSegment := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			if !strings.Contains(lastSegment, ".") {
				target := r.URL.Path + "/"
				if r.URL.RawQuery != "" {
					target += "?" + r.URL.RawQuery
				}
				http.Redirect(w, r, target, http.StatusMovedPermanently)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// StripInPlace strips trailing slash without redirect (rewrites path).
func StripInPlace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && strings.HasSuffix(r.URL.Path, "/") {
			r.URL.Path = strings.TrimRight(r.URL.Path, "/")
		}
		next.ServeHTTP(w, r)
	})
}
