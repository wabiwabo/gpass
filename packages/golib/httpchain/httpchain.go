// Package httpchain provides a fluent HTTP handler builder that
// combines routing, middleware, and error handling into a single
// composable chain. Designed for building service route tables.
package httpchain

import (
	"net/http"
)

// Route defines a single HTTP route.
type Route struct {
	Method      string
	Pattern     string
	Handler     http.HandlerFunc
	Middleware  []func(http.Handler) http.Handler
	Description string
}

// Router builds a route table with per-route middleware.
type Router struct {
	prefix     string
	routes     []Route
	middleware []func(http.Handler) http.Handler
}

// NewRouter creates a new route builder.
func NewRouter(prefix string) *Router {
	return &Router{prefix: prefix}
}

// Use adds middleware to all routes in this router.
func (r *Router) Use(mw func(http.Handler) http.Handler) *Router {
	r.middleware = append(r.middleware, mw)
	return r
}

// GET registers a GET route.
func (r *Router) GET(pattern string, handler http.HandlerFunc, mw ...func(http.Handler) http.Handler) *Router {
	return r.add("GET", pattern, handler, mw)
}

// POST registers a POST route.
func (r *Router) POST(pattern string, handler http.HandlerFunc, mw ...func(http.Handler) http.Handler) *Router {
	return r.add("POST", pattern, handler, mw)
}

// PUT registers a PUT route.
func (r *Router) PUT(pattern string, handler http.HandlerFunc, mw ...func(http.Handler) http.Handler) *Router {
	return r.add("PUT", pattern, handler, mw)
}

// PATCH registers a PATCH route.
func (r *Router) PATCH(pattern string, handler http.HandlerFunc, mw ...func(http.Handler) http.Handler) *Router {
	return r.add("PATCH", pattern, handler, mw)
}

// DELETE registers a DELETE route.
func (r *Router) DELETE(pattern string, handler http.HandlerFunc, mw ...func(http.Handler) http.Handler) *Router {
	return r.add("DELETE", pattern, handler, mw)
}

func (r *Router) add(method, pattern string, handler http.HandlerFunc, mw []func(http.Handler) http.Handler) *Router {
	r.routes = append(r.routes, Route{
		Method:     method,
		Pattern:    r.prefix + pattern,
		Handler:    handler,
		Middleware: mw,
	})
	return r
}

// Mount registers all routes on the given ServeMux.
func (r *Router) Mount(mux *http.ServeMux) {
	for _, route := range r.routes {
		var handler http.Handler = route.Handler

		// Apply per-route middleware (innermost first).
		for i := len(route.Middleware) - 1; i >= 0; i-- {
			handler = route.Middleware[i](handler)
		}

		// Apply router-level middleware.
		for i := len(r.middleware) - 1; i >= 0; i-- {
			handler = r.middleware[i](handler)
		}

		pattern := route.Method + " " + route.Pattern
		mux.Handle(pattern, handler)
	}
}

// Routes returns all registered routes.
func (r *Router) Routes() []Route {
	out := make([]Route, len(r.routes))
	copy(out, r.routes)
	return out
}

// Count returns the number of registered routes.
func (r *Router) Count() int {
	return len(r.routes)
}

// Group creates a sub-router with an additional path prefix.
func (r *Router) Group(prefix string) *Router {
	sub := NewRouter(r.prefix + prefix)
	// Inherit parent middleware.
	sub.middleware = append(sub.middleware, r.middleware...)
	return sub
}
