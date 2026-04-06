// Package mwnotfound provides custom 404 handler middleware.
// Returns JSON-formatted not found responses for API servers
// instead of the default Go plaintext response.
package mwnotfound

import (
	"fmt"
	"net/http"
)

// JSON returns a handler that responds with JSON 404.
func JSON() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error":"not_found","message":"the requested resource was not found","path":"%s"}`, r.URL.Path)
	})
}

// Handler returns a custom 404 handler with the given message.
func Handler(message string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error":"not_found","message":"%s"}`, message)
	})
}

// Middleware wraps a mux and handles 404s with JSON response.
func Middleware(mux http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use a response recorder to check if the mux wrote anything
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		mux.ServeHTTP(rec, r)
		// If the mux returned 404 and hasn't written a body, write JSON
		if rec.status == http.StatusNotFound && !rec.bodyWritten {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"error":"not_found","message":"the requested resource was not found","path":"%s"}`, r.URL.Path)
		}
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	bodyWritten bool
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.bodyWritten = true
	return r.ResponseWriter.Write(b)
}
