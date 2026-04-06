package mapx

import (
	"strconv"
	"testing"
)

func TestKeys(t *testing.T) {
	m := map[string]int{"c": 3, "a": 1, "b": 2}
	keys := Keys(m)
	if len(keys) != 3 {
		t.Fatalf("len = %d", len(keys))
	}
	if keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Errorf("Keys = %v (should be sorted)", keys)
	}
}

func TestKeys_Empty(t *testing.T) {
	keys := Keys(map[string]int{})
	if len(keys) != 0 {
		t.Error("should be empty")
	}
}

func TestValues(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2}
	vals := Values(m)
	if len(vals) != 2 {
		t.Errorf("len = %d", len(vals))
	}
}

func TestMerge(t *testing.T) {
	a := map[string]int{"x": 1, "y": 2}
	b := map[string]int{"y": 3, "z": 4}
	merged := Merge(a, b)

	if merged["x"] != 1 {
		t.Errorf("x = %d", merged["x"])
	}
	if merged["y"] != 3 {
		t.Errorf("y = %d (should be overridden)", merged["y"])
	}
	if merged["z"] != 4 {
		t.Errorf("z = %d", merged["z"])
	}
}

func TestMerge_Empty(t *testing.T) {
	merged := Merge[string, int]()
	if len(merged) != 0 {
		t.Error("should be empty")
	}
}

func TestFilter(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	filtered := Filter(m, func(k string, v int) bool { return v > 1 })
	if len(filtered) != 2 {
		t.Errorf("len = %d", len(filtered))
	}
	if _, ok := filtered["a"]; ok {
		t.Error("a should be filtered out")
	}
}

func TestMapValues(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2}
	strs := MapValues(m, strconv.Itoa)
	if strs["a"] != "1" || strs["b"] != "2" {
		t.Errorf("MapValues = %v", strs)
	}
}

func TestPick(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	picked := Pick(m, "a", "c")
	if len(picked) != 2 {
		t.Errorf("len = %d", len(picked))
	}
	if _, ok := picked["b"]; ok {
		t.Error("b should not be picked")
	}
}

func TestPick_MissingKey(t *testing.T) {
	m := map[string]int{"a": 1}
	picked := Pick(m, "a", "z")
	if len(picked) != 1 {
		t.Errorf("len = %d", len(picked))
	}
}

func TestOmit(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	omitted := Omit(m, "b")
	if len(omitted) != 2 {
		t.Errorf("len = %d", len(omitted))
	}
	if _, ok := omitted["b"]; ok {
		t.Error("b should be omitted")
	}
}

func TestContains(t *testing.T) {
	m := map[string]int{"a": 1}
	if !Contains(m, "a") {
		t.Error("should contain a")
	}
	if Contains(m, "b") {
		t.Error("should not contain b")
	}
}

func TestGetOr(t *testing.T) {
	m := map[string]int{"a": 1}
	if GetOr(m, "a", 99) != 1 {
		t.Error("should return existing value")
	}
	if GetOr(m, "z", 99) != 99 {
		t.Error("should return default")
	}
}

func TestInvert(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2}
	inv := Invert(m)
	if inv[1] != "a" || inv[2] != "b" {
		t.Errorf("Invert = %v", inv)
	}
}
