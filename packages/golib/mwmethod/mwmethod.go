// Package mwmethod provides HTTP method restriction middleware.
// Rejects requests with disallowed HTTP methods, returning
// 405 Method Not Allowed with proper Allow header.
package mwmethod

import (
	"net/http"
	"strings"
)

// Allow returns middleware that only permits the specified HTTP methods.
func Allow(methods ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(methods))
	for _, m := range methods {
		allowed[strings.ToUpper(m)] = true
	}
	allowHeader := strings.Join(methods, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !allowed[r.Method] {
				w.Header().Set("Allow", allowHeader)
				http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GET returns middleware that only allows GET and HEAD.
func GET(next http.Handler) http.Handler {
	return Allow(http.MethodGet, http.MethodHead)(next)
}

// POST returns middleware that only allows POST.
func POST(next http.Handler) http.Handler {
	return Allow(http.MethodPost)(next)
}

// ReadOnly returns middleware that only allows safe methods.
func ReadOnly(next http.Handler) http.Handler {
	return Allow(http.MethodGet, http.MethodHead, http.MethodOptions)(next)
}
