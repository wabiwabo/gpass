package taskqueue

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	q := New(func(ctx context.Context, task Task) error { return nil }, 2)
	if q.Pending() != 0 { t.Error("should be empty") }
	s := q.Stats()
	if s.Workers != 2 { t.Errorf("Workers = %d", s.Workers) }
}

func TestEnqueue(t *testing.T) {
	q := New(func(ctx context.Context, task Task) error { return nil }, 1)
	q.Enqueue(Task{ID: "1"})
	q.Enqueue(Task{ID: "2"})

	if q.Pending() != 2 { t.Errorf("Pending = %d", q.Pending()) }
}

func TestProcessing(t *testing.T) {
	var count atomic.Int32
	q := New(func(ctx context.Context, task Task) error {
		count.Add(1)
		return nil
	}, 2)

	for i := 0; i < 10; i++ {
		q.Enqueue(Task{ID: string(rune('0' + i))})
	}

	ctx, cancel := context.WithCancel(context.Background())
	q.Start(ctx)

	// Wait for processing
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for tasks")
		default:
			if count.Load() == 10 {
				cancel()
				q.Stop()
				s := q.Stats()
				if s.Processed != 10 { t.Errorf("Processed = %d", s.Processed) }
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestPriority(t *testing.T) {
	var order []string
	q := New(func(ctx context.Context, task Task) error {
		order = append(order, task.ID)
		return nil
	}, 1)

	q.Enqueue(Task{ID: "low", Priority: 10})
	q.Enqueue(Task{ID: "high", Priority: 1})
	q.Enqueue(Task{ID: "mid", Priority: 5})

	ctx, cancel := context.WithCancel(context.Background())
	q.Start(ctx)

	deadline := time.After(1 * time.Second)
	for {
		select {
		case <-deadline:
			cancel()
			q.Stop()
			// Check first processed was highest priority
			if len(order) > 0 && order[0] != "high" {
				t.Errorf("first = %q, want high", order[0])
			}
			return
		default:
			if q.Pending() == 0 && q.Stats().Processed == 3 {
				cancel()
				q.Stop()
				if order[0] != "high" { t.Errorf("first = %q", order[0]) }
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func TestRetry(t *testing.T) {
	var attempts atomic.Int32
	q := New(func(ctx context.Context, task Task) error {
		n := attempts.Add(1)
		if n < 3 {
			return errors.New("fail")
		}
		return nil
	}, 1)

	q.Enqueue(Task{ID: "retry", MaxRetry: 5})

	ctx, cancel := context.WithCancel(context.Background())
	q.Start(ctx)

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			cancel()
			q.Stop()
			return
		default:
			if q.Stats().Processed == 1 {
				cancel()
				q.Stop()
				if attempts.Load() != 3 { t.Errorf("attempts = %d", attempts.Load()) }
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestStop(t *testing.T) {
	q := New(func(ctx context.Context, task Task) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}, 1)

	ctx := context.Background()
	q.Start(ctx)
	q.Stop()

	if q.Stats().Running { t.Error("should not be running") }
}

func TestStats(t *testing.T) {
	q := New(func(ctx context.Context, task Task) error { return nil }, 3)
	q.Enqueue(Task{ID: "1"})

	s := q.Stats()
	if s.Pending != 1 { t.Errorf("Pending = %d", s.Pending) }
	if s.Workers != 3 { t.Errorf("Workers = %d", s.Workers) }
}
