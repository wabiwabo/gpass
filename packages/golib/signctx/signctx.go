// Package signctx provides signal-aware context creation for graceful
// shutdown propagation. When an OS signal (SIGINT, SIGTERM) is received,
// the returned context is canceled, allowing all goroutines holding
// the context to clean up.
package signctx

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// WithSignals creates a context that is canceled when one of the given
// signals is received. If no signals are specified, defaults to SIGINT
// and SIGTERM. Returns the context and a cancel function.
func WithSignals(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
	if len(signals) == 0 {
		signals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	}

	ctx, cancel := context.WithCancel(parent)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, signals...)

	go func() {
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
		}
		signal.Stop(ch)
	}()

	return ctx, cancel
}

// WithTimeout creates a signal-aware context with a maximum timeout.
// The context is canceled on signal OR timeout, whichever comes first.
func WithTimeout(parent context.Context, timeout time.Duration, signals ...os.Signal) (context.Context, context.CancelFunc) {
	ctx, cancel := WithSignals(parent, signals...)
	timerCtx, timerCancel := context.WithTimeout(ctx, timeout)

	return timerCtx, func() {
		timerCancel()
		cancel()
	}
}

// GracefulShutdown manages a two-phase shutdown: first cancel the
// work context, then wait for cleanup with a deadline.
type GracefulShutdown struct {
	workCtx    context.Context
	workCancel context.CancelFunc
	timeout    time.Duration
	mu         sync.Mutex
	hooks      []func(context.Context)
}

// NewGracefulShutdown creates a shutdown manager that cancels workCtx
// on signal and gives `timeout` for cleanup hooks to complete.
func NewGracefulShutdown(timeout time.Duration, signals ...os.Signal) *GracefulShutdown {
	workCtx, workCancel := WithSignals(context.Background(), signals...)

	return &GracefulShutdown{
		workCtx:    workCtx,
		workCancel: workCancel,
		timeout:    timeout,
	}
}

// Context returns the work context. Use this for all application work.
func (g *GracefulShutdown) Context() context.Context {
	return g.workCtx
}

// OnShutdown registers a cleanup hook. Hooks run with a deadline context.
func (g *GracefulShutdown) OnShutdown(fn func(context.Context)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.hooks = append(g.hooks, fn)
}

// Wait blocks until the work context is canceled, then runs hooks.
// Returns after all hooks complete or timeout.
func (g *GracefulShutdown) Wait() {
	<-g.workCtx.Done()

	ctx, cancel := context.WithTimeout(context.Background(), g.timeout)
	defer cancel()

	g.mu.Lock()
	hooks := make([]func(context.Context), len(g.hooks))
	copy(hooks, g.hooks)
	g.mu.Unlock()

	var wg sync.WaitGroup
	for _, hook := range hooks {
		wg.Add(1)
		go func(fn func(context.Context)) {
			defer wg.Done()
			fn(ctx)
		}(hook)
	}
	wg.Wait()
}

// Trigger manually triggers the shutdown (for testing).
func (g *GracefulShutdown) Trigger() {
	g.workCancel()
}

// Done returns a channel that's closed when shutdown is triggered.
func (g *GracefulShutdown) Done() <-chan struct{} {
	return g.workCtx.Done()
}
