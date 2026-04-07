package workqueue

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestQueue_DrainRemaining_ProcessesItemsAfterStop pins the post-stop
// drain branch: items already in the channels when Stop is called must
// still be processed before the worker returns. Without this, in-flight
// payments / signatures / webhooks would be silently dropped on
// graceful shutdown.
func TestQueue_DrainRemaining_ProcessesItemsAfterStop(t *testing.T) {
	var processed atomic.Int32
	q := New(Config{Workers: 1, MaxSize: 100}, func(ctx context.Context, it Item) error {
		processed.Add(1)
		return nil
	})

	// Enqueue 10 items across all priorities BEFORE Start so they're
	// already in the channels when the worker drains.
	for i := 0; i < 4; i++ {
		q.Enqueue(Item{ID: "h", Priority: PriorityHigh})
	}
	for i := 0; i < 3; i++ {
		q.Enqueue(Item{ID: "n", Priority: PriorityNormal})
	}
	for i := 0; i < 3; i++ {
		q.Enqueue(Item{ID: "l", Priority: PriorityLow})
	}

	q.Start()
	// Give the worker a beat to start consuming, then stop.
	time.Sleep(20 * time.Millisecond)
	q.Stop()

	if got := processed.Load(); got != 10 {
		t.Errorf("processed = %d, want 10 (drainRemaining must finish all queued items)", got)
	}
}

// TestQueue_RateLimiter_StopUnblocks covers the rate-limited worker's
// stop-channel branch (the limiter goroutine must respond to q.stop
// even while waiting on a tick).
func TestQueue_RateLimiter_StopUnblocks(t *testing.T) {
	q := New(Config{
		Workers:   1,
		MaxSize:   10,
		RateLimit: 0.5, // 1 tick every 2s — far longer than the test
	}, func(ctx context.Context, it Item) error { return nil })

	q.Start()
	stopped := make(chan struct{})
	go func() {
		q.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not unblock the rate-limited worker within 2s")
	}
}

// TestQueue_BackpressureDropsAndCounts covers the dropped counter when
// a priority channel is full. Each priority has its own buffer, so we
// fill exactly one and assert the counter increments per drop.
func TestQueue_BackpressureDropsAndCounts(t *testing.T) {
	q := New(Config{Workers: 1, MaxSize: 2}, func(ctx context.Context, it Item) error {
		// Block forever so the buffer fills up. Test calls Drain after.
		select {}
	})

	for i := 0; i < 2; i++ {
		if !q.Enqueue(Item{ID: "h", Priority: PriorityHigh}) {
			t.Errorf("Enqueue %d should have succeeded", i)
		}
	}
	// Third enqueue must be dropped (buffer size 2).
	if q.Enqueue(Item{ID: "h", Priority: PriorityHigh}) {
		t.Error("third Enqueue should have been dropped")
	}
	if q.Stats().Dropped != 1 {
		t.Errorf("dropped = %d, want 1", q.Stats().Dropped)
	}
}

// TestQueue_HandlerErrorIncrementsCounter pins the errors counter path:
// a handler that returns an error must NOT block the queue, must NOT
// retry automatically, and MUST increment the errors counter.
func TestQueue_HandlerErrorIncrementsCounter(t *testing.T) {
	var done sync.WaitGroup
	done.Add(3)
	q := New(Config{Workers: 1, MaxSize: 10}, func(ctx context.Context, it Item) error {
		defer done.Done()
		return context.DeadlineExceeded // any error works
	})
	q.Start()
	defer q.Stop()

	for i := 0; i < 3; i++ {
		q.Enqueue(Item{ID: "x", Priority: PriorityNormal})
	}
	doneCh := make(chan struct{})
	go func() { done.Wait(); close(doneCh) }()
	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("handler not invoked for all 3 items")
	}

	// Allow process() to finish updating counters before sampling.
	time.Sleep(20 * time.Millisecond)
	s := q.Stats()
	if s.Errors != 3 {
		t.Errorf("errors = %d, want 3", s.Errors)
	}
	if s.Processed != 3 {
		t.Errorf("processed = %d, want 3 (errors still count as processed)", s.Processed)
	}
}

// TestQueue_PriorityOrdering pins that High items are drained before
// Normal which are drained before Low. We enqueue interleaved and
// observe the actual processing order.
func TestQueue_PriorityOrdering_Cov(t *testing.T) {
	var (
		mu       sync.Mutex
		order    []string
	)
	q := New(Config{Workers: 1, MaxSize: 100}, func(ctx context.Context, it Item) error {
		mu.Lock()
		order = append(order, it.ID)
		mu.Unlock()
		return nil
	})

	// Pre-fill before Start so the worker sees all items at once.
	q.Enqueue(Item{ID: "L", Priority: PriorityLow})
	q.Enqueue(Item{ID: "L", Priority: PriorityLow})
	q.Enqueue(Item{ID: "N", Priority: PriorityNormal})
	q.Enqueue(Item{ID: "N", Priority: PriorityNormal})
	q.Enqueue(Item{ID: "H", Priority: PriorityHigh})
	q.Enqueue(Item{ID: "H", Priority: PriorityHigh})

	q.Start()
	// Wait for everything to drain.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(order)
		mu.Unlock()
		if n == 6 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	q.Stop()

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 6 {
		t.Fatalf("order = %v", order)
	}
	// First two must be H, last two must be L.
	if order[0] != "H" || order[1] != "H" {
		t.Errorf("first two should be H, got %v", order[:2])
	}
	if order[4] != "L" || order[5] != "L" {
		t.Errorf("last two should be L, got %v", order[4:])
	}
}
