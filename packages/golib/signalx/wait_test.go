package signalx

import (
	"syscall"
	"testing"
	"time"
)

// TestWaitForShutdown_CancelsOnSIGTERM closes the only 0%-coverage gap.
// signal.NotifyContext intercepts the signal so the test runner is not
// killed; the returned context is cancelled instead.
func TestWaitForShutdown_CancelsOnSIGTERM(t *testing.T) {
	ctx := WaitForShutdown()

	// Give the NotifyContext goroutine a beat to register.
	time.Sleep(50 * time.Millisecond)
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("kill self: %v", err)
	}

	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForShutdown context never cancelled after SIGTERM")
	}
}
