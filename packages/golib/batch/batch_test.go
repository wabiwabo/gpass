package batch

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestProcess_AllSucceed(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	results := Process(context.Background(), items, 3, func(_ context.Context, n int) (int, error) {
		return n * 2, nil
	})

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Err != nil {
			t.Errorf("item %d: unexpected error: %v", i, r.Err)
		}
		if r.Item != items[i]*2 {
			t.Errorf("item %d: got %d, want %d", i, r.Item, items[i]*2)
		}
		if r.Index != i {
			t.Errorf("item %d: index %d", i, r.Index)
		}
	}
}

func TestProcess_SomeFail(t *testing.T) {
	items := []string{"ok", "fail", "ok", "fail"}
	results := Process(context.Background(), items, 2, func(_ context.Context, s string) (string, error) {
		if s == "fail" {
			return "", errors.New("failed")
		}
		return s + "!", nil
	})

	succeeded := 0
	failed := 0
	for _, r := range results {
		if r.Err != nil {
			failed++
		} else {
			succeeded++
		}
	}
	if succeeded != 2 || failed != 2 {
		t.Errorf("succeeded=%d, failed=%d, want 2/2", succeeded, failed)
	}
}

func TestProcess_Empty(t *testing.T) {
	results := Process(context.Background(), []int{}, 5, func(_ context.Context, n int) (int, error) {
		return n, nil
	})
	if results != nil {
		t.Errorf("empty input should return nil, got %v", results)
	}
}

func TestProcess_ConcurrencyLimit(t *testing.T) {
	var maxConcurrent atomic.Int32
	var current atomic.Int32

	items := make([]int, 20)
	for i := range items {
		items[i] = i
	}

	Process(context.Background(), items, 3, func(_ context.Context, n int) (int, error) {
		cur := current.Add(1)
		for {
			old := maxConcurrent.Load()
			if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		current.Add(-1)
		return n, nil
	})

	if maxConcurrent.Load() > 3 {
		t.Errorf("max concurrent: got %d, want <= 3", maxConcurrent.Load())
	}
}

func TestProcess_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	items := make([]int, 100)
	var started atomic.Int32

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	results := Process(ctx, items, 2, func(ctx context.Context, n int) (int, error) {
		started.Add(1)
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(time.Second):
			return n, nil
		}
	})

	cancelled := 0
	for _, r := range results {
		if r.Err != nil {
			cancelled++
		}
	}
	if cancelled == 0 {
		t.Error("some items should fail with context cancellation")
	}
}

func TestProcess_PreservesOrder(t *testing.T) {
	items := []int{10, 20, 30, 40, 50}
	results := Process(context.Background(), items, 5, func(_ context.Context, n int) (int, error) {
		// Random delay to test ordering.
		time.Sleep(time.Duration(50-n) * time.Millisecond)
		return n, nil
	})

	for i, r := range results {
		if r.Item != items[i] {
			t.Errorf("results[%d] = %d, want %d (order not preserved)", i, r.Item, items[i])
		}
	}
}

func TestProcessWithRetry_SucceedsAfterRetry(t *testing.T) {
	var attempts atomic.Int32

	items := []string{"retry"}
	results := ProcessWithRetry(context.Background(), items, 1, 3, func(_ context.Context, s string) (string, error) {
		n := attempts.Add(1)
		if n < 3 {
			return "", errors.New("temporary")
		}
		return "success", nil
	})

	if results[0].Err != nil {
		t.Errorf("should succeed after retry: %v", results[0].Err)
	}
	if results[0].Item != "success" {
		t.Errorf("result: got %q", results[0].Item)
	}
}

func TestProcessWithRetry_ExhaustsRetries(t *testing.T) {
	items := []string{"permanent-fail"}
	results := ProcessWithRetry(context.Background(), items, 1, 2, func(_ context.Context, s string) (string, error) {
		return "", errors.New("permanent")
	})

	if results[0].Err == nil {
		t.Error("should fail after exhausting retries")
	}
}

func TestChunk(t *testing.T) {
	tests := []struct {
		name     string
		items    []int
		size     int
		expected int
	}{
		{"exact", []int{1, 2, 3, 4}, 2, 2},
		{"remainder", []int{1, 2, 3, 4, 5}, 2, 3},
		{"single", []int{1, 2, 3}, 10, 1},
		{"empty", []int{}, 5, 0},
		{"one-per-chunk", []int{1, 2, 3}, 1, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := Chunk(tt.items, tt.size)
			if len(chunks) != tt.expected {
				t.Errorf("got %d chunks, want %d", len(chunks), tt.expected)
			}
			// Verify all items are present.
			total := 0
			for _, c := range chunks {
				total += len(c)
			}
			if total != len(tt.items) {
				t.Errorf("total items: got %d, want %d", total, len(tt.items))
			}
		})
	}
}

func TestSummarize(t *testing.T) {
	results := []Result[string]{
		{Item: "a", Err: nil},
		{Item: "", Err: errors.New("fail")},
		{Item: "c", Err: nil},
	}

	stats := Summarize(results, 100*time.Millisecond)
	if stats.Total != 3 {
		t.Errorf("total: got %d", stats.Total)
	}
	if stats.Succeeded != 2 {
		t.Errorf("succeeded: got %d", stats.Succeeded)
	}
	if stats.Failed != 1 {
		t.Errorf("failed: got %d", stats.Failed)
	}
}
