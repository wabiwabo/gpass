// Package iterx provides generic iteration utilities for working
// with channels, generators, and sequential data processing.
package iterx

// Range generates integers from start to end (exclusive).
func Range(start, end int) []int {
	if end <= start {
		return nil
	}
	result := make([]int, 0, end-start)
	for i := start; i < end; i++ {
		result = append(result, i)
	}
	return result
}

// Repeat creates a slice with n copies of value.
func Repeat[T any](value T, n int) []T {
	if n <= 0 {
		return nil
	}
	result := make([]T, n)
	for i := range result {
		result[i] = value
	}
	return result
}

// Zip combines two slices into pairs. Truncates to shorter length.
func Zip[A, B any](a []A, b []B) []struct{ First A; Second B } {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	result := make([]struct{ First A; Second B }, n)
	for i := 0; i < n; i++ {
		result[i] = struct{ First A; Second B }{a[i], b[i]}
	}
	return result
}

// Enumerate adds indices to slice elements.
func Enumerate[T any](s []T) []struct{ Index int; Value T } {
	result := make([]struct{ Index int; Value T }, len(s))
	for i, v := range s {
		result[i] = struct{ Index int; Value T }{i, v}
	}
	return result
}

// GroupBy groups elements by a key function.
func GroupBy[T any, K comparable](s []T, key func(T) K) map[K][]T {
	result := make(map[K][]T)
	for _, item := range s {
		k := key(item)
		result[k] = append(result[k], item)
	}
	return result
}

// CountBy counts elements by a key function.
func CountBy[T any, K comparable](s []T, key func(T) K) map[K]int {
	result := make(map[K]int)
	for _, item := range s {
		result[key(item)]++
	}
	return result
}

// Any returns true if any element matches the predicate.
func Any[T any](s []T, fn func(T) bool) bool {
	for _, item := range s {
		if fn(item) {
			return true
		}
	}
	return false
}

// All returns true if all elements match the predicate.
func All[T any](s []T, fn func(T) bool) bool {
	for _, item := range s {
		if !fn(item) {
			return false
		}
	}
	return true
}

// None returns true if no elements match the predicate.
func None[T any](s []T, fn func(T) bool) bool {
	return !Any(s, fn)
}

// Sum sums numeric values.
func Sum[T ~int | ~int64 | ~float64](s []T) T {
	var total T
	for _, v := range s {
		total += v
	}
	return total
}

// Min returns the minimum value.
func Min[T ~int | ~int64 | ~float64 | ~string](s []T) T {
	if len(s) == 0 {
		var zero T
		return zero
	}
	m := s[0]
	for _, v := range s[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

// Max returns the maximum value.
func Max[T ~int | ~int64 | ~float64 | ~string](s []T) T {
	if len(s) == 0 {
		var zero T
		return zero
	}
	m := s[0]
	for _, v := range s[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
