package batch

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Result represents the outcome of processing a single item.
type Result[T any] struct {
	Item  T
	Index int
	Err   error
}

// Process executes fn for each item concurrently with bounded parallelism.
// Returns results in the same order as input items.
func Process[T any, R any](ctx context.Context, items []T, concurrency int, fn func(context.Context, T) (R, error)) []Result[R] {
	if concurrency <= 0 {
		concurrency = 1
	}
	if len(items) == 0 {
		return nil
	}

	results := make([]Result[R], len(items))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)
		go func(idx int, it T) {
			defer wg.Done()

			sem <- struct{}{} // acquire
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				results[idx] = Result[R]{Index: idx, Err: ctx.Err()}
				return
			default:
			}

			val, err := fn(ctx, it)
			results[idx] = Result[R]{Item: val, Index: idx, Err: err}
		}(i, item)
	}

	wg.Wait()
	return results
}

// ProcessWithRetry processes items with retry on failure.
func ProcessWithRetry[T any, R any](ctx context.Context, items []T, concurrency, maxRetries int, fn func(context.Context, T) (R, error)) []Result[R] {
	wrappedFn := func(ctx context.Context, item T) (R, error) {
		var lastErr error
		var result R
		for attempt := 0; attempt <= maxRetries; attempt++ {
			result, lastErr = fn(ctx, item)
			if lastErr == nil {
				return result, nil
			}
			if attempt < maxRetries {
				select {
				case <-ctx.Done():
					return result, ctx.Err()
				case <-time.After(time.Duration(1<<attempt) * 100 * time.Millisecond):
				}
			}
		}
		return result, lastErr
	}

	return Process(ctx, items, concurrency, wrappedFn)
}

// Chunk splits a slice into chunks of the given size.
func Chunk[T any](items []T, size int) [][]T {
	if size <= 0 {
		size = 1
	}
	var chunks [][]T
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[i:end])
	}
	return chunks
}

// Stats tracks batch processing statistics.
type Stats struct {
	Total     int64
	Succeeded int64
	Failed    int64
	Duration  time.Duration
}

// Summarize computes stats from batch results.
func Summarize[T any](results []Result[T], duration time.Duration) Stats {
	var succeeded, failed atomic.Int64
	for _, r := range results {
		if r.Err != nil {
			failed.Add(1)
		} else {
			succeeded.Add(1)
		}
	}
	return Stats{
		Total:     int64(len(results)),
		Succeeded: succeeded.Load(),
		Failed:    failed.Load(),
		Duration:  duration,
	}
}
