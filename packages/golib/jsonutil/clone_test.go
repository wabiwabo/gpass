package jsonutil

import (
	"math"
	"testing"
)

// TestClone_HappyPath_DeepCopy pins that the round-trip produces an
// independent copy — mutating the clone must not touch the original.
func TestClone_HappyPath_DeepCopy(t *testing.T) {
	type Inner struct{ X int }
	type Doc struct {
		Name  string
		Items []int
		Inner *Inner
	}
	src := Doc{Name: "hi", Items: []int{1, 2, 3}, Inner: &Inner{X: 7}}
	dst, err := Clone(src)
	if err != nil {
		t.Fatal(err)
	}
	dst.Items[0] = 999
	dst.Inner.X = 999
	if src.Items[0] != 1 {
		t.Errorf("clone shared slice: src=%v", src.Items)
	}
	if src.Inner.X != 7 {
		t.Errorf("clone shared pointer: src=%v", src.Inner.X)
	}
}

// TestClone_MarshalError pins the json.Marshal failure branch — passing
// a value that json cannot serialize (NaN/+Inf for float, channels, etc.)
// must surface the error and return zero.
func TestClone_MarshalError(t *testing.T) {
	// json.Marshal returns *json.UnsupportedValueError for NaN/Inf.
	_, err := Clone(math.NaN())
	if err == nil {
		t.Error("Clone(NaN) should error")
	}
	_, err = Clone(math.Inf(1))
	if err == nil {
		t.Error("Clone(+Inf) should error")
	}
}
