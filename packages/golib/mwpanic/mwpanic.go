// Package mwpanic provides lightweight panic recovery middleware.
// Alternative to mwrecover with minimal allocations and configurable
// error handler for production API servers.
package mwpanic

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Handler is called when a panic is recovered.
type Handler func(w http.ResponseWriter, r *http.Request, err interface{})

// DefaultHandler logs the panic and returns 500.
func DefaultHandler(w http.ResponseWriter, r *http.Request, err interface{}) {
	stack := debug.Stack()
	slog.Error("panic recovered",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("panic", fmt.Sprintf("%v", err)),
		slog.String("stack", string(stack)),
	)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprint(w, `{"error":"internal_server_error","message":"an unexpected error occurred"}`)
}

// Middleware returns panic recovery middleware with a custom handler.
func Middleware(handler Handler) func(http.Handler) http.Handler {
	if handler == nil {
		handler = DefaultHandler
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					handler(w, r, err)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Simple returns middleware with the default panic handler.
func Simple(next http.Handler) http.Handler {
	return Middleware(nil)(next)
}
