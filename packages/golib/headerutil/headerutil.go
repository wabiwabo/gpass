// Package headerutil provides HTTP header manipulation utilities.
// Extracts, parses, and validates common header values with
// type-safe accessors for Bearer tokens, content types, and IPs.
package headerutil

import (
	"net"
	"net/http"
	"strconv"
	"strings"
)

// BearerToken extracts a Bearer token from the Authorization header.
// Returns empty string if not present or not a Bearer token.
func BearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(auth[len(prefix):])
}

// ContentType extracts the media type without parameters.
func ContentType(r *http.Request) string {
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		return ""
	}
	idx := strings.IndexByte(ct, ';')
	if idx >= 0 {
		ct = ct[:idx]
	}
	return strings.TrimSpace(strings.ToLower(ct))
}

// IsJSON checks if the Content-Type is JSON.
func IsJSON(r *http.Request) bool {
	ct := ContentType(r)
	return ct == "application/json" || strings.HasSuffix(ct, "+json")
}

// AcceptsJSON checks if the client accepts JSON responses.
func AcceptsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if accept == "" {
		return true // default accepts anything
	}
	return strings.Contains(accept, "application/json") ||
		strings.Contains(accept, "*/*")
}

// RealIP extracts the client's real IP from headers or RemoteAddr.
// Checks X-Forwarded-For and X-Real-IP before falling back.
func RealIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first (client) IP
		if idx := strings.IndexByte(xff, ','); idx >= 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RequestID extracts the request ID from common headers.
func RequestID(r *http.Request) string {
	for _, h := range []string{"X-Request-ID", "X-Correlation-ID", "X-Trace-ID"} {
		if v := r.Header.Get(h); v != "" {
			return v
		}
	}
	return ""
}

// ContentLength returns the parsed Content-Length or -1 if not set/invalid.
func ContentLength(r *http.Request) int64 {
	cl := r.Header.Get("Content-Length")
	if cl == "" {
		return -1
	}
	n, err := strconv.ParseInt(cl, 10, 64)
	if err != nil || n < 0 {
		return -1
	}
	return n
}

// SetJSON sets Content-Type to application/json.
func SetJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
}

// SetNoCache sets headers to prevent caching.
func SetNoCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}

// SetNoSniff sets X-Content-Type-Options: nosniff.
func SetNoSniff(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

// CopyHeaders copies headers from src to dst.
func CopyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
