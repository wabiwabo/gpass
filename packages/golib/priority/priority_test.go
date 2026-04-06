package priority

import "testing"

func TestNewMin(t *testing.T) {
	q := NewMin[string]()
	if !q.IsEmpty() {
		t.Error("should be empty")
	}
}

func TestMin_PushPop(t *testing.T) {
	q := NewMin[string]()
	q.Push("low", 1)
	q.Push("high", 10)
	q.Push("mid", 5)

	v, p, ok := q.Pop()
	if !ok || v != "low" || p != 1 {
		t.Errorf("Pop = (%q, %d, %v), want (low, 1, true)", v, p, ok)
	}
}

func TestMax_PushPop(t *testing.T) {
	q := NewMax[string]()
	q.Push("low", 1)
	q.Push("high", 10)
	q.Push("mid", 5)

	v, p, ok := q.Pop()
	if !ok || v != "high" || p != 10 {
		t.Errorf("Pop = (%q, %d, %v), want (high, 10, true)", v, p, ok)
	}
}

func TestPop_Empty(t *testing.T) {
	q := NewMin[int]()
	_, _, ok := q.Pop()
	if ok {
		t.Error("should return false for empty")
	}
}

func TestPeek(t *testing.T) {
	q := NewMin[string]()
	q.Push("a", 5)
	q.Push("b", 1)

	v, p, ok := q.Peek()
	if !ok || v != "b" || p != 1 {
		t.Errorf("Peek = (%q, %d)", v, p)
	}
	if q.Len() != 2 {
		t.Error("Peek should not remove")
	}
}

func TestPeek_Empty(t *testing.T) {
	q := NewMin[int]()
	_, _, ok := q.Peek()
	if ok {
		t.Error("should return false")
	}
}

func TestLen(t *testing.T) {
	q := NewMin[int]()
	q.Push(1, 10)
	q.Push(2, 20)
	if q.Len() != 2 {
		t.Errorf("Len = %d", q.Len())
	}
}

func TestMin_Order(t *testing.T) {
	q := NewMin[int]()
	q.Push(100, 5)
	q.Push(200, 1)
	q.Push(300, 3)

	expected := []int{200, 300, 100}
	for i, want := range expected {
		v, _, ok := q.Pop()
		if !ok || v != want {
			t.Errorf("[%d] Pop = %d, want %d", i, v, want)
		}
	}
}

func TestMax_Order(t *testing.T) {
	q := NewMax[int]()
	q.Push(100, 5)
	q.Push(200, 1)
	q.Push(300, 3)

	expected := []int{100, 300, 200}
	for i, want := range expected {
		v, _, ok := q.Pop()
		if !ok || v != want {
			t.Errorf("[%d] Pop = %d, want %d", i, v, want)
		}
	}
}

func TestIsEmpty(t *testing.T) {
	q := NewMin[int]()
	if !q.IsEmpty() {
		t.Error("should be empty")
	}
	q.Push(1, 1)
	if q.IsEmpty() {
		t.Error("should not be empty")
	}
}
