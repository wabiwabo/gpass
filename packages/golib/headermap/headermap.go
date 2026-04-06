// Package headermap provides case-insensitive header management
// for HTTP-like protocols. Normalizes header names and provides
// typed getters for common value patterns.
package headermap

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// Headers is a case-insensitive header map.
type Headers struct {
	m map[string][]string
}

// New creates an empty header map.
func New() *Headers {
	return &Headers{m: make(map[string][]string)}
}

// FromMap creates a header map from a string map.
func FromMap(m map[string]string) *Headers {
	h := New()
	for k, v := range m {
		h.Set(k, v)
	}
	return h
}

// Set sets a header value, replacing any existing values.
func (h *Headers) Set(key, value string) {
	h.m[normalize(key)] = []string{value}
}

// Add adds a value to a header key.
func (h *Headers) Add(key, value string) {
	k := normalize(key)
	h.m[k] = append(h.m[k], value)
}

// Get returns the first value for a key.
func (h *Headers) Get(key string) string {
	vals := h.m[normalize(key)]
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// GetAll returns all values for a key.
func (h *Headers) GetAll(key string) []string {
	return h.m[normalize(key)]
}

// Has checks if a header exists.
func (h *Headers) Has(key string) bool {
	_, ok := h.m[normalize(key)]
	return ok
}

// Del deletes a header.
func (h *Headers) Del(key string) {
	delete(h.m, normalize(key))
}

// GetInt returns a header value as an integer.
func (h *Headers) GetInt(key string) (int, bool) {
	v := h.Get(key)
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	return n, true
}

// GetBool returns a header value as a boolean.
func (h *Headers) GetBool(key string) bool {
	v := strings.ToLower(h.Get(key))
	return v == "true" || v == "1" || v == "yes"
}

// GetTime parses a header value as RFC3339 time.
func (h *Headers) GetTime(key string) (time.Time, bool) {
	v := h.Get(key)
	if v == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// Keys returns all header keys sorted.
func (h *Headers) Keys() []string {
	keys := make([]string, 0, len(h.m))
	for k := range h.m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Len returns the number of headers.
func (h *Headers) Len() int {
	return len(h.m)
}

// ToMap returns a flat string map (first value only).
func (h *Headers) ToMap() map[string]string {
	result := make(map[string]string, len(h.m))
	for k, vals := range h.m {
		if len(vals) > 0 {
			result[k] = vals[0]
		}
	}
	return result
}

// Clone returns a deep copy.
func (h *Headers) Clone() *Headers {
	c := New()
	for k, vals := range h.m {
		copied := make([]string, len(vals))
		copy(copied, vals)
		c.m[k] = copied
	}
	return c
}

func normalize(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}
