// Package arena provides a simple memory arena allocator for
// batch allocation patterns. Reduces GC pressure by allocating
// objects from a contiguous pool that is freed all at once.
package arena

// Arena is a simple bump allocator for byte slices.
type Arena struct {
	chunks    [][]byte
	current   []byte
	offset    int
	chunkSize int
}

// New creates an arena with the given chunk size.
func New(chunkSize int) *Arena {
	if chunkSize < 1024 {
		chunkSize = 1024
	}
	chunk := make([]byte, chunkSize)
	return &Arena{
		chunks:    [][]byte{chunk},
		current:   chunk,
		chunkSize: chunkSize,
	}
}

// Alloc allocates n bytes from the arena.
func (a *Arena) Alloc(n int) []byte {
	if n <= 0 {
		return nil
	}
	if a.offset+n > len(a.current) {
		size := a.chunkSize
		if n > size {
			size = n
		}
		chunk := make([]byte, size)
		a.chunks = append(a.chunks, chunk)
		a.current = chunk
		a.offset = 0
	}
	slice := a.current[a.offset : a.offset+n]
	a.offset += n
	return slice
}

// AllocString copies a string into the arena and returns it.
func (a *Arena) AllocString(s string) string {
	buf := a.Alloc(len(s))
	copy(buf, s)
	return string(buf)
}

// Reset resets the arena, keeping the first chunk for reuse.
func (a *Arena) Reset() {
	if len(a.chunks) > 0 {
		a.current = a.chunks[0]
		a.chunks = a.chunks[:1]
	}
	a.offset = 0
}

// TotalAllocated returns total bytes allocated across all chunks.
func (a *Arena) TotalAllocated() int {
	total := 0
	for _, c := range a.chunks {
		total += len(c)
	}
	return total
}

// ChunkCount returns the number of chunks allocated.
func (a *Arena) ChunkCount() int {
	return len(a.chunks)
}
