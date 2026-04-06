package slotmap

import (
	"testing"
)

func TestInsertAndGet(t *testing.T) {
	m := New[string](4)
	h := m.Insert("hello")
	if h.IsZero() {
		t.Fatal("handle should not be zero")
	}
	v, ok := m.Get(h)
	if !ok {
		t.Fatal("Get should return true")
	}
	if v != "hello" {
		t.Errorf("got %q, want %q", v, "hello")
	}
}

func TestMultipleInserts(t *testing.T) {
	m := New[int](4)
	h1 := m.Insert(10)
	h2 := m.Insert(20)
	h3 := m.Insert(30)

	v1, ok1 := m.Get(h1)
	v2, ok2 := m.Get(h2)
	v3, ok3 := m.Get(h3)

	if !ok1 || v1 != 10 {
		t.Errorf("h1: got %d, ok=%v", v1, ok1)
	}
	if !ok2 || v2 != 20 {
		t.Errorf("h2: got %d, ok=%v", v2, ok2)
	}
	if !ok3 || v3 != 30 {
		t.Errorf("h3: got %d, ok=%v", v3, ok3)
	}
	if m.Len() != 3 {
		t.Errorf("Len: got %d, want 3", m.Len())
	}
}

func TestRemove(t *testing.T) {
	m := New[string](4)
	h := m.Insert("test")
	if !m.Remove(h) {
		t.Fatal("Remove should return true")
	}
	_, ok := m.Get(h)
	if ok {
		t.Error("Get after Remove should return false")
	}
	if m.Len() != 0 {
		t.Errorf("Len after Remove: got %d, want 0", m.Len())
	}
}

func TestRemoveInvalidHandle(t *testing.T) {
	m := New[int](4)
	h := Handle{index: 999, generation: 1}
	if m.Remove(h) {
		t.Error("Remove with invalid handle should return false")
	}
}

func TestGenerationProtection(t *testing.T) {
	m := New[string](4)
	h1 := m.Insert("first")
	m.Remove(h1)

	// Insert into the same slot (reuse)
	h2 := m.Insert("second")

	// Old handle should be invalid (generation mismatch)
	_, ok := m.Get(h1)
	if ok {
		t.Error("old handle should be invalid after slot reuse")
	}

	v, ok := m.Get(h2)
	if !ok || v != "second" {
		t.Errorf("new handle: got %q, ok=%v", v, ok)
	}
}

func TestSlotReuse(t *testing.T) {
	m := New[int](2)
	h1 := m.Insert(1)
	m.Remove(h1)
	h2 := m.Insert(2)

	// h2 should reuse h1's slot
	if h2.index != h1.index {
		t.Errorf("expected slot reuse: h1.index=%d, h2.index=%d", h1.index, h2.index)
	}
	if h2.generation <= h1.generation {
		t.Error("generation should increase on reuse")
	}
}

func TestUpdate(t *testing.T) {
	m := New[string](4)
	h := m.Insert("original")
	if !m.Update(h, "updated") {
		t.Fatal("Update should return true")
	}
	v, ok := m.Get(h)
	if !ok || v != "updated" {
		t.Errorf("got %q, ok=%v", v, ok)
	}
}

func TestUpdateInvalidHandle(t *testing.T) {
	m := New[int](4)
	h := m.Insert(1)
	m.Remove(h)
	if m.Update(h, 2) {
		t.Error("Update with stale handle should return false")
	}
}

func TestUpdateOutOfBounds(t *testing.T) {
	m := New[int](4)
	h := Handle{index: 100, generation: 1}
	if m.Update(h, 42) {
		t.Error("Update with out-of-bounds index should return false")
	}
}

func TestLen(t *testing.T) {
	m := New[int](4)
	if m.Len() != 0 {
		t.Errorf("empty Len: got %d", m.Len())
	}
	h1 := m.Insert(1)
	m.Insert(2)
	m.Insert(3)
	if m.Len() != 3 {
		t.Errorf("Len: got %d, want 3", m.Len())
	}
	m.Remove(h1)
	if m.Len() != 2 {
		t.Errorf("Len after remove: got %d, want 2", m.Len())
	}
}

func TestEach(t *testing.T) {
	m := New[int](4)
	m.Insert(10)
	h2 := m.Insert(20)
	m.Insert(30)
	m.Remove(h2)

	var sum int
	var count int
	m.Each(func(h Handle, v int) bool {
		sum += v
		count++
		return true
	})
	if count != 2 {
		t.Errorf("count: got %d, want 2", count)
	}
	if sum != 40 {
		t.Errorf("sum: got %d, want 40", sum)
	}
}

func TestEachEarlyStop(t *testing.T) {
	m := New[int](4)
	m.Insert(1)
	m.Insert(2)
	m.Insert(3)

	var count int
	m.Each(func(h Handle, v int) bool {
		count++
		return false // stop after first
	})
	if count != 1 {
		t.Errorf("count: got %d, want 1", count)
	}
}

func TestHandleIsZero(t *testing.T) {
	var h Handle
	if !h.IsZero() {
		t.Error("zero handle should be zero")
	}
	m := New[int](4)
	h = m.Insert(1)
	if h.IsZero() {
		t.Error("valid handle should not be zero")
	}
}

func TestDoubleRemove(t *testing.T) {
	m := New[int](4)
	h := m.Insert(42)
	if !m.Remove(h) {
		t.Fatal("first Remove should succeed")
	}
	if m.Remove(h) {
		t.Error("second Remove should fail")
	}
}

func TestLargeInsertRemoveCycle(t *testing.T) {
	m := New[int](16)
	handles := make([]Handle, 100)
	for i := 0; i < 100; i++ {
		handles[i] = m.Insert(i)
	}
	if m.Len() != 100 {
		t.Fatalf("Len: got %d, want 100", m.Len())
	}
	// Remove all even
	for i := 0; i < 100; i += 2 {
		m.Remove(handles[i])
	}
	if m.Len() != 50 {
		t.Errorf("Len after removes: got %d, want 50", m.Len())
	}
	// Reinsert
	for i := 0; i < 50; i++ {
		m.Insert(i + 1000)
	}
	if m.Len() != 100 {
		t.Errorf("Len after reinsert: got %d, want 100", m.Len())
	}
}
