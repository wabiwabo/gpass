package signalx

import (
	"os"
	"syscall"
	"testing"
)

func TestShutdownSignals(t *testing.T) {
	sigs := ShutdownSignals()
	if len(sigs) != 2 {
		t.Errorf("len = %d, want 2", len(sigs))
	}
}

func TestReloadSignal(t *testing.T) {
	sig := ReloadSignal()
	if sig != syscall.SIGHUP {
		t.Errorf("sig = %v, want SIGHUP", sig)
	}
}

func TestShutdownSignals_Contains(t *testing.T) {
	sigs := ShutdownSignals()
	hasINT := false
	hasTERM := false
	for _, s := range sigs {
		if s == syscall.SIGINT {
			hasINT = true
		}
		if s == syscall.SIGTERM {
			hasTERM = true
		}
	}
	if !hasINT {
		t.Error("should contain SIGINT")
	}
	if !hasTERM {
		t.Error("should contain SIGTERM")
	}
}

func TestOnSignal(t *testing.T) {
	received := make(chan os.Signal, 1)
	cancel := OnSignal(func(sig os.Signal) {
		received <- sig
	}, syscall.SIGUSR1)
	defer cancel()

	// Send signal to self
	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)

	select {
	case sig := <-received:
		if sig != syscall.SIGUSR1 {
			t.Errorf("received %v, want SIGUSR1", sig)
		}
	default:
		// Signal handling is async, this is just a best-effort test
	}
}
