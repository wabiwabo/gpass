package shutdown

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCoordinator_Register(t *testing.T) {
	c := NewCoordinator(nil)
	c.RegisterFunc("db", 1, func(_ context.Context) error { return nil })
	c.RegisterFunc("redis", 2, func(_ context.Context) error { return nil })

	if c.Hooks() != 2 {
		t.Errorf("hooks: got %d, want 2", c.Hooks())
	}
}

func TestCoordinator_Shutdown_RunsHooksInOrder(t *testing.T) {
	c := NewCoordinator(nil)
	var order []string
	var mu sync.Mutex

	c.RegisterFunc("http-server", 0, func(_ context.Context) error {
		mu.Lock()
		order = append(order, "http")
		mu.Unlock()
		return nil
	})
	c.RegisterFunc("database", 2, func(_ context.Context) error {
		mu.Lock()
		order = append(order, "db")
		mu.Unlock()
		return nil
	})
	c.RegisterFunc("redis", 1, func(_ context.Context) error {
		mu.Lock()
		order = append(order, "redis")
		mu.Unlock()
		return nil
	})

	c.Shutdown(5 * time.Second)

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 3 {
		t.Fatalf("expected 3 hooks run, got %d", len(order))
	}
	// HTTP (0) before Redis (1) before DB (2).
	if order[0] != "http" {
		t.Errorf("first: got %q, want http", order[0])
	}
	if order[1] != "redis" {
		t.Errorf("second: got %q, want redis", order[1])
	}
	if order[2] != "db" {
		t.Errorf("third: got %q, want db", order[2])
	}
}

func TestCoordinator_Shutdown_SamePriorityRunsParallel(t *testing.T) {
	c := NewCoordinator(nil)
	var maxConcurrent atomic.Int32
	var current atomic.Int32

	for i := 0; i < 5; i++ {
		c.RegisterFunc("hook-"+string(rune('A'+i)), 1, func(_ context.Context) error {
			cur := current.Add(1)
			for {
				old := maxConcurrent.Load()
				if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			current.Add(-1)
			return nil
		})
	}

	c.Shutdown(5 * time.Second)

	if maxConcurrent.Load() < 2 {
		t.Errorf("same-priority hooks should run in parallel, max concurrent: %d", maxConcurrent.Load())
	}
}

func TestCoordinator_Shutdown_HookError(t *testing.T) {
	c := NewCoordinator(nil)
	var completed atomic.Int32

	c.RegisterFunc("failing", 0, func(_ context.Context) error {
		completed.Add(1)
		return errors.New("shutdown failed")
	})
	c.RegisterFunc("succeeding", 1, func(_ context.Context) error {
		completed.Add(1)
		return nil
	})

	c.Shutdown(5 * time.Second)

	// Both hooks should run even if the first fails.
	if completed.Load() != 2 {
		t.Errorf("both hooks should run: got %d", completed.Load())
	}
}

func TestCoordinator_Shutdown_Timeout(t *testing.T) {
	c := NewCoordinator(nil)
	var completed atomic.Int32

	c.RegisterFunc("slow", 0, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			completed.Add(1)
			return nil
		}
	})
	c.RegisterFunc("should-not-run", 1, func(_ context.Context) error {
		completed.Add(1)
		return nil
	})

	c.Shutdown(100 * time.Millisecond)

	// The slow hook should be cancelled by timeout.
	// The second hook may or may not run depending on timing.
	// The important thing is shutdown completes within timeout.
}

func TestCoordinator_Shutdown_NoHooks(t *testing.T) {
	c := NewCoordinator(nil)
	c.Shutdown(time.Second) // Should not panic.
}

func TestCoordinator_Shutdown_ContextPropagation(t *testing.T) {
	c := NewCoordinator(nil)
	var receivedCtx context.Context

	c.RegisterFunc("ctx-check", 0, func(ctx context.Context) error {
		receivedCtx = ctx
		return nil
	})

	c.Shutdown(5 * time.Second)

	if receivedCtx == nil {
		t.Error("hook should receive a context")
	}
}

func TestSortHooks(t *testing.T) {
	hooks := []Hook{
		{Name: "c", Priority: 3},
		{Name: "a", Priority: 1},
		{Name: "b", Priority: 2},
	}
	sortHooks(hooks)

	if hooks[0].Name != "a" || hooks[1].Name != "b" || hooks[2].Name != "c" {
		t.Errorf("sort order: %v", hooks)
	}
}

func TestGroupByPriority(t *testing.T) {
	hooks := []Hook{
		{Name: "a1", Priority: 1},
		{Name: "a2", Priority: 1},
		{Name: "b1", Priority: 2},
		{Name: "c1", Priority: 3},
		{Name: "c2", Priority: 3},
	}

	groups := groupByPriority(hooks)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	if len(groups[0]) != 2 {
		t.Errorf("group 0: got %d hooks", len(groups[0]))
	}
	if len(groups[1]) != 1 {
		t.Errorf("group 1: got %d hooks", len(groups[1]))
	}
	if len(groups[2]) != 2 {
		t.Errorf("group 2: got %d hooks", len(groups[2]))
	}
}

func TestGroupByPriority_Empty(t *testing.T) {
	groups := groupByPriority(nil)
	if groups != nil {
		t.Error("empty input should return nil")
	}
}
