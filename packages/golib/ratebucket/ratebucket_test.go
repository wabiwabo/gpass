package ratebucket

import (
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	b := New(10, 5)
	if b.Capacity() != 10 {
		t.Errorf("Capacity = %f", b.Capacity())
	}
	if b.Rate() != 5 {
		t.Errorf("Rate = %f", b.Rate())
	}
}

func TestNew_StartsFulll(t *testing.T) {
	b := New(10, 5)
	if b.Tokens() != 10 {
		t.Errorf("Tokens = %f, want 10", b.Tokens())
	}
}

func TestAllow(t *testing.T) {
	b := New(3, 1)
	if !b.Allow() {
		t.Error("first should be allowed")
	}
	if !b.Allow() {
		t.Error("second should be allowed")
	}
	if !b.Allow() {
		t.Error("third should be allowed")
	}
	if b.Allow() {
		t.Error("fourth should be denied (capacity 3)")
	}
}

func TestAllowN(t *testing.T) {
	b := New(10, 5)
	if !b.AllowN(5) {
		t.Error("5 of 10 should be allowed")
	}
	if !b.AllowN(5) {
		t.Error("another 5 should be allowed")
	}
	if b.AllowN(1) {
		t.Error("should be denied (empty)")
	}
}

func TestAllowN_LargerThanCapacity(t *testing.T) {
	b := New(5, 1)
	if b.AllowN(6) {
		t.Error("request larger than capacity should be denied")
	}
}

func TestRefill(t *testing.T) {
	b := New(10, 1000) // 1000 tokens/sec
	// Drain all
	b.AllowN(10)

	time.Sleep(15 * time.Millisecond) // ~15 tokens added

	tokens := b.Tokens()
	if tokens < 10 {
		t.Logf("tokens after 15ms refill at 1000/s = %f", tokens)
	}
	if tokens > 10 {
		t.Errorf("tokens should not exceed capacity: %f", tokens)
	}
}

func TestRefill_CapsAtCapacity(t *testing.T) {
	b := New(5, 10000) // very fast refill
	b.AllowN(5)
	time.Sleep(10 * time.Millisecond)
	if b.Tokens() > 5 {
		t.Errorf("tokens should cap at capacity: %f", b.Tokens())
	}
}

func TestReset(t *testing.T) {
	b := New(10, 1)
	b.AllowN(10)

	b.Reset()
	tokens := b.Tokens()
	if tokens < 9.9 {
		t.Errorf("after reset tokens = %f, want ~10", tokens)
	}
}

func TestWaitTime_Available(t *testing.T) {
	b := New(10, 5)
	wait := b.WaitTime(1)
	if wait != 0 {
		t.Errorf("WaitTime = %v, want 0", wait)
	}
}

func TestWaitTime_NeedToWait(t *testing.T) {
	b := New(10, 10) // 10 tokens/sec
	b.AllowN(10)     // drain all

	wait := b.WaitTime(5) // need 5 tokens at 10/sec = 0.5s
	if wait < 400*time.Millisecond || wait > 600*time.Millisecond {
		t.Errorf("WaitTime = %v, want ~500ms", wait)
	}
}

func TestConcurrent(t *testing.T) {
	b := New(100, 100)
	var wg sync.WaitGroup

	allowed := int64(0)
	var mu sync.Mutex

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if b.Allow() {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// Should have allowed close to 100 (started full)
	if allowed > 101 {
		t.Errorf("allowed %d, should not exceed capacity + refill", allowed)
	}
}
