// Package priority provides a generic priority queue backed by
// a binary heap. Supports min-heap and max-heap configurations.
package priority

import "container/heap"

// Item is a priority queue entry.
type Item[T any] struct {
	Value    T
	Priority int
	index    int
}

// Queue is a generic priority queue.
type Queue[T any] struct {
	h *pqHeap[T]
}

// NewMin creates a min-priority queue (lowest priority first).
func NewMin[T any]() *Queue[T] {
	h := &pqHeap[T]{
		lessFunc: func(a, b int) bool { return a < b },
	}
	heap.Init(h)
	return &Queue[T]{h: h}
}

// NewMax creates a max-priority queue (highest priority first).
func NewMax[T any]() *Queue[T] {
	h := &pqHeap[T]{
		lessFunc: func(a, b int) bool { return a > b },
	}
	heap.Init(h)
	return &Queue[T]{h: h}
}

// Push adds an item to the queue.
func (q *Queue[T]) Push(value T, priority int) {
	heap.Push(q.h, &Item[T]{Value: value, Priority: priority})
}

// Pop removes and returns the highest-priority item.
func (q *Queue[T]) Pop() (T, int, bool) {
	if q.h.Len() == 0 {
		var zero T
		return zero, 0, false
	}
	item := heap.Pop(q.h).(*Item[T])
	return item.Value, item.Priority, true
}

// Peek returns the highest-priority item without removing it.
func (q *Queue[T]) Peek() (T, int, bool) {
	if q.h.Len() == 0 {
		var zero T
		return zero, 0, false
	}
	item := q.h.items[0]
	return item.Value, item.Priority, true
}

// Len returns the number of items.
func (q *Queue[T]) Len() int {
	return q.h.Len()
}

// IsEmpty returns true if the queue has no items.
func (q *Queue[T]) IsEmpty() bool {
	return q.h.Len() == 0
}

// pqHeap implements heap.Interface.
type pqHeap[T any] struct {
	items    []*Item[T]
	lessFunc func(a, b int) bool
}

func (h pqHeap[T]) Len() int { return len(h.items) }

func (h pqHeap[T]) Less(i, j int) bool {
	return h.lessFunc(h.items[i].Priority, h.items[j].Priority)
}

func (h pqHeap[T]) Swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
	h.items[i].index = i
	h.items[j].index = j
}

func (h *pqHeap[T]) Push(x interface{}) {
	item := x.(*Item[T])
	item.index = len(h.items)
	h.items = append(h.items, item)
}

func (h *pqHeap[T]) Pop() interface{} {
	old := h.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	h.items = old[:n-1]
	return item
}
