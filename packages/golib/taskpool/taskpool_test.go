package taskpool

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPool_BasicExecution(t *testing.T) {
	p := New[int](2)
	ctx := context.Background()
	p.Start(ctx)

	p.Submit(func(ctx context.Context) (int, error) { return 1, nil })
	p.Submit(func(ctx context.Context) (int, error) { return 2, nil })
	p.Submit(func(ctx context.Context) (int, error) { return 3, nil })

	results := p.Wait()
	if len(results) != 3 {
		t.Fatalf("results: got %d", len(results))
	}

	sum := 0
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
		}
		sum += r.Value
	}
	if sum != 6 {
		t.Errorf("sum: got %d, want 6", sum)
	}
}

func TestPool_ErrorHandling(t *testing.T) {
	p := New[string](2)
	p.Start(context.Background())

	testErr := errors.New("task failed")
	p.Submit(func(ctx context.Context) (string, error) { return "", testErr })
	p.Submit(func(ctx context.Context) (string, error) { return "ok", nil })

	results := p.Wait()

	errCount := 0
	for _, r := range results {
		if r.Err != nil {
			errCount++
		}
	}
	if errCount != 1 {
		t.Errorf("errors: got %d", errCount)
	}
}

func TestPool_Stats(t *testing.T) {
	p := New[int](2)
	p.Start(context.Background())

	p.Submit(func(ctx context.Context) (int, error) { return 1, nil })
	p.Submit(func(ctx context.Context) (int, error) { return 0, errors.New("fail") })

	p.Wait()

	submitted, completed, failed := p.Stats()
	if submitted != 2 {
		t.Errorf("submitted: got %d", submitted)
	}
	if completed != 2 {
		t.Errorf("completed: got %d", completed)
	}
	if failed != 1 {
		t.Errorf("failed: got %d", failed)
	}
}

func TestPool_Workers(t *testing.T) {
	p := New[int](4)
	if p.Workers() != 4 {
		t.Errorf("workers: got %d", p.Workers())
	}
}

func TestPool_DefaultWorkers(t *testing.T) {
	p := New[int](0) // Should default to 1.
	if p.Workers() != 1 {
		t.Errorf("default workers: got %d", p.Workers())
	}
}

func TestPool_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	p := New[int](1)
	p.Start(ctx)

	p.Submit(func(ctx context.Context) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(5 * time.Second):
			return 1, nil
		}
	})

	results := p.Wait()
	if len(results) != 1 {
		t.Fatalf("results: got %d", len(results))
	}
	if results[0].Err == nil {
		t.Error("should have context error")
	}
}

func TestPool_ResultIndex(t *testing.T) {
	p := New[string](1)
	p.Start(context.Background())

	p.Submit(func(ctx context.Context) (string, error) { return "first", nil })
	p.Submit(func(ctx context.Context) (string, error) { return "second", nil })

	results := p.Wait()
	for _, r := range results {
		if r.Index < 0 || r.Index > 1 {
			t.Errorf("invalid index: %d", r.Index)
		}
	}
}
