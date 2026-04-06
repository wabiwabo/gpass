// Package trie provides a generic trie (prefix tree) for string
// key lookups. Supports prefix matching, iteration, and longest
// prefix search. Useful for routing, autocomplete, and IP lookup.
package trie

// Trie is a generic prefix tree.
type Trie[V any] struct {
	root *node[V]
	size int
}

type node[V any] struct {
	children map[rune]*node[V]
	value    V
	hasValue bool
}

// New creates an empty trie.
func New[V any]() *Trie[V] {
	return &Trie[V]{root: &node[V]{children: make(map[rune]*node[V])}}
}

// Set inserts or updates a key-value pair.
func (t *Trie[V]) Set(key string, value V) {
	n := t.root
	for _, ch := range key {
		child, ok := n.children[ch]
		if !ok {
			child = &node[V]{children: make(map[rune]*node[V])}
			n.children[ch] = child
		}
		n = child
	}
	if !n.hasValue {
		t.size++
	}
	n.value = value
	n.hasValue = true
}

// Get retrieves a value by exact key.
func (t *Trie[V]) Get(key string) (V, bool) {
	n := t.find(key)
	if n == nil || !n.hasValue {
		var zero V
		return zero, false
	}
	return n.value, true
}

// Has checks if an exact key exists.
func (t *Trie[V]) Has(key string) bool {
	n := t.find(key)
	return n != nil && n.hasValue
}

// Delete removes a key. Returns true if found.
func (t *Trie[V]) Delete(key string) bool {
	n := t.find(key)
	if n == nil || !n.hasValue {
		return false
	}
	var zero V
	n.value = zero
	n.hasValue = false
	t.size--
	return true
}

// HasPrefix checks if any key starts with the prefix.
func (t *Trie[V]) HasPrefix(prefix string) bool {
	return t.find(prefix) != nil
}

// LongestPrefix returns the longest key that is a prefix of the input.
func (t *Trie[V]) LongestPrefix(s string) (string, V, bool) {
	var lastKey string
	var lastVal V
	found := false

	n := t.root
	pos := 0
	for i, ch := range s {
		if n.hasValue {
			lastKey = s[:i]
			lastVal = n.value
			found = true
		}
		child, ok := n.children[ch]
		if !ok {
			break
		}
		n = child
		pos = i + len(string(ch))
	}
	if n.hasValue && pos == len(s) {
		lastKey = s
		lastVal = n.value
		found = true
	} else if n.hasValue && pos < len(s) {
		// Traversal stopped mid-string but current node has value
		lastKey = s[:pos]
		lastVal = n.value
		found = true
	}

	return lastKey, lastVal, found
}

// Len returns the number of keys.
func (t *Trie[V]) Len() int {
	return t.size
}

func (t *Trie[V]) find(key string) *node[V] {
	n := t.root
	for _, ch := range key {
		child, ok := n.children[ch]
		if !ok {
			return nil
		}
		n = child
	}
	return n
}
