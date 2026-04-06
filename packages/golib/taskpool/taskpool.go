// Package taskpool provides a bounded task pool for executing
// functions concurrently with a configurable number of workers.
// Unlike worker (which processes items from a channel), taskpool
// lets you submit individual tasks and collect results.
package taskpool

import (
	"context"
	"sync"
	"sync/atomic"
)

// Result holds the outcome of a task execution.
type Result[T any] struct {
	Value T
	Err   error
	Index int // Original submission order.
}

// Pool manages concurrent task execution.
type Pool[T any] struct {
	workers int
	tasks   chan task[T]
	results chan Result[T]
	wg      sync.WaitGroup
	closed  atomic.Bool

	submitted atomic.Int64
	completed atomic.Int64
	failed    atomic.Int64
}

type task[T any] struct {
	fn    func(ctx context.Context) (T, error)
	index int
}

// New creates a task pool with the given number of workers.
func New[T any](workers int) *Pool[T] {
	if workers <= 0 {
		workers = 1
	}

	p := &Pool[T]{
		workers: workers,
		tasks:   make(chan task[T], workers*2),
		results: make(chan Result[T], workers*2),
	}

	return p
}

// Start launches the worker goroutines.
func (p *Pool[T]) Start(ctx context.Context) {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx)
	}
}

func (p *Pool[T]) worker(ctx context.Context) {
	defer p.wg.Done()
	for t := range p.tasks {
		select {
		case <-ctx.Done():
			var zero T
			p.results <- Result[T]{Value: zero, Err: ctx.Err(), Index: t.index}
			p.failed.Add(1)
		default:
			value, err := t.fn(ctx)
			if err != nil {
				p.failed.Add(1)
			}
			p.completed.Add(1)
			p.results <- Result[T]{Value: value, Err: err, Index: t.index}
		}
	}
}

// Submit adds a task to the pool.
func (p *Pool[T]) Submit(fn func(ctx context.Context) (T, error)) {
	idx := int(p.submitted.Add(1)) - 1
	p.tasks <- task[T]{fn: fn, index: idx}
}

// Close signals that no more tasks will be submitted.
// Call this before collecting results.
func (p *Pool[T]) Close() {
	if p.closed.Swap(true) {
		return
	}
	close(p.tasks)
}

// Wait waits for all workers to finish and returns all results.
func (p *Pool[T]) Wait() []Result[T] {
	p.Close()
	p.wg.Wait()
	close(p.results)

	var results []Result[T]
	for r := range p.results {
		results = append(results, r)
	}
	return results
}

// Stats returns execution statistics.
func (p *Pool[T]) Stats() (submitted, completed, failed int64) {
	return p.submitted.Load(), p.completed.Load(), p.failed.Load()
}

// Workers returns the number of workers.
func (p *Pool[T]) Workers() int {
	return p.workers
}
