// Package bloom provides a space-efficient probabilistic data structure
// for set membership testing. It can have false positives but never
// false negatives — useful for dedup checks, cache warming, and
// preventing unnecessary database lookups at scale.
package bloom

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"sync"
)

// Filter is a Bloom filter.
type Filter struct {
	mu   sync.RWMutex
	bits []uint64
	size uint64 // Total bits.
	k    uint64 // Number of hash functions.
	n    uint64 // Items added.
}

// New creates a Bloom filter sized for expectedItems with the given
// false positive rate (e.g., 0.01 for 1%).
func New(expectedItems uint64, fpRate float64) *Filter {
	if expectedItems == 0 {
		expectedItems = 1000
	}
	if fpRate <= 0 || fpRate >= 1 {
		fpRate = 0.01
	}

	// Optimal size: m = -n*ln(p) / (ln(2))^2
	m := uint64(math.Ceil(-float64(expectedItems) * math.Log(fpRate) / (math.Ln2 * math.Ln2)))
	// Optimal hash functions: k = (m/n) * ln(2)
	k := uint64(math.Ceil(float64(m) / float64(expectedItems) * math.Ln2))
	if k < 1 {
		k = 1
	}

	// Round m up to nearest 64 for clean uint64 array.
	words := (m + 63) / 64
	m = words * 64

	return &Filter{
		bits: make([]uint64, words),
		size: m,
		k:    k,
	}
}

// Add adds an item to the filter.
func (f *Filter) Add(data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()

	h1, h2 := hash(data)
	for i := uint64(0); i < f.k; i++ {
		pos := (h1 + i*h2) % f.size
		f.bits[pos/64] |= 1 << (pos % 64)
	}
	f.n++
}

// AddString adds a string item.
func (f *Filter) AddString(s string) {
	f.Add([]byte(s))
}

// Contains tests whether data might be in the set.
// False positives are possible; false negatives are not.
func (f *Filter) Contains(data []byte) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	h1, h2 := hash(data)
	for i := uint64(0); i < f.k; i++ {
		pos := (h1 + i*h2) % f.size
		if f.bits[pos/64]&(1<<(pos%64)) == 0 {
			return false
		}
	}
	return true
}

// ContainsString tests a string item.
func (f *Filter) ContainsString(s string) bool {
	return f.Contains([]byte(s))
}

// Count returns the number of items added.
func (f *Filter) Count() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.n
}

// EstimatedFPRate returns the current estimated false positive rate
// based on the number of items added.
func (f *Filter) EstimatedFPRate() float64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// p = (1 - e^(-kn/m))^k
	exp := math.Exp(-float64(f.k) * float64(f.n) / float64(f.size))
	return math.Pow(1-exp, float64(f.k))
}

// FillRatio returns the fraction of bits set (0.0-1.0).
func (f *Filter) FillRatio() float64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	set := uint64(0)
	for _, word := range f.bits {
		set += uint64(popcount(word))
	}
	return float64(set) / float64(f.size)
}

// Clear resets the filter.
func (f *Filter) Clear() {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i := range f.bits {
		f.bits[i] = 0
	}
	f.n = 0
}

// hash uses SHA-256 to generate two independent hashes.
func hash(data []byte) (uint64, uint64) {
	h := sha256.Sum256(data)
	h1 := binary.BigEndian.Uint64(h[0:8])
	h2 := binary.BigEndian.Uint64(h[8:16])
	return h1, h2
}

// popcount counts set bits (Hamming weight).
func popcount(x uint64) int {
	x = x - ((x >> 1) & 0x5555555555555555)
	x = (x & 0x3333333333333333) + ((x >> 2) & 0x3333333333333333)
	x = (x + (x >> 4)) & 0x0f0f0f0f0f0f0f0f
	return int((x * 0x0101010101010101) >> 56)
}
