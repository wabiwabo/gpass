// Package signalctx provides OS signal-aware contexts for graceful
// shutdown. Creates contexts that cancel on SIGINT/SIGTERM for
// clean server termination with configurable signal handling.
package signalctx

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// WithShutdown returns a context that cancels on SIGINT or SIGTERM.
// Call the returned cancel function to clean up signal handling.
func WithShutdown(parent context.Context) (context.Context, context.CancelFunc) {
	return WithSignals(parent, syscall.SIGINT, syscall.SIGTERM)
}

// WithSignals returns a context that cancels on the specified signals.
func WithSignals(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
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

// WaitForShutdown blocks until SIGINT or SIGTERM is received.
func WaitForShutdown() os.Signal {
	return WaitForSignal(syscall.SIGINT, syscall.SIGTERM)
}

// WaitForSignal blocks until one of the specified signals is received.
func WaitForSignal(signals ...os.Signal) os.Signal {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, signals...)
	defer signal.Stop(ch)
	return <-ch
}
