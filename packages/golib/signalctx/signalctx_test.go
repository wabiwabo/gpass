package signalctx

import (
	"context"
	"syscall"
	"testing"
	"time"
)

func TestWithShutdown(t *testing.T) {
	ctx, cancel := WithShutdown(context.Background())
	defer cancel()

	select {
	case <-ctx.Done():
		t.Fatal("context should not be done yet")
	default:
		// good
	}

	cancel()

	select {
	case <-ctx.Done():
		// good
	case <-time.After(time.Second):
		t.Fatal("context should be done after cancel")
	}
}

func TestWithSignals(t *testing.T) {
	ctx, cancel := WithSignals(context.Background(), syscall.SIGUSR1)
	defer cancel()

	// Send SIGUSR1 to self
	go func() {
		time.Sleep(10 * time.Millisecond)
		syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)
	}()

	select {
	case <-ctx.Done():
		// good — signal received
	case <-time.After(2 * time.Second):
		t.Fatal("context should have been cancelled by signal")
	}
}

func TestWithShutdownParentCancel(t *testing.T) {
	parent, parentCancel := context.WithCancel(context.Background())
	ctx, cancel := WithShutdown(parent)
	defer cancel()

	parentCancel()

	select {
	case <-ctx.Done():
		// good — parent cancel propagates
	case <-time.After(time.Second):
		t.Fatal("child context should be done when parent is cancelled")
	}
}

func TestWithShutdownCleanup(t *testing.T) {
	ctx, cancel := WithShutdown(context.Background())
	cancel()

	select {
	case <-ctx.Done():
		// good
	case <-time.After(time.Second):
		t.Fatal("context should be done after cleanup")
	}
}
