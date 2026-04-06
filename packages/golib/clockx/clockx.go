// Package clockx provides a clock interface for testable time
// operations. Production code uses real time; tests can inject
// a controllable fake clock.
package clockx

import (
	"sync"
	"time"
)

// Clock provides time operations.
type Clock interface {
	Now() time.Time
	Since(t time.Time) time.Duration
	After(d time.Duration) <-chan time.Time
}

// Real returns a clock that uses real time.
func Real() Clock {
	return realClock{}
}

type realClock struct{}

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) Since(t time.Time) time.Duration        { return time.Since(t) }
func (realClock) After(d time.Duration) <-chan time.Time  { return time.After(d) }

// Fake is a controllable clock for testing.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

// NewFake creates a fake clock set to the given time.
func NewFake(t time.Time) *Fake {
	return &Fake{now: t}
}

// Now returns the fake current time.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Since returns time elapsed since t.
func (f *Fake) Since(t time.Time) time.Duration {
	return f.Now().Sub(t)
}

// After returns a channel that fires immediately (for testing).
func (f *Fake) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	ch <- f.Now().Add(d)
	return ch
}

// Advance moves the clock forward.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

// Set sets the clock to a specific time.
func (f *Fake) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = t
}
