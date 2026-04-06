// Package bitset provides a compact bit set implementation.
// Memory-efficient boolean storage using uint64 words for
// permission flags, feature toggles, and set membership.
package bitset

// BitSet is a compact set of bits backed by uint64 words.
type BitSet struct {
	words []uint64
	size  int
}

// New creates a bit set with the given capacity.
func New(size int) *BitSet {
	if size < 0 {
		size = 0
	}
	words := (size + 63) / 64
	return &BitSet{
		words: make([]uint64, words),
		size:  size,
	}
}

// Set sets bit at position i.
func (b *BitSet) Set(i int) {
	if i < 0 || i >= b.size {
		return
	}
	b.words[i/64] |= 1 << uint(i%64)
}

// Clear clears bit at position i.
func (b *BitSet) Clear(i int) {
	if i < 0 || i >= b.size {
		return
	}
	b.words[i/64] &^= 1 << uint(i%64)
}

// Test returns true if bit at position i is set.
func (b *BitSet) Test(i int) bool {
	if i < 0 || i >= b.size {
		return false
	}
	return b.words[i/64]&(1<<uint(i%64)) != 0
}

// Toggle flips bit at position i.
func (b *BitSet) Toggle(i int) {
	if i < 0 || i >= b.size {
		return
	}
	b.words[i/64] ^= 1 << uint(i%64)
}

// Size returns the capacity.
func (b *BitSet) Size() int {
	return b.size
}

// Count returns the number of set bits (popcount).
func (b *BitSet) Count() int {
	count := 0
	for _, w := range b.words {
		count += popcount(w)
	}
	return count
}

func popcount(x uint64) int {
	// Kernighan's bit counting
	count := 0
	for x != 0 {
		x &= x - 1
		count++
	}
	return count
}

// ClearAll clears all bits.
func (b *BitSet) ClearAll() {
	for i := range b.words {
		b.words[i] = 0
	}
}

// SetAll sets all bits.
func (b *BitSet) SetAll() {
	for i := range b.words {
		b.words[i] = ^uint64(0)
	}
}

// And returns a new BitSet that is the AND of b and other.
func (b *BitSet) And(other *BitSet) *BitSet {
	size := b.size
	if other.size < size {
		size = other.size
	}
	result := New(size)
	minWords := len(result.words)
	if len(b.words) < minWords {
		minWords = len(b.words)
	}
	if len(other.words) < minWords {
		minWords = len(other.words)
	}
	for i := 0; i < minWords; i++ {
		result.words[i] = b.words[i] & other.words[i]
	}
	return result
}

// Or returns a new BitSet that is the OR of b and other.
func (b *BitSet) Or(other *BitSet) *BitSet {
	size := b.size
	if other.size > size {
		size = other.size
	}
	result := New(size)
	for i := 0; i < len(b.words) && i < len(result.words); i++ {
		result.words[i] = b.words[i]
	}
	for i := 0; i < len(other.words) && i < len(result.words); i++ {
		result.words[i] |= other.words[i]
	}
	return result
}

// IsEmpty returns true if no bits are set.
func (b *BitSet) IsEmpty() bool {
	for _, w := range b.words {
		if w != 0 {
			return false
		}
	}
	return true
}

// Equal checks if two bit sets have the same bits set.
func (b *BitSet) Equal(other *BitSet) bool {
	if b.size != other.size {
		return false
	}
	for i := range b.words {
		if b.words[i] != other.words[i] {
			return false
		}
	}
	return true
}
