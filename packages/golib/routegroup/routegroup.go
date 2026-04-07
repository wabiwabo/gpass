// Package routegroup provides HTTP route grouping with shared
// middleware and path prefixes. Organizes routes into logical
// groups for cleaner handler registration.
package routegroup

import (
	"net/http"
	"strings"
)

// Middleware is an HTTP middleware function.
type Middleware func(http.Handler) http.Handler

// Group organizes routes with shared prefix and middleware.
type Group struct {
	prefix     string
	middleware []Middleware
	mux        *http.ServeMux
}

// New creates a route group on the given ServeMux.
func New(mux *http.ServeMux, prefix string, middleware ...Middleware) *Group {
	return &Group{
		prefix:     strings.TrimRight(prefix, "/"),
		middleware: middleware,
		mux:        mux,
	}
}

// Handle registers a handler with the group's prefix and middleware.
//
// pattern may use the Go 1.22+ method-routing syntax ("GET /users").
// In that case the prefix is inserted between the method and the path:
//
//	g := New(mux, "/api")
//	g.Handle("GET /users", h)   // → mux.Handle("GET /api/users", h)
//
// Without a method prefix, the group prefix is simply prepended.
func (g *Group) Handle(pattern string, handler http.Handler) {
	wrapped := g.applyMiddleware(handler)
	g.mux.Handle(g.qualify(pattern), wrapped)
}

// qualify inserts the group prefix into a pattern, handling Go 1.22+
// method-routed patterns ("VERB /path") correctly.
func (g *Group) qualify(pattern string) string {
	if i := strings.Index(pattern, " "); i > 0 && isHTTPMethod(pattern[:i]) {
		return pattern[:i+1] + g.prefix + pattern[i+1:]
	}
	return g.prefix + pattern
}

// isHTTPMethod returns true for the standard verbs the net/http ServeMux
// recognises in method-routed patterns.
func isHTTPMethod(s string) bool {
	switch s {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "CONNECT", "TRACE":
		return true
	}
	return false
}

// HandleFunc registers a handler function.
func (g *Group) HandleFunc(pattern string, fn http.HandlerFunc) {
	g.Handle(pattern, fn)
}

// SubGroup creates a nested group with additional prefix and middleware.
func (g *Group) SubGroup(prefix string, middleware ...Middleware) *Group {
	return &Group{
		prefix:     g.prefix + strings.TrimRight(prefix, "/"),
		middleware: append(g.middleware, middleware...),
		mux:        g.mux,
	}
}

// Use adds middleware to the group.
func (g *Group) Use(mw ...Middleware) {
	g.middleware = append(g.middleware, mw...)
}

// Prefix returns the group's full prefix.
func (g *Group) Prefix() string {
	return g.prefix
}

func (g *Group) applyMiddleware(handler http.Handler) http.Handler {
	h := handler
	for i := len(g.middleware) - 1; i >= 0; i-- {
		h = g.middleware[i](h)
	}
	return h
}

// Route helpers for Go 1.22+ method routing.

// GET registers a GET handler.
func (g *Group) GET(path string, fn http.HandlerFunc) {
	g.Handle("GET "+path, fn)
}

// POST registers a POST handler.
func (g *Group) POST(path string, fn http.HandlerFunc) {
	g.Handle("POST "+path, fn)
}

// PUT registers a PUT handler.
func (g *Group) PUT(path string, fn http.HandlerFunc) {
	g.Handle("PUT "+path, fn)
}

// PATCH registers a PATCH handler.
func (g *Group) PATCH(path string, fn http.HandlerFunc) {
	g.Handle("PATCH "+path, fn)
}

// DELETE registers a DELETE handler.
func (g *Group) DELETE(path string, fn http.HandlerFunc) {
	g.Handle("DELETE "+path, fn)
}
