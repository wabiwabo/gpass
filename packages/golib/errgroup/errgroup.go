// Package errgroup provides a bounded error group for running
// concurrent tasks with a concurrency limit. Collects all errors
// and supports context cancellation on first error.
package errgroup

import (
	"context"
	"sync"
)

// Group manages a collection of goroutines with error collection.
type Group struct {
	wg     sync.WaitGroup
	mu     sync.Mutex
	errs   []error
	sem    chan struct{}
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a Group with the given concurrency limit.
// If limit <= 0, there is no limit.
func New(limit int) *Group {
	g := &Group{}
	if limit > 0 {
		g.sem = make(chan struct{}, limit)
	}
	return g
}

// WithContext creates a Group that cancels on first error.
func WithContext(ctx context.Context, limit int) (*Group, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	g := &Group{
		ctx:    ctx,
		cancel: cancel,
	}
	if limit > 0 {
		g.sem = make(chan struct{}, limit)
	}
	return g, ctx
}

// Go runs a function in a new goroutine.
func (g *Group) Go(fn func() error) {
	g.wg.Add(1)

	go func() {
		defer g.wg.Done()

		if g.sem != nil {
			g.sem <- struct{}{}
			defer func() { <-g.sem }()
		}

		// Check if context is already cancelled
		if g.ctx != nil {
			select {
			case <-g.ctx.Done():
				return
			default:
			}
		}

		if err := fn(); err != nil {
			g.mu.Lock()
			g.errs = append(g.errs, err)
			g.mu.Unlock()

			if g.cancel != nil {
				g.cancel()
			}
		}
	}()
}

// Wait blocks until all goroutines complete.
func (g *Group) Wait() []error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel()
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.errs
}

// WaitFirst blocks and returns the first error (or nil).
func (g *Group) WaitFirst() error {
	errs := g.Wait()
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Errors returns collected errors so far (non-blocking).
func (g *Group) Errors() []error {
	g.mu.Lock()
	defer g.mu.Unlock()
	result := make([]error, len(g.errs))
	copy(result, g.errs)
	return result
}
