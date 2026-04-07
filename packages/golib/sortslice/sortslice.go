// Package sortslice provides generic slice sorting utilities.
// Built on slices.SortFunc for type-safe, allocation-free sorting
// with common comparison helpers for strings, ints, and time.
package sortslice

import (
	"cmp"
	"slices"
	"strings"
	"time"
)

// Asc sorts a slice of ordered values in ascending order.
func Asc[T cmp.Ordered](s []T) {
	slices.SortFunc(s, func(a, b T) int {
		return cmp.Compare(a, b)
	})
}

// Desc sorts a slice of ordered values in descending order.
func Desc[T cmp.Ordered](s []T) {
	slices.SortFunc(s, func(a, b T) int {
		return cmp.Compare(b, a)
	})
}

// By sorts a slice using a key extraction function.
func By[T any, K cmp.Ordered](s []T, key func(T) K) {
	slices.SortFunc(s, func(a, b T) int {
		return cmp.Compare(key(a), key(b))
	})
}

// ByDesc sorts a slice in descending order using a key function.
func ByDesc[T any, K cmp.Ordered](s []T, key func(T) K) {
	slices.SortFunc(s, func(a, b T) int {
		return cmp.Compare(key(b), key(a))
	})
}

// ByTime sorts a slice by a time.Time key extraction function.
func ByTime[T any](s []T, key func(T) time.Time) {
	slices.SortFunc(s, func(a, b T) int {
		ta, tb := key(a), key(b)
		if ta.Before(tb) {
			return -1
		}
		if ta.After(tb) {
			return 1
		}
		return 0
	})
}

// ByTimeDesc sorts a slice by time in descending order.
func ByTimeDesc[T any](s []T, key func(T) time.Time) {
	slices.SortFunc(s, func(a, b T) int {
		ta, tb := key(a), key(b)
		if ta.After(tb) {
			return -1
		}
		if ta.Before(tb) {
			return 1
		}
		return 0
	})
}

// CaseInsensitive sorts strings case-insensitively.
func CaseInsensitive(s []string) {
	slices.SortFunc(s, func(a, b string) int {
		return strings.Compare(strings.ToLower(a), strings.ToLower(b))
	})
}

// Stable sorts a slice using a comparison function (stable sort).
func Stable[T any](s []T, cmpFn func(a, b T) int) {
	slices.SortStableFunc(s, cmpFn)
}

// IsSorted checks if a slice is sorted in ascending order.
func IsSorted[T cmp.Ordered](s []T) bool {
	return slices.IsSortedFunc(s, func(a, b T) int {
		return cmp.Compare(a, b)
	})
}

// Unique returns a new slice with duplicates removed (input must be sorted).
func Unique[T comparable](s []T) []T {
	if len(s) == 0 {
		return s
	}
	result := make([]T, 0, len(s))
	result = append(result, s[0])
	for i := 1; i < len(s); i++ {
		if s[i] != s[i-1] {
			result = append(result, s[i])
		}
	}
	return result
}
