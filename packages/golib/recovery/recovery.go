// Package recovery provides HTTP panic recovery middleware that
// converts panics into structured error responses instead of
// crashing the server.
package recovery

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Config controls recovery behavior.
type Config struct {
	// Logger for panic logging. Uses slog.Default() if nil.
	Logger *slog.Logger
	// IncludeStack includes stack trace in response (dev only).
	IncludeStack bool
	// OnPanic is called when a panic is recovered. Optional.
	OnPanic func(r *http.Request, err interface{}, stack []byte)
}

// Middleware returns HTTP middleware that recovers from panics.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					stack := debug.Stack()

					logger.Error("panic recovered",
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.String("error", fmt.Sprintf("%v", err)),
					)

					if cfg.OnPanic != nil {
						cfg.OnPanic(r, err, stack)
					}

					w.Header().Set("Content-Type", "application/problem+json")
					w.Header().Set("Cache-Control", "no-store")
					w.WriteHeader(http.StatusInternalServerError)

					response := map[string]interface{}{
						"type":   "about:blank",
						"title":  "Internal Server Error",
						"status": 500,
						"detail": "An unexpected error occurred",
					}

					if cfg.IncludeStack {
						response["stack"] = string(stack)
						response["panic"] = fmt.Sprintf("%v", err)
					}

					json.NewEncoder(w).Encode(response)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// Default returns a recovery middleware with default configuration.
func Default() func(http.Handler) http.Handler {
	return Middleware(Config{})
}
