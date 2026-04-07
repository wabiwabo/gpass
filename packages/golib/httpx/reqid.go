package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// HeaderRequestID is the canonical request-id header used by all GarudaPass
// services. Inbound value is preserved if present (trusted from the gateway);
// otherwise a fresh 128-bit random ID is generated.
const HeaderRequestID = "X-Request-Id"

type ctxKey struct{}

var requestIDKey = ctxKey{}

// RequestID middleware ensures every request has an X-Request-Id header
// (preserved or generated) and stashes it in the context for downstream
// handlers + slog correlation. Echoed back in the response header so
// clients can correlate failures with server logs.
func RequestID(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if id == "" || len(id) > 128 {
			id = newRequestID()
		}
		w.Header().Set(HeaderRequestID, id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext returns the request ID stashed by RequestID middleware,
// or empty string if not present.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Crypto/rand failure is essentially impossible; return a fixed
		// marker rather than empty so logs always have *something*.
		return "no-rand"
	}
	return hex.EncodeToString(b[:])
}
