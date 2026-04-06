package bitset

import (
	"testing"
)

func TestNew(t *testing.T) {
	b := New(100)
	if b.Size() != 100 {
		t.Errorf("Size: got %d, want 100", b.Size())
	}
	if b.Count() != 0 {
		t.Errorf("Count: got %d, want 0", b.Count())
	}
}

func TestNewZero(t *testing.T) {
	b := New(0)
	if b.Size() != 0 {
		t.Errorf("Size: got %d, want 0", b.Size())
	}
}

func TestNewNegative(t *testing.T) {
	b := New(-5)
	if b.Size() != 0 {
		t.Errorf("Size: got %d, want 0", b.Size())
	}
}

func TestSetAndTest(t *testing.T) {
	b := New(64)
	b.Set(0)
	b.Set(31)
	b.Set(63)

	if !b.Test(0) {
		t.Error("bit 0 should be set")
	}
	if !b.Test(31) {
		t.Error("bit 31 should be set")
	}
	if !b.Test(63) {
		t.Error("bit 63 should be set")
	}
	if b.Test(1) {
		t.Error("bit 1 should not be set")
	}
}

func TestSetOutOfBounds(t *testing.T) {
	b := New(10)
	b.Set(100) // should not panic
	b.Set(-1)  // should not panic
}

func TestTestOutOfBounds(t *testing.T) {
	b := New(10)
	if b.Test(100) {
		t.Error("out of bounds should return false")
	}
	if b.Test(-1) {
		t.Error("negative index should return false")
	}
}

func TestClear(t *testing.T) {
	b := New(64)
	b.Set(5)
	if !b.Test(5) {
		t.Fatal("bit 5 should be set")
	}
	b.Clear(5)
	if b.Test(5) {
		t.Error("bit 5 should be cleared")
	}
}

func TestToggle(t *testing.T) {
	b := New(64)
	b.Toggle(10)
	if !b.Test(10) {
		t.Error("bit 10 should be set after toggle")
	}
	b.Toggle(10)
	if b.Test(10) {
		t.Error("bit 10 should be cleared after second toggle")
	}
}

func TestCount(t *testing.T) {
	b := New(128)
	b.Set(0)
	b.Set(1)
	b.Set(64)
	b.Set(127)
	if b.Count() != 4 {
		t.Errorf("Count: got %d, want 4", b.Count())
	}
}

func TestClearAll(t *testing.T) {
	b := New(128)
	b.Set(0)
	b.Set(50)
	b.Set(127)
	b.ClearAll()
	if b.Count() != 0 {
		t.Errorf("Count after ClearAll: got %d, want 0", b.Count())
	}
}

func TestSetAll(t *testing.T) {
	b := New(128)
	b.SetAll()
	// All bits in words should be set, but Count counts all bits in words
	// which may exceed b.size. Count the valid bits manually.
	count := 0
	for i := 0; i < b.Size(); i++ {
		if b.Test(i) {
			count++
		}
	}
	if count != 128 {
		t.Errorf("set bits: got %d, want 128", count)
	}
}

func TestAnd(t *testing.T) {
	a := New(64)
	b := New(64)
	a.Set(0)
	a.Set(1)
	a.Set(2)
	b.Set(1)
	b.Set(2)
	b.Set(3)

	result := a.And(b)
	if !result.Test(1) || !result.Test(2) {
		t.Error("AND should have bits 1,2")
	}
	if result.Test(0) || result.Test(3) {
		t.Error("AND should not have bits 0,3")
	}
}

func TestOr(t *testing.T) {
	a := New(64)
	b := New(64)
	a.Set(0)
	a.Set(1)
	b.Set(2)
	b.Set(3)

	result := a.Or(b)
	for _, i := range []int{0, 1, 2, 3} {
		if !result.Test(i) {
			t.Errorf("OR should have bit %d", i)
		}
	}
}

func TestIsEmpty(t *testing.T) {
	b := New(64)
	if !b.IsEmpty() {
		t.Error("new bitset should be empty")
	}
	b.Set(10)
	if b.IsEmpty() {
		t.Error("bitset with set bit should not be empty")
	}
	b.Clear(10)
	if !b.IsEmpty() {
		t.Error("bitset after clearing all should be empty")
	}
}

func TestEqual(t *testing.T) {
	a := New(64)
	b := New(64)
	a.Set(5)
	b.Set(5)
	if !a.Equal(b) {
		t.Error("identical bitsets should be equal")
	}

	b.Set(10)
	if a.Equal(b) {
		t.Error("different bitsets should not be equal")
	}
}

func TestEqualDifferentSize(t *testing.T) {
	a := New(64)
	b := New(128)
	if a.Equal(b) {
		t.Error("different size bitsets should not be equal")
	}
}

func TestMultiWord(t *testing.T) {
	b := New(256)
	b.Set(0)
	b.Set(63)
	b.Set(64)
	b.Set(127)
	b.Set(128)
	b.Set(255)

	if b.Count() != 6 {
		t.Errorf("Count: got %d, want 6", b.Count())
	}
	for _, i := range []int{0, 63, 64, 127, 128, 255} {
		if !b.Test(i) {
			t.Errorf("bit %d should be set", i)
		}
	}
}

func TestPopcount(t *testing.T) {
	tests := []struct {
		val  uint64
		want int
	}{
		{0, 0},
		{1, 1},
		{0xFF, 8},
		{^uint64(0), 64},
		{0xAAAAAAAAAAAAAAAA, 32},
	}
	for _, tt := range tests {
		if got := popcount(tt.val); got != tt.want {
			t.Errorf("popcount(0x%x) = %d, want %d", tt.val, got, tt.want)
		}
	}
}
