// Package mapx provides generic map utility functions.
// Type-safe operations for map manipulation, merging,
// filtering, and transformation.
package mapx

import "sort"

// Keys returns all keys from a map sorted alphabetically.
func Keys[K ~string, V any](m map[K]V) []K {
	result := make([]K, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result
}

// Values returns all values from a map.
func Values[K comparable, V any](m map[K]V) []V {
	result := make([]V, 0, len(m))
	for _, v := range m {
		result = append(result, v)
	}
	return result
}

// Merge merges multiple maps. Later maps override earlier ones.
func Merge[K comparable, V any](maps ...map[K]V) map[K]V {
	result := make(map[K]V)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// Filter returns a new map with only entries matching the predicate.
func Filter[K comparable, V any](m map[K]V, fn func(K, V) bool) map[K]V {
	result := make(map[K]V)
	for k, v := range m {
		if fn(k, v) {
			result[k] = v
		}
	}
	return result
}

// MapValues transforms all values in a map.
func MapValues[K comparable, V, U any](m map[K]V, fn func(V) U) map[K]U {
	result := make(map[K]U, len(m))
	for k, v := range m {
		result[k] = fn(v)
	}
	return result
}

// Pick returns a new map with only the specified keys.
func Pick[K comparable, V any](m map[K]V, keys ...K) map[K]V {
	result := make(map[K]V)
	for _, k := range keys {
		if v, ok := m[k]; ok {
			result[k] = v
		}
	}
	return result
}

// Omit returns a new map without the specified keys.
func Omit[K comparable, V any](m map[K]V, keys ...K) map[K]V {
	exclude := make(map[K]bool, len(keys))
	for _, k := range keys {
		exclude[k] = true
	}
	result := make(map[K]V)
	for k, v := range m {
		if !exclude[k] {
			result[k] = v
		}
	}
	return result
}

// Contains checks if a map contains a key.
func Contains[K comparable, V any](m map[K]V, key K) bool {
	_, ok := m[key]
	return ok
}

// GetOr returns the value for a key, or a default if not found.
func GetOr[K comparable, V any](m map[K]V, key K, def V) V {
	if v, ok := m[key]; ok {
		return v
	}
	return def
}

// Invert swaps keys and values. Last write wins for duplicates.
func Invert[K, V comparable](m map[K]V) map[V]K {
	result := make(map[V]K, len(m))
	for k, v := range m {
		result[v] = k
	}
	return result
}
