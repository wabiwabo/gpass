package signalctx

import (
	"syscall"
	"testing"
	"time"
)

// TestWaitForSignal_DeliveredSignal sends SIGUSR1 to the test process and
// asserts WaitForSignal returns it. SIGUSR1 is chosen because it has no
// default action that would crash the test runner.
func TestWaitForSignal_DeliveredSignal(t *testing.T) {
	done := make(chan syscall.Signal, 1)
	go func() {
		sig := WaitForSignal(syscall.SIGUSR1)
		done <- sig.(syscall.Signal)
	}()

	// Give the goroutine a beat to register the signal handler.
	time.Sleep(50 * time.Millisecond)
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGUSR1); err != nil {
		t.Fatalf("kill self: %v", err)
	}

	select {
	case got := <-done:
		if got != syscall.SIGUSR1 {
			t.Errorf("got %v, want SIGUSR1", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForSignal did not return")
	}
}

// TestWaitForShutdown_DeliveredSIGTERM proves the convenience wrapper
// returns when SIGTERM is delivered. We send SIGTERM to ourselves; the
// signal package intercepts it via Notify so the process is not killed.
func TestWaitForShutdown_DeliveredSIGTERM(t *testing.T) {
	done := make(chan struct{})
	go func() {
		_ = WaitForShutdown()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("kill self: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForShutdown did not return")
	}
}
