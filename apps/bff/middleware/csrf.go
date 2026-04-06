package middleware

import (
	"crypto/subtle"
	"net/http"
)

// CSRF implements the double-submit cookie pattern with constant-time
// token comparison to prevent timing attacks.
//
// Safe methods (GET, HEAD, OPTIONS) are exempt per RFC 7231.
// For unsafe methods, the X-CSRF-Token header must match the gpass_csrf cookie value.
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		headerToken := r.Header.Get("X-CSRF-Token")
		if headerToken == "" {
			http.Error(w, `{"error":"missing_csrf_token","message":"X-CSRF-Token header is required"}`, http.StatusForbidden)
			return
		}

		cookie, err := r.Cookie("gpass_csrf")
		if err != nil || cookie.Value == "" {
			http.Error(w, `{"error":"missing_csrf_cookie","message":"CSRF cookie is missing"}`, http.StatusForbidden)
			return
		}

		// Constant-time comparison prevents timing attacks that could
		// allow an attacker to guess the CSRF token byte-by-byte.
		if subtle.ConstantTimeCompare([]byte(headerToken), []byte(cookie.Value)) != 1 {
			http.Error(w, `{"error":"csrf_mismatch","message":"CSRF token does not match"}`, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}
