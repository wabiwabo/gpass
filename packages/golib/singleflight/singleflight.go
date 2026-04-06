package singleflight

import (
	"context"
	"sync"
	"sync/atomic"
)

// Stats holds counters for singleflight usage.
type Stats struct {
	TotalCalls    int64
	SharedResults int64
	InFlightCount int64
}

// call represents an in-flight or completed function call.
type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error

	// Number of callers waiting (including the originator).
	dups int64
}

// Group manages deduplication of concurrent function calls.
type Group struct {
	mu    sync.Mutex
	calls map[string]*call

	totalCalls    atomic.Int64
	sharedResults atomic.Int64
}

// Do executes fn if no in-flight call exists for key, otherwise waits for
// the existing call to complete and returns its result. The shared return
// value is true if the result was produced by an earlier call.
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error, bool) {
	g.totalCalls.Add(1)

	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[string]*call)
	}

	if c, ok := g.calls[key]; ok {
		c.dups++
		g.mu.Unlock()
		c.wg.Wait()
		g.sharedResults.Add(1)
		return c.val, c.err, true
	}

	c := &call{}
	c.dups = 1
	c.wg.Add(1)
	g.calls[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()

	return c.val, c.err, false
}

// DoWithContext is a context-aware version of Do. If the context is cancelled
// before the function completes, the waiting caller receives the context error.
// The underlying function call still runs to completion for other waiters.
func (g *Group) DoWithContext(ctx context.Context, key string, fn func(context.Context) (interface{}, error)) (interface{}, error, bool) {
	g.totalCalls.Add(1)

	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[string]*call)
	}

	if c, ok := g.calls[key]; ok {
		c.dups++
		g.mu.Unlock()

		// Wait for either context cancellation or the call to finish.
		done := make(chan struct{})
		go func() {
			c.wg.Wait()
			close(done)
		}()

		select {
		case <-ctx.Done():
			return nil, ctx.Err(), true
		case <-done:
			g.sharedResults.Add(1)
			return c.val, c.err, true
		}
	}

	c := &call{}
	c.dups = 1
	c.wg.Add(1)
	g.calls[key] = c
	g.mu.Unlock()

	c.val, c.err = fn(ctx)
	c.wg.Done()

	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()

	return c.val, c.err, false
}

// Forget removes a key from the in-flight map, allowing the next caller
// to start a new call even if the previous one has not yet completed.
func (g *Group) Forget(key string) {
	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()
}

// Stats returns a snapshot of the group's usage statistics.
func (g *Group) Stats() Stats {
	g.mu.Lock()
	inFlight := int64(len(g.calls))
	g.mu.Unlock()

	return Stats{
		TotalCalls:    g.totalCalls.Load(),
		SharedResults: g.sharedResults.Load(),
		InFlightCount: inFlight,
	}
}
