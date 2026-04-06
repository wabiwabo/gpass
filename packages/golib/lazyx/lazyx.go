// Package lazyx provides lazy initialization primitives.
// Values are computed once on first access, then cached.
// Thread-safe using sync.Once semantics.
package lazyx

import "sync"

// Value is a lazily initialized value.
type Value[T any] struct {
	once sync.Once
	fn   func() T
	val  T
}

// New creates a lazy value with the given initializer.
func New[T any](fn func() T) *Value[T] {
	return &Value[T]{fn: fn}
}

// Get returns the value, initializing it on first call.
func (v *Value[T]) Get() T {
	v.once.Do(func() {
		v.val = v.fn()
	})
	return v.val
}

// ErrorValue is a lazily initialized value that may fail.
type ErrorValue[T any] struct {
	once sync.Once
	fn   func() (T, error)
	val  T
	err  error
}

// NewWithError creates a lazy value that may return an error.
func NewWithError[T any](fn func() (T, error)) *ErrorValue[T] {
	return &ErrorValue[T]{fn: fn}
}

// Get returns the value or error, initializing on first call.
func (v *ErrorValue[T]) Get() (T, error) {
	v.once.Do(func() {
		v.val, v.err = v.fn()
	})
	return v.val, v.err
}

// MustGet returns the value, panicking on error.
func (v *ErrorValue[T]) MustGet() T {
	val, err := v.Get()
	if err != nil {
		panic(err)
	}
	return val
}
