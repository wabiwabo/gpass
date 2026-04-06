// Package slotmap provides a generational slot map data structure.
// Allows O(1) insert, lookup, and delete with stable handles that
// detect use-after-free via generation counters.
package slotmap

// Handle is a key into the slot map with generation for safety.
type Handle struct {
	index      uint32
	generation uint32
}

// IsZero returns true for the zero/invalid handle.
func (h Handle) IsZero() bool {
	return h.index == 0 && h.generation == 0
}

type slot[T any] struct {
	value      T
	generation uint32
	occupied   bool
}

// Map is a generational slot map.
type Map[T any] struct {
	slots    []slot[T]
	freeList []uint32
	count    int
}

// New creates a slot map with initial capacity.
func New[T any](capacity int) *Map[T] {
	return &Map[T]{
		slots:    make([]slot[T], 0, capacity),
		freeList: make([]uint32, 0),
	}
}

// Insert adds a value and returns its handle.
func (m *Map[T]) Insert(value T) Handle {
	if len(m.freeList) > 0 {
		idx := m.freeList[len(m.freeList)-1]
		m.freeList = m.freeList[:len(m.freeList)-1]
		m.slots[idx].value = value
		m.slots[idx].generation++
		m.slots[idx].occupied = true
		m.count++
		return Handle{index: idx, generation: m.slots[idx].generation}
	}

	idx := uint32(len(m.slots))
	m.slots = append(m.slots, slot[T]{
		value:      value,
		generation: 1,
		occupied:   true,
	})
	m.count++
	return Handle{index: idx, generation: 1}
}

// Get retrieves a value by handle. Returns zero value and false if invalid.
func (m *Map[T]) Get(h Handle) (T, bool) {
	if int(h.index) >= len(m.slots) {
		var zero T
		return zero, false
	}
	s := &m.slots[h.index]
	if !s.occupied || s.generation != h.generation {
		var zero T
		return zero, false
	}
	return s.value, true
}

// Remove deletes a value by handle. Returns false if handle was invalid.
func (m *Map[T]) Remove(h Handle) bool {
	if int(h.index) >= len(m.slots) {
		return false
	}
	s := &m.slots[h.index]
	if !s.occupied || s.generation != h.generation {
		return false
	}
	var zero T
	s.value = zero
	s.occupied = false
	m.freeList = append(m.freeList, h.index)
	m.count--
	return true
}

// Update replaces a value by handle. Returns false if handle was invalid.
func (m *Map[T]) Update(h Handle, value T) bool {
	if int(h.index) >= len(m.slots) {
		return false
	}
	s := &m.slots[h.index]
	if !s.occupied || s.generation != h.generation {
		return false
	}
	s.value = value
	return true
}

// Len returns the number of occupied slots.
func (m *Map[T]) Len() int {
	return m.count
}

// Each iterates over all occupied slots.
func (m *Map[T]) Each(fn func(Handle, T) bool) {
	for i, s := range m.slots {
		if s.occupied {
			if !fn(Handle{index: uint32(i), generation: s.generation}, s.value) {
				return
			}
		}
	}
}
