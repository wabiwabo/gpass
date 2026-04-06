// Package ringbuf provides a generic ring buffer (circular buffer).
// Fixed-size buffer that overwrites oldest entries when full.
// Useful for recent event logs, metric windows, and bounded queues.
package ringbuf

// Buffer is a generic ring buffer.
type Buffer[T any] struct {
	data  []T
	head  int
	count int
	cap   int
}

// New creates a ring buffer with the given capacity.
func New[T any](capacity int) *Buffer[T] {
	if capacity <= 0 {
		capacity = 16
	}
	return &Buffer[T]{
		data: make([]T, capacity),
		cap:  capacity,
	}
}

// Push adds an element. Overwrites oldest if full.
func (b *Buffer[T]) Push(v T) {
	idx := (b.head + b.count) % b.cap
	b.data[idx] = v
	if b.count == b.cap {
		b.head = (b.head + 1) % b.cap
	} else {
		b.count++
	}
}

// Pop removes and returns the oldest element.
func (b *Buffer[T]) Pop() (T, bool) {
	if b.count == 0 {
		var zero T
		return zero, false
	}
	v := b.data[b.head]
	b.head = (b.head + 1) % b.cap
	b.count--
	return v, true
}

// Peek returns the oldest element without removing it.
func (b *Buffer[T]) Peek() (T, bool) {
	if b.count == 0 {
		var zero T
		return zero, false
	}
	return b.data[b.head], true
}

// PeekLast returns the newest element without removing it.
func (b *Buffer[T]) PeekLast() (T, bool) {
	if b.count == 0 {
		var zero T
		return zero, false
	}
	idx := (b.head + b.count - 1) % b.cap
	return b.data[idx], true
}

// Len returns the number of elements.
func (b *Buffer[T]) Len() int {
	return b.count
}

// Cap returns the buffer capacity.
func (b *Buffer[T]) Cap() int {
	return b.cap
}

// IsFull returns true if the buffer is at capacity.
func (b *Buffer[T]) IsFull() bool {
	return b.count == b.cap
}

// IsEmpty returns true if the buffer has no elements.
func (b *Buffer[T]) IsEmpty() bool {
	return b.count == 0
}

// Clear removes all elements.
func (b *Buffer[T]) Clear() {
	b.head = 0
	b.count = 0
}

// ToSlice returns all elements in order (oldest first).
func (b *Buffer[T]) ToSlice() []T {
	result := make([]T, b.count)
	for i := 0; i < b.count; i++ {
		result[i] = b.data[(b.head+i)%b.cap]
	}
	return result
}

// Each calls fn for each element in order (oldest first).
func (b *Buffer[T]) Each(fn func(T)) {
	for i := 0; i < b.count; i++ {
		fn(b.data[(b.head+i)%b.cap])
	}
}
