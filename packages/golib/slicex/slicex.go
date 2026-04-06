// Package slicex provides generic slice utility functions.
// Fills gaps in the standard library with type-safe operations
// for filtering, mapping, and transforming slices.
package slicex

// Contains checks if a slice contains an element.
func Contains[T comparable](s []T, v T) bool {
	for _, item := range s {
		if item == v {
			return true
		}
	}
	return false
}

// Filter returns elements matching the predicate.
func Filter[T any](s []T, fn func(T) bool) []T {
	result := make([]T, 0, len(s))
	for _, item := range s {
		if fn(item) {
			result = append(result, item)
		}
	}
	return result
}

// Map transforms each element.
func Map[T, U any](s []T, fn func(T) U) []U {
	result := make([]U, len(s))
	for i, item := range s {
		result[i] = fn(item)
	}
	return result
}

// Unique returns deduplicated elements preserving order.
func Unique[T comparable](s []T) []T {
	seen := make(map[T]bool, len(s))
	result := make([]T, 0, len(s))
	for _, item := range s {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

// Chunk splits a slice into chunks of the given size.
func Chunk[T any](s []T, size int) [][]T {
	if size <= 0 {
		return nil
	}
	chunks := make([][]T, 0, (len(s)+size-1)/size)
	for i := 0; i < len(s); i += size {
		end := i + size
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[i:end])
	}
	return chunks
}

// Flatten flattens a slice of slices.
func Flatten[T any](s [][]T) []T {
	total := 0
	for _, inner := range s {
		total += len(inner)
	}
	result := make([]T, 0, total)
	for _, inner := range s {
		result = append(result, inner...)
	}
	return result
}

// First returns the first element matching the predicate.
func First[T any](s []T, fn func(T) bool) (T, bool) {
	for _, item := range s {
		if fn(item) {
			return item, true
		}
	}
	var zero T
	return zero, false
}

// Last returns the last element matching the predicate.
func Last[T any](s []T, fn func(T) bool) (T, bool) {
	for i := len(s) - 1; i >= 0; i-- {
		if fn(s[i]) {
			return s[i], true
		}
	}
	var zero T
	return zero, false
}

// Reduce reduces a slice to a single value.
func Reduce[T, U any](s []T, initial U, fn func(U, T) U) U {
	acc := initial
	for _, item := range s {
		acc = fn(acc, item)
	}
	return acc
}

// Partition splits a slice into two based on a predicate.
func Partition[T any](s []T, fn func(T) bool) (match, rest []T) {
	match = make([]T, 0)
	rest = make([]T, 0)
	for _, item := range s {
		if fn(item) {
			match = append(match, item)
		} else {
			rest = append(rest, item)
		}
	}
	return
}

// Reverse returns a reversed copy of the slice.
func Reverse[T any](s []T) []T {
	result := make([]T, len(s))
	for i, item := range s {
		result[len(s)-1-i] = item
	}
	return result
}
