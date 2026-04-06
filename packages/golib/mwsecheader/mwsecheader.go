// Package mwsecheader provides security headers middleware.
// Sets recommended security headers per OWASP guidelines:
// CSP, HSTS, X-Content-Type-Options, Permissions-Policy, etc.
package mwsecheader

import "net/http"

// Config controls which security headers to set.
type Config struct {
	ContentSecurityPolicy string
	StrictTransport       string
	XContentTypeOptions   string
	XFrameOptions         string
	PermissionsPolicy     string
	ReferrerPolicy        string
	CrossOriginOpener     string
	CrossOriginResource   string
}

// DefaultConfig returns OWASP-recommended security headers.
func DefaultConfig() Config {
	return Config{
		ContentSecurityPolicy: "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; frame-ancestors 'none'",
		StrictTransport:       "max-age=63072000; includeSubDomains; preload",
		XContentTypeOptions:   "nosniff",
		XFrameOptions:         "DENY",
		PermissionsPolicy:     "camera=(), microphone=(), geolocation=(), payment=()",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		CrossOriginOpener:     "same-origin",
		CrossOriginResource:   "same-origin",
	}
}

// APIConfig returns security headers suitable for JSON APIs.
func APIConfig() Config {
	return Config{
		ContentSecurityPolicy: "default-src 'none'; frame-ancestors 'none'",
		StrictTransport:       "max-age=63072000; includeSubDomains; preload",
		XContentTypeOptions:   "nosniff",
		XFrameOptions:         "DENY",
		ReferrerPolicy:        "no-referrer",
		CrossOriginOpener:     "same-origin",
		CrossOriginResource:   "same-origin",
	}
}

// Middleware returns security headers middleware.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.ContentSecurityPolicy != "" {
				w.Header().Set("Content-Security-Policy", cfg.ContentSecurityPolicy)
			}
			if cfg.StrictTransport != "" {
				w.Header().Set("Strict-Transport-Security", cfg.StrictTransport)
			}
			if cfg.XContentTypeOptions != "" {
				w.Header().Set("X-Content-Type-Options", cfg.XContentTypeOptions)
			}
			if cfg.XFrameOptions != "" {
				w.Header().Set("X-Frame-Options", cfg.XFrameOptions)
			}
			if cfg.PermissionsPolicy != "" {
				w.Header().Set("Permissions-Policy", cfg.PermissionsPolicy)
			}
			if cfg.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", cfg.ReferrerPolicy)
			}
			if cfg.CrossOriginOpener != "" {
				w.Header().Set("Cross-Origin-Opener-Policy", cfg.CrossOriginOpener)
			}
			if cfg.CrossOriginResource != "" {
				w.Header().Set("Cross-Origin-Resource-Policy", cfg.CrossOriginResource)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Simple returns middleware with default security headers.
func Simple(next http.Handler) http.Handler {
	return Middleware(DefaultConfig())(next)
}
