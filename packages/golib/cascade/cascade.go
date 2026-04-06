// Package cascade provides cascading fallback for multi-source data
// fetching. It tries sources in priority order and falls back to the
// next source on failure — useful for cache→database→API chains.
package cascade

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Source fetches data from one tier.
type Source[T any] struct {
	Name    string
	Fetch   func(ctx context.Context, key string) (T, error)
	Store   func(ctx context.Context, key string, value T) error // Optional: populate this source on miss.
	Timeout time.Duration
}

// Chain tries sources in order until one succeeds.
type Chain[T any] struct {
	sources []Source[T]
	stats   chainStats
}

type chainStats struct {
	hits   sync.Map // source name → *atomic.Int64
	misses sync.Map
	errors sync.Map
}

// NewChain creates a new cascade chain from sources in priority order.
func NewChain[T any](sources ...Source[T]) *Chain[T] {
	return &Chain[T]{sources: sources}
}

// Result holds the fetch result with metadata.
type Result[T any] struct {
	Value  T
	Source string        // Which source provided the value.
	Depth  int           // How many sources were tried (1-indexed).
	Latency time.Duration
}

// Get fetches a value, trying sources in order.
// On a cache miss (source N fails, source N+1 succeeds), it optionally
// back-populates source N via its Store function.
func (c *Chain[T]) Get(ctx context.Context, key string) (Result[T], error) {
	start := time.Now()

	var lastErr error
	for i, src := range c.sources {
		srcCtx := ctx
		if src.Timeout > 0 {
			var cancel context.CancelFunc
			srcCtx, cancel = context.WithTimeout(ctx, src.Timeout)
			defer cancel()
		}

		value, err := src.Fetch(srcCtx, key)
		if err == nil {
			c.recordHit(src.Name)

			// Back-populate missed sources.
			for j := 0; j < i; j++ {
				if c.sources[j].Store != nil {
					go c.sources[j].Store(context.Background(), key, value)
				}
			}

			return Result[T]{
				Value:   value,
				Source:  src.Name,
				Depth:   i + 1,
				Latency: time.Since(start),
			}, nil
		}

		c.recordMiss(src.Name)
		lastErr = err
	}

	var zero T
	return Result[T]{Value: zero, Depth: len(c.sources), Latency: time.Since(start)},
		fmt.Errorf("cascade: all %d sources failed, last error: %w", len(c.sources), lastErr)
}

func (c *Chain[T]) recordHit(name string) {
	v, _ := c.stats.hits.LoadOrStore(name, &atomic.Int64{})
	v.(*atomic.Int64).Add(1)
}

func (c *Chain[T]) recordMiss(name string) {
	v, _ := c.stats.misses.LoadOrStore(name, &atomic.Int64{})
	v.(*atomic.Int64).Add(1)
}

// Stats returns per-source hit/miss counts.
func (c *Chain[T]) Stats() map[string]SourceStats {
	result := make(map[string]SourceStats)
	for _, src := range c.sources {
		ss := SourceStats{}
		if v, ok := c.stats.hits.Load(src.Name); ok {
			ss.Hits = v.(*atomic.Int64).Load()
		}
		if v, ok := c.stats.misses.Load(src.Name); ok {
			ss.Misses = v.(*atomic.Int64).Load()
		}
		result[src.Name] = ss
	}
	return result
}

// SourceStats holds per-source statistics.
type SourceStats struct {
	Hits   int64 `json:"hits"`
	Misses int64 `json:"misses"`
}

// HitRatio returns the hit ratio (0.0-1.0) for a source.
func (s SourceStats) HitRatio() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total)
}
