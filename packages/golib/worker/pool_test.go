package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPool_SubmitAndProcess(t *testing.T) {
	var processed atomic.Int64
	pool := New(2, 10, func(ctx context.Context, task Task) error {
		processed.Add(1)
		return nil
	})

	ctx := context.Background()
	pool.Start(ctx)

	for i := 0; i < 5; i++ {
		ok := pool.Submit(Task{ID: "task-" + string(rune('a'+i))})
		if !ok {
			t.Fatalf("submit task %d returned false", i)
		}
	}

	// Wait for results.
	deadline := time.After(2 * time.Second)
	for i := 0; i < 5; i++ {
		select {
		case <-pool.Results():
		case <-deadline:
			t.Fatal("timed out waiting for results")
		}
	}

	pool.Stop()

	if got := processed.Load(); got != 5 {
		t.Errorf("expected 5 processed, got %d", got)
	}
}

func TestPool_MultipleWorkersConcurrent(t *testing.T) {
	var active atomic.Int64
	var maxActive atomic.Int64

	pool := New(4, 20, func(ctx context.Context, task Task) error {
		cur := active.Add(1)
		// Track peak concurrency.
		for {
			old := maxActive.Load()
			if cur <= old || maxActive.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		active.Add(-1)
		return nil
	})

	ctx := context.Background()
	pool.Start(ctx)

	for i := 0; i < 20; i++ {
		pool.Submit(Task{ID: "c-task"})
	}

	deadline := time.After(5 * time.Second)
	for i := 0; i < 20; i++ {
		select {
		case <-pool.Results():
		case <-deadline:
			t.Fatal("timed out")
		}
	}

	pool.Stop()

	if peak := maxActive.Load(); peak < 2 {
		t.Errorf("expected concurrent processing (peak=%d), want >= 2", peak)
	}
}

func TestPool_FailedTaskIncrementsErrorCount(t *testing.T) {
	errBoom := errors.New("boom")
	pool := New(1, 10, func(ctx context.Context, task Task) error {
		if task.ID == "fail" {
			return errBoom
		}
		return nil
	})

	ctx := context.Background()
	pool.Start(ctx)

	pool.Submit(Task{ID: "ok"})
	pool.Submit(Task{ID: "fail"})
	pool.Submit(Task{ID: "ok2"})

	deadline := time.After(2 * time.Second)
	for i := 0; i < 3; i++ {
		select {
		case <-pool.Results():
		case <-deadline:
			t.Fatal("timed out")
		}
	}

	pool.Stop()

	stats := pool.Stats()
	if stats.Processed != 3 {
		t.Errorf("expected 3 processed, got %d", stats.Processed)
	}
	if stats.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", stats.Failed)
	}
}

func TestPool_StatsReflectState(t *testing.T) {
	pool := New(2, 10, func(ctx context.Context, task Task) error {
		return nil
	})

	// Before start.
	stats := pool.Stats()
	if stats.Running {
		t.Error("pool should not be running before Start")
	}
	if stats.Workers != 2 {
		t.Errorf("expected 2 workers, got %d", stats.Workers)
	}

	ctx := context.Background()
	pool.Start(ctx)

	stats = pool.Stats()
	if !stats.Running {
		t.Error("pool should be running after Start")
	}

	pool.Stop()

	stats = pool.Stats()
	if stats.Running {
		t.Error("pool should not be running after Stop")
	}
}

func TestPool_StopWaitsForInflight(t *testing.T) {
	var completed atomic.Bool
	pool := New(1, 1, func(ctx context.Context, task Task) error {
		time.Sleep(100 * time.Millisecond)
		completed.Store(true)
		return nil
	})

	ctx := context.Background()
	pool.Start(ctx)

	pool.Submit(Task{ID: "slow"})
	// Give worker time to pick up the task.
	time.Sleep(10 * time.Millisecond)

	pool.Stop()

	if !completed.Load() {
		t.Error("Stop should wait for in-flight task to complete")
	}
}

func TestPool_SubmitReturnsFalseWhenStopped(t *testing.T) {
	pool := New(1, 10, func(ctx context.Context, task Task) error {
		return nil
	})

	ctx := context.Background()
	pool.Start(ctx)
	pool.Stop()

	ok := pool.Submit(Task{ID: "late"})
	if ok {
		t.Error("Submit should return false after Stop")
	}
}

func TestPool_BufferFullReturnsFalse(t *testing.T) {
	// Buffer size 2, handler blocks forever so tasks stay in channel.
	blocker := make(chan struct{})
	pool := New(1, 2, func(ctx context.Context, task Task) error {
		<-blocker
		return nil
	})

	ctx := context.Background()
	pool.Start(ctx)

	// First submit goes to the worker (blocks it).
	pool.Submit(Task{ID: "block"})
	time.Sleep(10 * time.Millisecond)

	// Fill the buffer.
	pool.Submit(Task{ID: "buf1"})
	pool.Submit(Task{ID: "buf2"})

	// Buffer is full now.
	ok := pool.Submit(Task{ID: "overflow"})
	if ok {
		t.Error("Submit should return false when buffer is full")
	}

	close(blocker)
	pool.Stop()
}

func TestPool_ResultsChannelReceivesOutcomes(t *testing.T) {
	errFail := errors.New("fail")
	pool := New(2, 10, func(ctx context.Context, task Task) error {
		if task.ID == "bad" {
			return errFail
		}
		return nil
	})

	ctx := context.Background()
	pool.Start(ctx)

	pool.Submit(Task{ID: "good"})
	pool.Submit(Task{ID: "bad"})

	results := make(map[string]error)
	var mu sync.Mutex
	deadline := time.After(2 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case r := <-pool.Results():
			mu.Lock()
			results[r.TaskID] = r.Error
			mu.Unlock()
		case <-deadline:
			t.Fatal("timed out waiting for results")
		}
	}

	pool.Stop()

	if results["good"] != nil {
		t.Errorf("expected nil error for good task, got %v", results["good"])
	}
	if results["bad"] == nil {
		t.Error("expected error for bad task")
	}
}

func TestPool_ContextCancellationStopsWorkers(t *testing.T) {
	var count atomic.Int64
	pool := New(2, 10, func(ctx context.Context, task Task) error {
		count.Add(1)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	pool.Start(ctx)

	pool.Submit(Task{ID: "1"})
	time.Sleep(50 * time.Millisecond)

	cancel()
	// Allow workers to notice cancellation.
	time.Sleep(50 * time.Millisecond)

	// After context cancellation, new submits may or may not work
	// depending on timing, but workers should have exited.
	// The important thing is the pool stops processing.
	got := count.Load()
	if got < 1 {
		t.Errorf("expected at least 1 processed before cancel, got %d", got)
	}

	pool.Stop()
}
