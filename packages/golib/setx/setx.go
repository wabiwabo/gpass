// Package setx provides a generic set type backed by a map.
// Supports standard set operations: union, intersection,
// difference, and subset checks.
package setx

import "sort"

// Set is a generic set.
type Set[T comparable] struct {
	m map[T]struct{}
}

// New creates a set from elements.
func New[T comparable](elements ...T) Set[T] {
	s := Set[T]{m: make(map[T]struct{}, len(elements))}
	for _, e := range elements {
		s.m[e] = struct{}{}
	}
	return s
}

// Add adds an element to the set.
func (s Set[T]) Add(v T) {
	s.m[v] = struct{}{}
}

// Remove removes an element from the set.
func (s Set[T]) Remove(v T) {
	delete(s.m, v)
}

// Contains checks if the set contains an element.
func (s Set[T]) Contains(v T) bool {
	_, ok := s.m[v]
	return ok
}

// Len returns the number of elements.
func (s Set[T]) Len() int {
	return len(s.m)
}

// IsEmpty returns true if the set has no elements.
func (s Set[T]) IsEmpty() bool {
	return len(s.m) == 0
}

// Elements returns all elements as a slice.
func (s Set[T]) Elements() []T {
	result := make([]T, 0, len(s.m))
	for v := range s.m {
		result = append(result, v)
	}
	return result
}

// Union returns a new set with all elements from both sets.
func (s Set[T]) Union(other Set[T]) Set[T] {
	result := New[T]()
	for v := range s.m {
		result.Add(v)
	}
	for v := range other.m {
		result.Add(v)
	}
	return result
}

// Intersect returns a new set with elements in both sets.
func (s Set[T]) Intersect(other Set[T]) Set[T] {
	result := New[T]()
	for v := range s.m {
		if other.Contains(v) {
			result.Add(v)
		}
	}
	return result
}

// Difference returns a new set with elements in s but not in other.
func (s Set[T]) Difference(other Set[T]) Set[T] {
	result := New[T]()
	for v := range s.m {
		if !other.Contains(v) {
			result.Add(v)
		}
	}
	return result
}

// IsSubset checks if s is a subset of other.
func (s Set[T]) IsSubset(other Set[T]) bool {
	for v := range s.m {
		if !other.Contains(v) {
			return false
		}
	}
	return true
}

// IsSuperset checks if s is a superset of other.
func (s Set[T]) IsSuperset(other Set[T]) bool {
	return other.IsSubset(s)
}

// Equal checks if two sets have the same elements.
func (s Set[T]) Equal(other Set[T]) bool {
	if s.Len() != other.Len() {
		return false
	}
	return s.IsSubset(other)
}

// StringSet is a convenience type for string sets with sorted output.
type StringSet = Set[string]

// SortedStrings returns the elements of a string set sorted.
func SortedStrings(s Set[string]) []string {
	elements := s.Elements()
	sort.Strings(elements)
	return elements
}
