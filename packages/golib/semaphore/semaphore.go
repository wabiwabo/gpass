// Package semaphore provides a weighted counting semaphore for
// concurrency control. It supports both blocking and non-blocking
// acquire with context cancellation and timeout.
package semaphore

import (
	"context"
	"sync"
)

// Weighted is a weighted counting semaphore.
type Weighted struct {
	mu      sync.Mutex
	size    int64
	cur     int64
	waiters []waiter
}

type waiter struct {
	n    int64
	done chan struct{}
}

// NewWeighted creates a semaphore with the given maximum weight.
func NewWeighted(n int64) *Weighted {
	return &Weighted{size: n}
}

// Acquire acquires the semaphore with weight n, blocking until resources
// are available or ctx is done.
func (s *Weighted) Acquire(ctx context.Context, n int64) error {
	s.mu.Lock()

	if s.cur+n <= s.size && len(s.waiters) == 0 {
		s.cur += n
		s.mu.Unlock()
		return nil
	}

	// Need to wait.
	w := waiter{n: n, done: make(chan struct{})}
	s.waiters = append(s.waiters, w)
	s.mu.Unlock()

	select {
	case <-ctx.Done():
		s.mu.Lock()
		// Remove from waiters.
		for i, ww := range s.waiters {
			if ww.done == w.done {
				s.waiters = append(s.waiters[:i], s.waiters[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
		return ctx.Err()
	case <-w.done:
		return nil
	}
}

// TryAcquire attempts to acquire without blocking.
// Returns true if successful, false if resources aren't available.
func (s *Weighted) TryAcquire(n int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cur+n <= s.size && len(s.waiters) == 0 {
		s.cur += n
		return true
	}
	return false
}

// Release releases the semaphore with weight n.
func (s *Weighted) Release(n int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cur -= n
	if s.cur < 0 {
		panic("semaphore: released more than acquired")
	}

	// Wake waiters that can proceed.
	for len(s.waiters) > 0 {
		w := s.waiters[0]
		if s.cur+w.n > s.size {
			break
		}
		s.cur += w.n
		s.waiters = s.waiters[1:]
		close(w.done)
	}
}

// Available returns the remaining capacity.
func (s *Weighted) Available() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.size - s.cur
}

// Size returns the maximum weight.
func (s *Weighted) Size() int64 {
	return s.size
}

// InUse returns the currently acquired weight.
func (s *Weighted) InUse() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur
}

// WaiterCount returns the number of blocked waiters.
func (s *Weighted) WaiterCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.waiters)
}
