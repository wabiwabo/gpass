package ratelimit_key

import (
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	l := New(10, 5, 10*time.Minute)
	if l.Count() != 0 {
		t.Errorf("Count = %d", l.Count())
	}
}

func TestAllow(t *testing.T) {
	l := New(1, 3, 10*time.Minute)

	// First 3 should be allowed (burst)
	for i := 0; i < 3; i++ {
		if !l.Allow("user-1") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}
	// 4th should be denied
	if l.Allow("user-1") {
		t.Error("4th should be denied (burst exhausted)")
	}
}

func TestAllow_PerKey(t *testing.T) {
	l := New(1, 2, 10*time.Minute)

	l.Allow("user-1")
	l.Allow("user-1")
	if l.Allow("user-1") {
		t.Error("user-1 should be rate limited")
	}

	// user-2 has its own bucket
	if !l.Allow("user-2") {
		t.Error("user-2 should be allowed")
	}
}

func TestAllow_Refill(t *testing.T) {
	l := New(1000, 1, 10*time.Minute) // 1000/s
	l.Allow("key") // consume 1

	time.Sleep(5 * time.Millisecond) // ~5 tokens added

	if !l.Allow("key") {
		t.Error("should be allowed after refill")
	}
}

func TestRemaining(t *testing.T) {
	l := New(10, 5, 10*time.Minute)

	r := l.Remaining("new-key")
	if r != 5 {
		t.Errorf("Remaining = %f, want 5 (full bucket)", r)
	}

	l.Allow("new-key")
	l.Allow("new-key")

	r = l.Remaining("new-key")
	if r < 2.9 || r > 3.1 {
		t.Errorf("Remaining = %f, want ~3", r)
	}
}

func TestReset(t *testing.T) {
	l := New(1, 3, 10*time.Minute)
	l.Allow("key")
	l.Allow("key")
	l.Allow("key")

	l.Reset("key")
	if !l.Allow("key") {
		t.Error("should be allowed after reset")
	}
}

func TestPurge(t *testing.T) {
	l := New(10, 5, 1*time.Millisecond)
	l.Allow("key-1")
	l.Allow("key-2")

	time.Sleep(5 * time.Millisecond)

	removed := l.Purge()
	if removed != 2 {
		t.Errorf("removed = %d", removed)
	}
	if l.Count() != 0 {
		t.Errorf("Count = %d", l.Count())
	}
}

func TestCount(t *testing.T) {
	l := New(10, 5, 10*time.Minute)
	l.Allow("a")
	l.Allow("b")
	l.Allow("c")

	if l.Count() != 3 {
		t.Errorf("Count = %d", l.Count())
	}
}

func TestDefaultTTL(t *testing.T) {
	l := New(10, 5, 0)
	if l.ttl != 10*time.Minute {
		t.Errorf("ttl = %v", l.ttl)
	}
}

func TestConcurrent(t *testing.T) {
	l := New(1000, 100, 10*time.Minute)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			l.Allow("shared-key")
		}(i)
	}
	wg.Wait()
}
