package setx

import (
	"testing"
)

func TestNew(t *testing.T) {
	s := New("a", "b", "c")
	if s.Len() != 3 {
		t.Errorf("Len = %d", s.Len())
	}
}

func TestNew_Dedup(t *testing.T) {
	s := New("a", "b", "a")
	if s.Len() != 2 {
		t.Errorf("Len = %d, want 2", s.Len())
	}
}

func TestAdd(t *testing.T) {
	s := New[string]()
	s.Add("a")
	s.Add("b")
	if s.Len() != 2 {
		t.Errorf("Len = %d", s.Len())
	}
}

func TestRemove(t *testing.T) {
	s := New("a", "b", "c")
	s.Remove("b")
	if s.Len() != 2 {
		t.Errorf("Len = %d", s.Len())
	}
	if s.Contains("b") {
		t.Error("should not contain b")
	}
}

func TestContains(t *testing.T) {
	s := New(1, 2, 3)
	if !s.Contains(2) {
		t.Error("should contain 2")
	}
	if s.Contains(4) {
		t.Error("should not contain 4")
	}
}

func TestIsEmpty(t *testing.T) {
	if !New[string]().IsEmpty() {
		t.Error("should be empty")
	}
	if New("a").IsEmpty() {
		t.Error("should not be empty")
	}
}

func TestUnion(t *testing.T) {
	a := New("x", "y")
	b := New("y", "z")
	u := a.Union(b)
	if u.Len() != 3 {
		t.Errorf("Len = %d", u.Len())
	}
	for _, v := range []string{"x", "y", "z"} {
		if !u.Contains(v) {
			t.Errorf("should contain %q", v)
		}
	}
}

func TestIntersect(t *testing.T) {
	a := New("x", "y", "z")
	b := New("y", "z", "w")
	i := a.Intersect(b)
	if i.Len() != 2 {
		t.Errorf("Len = %d", i.Len())
	}
	if !i.Contains("y") || !i.Contains("z") {
		t.Error("should contain y and z")
	}
}

func TestDifference(t *testing.T) {
	a := New("x", "y", "z")
	b := New("y")
	d := a.Difference(b)
	if d.Len() != 2 {
		t.Errorf("Len = %d", d.Len())
	}
	if !d.Contains("x") || !d.Contains("z") {
		t.Error("should contain x and z")
	}
	if d.Contains("y") {
		t.Error("should not contain y")
	}
}

func TestIsSubset(t *testing.T) {
	a := New("a", "b")
	b := New("a", "b", "c")
	if !a.IsSubset(b) {
		t.Error("a should be subset of b")
	}
	if b.IsSubset(a) {
		t.Error("b should not be subset of a")
	}
}

func TestIsSuperset(t *testing.T) {
	a := New("a", "b", "c")
	b := New("a", "b")
	if !a.IsSuperset(b) {
		t.Error("a should be superset of b")
	}
}

func TestEqual(t *testing.T) {
	a := New("a", "b")
	b := New("b", "a")
	c := New("a", "b", "c")
	if !a.Equal(b) {
		t.Error("a and b should be equal")
	}
	if a.Equal(c) {
		t.Error("a and c should not be equal")
	}
}

func TestSortedStrings(t *testing.T) {
	s := New("c", "a", "b")
	sorted := SortedStrings(s)
	if sorted[0] != "a" || sorted[1] != "b" || sorted[2] != "c" {
		t.Errorf("sorted = %v", sorted)
	}
}

func TestElements(t *testing.T) {
	s := New(1, 2, 3)
	elems := s.Elements()
	if len(elems) != 3 {
		t.Errorf("len = %d", len(elems))
	}
}

func TestEmpty_Operations(t *testing.T) {
	a := New[int]()
	b := New(1, 2)

	u := a.Union(b)
	if u.Len() != 2 {
		t.Error("union with empty")
	}

	i := a.Intersect(b)
	if !i.IsEmpty() {
		t.Error("intersect with empty should be empty")
	}

	d := a.Difference(b)
	if !d.IsEmpty() {
		t.Error("difference of empty should be empty")
	}
}

func TestSet_DoesNotMutateOriginal(t *testing.T) {
	a := New("x", "y")
	b := New("y", "z")
	a.Union(b)
	if a.Len() != 2 {
		t.Error("original should not be mutated")
	}
}
