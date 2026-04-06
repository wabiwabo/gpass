// Package mathx provides generic math utilities.
// Min, Max, Clamp, Abs, and other common operations for
// ordered numeric types using Go 1.21+ generics.
package mathx

import "cmp"

// Min returns the smaller of two values.
func Min[T cmp.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

// Max returns the larger of two values.
func Max[T cmp.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

// Clamp restricts a value to the range [lo, hi].
func Clamp[T cmp.Ordered](v, lo, hi T) T {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Abs returns the absolute value of a signed number.
func Abs[T interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~float32 | ~float64
}](v T) T {
	if v < 0 {
		return -v
	}
	return v
}

// Sum returns the sum of all values.
func Sum[T interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}](values []T) T {
	var s T
	for _, v := range values {
		s += v
	}
	return s
}

// Average returns the mean of values.
func Average[T interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}](values []T) float64 {
	if len(values) == 0 {
		return 0
	}
	var s float64
	for _, v := range values {
		s += float64(v)
	}
	return s / float64(len(values))
}

// Percent calculates percentage: (part / total) * 100.
func Percent(part, total float64) float64 {
	if total == 0 {
		return 0
	}
	return (part / total) * 100
}

// DivCeil returns the ceiling of integer division.
func DivCeil(a, b int) int {
	if b == 0 {
		return 0
	}
	return (a + b - 1) / b
}

// InRange checks if v is in [lo, hi] inclusive.
func InRange[T cmp.Ordered](v, lo, hi T) bool {
	return v >= lo && v <= hi
}
