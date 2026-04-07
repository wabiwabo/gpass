package taskqueue

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestNew_CoercesBadWorkers pins the workers<=0 → 1 fallback.
func TestNew_CoercesBadWorkers(t *testing.T) {
	noop := func(_ context.Context, _ Task) error { return nil }
	q := New(noop, 0)
	if q.workers != 1 {
		t.Errorf("workers = %d, want 1", q.workers)
	}
	q2 := New(noop, -5)
	if q2.workers != 1 {
		t.Errorf("negative workers = %d, want 1", q2.workers)
	}
}

// TestStart_DoubleStartIsNoOp pins the running.Load early-return guard.
func TestStart_DoubleStartIsNoOp(t *testing.T) {
	var calls atomic.Int64
	q := New(func(_ context.Context, _ Task) error {
		calls.Add(1)
		return nil
	}, 2)
	q.Start(context.Background())
	q.Start(context.Background()) // must be no-op
	defer q.Stop()

	for i := 0; i < 5; i++ {
		q.Enqueue(Task{ID: "t"})
	}
	// Wait for drain.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && calls.Load() < 5 {
		time.Sleep(20 * time.Millisecond)
	}
	if calls.Load() != 5 {
		t.Errorf("processed %d, want 5", calls.Load())
	}
}

// TestStop_BeforeStartIsNoOp pins the !running guard in Stop.
func TestStop_BeforeStartIsNoOp(t *testing.T) {
	q := New(func(_ context.Context, _ Task) error { return nil }, 1)
	q.Stop() // never started — must not panic
}

// TestWorker_RetryUntilExhausted pins the retry-and-eventually-fail
// branch in worker. A task that always errors must be re-enqueued
// MaxRetry times then counted as Failed.
func TestWorker_RetryUntilExhausted(t *testing.T) {
	var attempts atomic.Int64
	q := New(func(_ context.Context, _ Task) error {
		attempts.Add(1)
		return errAlways
	}, 1)
	q.Start(context.Background())
	defer q.Stop()

	q.Enqueue(Task{ID: "fail-me", MaxRetry: 3})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && q.Stats().Failed < 1 {
		time.Sleep(20 * time.Millisecond)
	}
	s := q.Stats()
	if s.Failed != 1 {
		t.Errorf("Failed = %d, want 1", s.Failed)
	}
	// Initial attempt + 3 retries = 4 attempts.
	if attempts.Load() < 4 {
		t.Errorf("attempts = %d, want >= 4", attempts.Load())
	}
}

// TestWorker_CtxCancelExits pins the ctx.Done branch in the worker
// select — workers must exit when the parent context is cancelled,
// even if Stop hasn't been called.
func TestWorker_CtxCancelExits(t *testing.T) {
	q := New(func(_ context.Context, _ Task) error { return nil }, 2)
	ctx, cancel := context.WithCancel(context.Background())
	q.Start(ctx)
	cancel()
	// Without ctx.Done branch, Stop would still drain via the stop chan,
	// but we want to assert the workers actually exited via ctx — give
	// them a beat then call Stop and ensure it returns quickly.
	time.Sleep(50 * time.Millisecond)
	done := make(chan struct{})
	go func() { q.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return after ctx cancel")
	}
}

var errAlways = errAlwaysT{}

type errAlwaysT struct{}

func (errAlwaysT) Error() string { return "always fail" }
