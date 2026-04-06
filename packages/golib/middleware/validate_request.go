package middleware

import (
	"net/http"
	"strings"
)

// MaxBodySize returns middleware that limits request body size.
// Requests with a body exceeding maxBytes receive a 413 Payload Too Large response.
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireContentType returns middleware that requires a specific Content-Type header.
// GET, HEAD, OPTIONS, and DELETE requests skip the check.
// Requests with a mismatched Content-Type receive a 415 Unsupported Media Type response.
func RequireContentType(contentType string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodDelete:
				next.ServeHTTP(w, r)
				return
			}

			ct := r.Header.Get("Content-Type")
			// Compare the media type portion, ignoring parameters like charset
			mediaType := strings.SplitN(ct, ";", 2)[0]
			mediaType = strings.TrimSpace(mediaType)

			if !strings.EqualFold(mediaType, contentType) {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusUnsupportedMediaType)
				w.Write([]byte(`{"error":"unsupported_media_type","message":"Content-Type must be ` + contentType + `"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireHeader returns middleware that requires a specific header to be present.
// Requests missing the header receive a 400 Bad Request response.
func RequireHeader(header string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(header) == "" {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"missing_header","message":"required header ` + header + ` is missing"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
