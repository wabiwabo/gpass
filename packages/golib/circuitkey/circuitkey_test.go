package circuitkey

import (
	"sync"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Threshold != 5 {
		t.Errorf("Threshold: got %d", cfg.Threshold)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout: got %v", cfg.Timeout)
	}
}

func TestClosedAllows(t *testing.T) {
	r := NewRegistry(Config{Threshold: 3, Timeout: time.Second})
	if !r.Allow("svc-a") {
		t.Error("closed breaker should allow")
	}
}

func TestOpenAfterThreshold(t *testing.T) {
	r := NewRegistry(Config{Threshold: 3, Timeout: time.Second})
	for i := 0; i < 3; i++ {
		r.RecordFailure("svc-a")
	}
	if r.GetState("svc-a") != Open {
		t.Errorf("state: got %v, want open", r.GetState("svc-a"))
	}
	if r.Allow("svc-a") {
		t.Error("open breaker should not allow")
	}
}

func TestHalfOpenAfterTimeout(t *testing.T) {
	r := NewRegistry(Config{Threshold: 2, Timeout: 10 * time.Millisecond, MaxHalfOpen: 1})
	r.RecordFailure("svc-b")
	r.RecordFailure("svc-b")
	if r.GetState("svc-b") != Open {
		t.Fatal("should be open")
	}

	time.Sleep(20 * time.Millisecond)
	if !r.Allow("svc-b") {
		t.Error("should allow after timeout (half-open)")
	}
	if r.GetState("svc-b") != HalfOpen {
		t.Errorf("state: got %v, want half-open", r.GetState("svc-b"))
	}
}

func TestHalfOpenToClosedOnSuccess(t *testing.T) {
	r := NewRegistry(Config{Threshold: 1, Timeout: 10 * time.Millisecond, MaxHalfOpen: 1})
	r.RecordFailure("svc-c")
	time.Sleep(20 * time.Millisecond)
	r.Allow("svc-c") // triggers half-open
	r.RecordSuccess("svc-c")
	if r.GetState("svc-c") != Closed {
		t.Errorf("state: got %v, want closed", r.GetState("svc-c"))
	}
}

func TestHalfOpenToOpenOnFailure(t *testing.T) {
	r := NewRegistry(Config{Threshold: 1, Timeout: 10 * time.Millisecond, MaxHalfOpen: 1})
	r.RecordFailure("svc-d")
	time.Sleep(20 * time.Millisecond)
	r.Allow("svc-d") // triggers half-open
	r.RecordFailure("svc-d")
	if r.GetState("svc-d") != Open {
		t.Errorf("state: got %v, want open", r.GetState("svc-d"))
	}
}

func TestSuccessResetsFailures(t *testing.T) {
	r := NewRegistry(Config{Threshold: 3, Timeout: time.Second})
	r.RecordFailure("svc-e")
	r.RecordFailure("svc-e")
	r.RecordSuccess("svc-e")
	r.RecordFailure("svc-e") // should need 3 more failures
	if r.GetState("svc-e") != Closed {
		t.Error("success should reset failure count")
	}
}

func TestReset(t *testing.T) {
	r := NewRegistry(Config{Threshold: 1, Timeout: time.Minute})
	r.RecordFailure("svc-f")
	if r.GetState("svc-f") != Open {
		t.Fatal("should be open")
	}
	r.Reset("svc-f")
	if r.GetState("svc-f") != Closed {
		t.Error("should be closed after reset")
	}
	if !r.Allow("svc-f") {
		t.Error("should allow after reset")
	}
}

func TestKeys(t *testing.T) {
	r := NewRegistry(DefaultConfig())
	r.Allow("a")
	r.Allow("b")
	r.Allow("c")
	keys := r.Keys()
	if len(keys) != 3 {
		t.Errorf("Keys: got %d, want 3", len(keys))
	}
}

func TestStatus(t *testing.T) {
	r := NewRegistry(Config{Threshold: 2, Timeout: time.Minute})
	r.Allow("healthy")
	r.RecordFailure("failing")
	r.RecordFailure("failing")

	status := r.Status()
	if len(status) != 2 {
		t.Errorf("Status entries: got %d, want 2", len(status))
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{Closed, "closed"},
		{Open, "open"},
		{HalfOpen, "half-open"},
		{State(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestIndependentBreakers(t *testing.T) {
	r := NewRegistry(Config{Threshold: 1, Timeout: time.Minute})
	r.RecordFailure("svc-x")
	if r.GetState("svc-x") != Open {
		t.Error("svc-x should be open")
	}
	if r.GetState("svc-y") != Closed {
		t.Error("svc-y should be closed (independent)")
	}
}

func TestConcurrentAccess(t *testing.T) {
	r := NewRegistry(Config{Threshold: 100, Timeout: time.Second})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "concurrent"
			r.Allow(key)
			if n%2 == 0 {
				r.RecordSuccess(key)
			} else {
				r.RecordFailure(key)
			}
			r.GetState(key)
		}(i)
	}
	wg.Wait()
}
