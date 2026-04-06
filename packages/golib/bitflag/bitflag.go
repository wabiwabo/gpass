// Package bitflag provides type-safe bitwise flag operations.
// Supports setting, clearing, toggling, and checking flags
// with named flag definitions.
package bitflag

// Flags is a set of bit flags.
type Flags uint64

// Set enables a flag.
func (f Flags) Set(flag Flags) Flags {
	return f | flag
}

// Clear disables a flag.
func (f Flags) Clear(flag Flags) Flags {
	return f &^ flag
}

// Toggle flips a flag.
func (f Flags) Toggle(flag Flags) Flags {
	return f ^ flag
}

// Has checks if a flag is set.
func (f Flags) Has(flag Flags) bool {
	return f&flag == flag
}

// HasAny checks if any of the given flags are set.
func (f Flags) HasAny(flags Flags) bool {
	return f&flags != 0
}

// HasAll checks if all given flags are set.
func (f Flags) HasAll(flags Flags) bool {
	return f&flags == flags
}

// IsZero returns true if no flags are set.
func (f Flags) IsZero() bool {
	return f == 0
}

// Count returns the number of set bits.
func (f Flags) Count() int {
	n := 0
	v := uint64(f)
	for v != 0 {
		n++
		v &= v - 1 // clear least significant set bit
	}
	return n
}

// String returns a binary representation.
func (f Flags) String() string {
	if f == 0 {
		return "0"
	}
	v := uint64(f)
	result := make([]byte, 0, 64)
	started := false
	for i := 63; i >= 0; i-- {
		if v&(1<<uint(i)) != 0 {
			started = true
			result = append(result, '1')
		} else if started {
			result = append(result, '0')
		}
	}
	return string(result)
}

// Named provides named flag definitions for readable code.
type Named struct {
	names map[string]Flags
}

// NewNamed creates a named flag set.
func NewNamed() *Named {
	return &Named{names: make(map[string]Flags)}
}

// Define registers a named flag.
func (n *Named) Define(name string, flag Flags) {
	n.names[name] = flag
}

// Get returns a flag by name.
func (n *Named) Get(name string) (Flags, bool) {
	f, ok := n.names[name]
	return f, ok
}

// Names returns all flag names that are set.
func (n *Named) Names(flags Flags) []string {
	var result []string
	for name, f := range n.names {
		if flags.Has(f) {
			result = append(result, name)
		}
	}
	return result
}
