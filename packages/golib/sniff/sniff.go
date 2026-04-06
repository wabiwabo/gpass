// Package sniff provides content type sniffing prevention middleware
// and helpers. Prevents browsers from MIME-sniffing responses away
// from the declared Content-Type, which can lead to XSS attacks.
package sniff

import (
	"net/http"
)

// NoSniff sets the X-Content-Type-Options: nosniff header.
func NoSniff(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders sets a comprehensive set of security headers.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "0") // Disabled per OWASP (use CSP instead).
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Cache-Control", "no-store")

		next.ServeHTTP(w, r)
	})
}

// HSTS sets Strict-Transport-Security headers for HTTPS enforcement.
func HSTS(maxAge int, includeSubdomains, preload bool) func(http.Handler) http.Handler {
	value := "max-age=" + itoa(maxAge)
	if includeSubdomains {
		value += "; includeSubDomains"
	}
	if preload {
		value += "; preload"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Strict-Transport-Security", value)
			next.ServeHTTP(w, r)
		})
	}
}

// CSP sets Content-Security-Policy header.
func CSP(policy string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Security-Policy", policy)
			next.ServeHTTP(w, r)
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
