package ratelimit

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestAllowWithinRate(t *testing.T) {
	l := New(10, 10) // 10 req/s, burst of 10

	for i := range 10 {
		if !l.Allow("user1") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
}

func TestAllowFailsWhenBurstExceeded(t *testing.T) {
	l := New(1, 3) // 1 req/s, burst of 3

	// Use all tokens.
	for range 3 {
		if !l.Allow("user1") {
			t.Fatal("should be allowed within burst")
		}
	}

	// Next request should be denied.
	if l.Allow("user1") {
		t.Fatal("should be denied after burst exceeded")
	}
}

func TestTokensRefillOverTime(t *testing.T) {
	l := New(100, 5) // 100 req/s, burst of 5

	// Exhaust all tokens.
	for range 5 {
		l.Allow("user1")
	}

	if l.Allow("user1") {
		t.Fatal("should be denied immediately after exhaustion")
	}

	// Wait for tokens to refill.
	time.Sleep(60 * time.Millisecond)

	if !l.Allow("user1") {
		t.Fatal("should be allowed after refill time")
	}
}

func TestAllowNGreaterThanBurstFails(t *testing.T) {
	l := New(10, 5)

	if l.AllowN("user1", 6) {
		t.Fatal("AllowN with n > burst should fail")
	}
}

func TestAllowNWithinBurst(t *testing.T) {
	l := New(10, 10)

	if !l.AllowN("user1", 5) {
		t.Fatal("AllowN(5) should succeed with burst=10")
	}
	if !l.AllowN("user1", 5) {
		t.Fatal("AllowN(5) should succeed, 5 tokens remaining")
	}
	if l.AllowN("user1", 1) {
		t.Fatal("AllowN(1) should fail, 0 tokens remaining")
	}
}

func TestResetClearsBucket(t *testing.T) {
	l := New(1, 2)

	// Exhaust tokens.
	l.Allow("user1")
	l.Allow("user1")

	if l.Allow("user1") {
		t.Fatal("should be denied after exhaustion")
	}

	l.Reset("user1")

	// After reset, should have full burst again.
	if !l.Allow("user1") {
		t.Fatal("should be allowed after reset")
	}
}

func TestCleanupRemovesStaleEntries(t *testing.T) {
	l := New(10, 10)

	l.Allow("stale_user")

	// Manually set lastCheck to the past.
	l.mu.Lock()
	l.buckets["stale_user"].lastCheck = time.Now().Add(-10 * time.Minute)
	l.mu.Unlock()

	l.Allow("fresh_user")

	l.Cleanup(5 * time.Minute)

	l.mu.Lock()
	_, staleExists := l.buckets["stale_user"]
	_, freshExists := l.buckets["fresh_user"]
	l.mu.Unlock()

	if staleExists {
		t.Error("stale_user should have been cleaned up")
	}
	if !freshExists {
		t.Error("fresh_user should still exist")
	}
}

func TestConcurrentAccess(t *testing.T) {
	l := New(1000, 100)
	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("user%d", id%10)
			for range 100 {
				l.Allow(key)
			}
		}(i)
	}

	wg.Wait()
}

func TestSeparateKeysAreIndependent(t *testing.T) {
	l := New(1, 1)

	if !l.Allow("user1") {
		t.Fatal("user1 first request should be allowed")
	}
	if l.Allow("user1") {
		t.Fatal("user1 second request should be denied")
	}

	// user2 should be independent.
	if !l.Allow("user2") {
		t.Fatal("user2 first request should be allowed")
	}
}
