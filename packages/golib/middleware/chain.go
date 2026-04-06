package middleware

import "net/http"

// Chain composes multiple middleware into a single middleware.
// Applied in order: first middleware is outermost (executes first).
// Chain(handler, A, B, C) produces: A(B(C(handler)))
func Chain(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	// Apply in reverse so the first middleware is outermost.
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// ChainFunc is like Chain but accepts http.HandlerFunc.
func ChainFunc(handler http.HandlerFunc, middlewares ...func(http.Handler) http.Handler) http.Handler {
	return Chain(handler, middlewares...)
}

// Builder provides a fluent API for building middleware chains.
type Builder struct {
	middlewares []func(http.Handler) http.Handler
}

// NewBuilder creates a new middleware chain builder.
func NewBuilder() *Builder {
	return &Builder{}
}

// Use adds a middleware to the chain.
func (b *Builder) Use(mw func(http.Handler) http.Handler) *Builder {
	b.middlewares = append(b.middlewares, mw)
	return b
}

// UseIf conditionally adds a middleware to the chain.
// The middleware is only added if condition is true.
func (b *Builder) UseIf(condition bool, mw func(http.Handler) http.Handler) *Builder {
	if condition {
		b.middlewares = append(b.middlewares, mw)
	}
	return b
}

// Build composes all added middleware and wraps the given handler.
func (b *Builder) Build(handler http.Handler) http.Handler {
	return Chain(handler, b.middlewares...)
}
