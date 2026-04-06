// Package sortx provides generic sorting utilities with common
// sort patterns: sort by field, stable sort, top-N, and multi-key
// sorting for struct slices.
package sortx

import "sort"

// By sorts a slice by the result of a key function.
func By[T any, K ~int | ~int64 | ~float64 | ~string](s []T, key func(T) K) {
	sort.Slice(s, func(i, j int) bool {
		return key(s[i]) < key(s[j])
	})
}

// ByDesc sorts a slice in descending order by key.
func ByDesc[T any, K ~int | ~int64 | ~float64 | ~string](s []T, key func(T) K) {
	sort.Slice(s, func(i, j int) bool {
		return key(s[i]) > key(s[j])
	})
}

// StableBy sorts a slice with stable ordering by key.
func StableBy[T any, K ~int | ~int64 | ~float64 | ~string](s []T, key func(T) K) {
	sort.SliceStable(s, func(i, j int) bool {
		return key(s[i]) < key(s[j])
	})
}

// TopN returns the top N elements sorted by key (ascending).
// If n > len(s), returns all elements.
func TopN[T any, K ~int | ~int64 | ~float64 | ~string](s []T, n int, key func(T) K) []T {
	if n <= 0 {
		return nil
	}
	if n >= len(s) {
		result := make([]T, len(s))
		copy(result, s)
		By(result, key)
		return result
	}
	result := make([]T, len(s))
	copy(result, s)
	By(result, key)
	return result[:n]
}

// BottomN returns the bottom N elements sorted by key (descending).
func BottomN[T any, K ~int | ~int64 | ~float64 | ~string](s []T, n int, key func(T) K) []T {
	if n <= 0 {
		return nil
	}
	if n >= len(s) {
		result := make([]T, len(s))
		copy(result, s)
		ByDesc(result, key)
		return result
	}
	result := make([]T, len(s))
	copy(result, s)
	ByDesc(result, key)
	return result[:n]
}

// IsSorted checks if a slice is sorted by key.
func IsSorted[T any, K ~int | ~int64 | ~float64 | ~string](s []T, key func(T) K) bool {
	for i := 1; i < len(s); i++ {
		if key(s[i]) < key(s[i-1]) {
			return false
		}
	}
	return true
}
