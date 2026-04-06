// Package mwchain provides composable middleware chain construction
// with named stages, conditional application, and introspection.
// Designed for building standardized middleware stacks across services.
package mwchain

import (
	"net/http"
)

// Middleware is a standard HTTP middleware function.
type Middleware func(http.Handler) http.Handler

// Stage represents a named middleware in the chain.
type Stage struct {
	Name       string
	Middleware Middleware
	Enabled    bool
}

// Chain builds and manages a middleware chain.
type Chain struct {
	stages []Stage
}

// New creates an empty middleware chain.
func New() *Chain {
	return &Chain{}
}

// Use adds a middleware to the chain.
func (c *Chain) Use(name string, mw Middleware) *Chain {
	c.stages = append(c.stages, Stage{
		Name:       name,
		Middleware: mw,
		Enabled:    true,
	})
	return c
}

// UseIf adds a middleware that is only applied if condition is true.
func (c *Chain) UseIf(name string, condition bool, mw Middleware) *Chain {
	c.stages = append(c.stages, Stage{
		Name:       name,
		Middleware: mw,
		Enabled:    condition,
	})
	return c
}

// Then applies the chain to a final handler.
func (c *Chain) Then(handler http.Handler) http.Handler {
	// Apply in reverse order so first Use() is outermost.
	for i := len(c.stages) - 1; i >= 0; i-- {
		if c.stages[i].Enabled {
			handler = c.stages[i].Middleware(handler)
		}
	}
	return handler
}

// ThenFunc applies the chain to a handler function.
func (c *Chain) ThenFunc(fn http.HandlerFunc) http.Handler {
	return c.Then(fn)
}

// Names returns the names of all enabled stages in order.
func (c *Chain) Names() []string {
	names := make([]string, 0, len(c.stages))
	for _, s := range c.stages {
		if s.Enabled {
			names = append(names, s.Name)
		}
	}
	return names
}

// Len returns the total number of stages (including disabled).
func (c *Chain) Len() int {
	return len(c.stages)
}

// EnabledCount returns the number of enabled stages.
func (c *Chain) EnabledCount() int {
	count := 0
	for _, s := range c.stages {
		if s.Enabled {
			count++
		}
	}
	return count
}

// Merge combines two chains. The other chain's stages are appended.
func (c *Chain) Merge(other *Chain) *Chain {
	merged := &Chain{
		stages: make([]Stage, 0, len(c.stages)+len(other.stages)),
	}
	merged.stages = append(merged.stages, c.stages...)
	merged.stages = append(merged.stages, other.stages...)
	return merged
}

// Standard returns a pre-built enterprise middleware chain.
// Pass nil for any middleware you don't want to include.
func Standard(
	recovery Middleware,
	requestID Middleware,
	logging Middleware,
	security Middleware,
	timeout Middleware,
) *Chain {
	c := New()
	if recovery != nil {
		c.Use("recovery", recovery)
	}
	if requestID != nil {
		c.Use("request_id", requestID)
	}
	if logging != nil {
		c.Use("logging", logging)
	}
	if security != nil {
		c.Use("security", security)
	}
	if timeout != nil {
		c.Use("timeout", timeout)
	}
	return c
}
