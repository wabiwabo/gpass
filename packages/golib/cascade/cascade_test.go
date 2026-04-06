package cascade

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestChain_FirstSourceHit(t *testing.T) {
	chain := NewChain[string](
		Source[string]{
			Name:  "cache",
			Fetch: func(ctx context.Context, key string) (string, error) { return "cached", nil },
		},
		Source[string]{
			Name:  "db",
			Fetch: func(ctx context.Context, key string) (string, error) { return "from-db", nil },
		},
	)

	result, err := chain.Get(context.Background(), "key")
	if err != nil {
		t.Fatal(err)
	}
	if result.Value != "cached" {
		t.Errorf("value: got %q", result.Value)
	}
	if result.Source != "cache" {
		t.Errorf("source: got %q", result.Source)
	}
	if result.Depth != 1 {
		t.Errorf("depth: got %d", result.Depth)
	}
}

func TestChain_FallbackToSecond(t *testing.T) {
	chain := NewChain[string](
		Source[string]{
			Name:  "cache",
			Fetch: func(ctx context.Context, key string) (string, error) { return "", errors.New("miss") },
		},
		Source[string]{
			Name:  "db",
			Fetch: func(ctx context.Context, key string) (string, error) { return "from-db", nil },
		},
	)

	result, err := chain.Get(context.Background(), "key")
	if err != nil {
		t.Fatal(err)
	}
	if result.Value != "from-db" {
		t.Errorf("value: got %q", result.Value)
	}
	if result.Source != "db" {
		t.Errorf("source: got %q", result.Source)
	}
	if result.Depth != 2 {
		t.Errorf("depth: got %d", result.Depth)
	}
}

func TestChain_AllFail(t *testing.T) {
	chain := NewChain[string](
		Source[string]{
			Name:  "cache",
			Fetch: func(ctx context.Context, key string) (string, error) { return "", errors.New("miss") },
		},
		Source[string]{
			Name:  "db",
			Fetch: func(ctx context.Context, key string) (string, error) { return "", errors.New("down") },
		},
	)

	_, err := chain.Get(context.Background(), "key")
	if err == nil {
		t.Error("should fail when all sources fail")
	}
}

func TestChain_BackPopulate(t *testing.T) {
	var populated atomic.Bool
	chain := NewChain[string](
		Source[string]{
			Name:  "cache",
			Fetch: func(ctx context.Context, key string) (string, error) { return "", errors.New("miss") },
			Store: func(ctx context.Context, key string, value string) error {
				populated.Store(true)
				return nil
			},
		},
		Source[string]{
			Name:  "db",
			Fetch: func(ctx context.Context, key string) (string, error) { return "from-db", nil },
		},
	)

	result, err := chain.Get(context.Background(), "key")
	if err != nil {
		t.Fatal(err)
	}
	if result.Value != "from-db" {
		t.Error("should get value from db")
	}

	// Give time for async back-populate.
	time.Sleep(50 * time.Millisecond)

	if !populated.Load() {
		t.Error("should back-populate cache")
	}
}

func TestChain_Stats(t *testing.T) {
	chain := NewChain[string](
		Source[string]{
			Name:  "cache",
			Fetch: func(ctx context.Context, key string) (string, error) { return "", errors.New("miss") },
		},
		Source[string]{
			Name:  "db",
			Fetch: func(ctx context.Context, key string) (string, error) { return "value", nil },
		},
	)

	chain.Get(context.Background(), "key1")
	chain.Get(context.Background(), "key2")

	stats := chain.Stats()
	cacheStats := stats["cache"]
	dbStats := stats["db"]

	if cacheStats.Misses != 2 {
		t.Errorf("cache misses: got %d", cacheStats.Misses)
	}
	if cacheStats.Hits != 0 {
		t.Errorf("cache hits: got %d", cacheStats.Hits)
	}
	if dbStats.Hits != 2 {
		t.Errorf("db hits: got %d", dbStats.Hits)
	}
}

func TestChain_SourceTimeout(t *testing.T) {
	chain := NewChain[string](
		Source[string]{
			Name:    "slow",
			Timeout: 50 * time.Millisecond,
			Fetch: func(ctx context.Context, key string) (string, error) {
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(5 * time.Second):
					return "slow", nil
				}
			},
		},
		Source[string]{
			Name:  "fast",
			Fetch: func(ctx context.Context, key string) (string, error) { return "fast", nil },
		},
	)

	result, err := chain.Get(context.Background(), "key")
	if err != nil {
		t.Fatal(err)
	}
	if result.Source != "fast" {
		t.Errorf("should fall back to fast source: got %q", result.Source)
	}
}

func TestChain_Latency(t *testing.T) {
	chain := NewChain[string](
		Source[string]{
			Name: "cache",
			Fetch: func(ctx context.Context, key string) (string, error) {
				time.Sleep(10 * time.Millisecond)
				return "value", nil
			},
		},
	)

	result, _ := chain.Get(context.Background(), "key")
	if result.Latency < 5*time.Millisecond {
		t.Errorf("latency should be at least 5ms: got %v", result.Latency)
	}
}

func TestSourceStats_HitRatio(t *testing.T) {
	s := SourceStats{Hits: 75, Misses: 25}
	if r := s.HitRatio(); r != 0.75 {
		t.Errorf("ratio: got %f", r)
	}

	s = SourceStats{}
	if r := s.HitRatio(); r != 0 {
		t.Error("empty should return 0")
	}
}

func TestChain_ThreeSources(t *testing.T) {
	chain := NewChain[string](
		Source[string]{
			Name:  "l1",
			Fetch: func(ctx context.Context, key string) (string, error) { return "", errors.New("miss") },
		},
		Source[string]{
			Name:  "l2",
			Fetch: func(ctx context.Context, key string) (string, error) { return "", errors.New("miss") },
		},
		Source[string]{
			Name:  "l3",
			Fetch: func(ctx context.Context, key string) (string, error) { return "deep", nil },
		},
	)

	result, err := chain.Get(context.Background(), "key")
	if err != nil {
		t.Fatal(err)
	}
	if result.Depth != 3 {
		t.Errorf("depth: got %d", result.Depth)
	}
	if result.Source != "l3" {
		t.Errorf("source: got %q", result.Source)
	}
}
