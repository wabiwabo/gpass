package taskpool

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestPool_ContextCancellationMidFlight covers the worker's ctx.Done()
// branch: a task that's still queued when the context is cancelled
// must be reported with the cancellation error and counted as failed
// (not silently dropped).
func TestPool_ContextCancellationMidFlight(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	p := New[int](2)
	p.Start(ctx)

	// Submit a few quick tasks before cancellation.
	for i := 0; i < 5; i++ {
		p.Submit(func(c context.Context) (int, error) {
			return 1, nil
		})
	}

	// Cancel before all results come back, then keep submitting more
	// tasks that the worker will see ctx.Done() on.
	cancel()

	for i := 0; i < 10; i++ {
		p.Submit(func(c context.Context) (int, error) {
			return 0, nil
		})
	}

	results := p.Wait()
	if len(results) != 15 {
		t.Errorf("got %d results, want 15", len(results))
	}

	// At least some of the post-cancel tasks must have hit the ctx.Done()
	// branch and reported context.Canceled.
	canceled := 0
	for _, r := range results {
		if errors.Is(r.Err, context.Canceled) {
			canceled++
		}
	}
	if canceled == 0 {
		t.Error("no tasks reported context.Canceled — ctx.Done() branch not exercised")
	}
}

// TestPool_CloseIsIdempotent covers the closed.Swap branch in Close:
// a second Close call must NOT panic on close-of-closed channel.
func TestPool_CloseIsIdempotent(t *testing.T) {
	p := New[string](1)
	p.Start(context.Background())
	p.Submit(func(ctx context.Context) (string, error) { return "ok", nil })
	p.Close()
	// Second Close must be a no-op.
	p.Close()
	// Wait calls Close a third time internally — also must be safe.
	results := p.Wait()
	if len(results) != 1 || results[0].Value != "ok" {
		t.Errorf("got %+v, want one ok result", results)
	}
}

// TestPool_FailureCounter pins that failed tasks increment the failed
// counter (not just completed) — used by SLO metric exporters.
func TestPool_FailureCounter(t *testing.T) {
	p := New[int](2)
	p.Start(context.Background())
	for i := 0; i < 3; i++ {
		p.Submit(func(ctx context.Context) (int, error) { return 0, errors.New("boom") })
	}
	for i := 0; i < 2; i++ {
		p.Submit(func(ctx context.Context) (int, error) { return 1, nil })
	}
	results := p.Wait()
	if len(results) != 5 {
		t.Fatalf("got %d results, want 5", len(results))
	}
	submitted, completed, failed := p.Stats()
	if submitted != 5 {
		t.Errorf("submitted = %d, want 5", submitted)
	}
	if failed != 3 {
		t.Errorf("failed = %d, want 3", failed)
	}
	if completed != 5 {
		// completed counts ALL tasks reaching a result, including failures.
		t.Errorf("completed = %d, want 5 (errors still count as completed)", completed)
	}
}

// TestPool_NewClampsZeroWorkers pins that zero or negative worker count
// gets clamped to 1 (otherwise the channel would deadlock immediately).
func TestPool_NewClampsZeroWorkers(t *testing.T) {
	for _, n := range []int{0, -5} {
		p := New[int](n)
		if p.Workers() != 1 {
			t.Errorf("New(%d).Workers() = %d, want 1", n, p.Workers())
		}
	}
}

// TestPool_OrderingPreservedViaIndex pins that the Result.Index field
// preserves submission order even when results come back out of order
// (which they will with multiple workers and varying task durations).
func TestPool_OrderingPreservedViaIndex(t *testing.T) {
	p := New[int](4)
	p.Start(context.Background())

	// Submit 10 tasks with descending sleep durations so the last one
	// finishes first.
	for i := 0; i < 10; i++ {
		i := i
		p.Submit(func(ctx context.Context) (int, error) {
			time.Sleep(time.Duration(10-i) * time.Millisecond)
			return i, nil
		})
	}

	results := p.Wait()
	if len(results) != 10 {
		t.Fatalf("got %d, want 10", len(results))
	}
	// Build a map: index → value. Since we returned the index as the
	// value, they must match for every entry.
	for _, r := range results {
		if r.Value != r.Index {
			t.Errorf("Index %d got Value %d (mismatch)", r.Index, r.Value)
		}
	}
}
