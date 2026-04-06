// Package signalx provides OS signal handling utilities.
// Wraps signal notification with context integration, making
// it easy to handle graceful shutdown in services.
package signalx

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// WaitForShutdown blocks until SIGINT or SIGTERM is received.
// Returns a context that is cancelled when the signal arrives.
func WaitForShutdown() context.Context {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		stop()
	}()
	return ctx
}

// OnSignal calls the function when one of the specified signals is received.
func OnSignal(fn func(os.Signal), signals ...os.Signal) func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, signals...)

	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case sig := <-ch:
			fn(sig)
		case <-done:
		}
	}()

	return func() {
		signal.Stop(ch)
		close(ch)
	}
}

// ShutdownSignals returns the standard shutdown signals.
func ShutdownSignals() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM}
}

// ReloadSignal returns SIGHUP, commonly used for config reload.
func ReloadSignal() os.Signal {
	return syscall.SIGHUP
}
