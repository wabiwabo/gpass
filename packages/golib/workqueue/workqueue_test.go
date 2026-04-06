package workqueue

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestQueue_Enqueue_Process(t *testing.T) {
	var processed atomic.Int32

	q := New(Config{Workers: 1}, func(_ context.Context, item Item) error {
		processed.Add(1)
		return nil
	})
	q.Start()

	q.Enqueue(Item{ID: "1", Priority: PriorityNormal})
	q.Enqueue(Item{ID: "2", Priority: PriorityNormal})

	time.Sleep(100 * time.Millisecond)
	q.Stop()

	if processed.Load() != 2 {
		t.Errorf("processed: got %d, want 2", processed.Load())
	}
}

func TestQueue_PriorityOrdering(t *testing.T) {
	var order []string
	var mu sync.Mutex

	// Use a blocking start to queue items before processing.
	q := New(Config{Workers: 1, MaxSize: 100}, func(_ context.Context, item Item) error {
		mu.Lock()
		order = append(order, item.ID)
		mu.Unlock()
		return nil
	})

	// Enqueue before starting.
	q.Enqueue(Item{ID: "low", Priority: PriorityLow})
	q.Enqueue(Item{ID: "normal", Priority: PriorityNormal})
	q.Enqueue(Item{ID: "high", Priority: PriorityHigh})

	q.Start()
	time.Sleep(100 * time.Millisecond)
	q.Stop()

	mu.Lock()
	defer mu.Unlock()

	if len(order) != 3 {
		t.Fatalf("expected 3 items processed, got %d", len(order))
	}
	// High should be first.
	if order[0] != "high" {
		t.Errorf("first item should be high priority, got %q", order[0])
	}
}

func TestQueue_Backpressure(t *testing.T) {
	q := New(Config{Workers: 1, MaxSize: 2}, func(_ context.Context, item Item) error {
		time.Sleep(time.Second)
		return nil
	})

	// Fill the queue.
	q.Enqueue(Item{ID: "1", Priority: PriorityNormal})
	q.Enqueue(Item{ID: "2", Priority: PriorityNormal})

	// Third should be dropped.
	ok := q.Enqueue(Item{ID: "3", Priority: PriorityNormal})
	if ok {
		t.Error("should return false when queue is full")
	}

	stats := q.Stats()
	if stats.Dropped != 1 {
		t.Errorf("dropped: got %d, want 1", stats.Dropped)
	}
}

func TestQueue_ErrorTracking(t *testing.T) {
	q := New(Config{Workers: 1}, func(_ context.Context, item Item) error {
		if item.ID == "fail" {
			return errors.New("processing failed")
		}
		return nil
	})
	q.Start()

	q.Enqueue(Item{ID: "ok", Priority: PriorityNormal})
	q.Enqueue(Item{ID: "fail", Priority: PriorityNormal})

	time.Sleep(100 * time.Millisecond)
	q.Stop()

	stats := q.Stats()
	if stats.Errors != 1 {
		t.Errorf("errors: got %d, want 1", stats.Errors)
	}
	if stats.Processed != 2 {
		t.Errorf("processed: got %d, want 2", stats.Processed)
	}
}

func TestQueue_MultipleWorkers(t *testing.T) {
	var processed atomic.Int32

	q := New(Config{Workers: 4, MaxSize: 100}, func(_ context.Context, item Item) error {
		processed.Add(1)
		time.Sleep(10 * time.Millisecond)
		return nil
	})
	q.Start()

	for i := 0; i < 20; i++ {
		q.Enqueue(Item{ID: string(rune('A' + i)), Priority: PriorityNormal})
	}

	time.Sleep(200 * time.Millisecond)
	q.Stop()

	if processed.Load() != 20 {
		t.Errorf("processed: got %d, want 20", processed.Load())
	}
}

func TestQueue_Drain(t *testing.T) {
	var processed atomic.Int32

	q := New(Config{Workers: 2, MaxSize: 100}, func(_ context.Context, item Item) error {
		processed.Add(1)
		return nil
	})

	for i := 0; i < 5; i++ {
		q.Enqueue(Item{ID: string(rune('A' + i)), Priority: PriorityNormal})
	}

	q.Start()
	q.Drain(time.Second)

	if processed.Load() != 5 {
		t.Errorf("drain should process all remaining: got %d", processed.Load())
	}
}

func TestQueue_Len(t *testing.T) {
	q := New(Config{Workers: 1, MaxSize: 100}, func(_ context.Context, item Item) error {
		return nil
	})

	q.Enqueue(Item{ID: "1", Priority: PriorityHigh})
	q.Enqueue(Item{ID: "2", Priority: PriorityNormal})
	q.Enqueue(Item{ID: "3", Priority: PriorityLow})

	if q.Len() != 3 {
		t.Errorf("len: got %d, want 3", q.Len())
	}
}

func TestQueue_Stats(t *testing.T) {
	q := New(Config{Workers: 1, MaxSize: 100}, func(_ context.Context, item Item) error {
		return nil
	})

	q.Enqueue(Item{ID: "h", Priority: PriorityHigh})
	q.Enqueue(Item{ID: "n", Priority: PriorityNormal})
	q.Enqueue(Item{ID: "l", Priority: PriorityLow})

	stats := q.Stats()
	if stats.HighPending != 1 {
		t.Errorf("high pending: got %d", stats.HighPending)
	}
	if stats.NormalPending != 1 {
		t.Errorf("normal pending: got %d", stats.NormalPending)
	}
	if stats.LowPending != 1 {
		t.Errorf("low pending: got %d", stats.LowPending)
	}
}

func TestQueue_ConcurrentEnqueue(t *testing.T) {
	var processed atomic.Int32

	q := New(Config{Workers: 4, MaxSize: 200}, func(_ context.Context, item Item) error {
		processed.Add(1)
		return nil
	})
	q.Start()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			q.Enqueue(Item{ID: string(rune(n)), Priority: Priority(n % 3)})
		}(i)
	}
	wg.Wait()

	time.Sleep(200 * time.Millisecond)
	q.Stop()

	stats := q.Stats()
	total := stats.Processed + stats.Dropped
	if total < 90 {
		t.Errorf("expected most items handled, got processed=%d dropped=%d", stats.Processed, stats.Dropped)
	}
}
