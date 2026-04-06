package ringbuf

import "testing"

func TestNew(t *testing.T) {
	b := New[int](5)
	if b.Cap() != 5 {
		t.Errorf("Cap = %d", b.Cap())
	}
	if b.Len() != 0 {
		t.Errorf("Len = %d", b.Len())
	}
	if !b.IsEmpty() {
		t.Error("should be empty")
	}
}

func TestNew_ZeroCapacity(t *testing.T) {
	b := New[int](0)
	if b.Cap() != 16 {
		t.Errorf("Cap = %d, want 16 (default)", b.Cap())
	}
}

func TestPush_Pop(t *testing.T) {
	b := New[string](3)
	b.Push("a")
	b.Push("b")
	b.Push("c")

	v, ok := b.Pop()
	if !ok || v != "a" {
		t.Errorf("Pop = (%q, %v)", v, ok)
	}
	v, ok = b.Pop()
	if !ok || v != "b" {
		t.Errorf("Pop = (%q, %v)", v, ok)
	}
}

func TestPop_Empty(t *testing.T) {
	b := New[int](3)
	_, ok := b.Pop()
	if ok {
		t.Error("should return false for empty")
	}
}

func TestPush_Overflow(t *testing.T) {
	b := New[int](3)
	b.Push(1)
	b.Push(2)
	b.Push(3)
	b.Push(4) // overwrites 1
	b.Push(5) // overwrites 2

	if b.Len() != 3 {
		t.Errorf("Len = %d", b.Len())
	}

	v, _ := b.Pop()
	if v != 3 {
		t.Errorf("oldest = %d, want 3", v)
	}
}

func TestPeek(t *testing.T) {
	b := New[int](3)
	b.Push(10)
	b.Push(20)

	v, ok := b.Peek()
	if !ok || v != 10 {
		t.Errorf("Peek = (%d, %v)", v, ok)
	}
	// Should not remove
	if b.Len() != 2 {
		t.Error("Peek should not remove")
	}
}

func TestPeek_Empty(t *testing.T) {
	b := New[int](3)
	_, ok := b.Peek()
	if ok {
		t.Error("should return false")
	}
}

func TestPeekLast(t *testing.T) {
	b := New[int](3)
	b.Push(10)
	b.Push(20)
	b.Push(30)

	v, ok := b.PeekLast()
	if !ok || v != 30 {
		t.Errorf("PeekLast = (%d, %v)", v, ok)
	}
}

func TestIsFull(t *testing.T) {
	b := New[int](2)
	if b.IsFull() {
		t.Error("should not be full")
	}
	b.Push(1)
	b.Push(2)
	if !b.IsFull() {
		t.Error("should be full")
	}
}

func TestClear(t *testing.T) {
	b := New[int](3)
	b.Push(1)
	b.Push(2)
	b.Clear()

	if !b.IsEmpty() {
		t.Error("should be empty after clear")
	}
}

func TestToSlice(t *testing.T) {
	b := New[int](5)
	b.Push(1)
	b.Push(2)
	b.Push(3)

	s := b.ToSlice()
	if len(s) != 3 || s[0] != 1 || s[1] != 2 || s[2] != 3 {
		t.Errorf("ToSlice = %v", s)
	}
}

func TestToSlice_Wrapped(t *testing.T) {
	b := New[int](3)
	b.Push(1)
	b.Push(2)
	b.Push(3)
	b.Push(4) // wraps: [4, 2, 3] with head at 2

	s := b.ToSlice()
	if len(s) != 3 || s[0] != 2 || s[1] != 3 || s[2] != 4 {
		t.Errorf("ToSlice wrapped = %v", s)
	}
}

func TestEach(t *testing.T) {
	b := New[int](3)
	b.Push(10)
	b.Push(20)
	b.Push(30)

	sum := 0
	b.Each(func(v int) { sum += v })
	if sum != 60 {
		t.Errorf("sum = %d", sum)
	}
}

func TestPush_Pop_Interleave(t *testing.T) {
	b := New[int](3)
	b.Push(1)
	b.Push(2)
	b.Pop()    // remove 1
	b.Push(3)
	b.Push(4)

	s := b.ToSlice()
	if len(s) != 3 || s[0] != 2 || s[1] != 3 || s[2] != 4 {
		t.Errorf("interleave = %v", s)
	}
}

func TestGenericType_String(t *testing.T) {
	b := New[string](2)
	b.Push("hello")
	b.Push("world")

	v, _ := b.Pop()
	if v != "hello" {
		t.Errorf("v = %q", v)
	}
}
