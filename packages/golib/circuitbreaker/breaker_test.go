package circuitbreaker

import (
	"sync"
	"testing"
	"time"
)

func TestStartsClosed(t *testing.T) {
	cb := New(3, time.Second)
	if s := cb.State(); s != StateClosed {
		t.Errorf("expected closed, got %s", s)
	}
	if !cb.Allow() {
		t.Error("expected Allow() to return true when closed")
	}
}

func TestTripsAfterThreshold(t *testing.T) {
	cb := New(3, time.Second)

	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != StateClosed {
		t.Fatal("should still be closed before threshold")
	}

	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatalf("expected open after 3 failures, got %s", cb.State())
	}
	if cb.Allow() {
		t.Error("should not allow when open")
	}
}

func TestCooldownTransitionsToHalfOpen(t *testing.T) {
	cb := New(1, 10*time.Millisecond)

	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatal("expected open")
	}

	time.Sleep(20 * time.Millisecond)

	if !cb.Allow() {
		t.Error("should allow after cooldown (half-open)")
	}
	if cb.State() != StateHalfOpen {
		t.Errorf("expected half-open, got %s", cb.State())
	}
}

func TestSuccessResets(t *testing.T) {
	cb := New(1, 10*time.Millisecond)

	cb.RecordFailure()
	time.Sleep(20 * time.Millisecond)
	cb.Allow() // transition to half-open

	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Errorf("expected closed after success, got %s", cb.State())
	}
	if cb.failures != 0 {
		t.Errorf("expected 0 failures, got %d", cb.failures)
	}
}

func TestConcurrentAccess(t *testing.T) {
	cb := New(100, time.Second)
	var wg sync.WaitGroup

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cb.Allow()
			cb.RecordFailure()
			cb.State()
		}()
	}

	wg.Wait()
	// No race conditions — test passes if it completes without panic.
}
