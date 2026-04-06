package interval

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunner_Basic(t *testing.T) {
	r := NewRunner()
	var count atomic.Int64

	r.Add(Task{
		Name:     "counter",
		Interval: 20 * time.Millisecond,
		Fn:       func(ctx context.Context) error { count.Add(1); return nil },
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	r.Start(ctx)
	<-ctx.Done()
	r.Stop()

	if count.Load() < 2 {
		t.Errorf("should run at least twice: got %d", count.Load())
	}
}

func TestRunner_Stats(t *testing.T) {
	r := NewRunner()
	r.Add(Task{
		Name:     "stats-test",
		Interval: 10 * time.Millisecond,
		Fn:       func(ctx context.Context) error { return nil },
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	r.Start(ctx)
	<-ctx.Done()
	r.Stop()

	runs, errors := r.Stats("stats-test")
	if runs < 1 {
		t.Errorf("runs: got %d", runs)
	}
	if errors != 0 {
		t.Errorf("errors: got %d", errors)
	}
}

func TestRunner_ErrorStats(t *testing.T) {
	r := NewRunner()
	r.Add(Task{
		Name:     "fail-task",
		Interval: 10 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			return context.DeadlineExceeded
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	r.Start(ctx)
	<-ctx.Done()
	r.Stop()

	_, errors := r.Stats("fail-task")
	if errors < 1 {
		t.Errorf("errors should be > 0: got %d", errors)
	}
}

func TestRunner_MultipleTasks(t *testing.T) {
	r := NewRunner()
	var a, b atomic.Int64

	r.Add(Task{Name: "a", Interval: 15 * time.Millisecond, Fn: func(ctx context.Context) error { a.Add(1); return nil }})
	r.Add(Task{Name: "b", Interval: 15 * time.Millisecond, Fn: func(ctx context.Context) error { b.Add(1); return nil }})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	r.Start(ctx)
	<-ctx.Done()
	r.Stop()

	if a.Load() < 1 || b.Load() < 1 {
		t.Errorf("both tasks should run: a=%d, b=%d", a.Load(), b.Load())
	}
}

func TestRunner_StopIdempotent(t *testing.T) {
	r := NewRunner()
	r.Stop() // Should not panic when not started.
}

func TestRunner_TaskCount(t *testing.T) {
	r := NewRunner()
	r.Add(Task{Name: "a", Interval: time.Second, Fn: func(ctx context.Context) error { return nil }})
	r.Add(Task{Name: "b", Interval: time.Second, Fn: func(ctx context.Context) error { return nil }})

	if r.TaskCount() != 2 {
		t.Errorf("task count: got %d", r.TaskCount())
	}
}

func TestRunner_IsRunning(t *testing.T) {
	r := NewRunner()
	r.Add(Task{Name: "t", Interval: time.Second, Fn: func(ctx context.Context) error { return nil }})

	if r.IsRunning() {
		t.Error("should not be running initially")
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.Start(ctx)

	if !r.IsRunning() {
		t.Error("should be running after start")
	}

	cancel()
	r.Stop()

	if r.IsRunning() {
		t.Error("should not be running after stop")
	}
}

func TestRunner_DefaultInterval(t *testing.T) {
	r := NewRunner()
	r.Add(Task{Name: "default", Fn: func(ctx context.Context) error { return nil }})
	// Should not panic with zero interval — defaults to 1 minute.
	if r.TaskCount() != 1 {
		t.Error("should add task")
	}
}

func TestRunner_DoubleStart(t *testing.T) {
	r := NewRunner()
	r.Add(Task{Name: "t", Interval: time.Second, Fn: func(ctx context.Context) error { return nil }})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r.Start(ctx)
	r.Start(ctx) // Should not start twice.
	r.Stop()
}

func TestRunner_StatsUnknownTask(t *testing.T) {
	r := NewRunner()
	runs, errors := r.Stats("nonexistent")
	if runs != 0 || errors != 0 {
		t.Error("unknown task should return 0")
	}
}
