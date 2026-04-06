package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// RequestID generates a unique request ID for each incoming request and
// propagates it via context and the X-Request-Id response header.
// If the incoming request already has an X-Request-Id header (from a
// reverse proxy like Kong), it is reused.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-Id")
		if rid == "" {
			rid = generateRequestID()
		}

		w.Header().Set("X-Request-Id", rid)
		ctx := context.WithValue(r.Context(), requestIDKey, rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID extracts the request ID from the context.
func GetRequestID(ctx context.Context) string {
	if rid, ok := ctx.Value(requestIDKey).(string); ok {
		return rid
	}
	return ""
}

func generateRequestID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
