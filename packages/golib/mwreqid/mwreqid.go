// Package mwreqid provides request ID middleware for HTTP handlers.
// Generates or propagates unique request IDs for distributed tracing
// and log correlation across microservices.
package mwreqid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type contextKey int

const reqIDKey contextKey = iota

// HeaderName is the default request ID header.
const HeaderName = "X-Request-ID"

// Generate creates a random request ID (16 hex characters).
func Generate() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Middleware adds a request ID to each request.
// Uses existing X-Request-ID header if present, otherwise generates one.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderName)
		if id == "" {
			id = Generate()
		}
		w.Header().Set(HeaderName, id)
		ctx := context.WithValue(r.Context(), reqIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext extracts the request ID from context.
func FromContext(ctx context.Context) string {
	if id, ok := ctx.Value(reqIDKey).(string); ok {
		return id
	}
	return ""
}

// FromRequest extracts the request ID from a request's context.
func FromRequest(r *http.Request) string {
	return FromContext(r.Context())
}
