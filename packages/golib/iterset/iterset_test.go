package iterset

import (
	"sort"
	"testing"
)

func TestSet_New(t *testing.T) {
	s := New("a", "b", "c")
	if s.Len() != 3 {
		t.Errorf("len: got %d", s.Len())
	}
}

func TestSet_AddContains(t *testing.T) {
	s := New[string]()
	s.Add("x")
	if !s.Contains("x") {
		t.Error("should contain x")
	}
	if s.Contains("y") {
		t.Error("should not contain y")
	}
}

func TestSet_Remove(t *testing.T) {
	s := New("a", "b", "c")
	s.Remove("b")
	if s.Contains("b") {
		t.Error("b should be removed")
	}
	if s.Len() != 2 {
		t.Errorf("len: got %d", s.Len())
	}
}

func TestSet_Items(t *testing.T) {
	s := New("a", "b", "c")
	items := s.Items()
	sort.Strings(items)
	if len(items) != 3 || items[0] != "a" || items[2] != "c" {
		t.Errorf("items: got %v", items)
	}
}

func TestSet_Union(t *testing.T) {
	a := New(1, 2, 3)
	b := New(3, 4, 5)

	u := a.Union(b)
	if u.Len() != 5 {
		t.Errorf("union: got %d", u.Len())
	}
	for _, v := range []int{1, 2, 3, 4, 5} {
		if !u.Contains(v) {
			t.Errorf("union should contain %d", v)
		}
	}
}

func TestSet_Intersection(t *testing.T) {
	a := New(1, 2, 3, 4)
	b := New(3, 4, 5, 6)

	i := a.Intersection(b)
	if i.Len() != 2 {
		t.Errorf("intersection: got %d", i.Len())
	}
	if !i.Contains(3) || !i.Contains(4) {
		t.Error("should contain 3 and 4")
	}
}

func TestSet_Intersection_Empty(t *testing.T) {
	a := New(1, 2)
	b := New(3, 4)

	i := a.Intersection(b)
	if i.Len() != 0 {
		t.Error("disjoint sets should have empty intersection")
	}
}

func TestSet_Difference(t *testing.T) {
	a := New(1, 2, 3)
	b := New(2, 3, 4)

	d := a.Difference(b)
	if d.Len() != 1 || !d.Contains(1) {
		t.Errorf("difference: got %v", d.Items())
	}
}

func TestSet_IsSubset(t *testing.T) {
	a := New(1, 2)
	b := New(1, 2, 3)

	if !a.IsSubset(b) {
		t.Error("{1,2} should be subset of {1,2,3}")
	}
	if b.IsSubset(a) {
		t.Error("{1,2,3} should not be subset of {1,2}")
	}
}

func TestSet_IsSuperset(t *testing.T) {
	a := New(1, 2, 3)
	b := New(1, 2)

	if !a.IsSuperset(b) {
		t.Error("{1,2,3} should be superset of {1,2}")
	}
}

func TestSet_Equal(t *testing.T) {
	a := New(1, 2, 3)
	b := New(3, 2, 1)

	if !a.Equal(b) {
		t.Error("should be equal regardless of order")
	}

	c := New(1, 2)
	if a.Equal(c) {
		t.Error("different sizes should not be equal")
	}
}

func TestSet_Clear(t *testing.T) {
	s := New(1, 2, 3)
	s.Clear()
	if s.Len() != 0 {
		t.Error("should be empty after clear")
	}
}

func TestSet_IsEmpty(t *testing.T) {
	s := New[int]()
	if !s.IsEmpty() {
		t.Error("new set should be empty")
	}
	s.Add(1)
	if s.IsEmpty() {
		t.Error("should not be empty")
	}
}

func TestSet_Duplicates(t *testing.T) {
	s := New(1, 1, 2, 2, 3)
	if s.Len() != 3 {
		t.Errorf("duplicates: got %d", s.Len())
	}
}

func TestSet_StringType(t *testing.T) {
	s := New("admin", "user", "viewer")
	if !s.Contains("admin") {
		t.Error("string set should work")
	}
}
