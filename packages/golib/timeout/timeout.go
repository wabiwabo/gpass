// Package timeout provides per-route HTTP timeout middleware.
// It wraps requests with context deadlines and returns 504 Gateway
// Timeout if the handler doesn't complete in time.
package timeout

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Middleware returns HTTP middleware that enforces a timeout on requests.
func Middleware(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()

			tw := &timeoutWriter{
				ResponseWriter: w,
				done:           make(chan struct{}),
			}

			go func() {
				next.ServeHTTP(tw, r.WithContext(ctx))
				close(tw.done)
			}()

			select {
			case <-tw.done:
				// Handler completed.
			case <-ctx.Done():
				tw.mu.Lock()
				defer tw.mu.Unlock()
				if !tw.wroteHeader {
					tw.wroteHeader = true
					w.Header().Set("Content-Type", "application/problem+json")
					w.WriteHeader(http.StatusGatewayTimeout)
					w.Write([]byte(`{"type":"about:blank","title":"Gateway Timeout","status":504,"detail":"Request timed out"}`))
				}
			}
		})
	}
}

type timeoutWriter struct {
	http.ResponseWriter
	mu          sync.Mutex
	wroteHeader bool
	done        chan struct{}
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.wroteHeader {
		return
	}
	tw.wroteHeader = true
	tw.ResponseWriter.WriteHeader(code)
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.wroteHeader {
		return tw.ResponseWriter.Write(b)
	}
	tw.wroteHeader = true
	return tw.ResponseWriter.Write(b)
}
