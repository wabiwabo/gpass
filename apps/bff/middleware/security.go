package middleware

import (
	"net/http"
)

// SecurityHeaders adds OWASP-recommended HTTP security headers to all responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()

		// Prevent MIME-sniffing
		h.Set("X-Content-Type-Options", "nosniff")

		// Clickjacking protection
		h.Set("X-Frame-Options", "DENY")

		// XSS filter (legacy browsers)
		h.Set("X-XSS-Protection", "1; mode=block")

		// Referrer policy — only send origin for cross-origin requests
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions policy — disable unnecessary browser APIs
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")

		// Cache control for API responses
		h.Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
		h.Set("Pragma", "no-cache")

		next.ServeHTTP(w, r)
	})
}
