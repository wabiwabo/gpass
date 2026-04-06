package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"sync"
	"time"
)

// Dedup returns middleware that rejects duplicate requests based on
// a hash of method + path + body + user_id within a time window.
// This prevents accidental double-submits without requiring an Idempotency-Key header.
// Only applies to POST, PUT, and PATCH methods. GET and DELETE are passed through.
// Duplicate requests within the window receive a 409 Conflict response.
func Dedup(window time.Duration) func(http.Handler) http.Handler {
	var mu sync.Mutex
	seen := make(map[string]time.Time)

	// Background cleanup of expired entries.
	go func() {
		ticker := time.NewTicker(window)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			now := time.Now()
			for k, exp := range seen {
				if now.After(exp) {
					delete(seen, k)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only deduplicate mutating methods.
			if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
				next.ServeHTTP(w, r)
				return
			}

			// Read and restore the body.
			var bodyBytes []byte
			if r.Body != nil {
				bodyBytes, _ = io.ReadAll(r.Body)
				r.Body.Close()
				r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}

			// Build the dedup key from method + path + body + user identity.
			h := sha256.New()
			h.Write([]byte(r.Method))
			h.Write([]byte(r.URL.Path))
			h.Write(bodyBytes)
			// Include user identity if available (from X-User-ID header).
			userID := r.Header.Get("X-User-ID")
			h.Write([]byte(userID))
			key := hex.EncodeToString(h.Sum(nil))

			mu.Lock()
			expiry, exists := seen[key]
			now := time.Now()
			if exists && now.Before(expiry) {
				mu.Unlock()
				http.Error(w, "Duplicate request", http.StatusConflict)
				return
			}
			seen[key] = now.Add(window)
			mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}
