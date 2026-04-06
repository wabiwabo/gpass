package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync/atomic"
	"time"
)

type ctxKey struct{}

// Generate creates a new unique request ID.
// Format: timestamp(8 hex) + random(16 hex) = 24 chars.
func Generate() string {
	ts := time.Now().UnixMilli()
	tsHex := hex.EncodeToString([]byte{
		byte(ts >> 24), byte(ts >> 16), byte(ts >> 8), byte(ts),
	})

	random := make([]byte, 8)
	rand.Read(random)

	return tsHex + hex.EncodeToString(random)
}

// FromContext extracts the request ID from context.
func FromContext(ctx context.Context) string {
	if id, ok := ctx.Value(ctxKey{}).(string); ok {
		return id
	}
	return ""
}

// ToContext stores a request ID in context.
func ToContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromRequest extracts the request ID from the X-Request-Id header.
func FromRequest(r *http.Request) string {
	return r.Header.Get("X-Request-Id")
}

// Middleware injects a request ID into the request context and response headers.
// If the request already has X-Request-Id, it's preserved; otherwise a new one is generated.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = Generate()
			r.Header.Set("X-Request-Id", id)
		}

		ctx := ToContext(r.Context(), id)
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// SequentialGenerator generates sequential IDs with a prefix. Useful for testing.
type SequentialGenerator struct {
	prefix string
	seq    atomic.Int64
}

// NewSequentialGenerator creates a sequential ID generator.
func NewSequentialGenerator(prefix string) *SequentialGenerator {
	return &SequentialGenerator{prefix: prefix}
}

// Next returns the next sequential ID.
func (g *SequentialGenerator) Next() string {
	n := g.seq.Add(1)
	return g.prefix + "-" + hex.EncodeToString([]byte{
		byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n),
	})
}
