package slidingwindow

import (
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	w := New(10, 1*time.Minute)
	if w.Limit() != 10 {
		t.Errorf("Limit = %d", w.Limit())
	}
	if w.WindowDuration() != 1*time.Minute {
		t.Errorf("Window = %v", w.WindowDuration())
	}
}

func TestAllow(t *testing.T) {
	w := New(3, 1*time.Minute)
	if !w.Allow() {
		t.Error("1st should be allowed")
	}
	if !w.Allow() {
		t.Error("2nd should be allowed")
	}
	if !w.Allow() {
		t.Error("3rd should be allowed")
	}
	if w.Allow() {
		t.Error("4th should be denied")
	}
}

func TestAllowAt(t *testing.T) {
	w := New(2, 1*time.Second)
	now := time.Now()

	w.AllowAt(now)
	w.AllowAt(now.Add(100 * time.Millisecond))

	// At now + 200ms, still in window
	if w.AllowAt(now.Add(200 * time.Millisecond)) {
		t.Error("should be denied (2 in window)")
	}

	// At now + 1.1s, first request has expired
	if !w.AllowAt(now.Add(1100 * time.Millisecond)) {
		t.Error("should be allowed (first request expired)")
	}
}

func TestCount(t *testing.T) {
	w := New(10, 1*time.Minute)
	w.Allow()
	w.Allow()
	w.Allow()

	if c := w.Count(); c != 3 {
		t.Errorf("Count = %d, want 3", c)
	}
}

func TestRemaining(t *testing.T) {
	w := New(5, 1*time.Minute)
	w.Allow()
	w.Allow()

	if r := w.Remaining(); r != 3 {
		t.Errorf("Remaining = %d, want 3", r)
	}
}

func TestRemaining_Exhausted(t *testing.T) {
	w := New(2, 1*time.Minute)
	w.Allow()
	w.Allow()

	if r := w.Remaining(); r != 0 {
		t.Errorf("Remaining = %d, want 0", r)
	}
}

func TestReset(t *testing.T) {
	w := New(5, 1*time.Minute)
	w.Allow()
	w.Allow()

	w.Reset()
	if c := w.Count(); c != 0 {
		t.Errorf("Count after reset = %d", c)
	}
}

func TestNextAllowed_Available(t *testing.T) {
	w := New(5, 1*time.Minute)
	w.Allow()

	next := w.NextAllowed()
	if !next.IsZero() {
		t.Errorf("NextAllowed = %v, want zero (slots available)", next)
	}
}

func TestNextAllowed_Full(t *testing.T) {
	w := New(2, 1*time.Second)
	w.Allow()
	w.Allow()

	next := w.NextAllowed()
	if next.IsZero() {
		t.Error("NextAllowed should not be zero when full")
	}

	// Should be roughly 1 second from now
	until := time.Until(next)
	if until < 500*time.Millisecond || until > 1500*time.Millisecond {
		t.Errorf("NextAllowed in %v, want ~1s", until)
	}
}

func TestEviction(t *testing.T) {
	w := New(2, 50*time.Millisecond)
	w.Allow()
	w.Allow()

	// Full
	if w.Allow() {
		t.Error("should be full")
	}

	time.Sleep(60 * time.Millisecond)

	// After window, should allow again
	if !w.Allow() {
		t.Error("should allow after eviction")
	}
}

func TestConcurrent(t *testing.T) {
	w := New(100, 1*time.Second)
	var wg sync.WaitGroup

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.Allow()
		}()
	}
	wg.Wait()

	if w.Count() > 100 {
		t.Errorf("Count = %d, should not exceed limit", w.Count())
	}
}
