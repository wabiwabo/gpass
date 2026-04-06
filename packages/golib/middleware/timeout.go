package middleware

import (
	"context"
	"net/http"
	"time"
)

// Timeout returns middleware that limits request processing time.
// If the handler exceeds the timeout, the request context is cancelled
// and a 503 Service Unavailable is returned.
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()

			done := make(chan struct{})
			tw := &timeoutWriter{ResponseWriter: w}

			go func() {
				next.ServeHTTP(tw, r.WithContext(ctx))
				close(done)
			}()

			select {
			case <-done:
				// Handler completed normally
			case <-ctx.Done():
				if !tw.written {
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusServiceUnavailable)
					w.Write([]byte(`{"error":"request_timeout","message":"request processing exceeded time limit"}`))
				}
			}
		})
	}
}

type timeoutWriter struct {
	http.ResponseWriter
	written bool
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.written = true
	tw.ResponseWriter.WriteHeader(code)
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.written = true
	return tw.ResponseWriter.Write(b)
}
