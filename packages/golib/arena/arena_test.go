package arena

import (
	"testing"
)

func TestNew(t *testing.T) {
	a := New(4096)
	if a.ChunkCount() != 1 {
		t.Errorf("ChunkCount: got %d, want 1", a.ChunkCount())
	}
	if a.TotalAllocated() != 4096 {
		t.Errorf("TotalAllocated: got %d, want 4096", a.TotalAllocated())
	}
}

func TestNewMinChunkSize(t *testing.T) {
	a := New(10) // should be bumped to 1024
	if a.TotalAllocated() < 1024 {
		t.Errorf("minimum chunk size: got %d", a.TotalAllocated())
	}
}

func TestAlloc(t *testing.T) {
	a := New(1024)
	buf := a.Alloc(100)
	if len(buf) != 100 {
		t.Errorf("Alloc length: got %d, want 100", len(buf))
	}
	// Should be zeroed
	for i, b := range buf {
		if b != 0 {
			t.Errorf("byte %d: got 0x%02x, want 0x00", i, b)
			break
		}
	}
}

func TestAllocZero(t *testing.T) {
	a := New(1024)
	if buf := a.Alloc(0); buf != nil {
		t.Error("Alloc(0) should return nil")
	}
	if buf := a.Alloc(-1); buf != nil {
		t.Error("Alloc(-1) should return nil")
	}
}

func TestAllocMultiple(t *testing.T) {
	a := New(1024)
	b1 := a.Alloc(100)
	b2 := a.Alloc(200)
	b3 := a.Alloc(300)

	if len(b1) != 100 || len(b2) != 200 || len(b3) != 300 {
		t.Error("allocation sizes incorrect")
	}
	// Verify they don't overlap by writing different values
	b1[0] = 1
	b2[0] = 2
	b3[0] = 3
	if b1[0] != 1 || b2[0] != 2 || b3[0] != 3 {
		t.Error("allocations overlap")
	}
}

func TestAllocOverflow(t *testing.T) {
	a := New(1024)
	// Allocate more than chunk size to trigger new chunk
	a.Alloc(900)
	a.Alloc(200) // should trigger new chunk
	if a.ChunkCount() < 2 {
		t.Errorf("should have allocated new chunk, got %d", a.ChunkCount())
	}
}

func TestAllocLargerThanChunk(t *testing.T) {
	a := New(1024)
	buf := a.Alloc(2000) // larger than chunk size
	if len(buf) != 2000 {
		t.Errorf("large alloc: got %d, want 2000", len(buf))
	}
}

func TestAllocString(t *testing.T) {
	a := New(1024)
	s := a.AllocString("hello world")
	if s != "hello world" {
		t.Errorf("got %q, want %q", s, "hello world")
	}
}

func TestAllocStringEmpty(t *testing.T) {
	a := New(1024)
	s := a.AllocString("")
	if s != "" {
		t.Errorf("got %q, want empty", s)
	}
}

func TestAllocStringIndependent(t *testing.T) {
	a := New(1024)
	s1 := a.AllocString("first")
	s2 := a.AllocString("second")
	if s1 != "first" || s2 != "second" {
		t.Errorf("got %q, %q", s1, s2)
	}
}

func TestReset(t *testing.T) {
	a := New(1024)
	a.Alloc(500)
	a.Alloc(600) // triggers second chunk
	if a.ChunkCount() < 2 {
		t.Fatal("should have multiple chunks")
	}

	a.Reset()
	if a.ChunkCount() != 1 {
		t.Errorf("after Reset ChunkCount: got %d, want 1", a.ChunkCount())
	}

	// Should be able to allocate again
	buf := a.Alloc(100)
	if len(buf) != 100 {
		t.Errorf("alloc after reset: got %d", len(buf))
	}
}

func TestTotalAllocated(t *testing.T) {
	a := New(1024)
	initial := a.TotalAllocated()
	a.Alloc(2000) // triggers new chunk
	if a.TotalAllocated() <= initial {
		t.Error("TotalAllocated should increase after overflow")
	}
}

func TestManySmallAllocs(t *testing.T) {
	a := New(4096)
	for i := 0; i < 1000; i++ {
		buf := a.Alloc(4)
		buf[0] = byte(i)
	}
	// Should not panic and should allocate chunks as needed
	if a.TotalAllocated() < 4000 {
		t.Errorf("TotalAllocated too small: %d", a.TotalAllocated())
	}
}

func BenchmarkAlloc(b *testing.B) {
	a := New(65536)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%1000 == 0 {
			a.Reset()
		}
		a.Alloc(64)
	}
}
