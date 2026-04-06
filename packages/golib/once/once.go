// Package once provides enhanced once-execution primitives
// beyond sync.Once. Includes resettable once, value-returning
// once, and conditional once patterns.
package once

import (
	"sync"
	"sync/atomic"
)

// Resettable is a sync.Once that can be reset.
type Resettable struct {
	mu   sync.Mutex
	done atomic.Int32
}

// Do executes fn if it hasn't been called since the last Reset.
func (o *Resettable) Do(fn func()) {
	if o.done.Load() == 1 {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.done.Load() == 0 {
		fn()
		o.done.Store(1)
	}
}

// Reset allows the next Do call to execute.
func (o *Resettable) Reset() {
	o.done.Store(0)
}

// Done returns true if fn has been executed.
func (o *Resettable) Done() bool {
	return o.done.Load() == 1
}

// Value executes fn once and caches the result.
type Value[T any] struct {
	once sync.Once
	fn   func() T
	val  T
}

// NewValue creates a once-value.
func NewValue[T any](fn func() T) *Value[T] {
	return &Value[T]{fn: fn}
}

// Get returns the cached value, executing fn on first call.
func (v *Value[T]) Get() T {
	v.once.Do(func() {
		v.val = v.fn()
	})
	return v.val
}

// ErrorValue executes fn once, caching both value and error.
type ErrorValue[T any] struct {
	once sync.Once
	fn   func() (T, error)
	val  T
	err  error
}

// NewErrorValue creates a once-error-value.
func NewErrorValue[T any](fn func() (T, error)) *ErrorValue[T] {
	return &ErrorValue[T]{fn: fn}
}

// Get returns the cached result.
func (v *ErrorValue[T]) Get() (T, error) {
	v.once.Do(func() {
		v.val, v.err = v.fn()
	})
	return v.val, v.err
}

// RetryOnError executes fn once, but retries on error.
type RetryOnError[T any] struct {
	mu  sync.Mutex
	fn  func() (T, error)
	val T
	ok  bool
}

// NewRetryOnError creates a retry-on-error once.
func NewRetryOnError[T any](fn func() (T, error)) *RetryOnError[T] {
	return &RetryOnError[T]{fn: fn}
}

// Get returns the value, retrying fn if previous attempts failed.
func (v *RetryOnError[T]) Get() (T, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.ok {
		return v.val, nil
	}

	val, err := v.fn()
	if err != nil {
		return val, err
	}

	v.val = val
	v.ok = true
	return val, nil
}
