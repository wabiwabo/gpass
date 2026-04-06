// Package keypair provides key-value pair utilities for building
// ordered parameter lists. Used for canonical string construction
// in HMAC signing, query string building, and form encoding.
package keypair

import (
	"net/url"
	"sort"
	"strings"
)

// Pair is a key-value pair.
type Pair struct {
	Key   string
	Value string
}

// Pairs is an ordered list of key-value pairs.
type Pairs []Pair

// New creates pairs from alternating key-value strings.
func New(keyvals ...string) Pairs {
	pairs := make(Pairs, 0, len(keyvals)/2)
	for i := 0; i+1 < len(keyvals); i += 2 {
		pairs = append(pairs, Pair{Key: keyvals[i], Value: keyvals[i+1]})
	}
	return pairs
}

// FromMap creates pairs from a map (sorted by key).
func FromMap(m map[string]string) Pairs {
	pairs := make(Pairs, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, Pair{Key: k, Value: v})
	}
	pairs.Sort()
	return pairs
}

// Add appends a pair.
func (p *Pairs) Add(key, value string) {
	*p = append(*p, Pair{Key: key, Value: value})
}

// Get returns the first value for a key.
func (p Pairs) Get(key string) string {
	for _, pair := range p {
		if pair.Key == key {
			return pair.Value
		}
	}
	return ""
}

// Has checks if a key exists.
func (p Pairs) Has(key string) bool {
	for _, pair := range p {
		if pair.Key == key {
			return true
		}
	}
	return false
}

// Sort sorts pairs by key.
func (p Pairs) Sort() {
	sort.Slice(p, func(i, j int) bool {
		return p[i].Key < p[j].Key
	})
}

// Canonical returns a canonical string representation.
// Format: key1=value1&key2=value2 (sorted by key).
func (p Pairs) Canonical(separator, kvSep string) string {
	sorted := make(Pairs, len(p))
	copy(sorted, p)
	sorted.Sort()

	parts := make([]string, len(sorted))
	for i, pair := range sorted {
		parts[i] = pair.Key + kvSep + pair.Value
	}
	return strings.Join(parts, separator)
}

// QueryString returns URL-encoded query string.
func (p Pairs) QueryString() string {
	v := url.Values{}
	for _, pair := range p {
		v.Add(pair.Key, pair.Value)
	}
	return v.Encode()
}

// ToMap converts to a string map (last value wins for duplicates).
func (p Pairs) ToMap() map[string]string {
	m := make(map[string]string, len(p))
	for _, pair := range p {
		m[pair.Key] = pair.Value
	}
	return m
}

// Len returns the number of pairs.
func (p Pairs) Len() int {
	return len(p)
}

// Keys returns all keys.
func (p Pairs) Keys() []string {
	keys := make([]string, len(p))
	for i, pair := range p {
		keys[i] = pair.Key
	}
	return keys
}

// Values returns all values.
func (p Pairs) Values() []string {
	vals := make([]string, len(p))
	for i, pair := range p {
		vals[i] = pair.Value
	}
	return vals
}
