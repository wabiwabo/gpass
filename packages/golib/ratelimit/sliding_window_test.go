package ratelimit

import (
	"sync"
	"testing"
	"time"
)

func TestSlidingWindow_AllowWithinLimit(t *testing.T) {
	sw := NewSlidingWindow(10, time.Second)

	for i := 0; i < 10; i++ {
		if !sw.Allow("client-1") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}
}

func TestSlidingWindow_RejectsOverLimit(t *testing.T) {
	sw := NewSlidingWindow(5, time.Second)

	for i := 0; i < 5; i++ {
		sw.Allow("client-1")
	}

	if sw.Allow("client-1") {
		t.Error("6th request should be rejected")
	}
}

func TestSlidingWindow_IndependentKeys(t *testing.T) {
	sw := NewSlidingWindow(2, time.Second)

	sw.Allow("a")
	sw.Allow("a")

	// Key "b" should have its own limit.
	if !sw.Allow("b") {
		t.Error("key b should be allowed independently")
	}
}

func TestSlidingWindow_AllowN(t *testing.T) {
	sw := NewSlidingWindow(10, time.Second)

	if !sw.AllowN("k", 5) {
		t.Error("5 requests should be allowed")
	}
	if !sw.AllowN("k", 5) {
		t.Error("10 total should be allowed")
	}
	if sw.AllowN("k", 1) {
		t.Error("11th should be rejected")
	}
}

func TestSlidingWindow_AllowN_ExceedsLimit(t *testing.T) {
	sw := NewSlidingWindow(5, time.Second)

	if sw.AllowN("k", 6) {
		t.Error("6 requests with limit 5 should be rejected")
	}
}

func TestSlidingWindow_WindowSliding(t *testing.T) {
	sw := NewSlidingWindow(10, 100*time.Millisecond)

	// Fill the window.
	for i := 0; i < 10; i++ {
		sw.Allow("k")
	}

	// Wait for window to pass.
	time.Sleep(110 * time.Millisecond)

	// Should allow again since we're in a new window.
	// Previous window had 10 requests but with sliding weight ~0.
	if !sw.Allow("k") {
		t.Error("should allow after window passes")
	}
}

func TestSlidingWindow_SlidingApproximation(t *testing.T) {
	sw := NewSlidingWindow(10, 200*time.Millisecond)

	// Fill first window.
	for i := 0; i < 10; i++ {
		sw.Allow("k")
	}

	// Wait for full window + ~half of next window.
	// This shifts: prevCount=10, currCount=0, elapsed ~50% into new window.
	// Effective = 10 * 0.5 + 0 = 5, so 5 more should be allowed.
	time.Sleep(300 * time.Millisecond)

	allowed := 0
	for i := 0; i < 10; i++ {
		if sw.Allow("k") {
			allowed++
		}
	}

	// Should allow approximately 4-8 (sliding window approximation with timing variance).
	if allowed < 3 || allowed > 9 {
		t.Errorf("expected ~5 allowed in sliding window, got %d", allowed)
	}
}

func TestSlidingWindow_TwoWindowsExpired(t *testing.T) {
	sw := NewSlidingWindow(5, 50*time.Millisecond)

	for i := 0; i < 5; i++ {
		sw.Allow("k")
	}

	// Wait for 2 full windows.
	time.Sleep(110 * time.Millisecond)

	// Should allow full limit again.
	for i := 0; i < 5; i++ {
		if !sw.Allow("k") {
			t.Errorf("request %d should be allowed after 2 windows", i+1)
		}
	}
}

func TestSlidingWindow_Remaining(t *testing.T) {
	sw := NewSlidingWindow(10, time.Second)

	if r := sw.Remaining("new"); r != 10 {
		t.Errorf("new key remaining: got %d, want 10", r)
	}

	sw.Allow("k")
	sw.Allow("k")
	sw.Allow("k")

	r := sw.Remaining("k")
	if r != 7 {
		t.Errorf("remaining after 3 requests: got %d, want 7", r)
	}
}

func TestSlidingWindow_Reset(t *testing.T) {
	sw := NewSlidingWindow(5, time.Second)

	for i := 0; i < 5; i++ {
		sw.Allow("k")
	}

	sw.Reset("k")

	if !sw.Allow("k") {
		t.Error("should allow after reset")
	}
}

func TestSlidingWindow_Cleanup(t *testing.T) {
	sw := NewSlidingWindow(5, 50*time.Millisecond)

	sw.Allow("a")
	sw.Allow("b")

	time.Sleep(110 * time.Millisecond)

	sw.Cleanup()

	// After cleanup, keys should be removed.
	if r := sw.Remaining("a"); r != 5 {
		t.Errorf("after cleanup remaining: got %d, want 5", r)
	}
}

func TestSlidingWindow_Config(t *testing.T) {
	sw := NewSlidingWindow(100, 5*time.Minute)
	if sw.Limit() != 100 {
		t.Errorf("limit: got %d, want 100", sw.Limit())
	}
	if sw.Window() != 5*time.Minute {
		t.Errorf("window: got %v, want 5m", sw.Window())
	}
}

func TestSlidingWindow_ConcurrentAccess(t *testing.T) {
	sw := NewSlidingWindow(1000, time.Second)

	var wg sync.WaitGroup
	allowed := make([]int, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if sw.Allow("shared") {
					allowed[idx]++
				}
			}
		}(i)
	}
	wg.Wait()

	total := 0
	for _, a := range allowed {
		total += a
	}

	if total > 1000 {
		t.Errorf("total allowed %d exceeds limit 1000", total)
	}
}
