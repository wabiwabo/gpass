// Package iterset provides generic set operations using Go generics.
// Useful for permission checking, tag intersection, and entity
// relationship computations across services.
package iterset

// Set is a generic hash set.
type Set[T comparable] struct {
	items map[T]struct{}
}

// New creates a new set from items.
func New[T comparable](items ...T) *Set[T] {
	s := &Set[T]{items: make(map[T]struct{}, len(items))}
	for _, item := range items {
		s.items[item] = struct{}{}
	}
	return s
}

// Add adds items to the set.
func (s *Set[T]) Add(items ...T) {
	for _, item := range items {
		s.items[item] = struct{}{}
	}
}

// Remove removes an item.
func (s *Set[T]) Remove(item T) {
	delete(s.items, item)
}

// Contains checks membership.
func (s *Set[T]) Contains(item T) bool {
	_, ok := s.items[item]
	return ok
}

// Len returns the size.
func (s *Set[T]) Len() int {
	return len(s.items)
}

// Items returns all items as a slice.
func (s *Set[T]) Items() []T {
	result := make([]T, 0, len(s.items))
	for item := range s.items {
		result = append(result, item)
	}
	return result
}

// Union returns a new set with all items from both sets.
func (s *Set[T]) Union(other *Set[T]) *Set[T] {
	result := New[T]()
	for item := range s.items {
		result.Add(item)
	}
	for item := range other.items {
		result.Add(item)
	}
	return result
}

// Intersection returns a new set with items in both sets.
func (s *Set[T]) Intersection(other *Set[T]) *Set[T] {
	result := New[T]()
	// Iterate over the smaller set.
	small, big := s, other
	if s.Len() > other.Len() {
		small, big = other, s
	}
	for item := range small.items {
		if big.Contains(item) {
			result.Add(item)
		}
	}
	return result
}

// Difference returns items in s but not in other.
func (s *Set[T]) Difference(other *Set[T]) *Set[T] {
	result := New[T]()
	for item := range s.items {
		if !other.Contains(item) {
			result.Add(item)
		}
	}
	return result
}

// IsSubset checks if all items in s are in other.
func (s *Set[T]) IsSubset(other *Set[T]) bool {
	for item := range s.items {
		if !other.Contains(item) {
			return false
		}
	}
	return true
}

// IsSuperset checks if s contains all items in other.
func (s *Set[T]) IsSuperset(other *Set[T]) bool {
	return other.IsSubset(s)
}

// Equal checks if both sets contain the same items.
func (s *Set[T]) Equal(other *Set[T]) bool {
	if s.Len() != other.Len() {
		return false
	}
	return s.IsSubset(other)
}

// Clear removes all items.
func (s *Set[T]) Clear() {
	s.items = make(map[T]struct{})
}

// IsEmpty checks if the set has no items.
func (s *Set[T]) IsEmpty() bool {
	return len(s.items) == 0
}
