// Package atomicx provides generic atomic value wrappers.
// Extends sync/atomic with type-safe operations for common types.
package atomicx

import (
	"sync/atomic"
	"time"
)

// Value is a generic atomic value.
type Value[T any] struct {
	v atomic.Value
}

// NewValue creates an atomic value with an initial value.
func NewValue[T any](initial T) *Value[T] {
	v := &Value[T]{}
	v.v.Store(initial)
	return v
}

// Load returns the current value.
func (v *Value[T]) Load() T {
	val := v.v.Load()
	if val == nil {
		var zero T
		return zero
	}
	return val.(T)
}

// Store sets the value.
func (v *Value[T]) Store(val T) {
	v.v.Store(val)
}

// Bool is an atomic boolean.
type Bool struct {
	v atomic.Int32
}

// NewBool creates an atomic boolean.
func NewBool(initial bool) *Bool {
	b := &Bool{}
	if initial {
		b.v.Store(1)
	}
	return b
}

// Load returns the current value.
func (b *Bool) Load() bool {
	return b.v.Load() != 0
}

// Store sets the value.
func (b *Bool) Store(val bool) {
	if val {
		b.v.Store(1)
	} else {
		b.v.Store(0)
	}
}

// Toggle flips the value and returns the new value.
func (b *Bool) Toggle() bool {
	for {
		old := b.v.Load()
		newVal := int32(1)
		if old != 0 {
			newVal = 0
		}
		if b.v.CompareAndSwap(old, newVal) {
			return newVal != 0
		}
	}
}

// CompareAndSwap atomically sets to new if current == old.
func (b *Bool) CompareAndSwap(old, new bool) bool {
	oldInt := int32(0)
	newInt := int32(0)
	if old {
		oldInt = 1
	}
	if new {
		newInt = 1
	}
	return b.v.CompareAndSwap(oldInt, newInt)
}

// Duration is an atomic time.Duration.
type Duration struct {
	v atomic.Int64
}

// NewDuration creates an atomic duration.
func NewDuration(initial time.Duration) *Duration {
	d := &Duration{}
	d.v.Store(int64(initial))
	return d
}

// Load returns the current duration.
func (d *Duration) Load() time.Duration {
	return time.Duration(d.v.Load())
}

// Store sets the duration.
func (d *Duration) Store(val time.Duration) {
	d.v.Store(int64(val))
}

// String is an atomic string using atomic.Value.
type String struct {
	v atomic.Value
}

// NewString creates an atomic string.
func NewString(initial string) *String {
	s := &String{}
	s.v.Store(initial)
	return s
}

// Load returns the current string.
func (s *String) Load() string {
	v := s.v.Load()
	if v == nil {
		return ""
	}
	return v.(string)
}

// Store sets the string.
func (s *String) Store(val string) {
	s.v.Store(val)
}
