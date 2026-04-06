// Package chanutil provides channel utility functions for common
// concurrent patterns. Fan-in, merge, and timeout helpers for
// clean goroutine coordination.
package chanutil

import (
	"context"
	"sync"
	"time"
)

// Merge combines multiple channels into one. The output channel
// closes when all input channels close.
func Merge[T any](channels ...<-chan T) <-chan T {
	out := make(chan T)
	var wg sync.WaitGroup
	for _, ch := range channels {
		wg.Add(1)
		go func(c <-chan T) {
			defer wg.Done()
			for v := range c {
				out <- v
			}
		}(ch)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// OrDone reads from a channel until context is done.
func OrDone[T any](ctx context.Context, ch <-chan T) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-ch:
				if !ok {
					return
				}
				select {
				case out <- v:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}

// WithTimeout reads from a channel with a timeout per item.
func WithTimeout[T any](ch <-chan T, timeout time.Duration) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for {
			select {
			case v, ok := <-ch:
				if !ok {
					return
				}
				out <- v
			case <-time.After(timeout):
				return
			}
		}
	}()
	return out
}

// Generate creates a channel that emits values from a function.
func Generate[T any](ctx context.Context, fn func() T) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case out <- fn():
			}
		}
	}()
	return out
}

// Collect reads all values from a channel into a slice.
func Collect[T any](ch <-chan T) []T {
	var result []T
	for v := range ch {
		result = append(result, v)
	}
	return result
}

// SendCtx sends a value to a channel, respecting context cancellation.
func SendCtx[T any](ctx context.Context, ch chan<- T, value T) bool {
	select {
	case ch <- value:
		return true
	case <-ctx.Done():
		return false
	}
}
