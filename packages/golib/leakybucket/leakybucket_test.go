package leakybucket

import (
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	b := New(5, 100*time.Millisecond)
	if b.Capacity() != 5 {
		t.Errorf("Capacity = %d", b.Capacity())
	}
}

func TestAllow(t *testing.T) {
	b := New(3, 1*time.Hour) // slow drain
	if !b.Allow() {
		t.Error("1st should be allowed")
	}
	if !b.Allow() {
		t.Error("2nd should be allowed")
	}
	if !b.Allow() {
		t.Error("3rd should be allowed")
	}
	if b.Allow() {
		t.Error("4th should be rejected (capacity 3)")
	}
}

func TestDrip(t *testing.T) {
	b := New(2, 10*time.Millisecond)
	b.Allow()
	b.Allow()

	// Queue full
	if b.Allow() {
		t.Error("should be full")
	}

	// Wait for drip
	time.Sleep(15 * time.Millisecond)

	// Should have drained at least 1
	if !b.Allow() {
		t.Error("should allow after drip")
	}
}

func TestQueue(t *testing.T) {
	b := New(5, 1*time.Hour)
	b.Allow()
	b.Allow()

	if q := b.Queue(); q != 2 {
		t.Errorf("Queue = %d, want 2", q)
	}
}

func TestQueue_Empty(t *testing.T) {
	b := New(5, 1*time.Hour)
	if q := b.Queue(); q != 0 {
		t.Errorf("Queue = %d, want 0", q)
	}
}

func TestReset(t *testing.T) {
	b := New(3, 1*time.Hour)
	b.Allow()
	b.Allow()
	b.Allow()

	b.Reset()
	if q := b.Queue(); q != 0 {
		t.Errorf("after reset Queue = %d", q)
	}
	if !b.Allow() {
		t.Error("should allow after reset")
	}
}

func TestDrip_Multiple(t *testing.T) {
	b := New(5, 5*time.Millisecond)
	for i := 0; i < 5; i++ {
		b.Allow()
	}

	time.Sleep(30 * time.Millisecond) // should drain all 5

	q := b.Queue()
	if q != 0 {
		t.Errorf("Queue = %d after long wait, want 0", q)
	}
}

func TestConcurrent(t *testing.T) {
	b := New(100, 1*time.Hour)
	var wg sync.WaitGroup

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Allow()
		}()
	}
	wg.Wait()
}
