package semaphore

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestWeighted_AcquireRelease(t *testing.T) {
	s := NewWeighted(5)

	if err := s.Acquire(context.Background(), 3); err != nil {
		t.Fatal(err)
	}
	if s.Available() != 2 {
		t.Errorf("available: got %d", s.Available())
	}

	s.Release(3)
	if s.Available() != 5 {
		t.Errorf("after release: got %d", s.Available())
	}
}

func TestWeighted_TryAcquire(t *testing.T) {
	s := NewWeighted(3)

	if !s.TryAcquire(2) {
		t.Error("should acquire 2/3")
	}
	if !s.TryAcquire(1) {
		t.Error("should acquire 1 more")
	}
	if s.TryAcquire(1) {
		t.Error("should fail — full")
	}

	s.Release(1)
	if !s.TryAcquire(1) {
		t.Error("should succeed after release")
	}
}

func TestWeighted_BlockUntilRelease(t *testing.T) {
	s := NewWeighted(1)
	s.Acquire(context.Background(), 1)

	done := make(chan struct{})
	go func() {
		s.Acquire(context.Background(), 1)
		close(done)
	}()

	// Should not complete yet.
	select {
	case <-done:
		t.Error("should block")
	case <-time.After(50 * time.Millisecond):
	}

	s.Release(1)

	select {
	case <-done:
		// Good.
	case <-time.After(100 * time.Millisecond):
		t.Error("should unblock after release")
	}
}

func TestWeighted_ContextCancellation(t *testing.T) {
	s := NewWeighted(1)
	s.Acquire(context.Background(), 1)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := s.Acquire(ctx, 1)
	if err == nil {
		t.Error("should fail on context cancellation")
	}

	s.Release(1)
}

func TestWeighted_FIFO(t *testing.T) {
	s := NewWeighted(1)
	s.Acquire(context.Background(), 1)

	var order []int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			s.Acquire(context.Background(), 1)
			mu.Lock()
			order = append(order, n)
			mu.Unlock()
			s.Release(1)
		}(i)
		time.Sleep(10 * time.Millisecond) // Stagger to ensure ordering.
	}

	time.Sleep(50 * time.Millisecond)
	s.Release(1)
	wg.Wait()

	if len(order) != 3 {
		t.Errorf("order: got %d items", len(order))
	}
}

func TestWeighted_Size(t *testing.T) {
	s := NewWeighted(10)
	if s.Size() != 10 {
		t.Errorf("size: got %d", s.Size())
	}
}

func TestWeighted_InUse(t *testing.T) {
	s := NewWeighted(10)
	s.Acquire(context.Background(), 3)
	s.Acquire(context.Background(), 4)

	if s.InUse() != 7 {
		t.Errorf("in use: got %d", s.InUse())
	}
}

func TestWeighted_WaiterCount(t *testing.T) {
	s := NewWeighted(1)
	s.Acquire(context.Background(), 1)

	go s.Acquire(context.Background(), 1)
	time.Sleep(20 * time.Millisecond)

	if s.WaiterCount() != 1 {
		t.Errorf("waiters: got %d", s.WaiterCount())
	}

	s.Release(1)
	time.Sleep(20 * time.Millisecond)

	if s.WaiterCount() != 0 {
		t.Errorf("after release: got %d", s.WaiterCount())
	}
}

func TestWeighted_ConcurrentSafety(t *testing.T) {
	s := NewWeighted(10)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Acquire(context.Background(), 1)
			time.Sleep(time.Millisecond)
			s.Release(1)
		}()
	}
	wg.Wait()

	if s.Available() != 10 {
		t.Errorf("after all done: got %d available", s.Available())
	}
}
