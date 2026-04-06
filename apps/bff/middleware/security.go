package middleware

import (
	"net/http"
)

// SecurityHeaders adds OWASP-recommended HTTP security headers to all responses.
// When enableHSTS is true, adds Strict-Transport-Security for HTTPS enforcement.
func SecurityHeaders(next http.Handler) http.Handler {
	return SecurityHeadersWithOptions(next, false)
}

// SecurityHeadersWithOptions allows configuring HSTS for production environments.
func SecurityHeadersWithOptions(next http.Handler, enableHSTS bool) http.Handler {
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

		// HSTS — enforce HTTPS for 1 year, include subdomains
		// Critical for identity platforms to prevent SSL stripping attacks
		if enableHSTS {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		next.ServeHTTP(w, r)
	})
}
