package slicex

import (
	"strconv"
	"testing"
)

func TestContains(t *testing.T) {
	if !Contains([]string{"a", "b", "c"}, "b") {
		t.Error("should contain b")
	}
	if Contains([]string{"a", "b"}, "z") {
		t.Error("should not contain z")
	}
	if Contains([]string{}, "a") {
		t.Error("empty slice should not contain")
	}
}

func TestFilter(t *testing.T) {
	evens := Filter([]int{1, 2, 3, 4, 5, 6}, func(n int) bool { return n%2 == 0 })
	if len(evens) != 3 {
		t.Errorf("len = %d", len(evens))
	}
	for _, n := range evens {
		if n%2 != 0 {
			t.Errorf("non-even: %d", n)
		}
	}
}

func TestFilter_Empty(t *testing.T) {
	result := Filter([]int{1, 2, 3}, func(n int) bool { return n > 10 })
	if len(result) != 0 {
		t.Errorf("len = %d", len(result))
	}
}

func TestMap(t *testing.T) {
	strs := Map([]int{1, 2, 3}, strconv.Itoa)
	if len(strs) != 3 || strs[0] != "1" || strs[2] != "3" {
		t.Errorf("Map = %v", strs)
	}
}

func TestMap_Empty(t *testing.T) {
	result := Map([]int{}, strconv.Itoa)
	if len(result) != 0 {
		t.Errorf("len = %d", len(result))
	}
}

func TestUnique(t *testing.T) {
	result := Unique([]string{"a", "b", "a", "c", "b"})
	if len(result) != 3 {
		t.Errorf("len = %d", len(result))
	}
	if result[0] != "a" || result[1] != "b" || result[2] != "c" {
		t.Errorf("Unique = %v (should preserve order)", result)
	}
}

func TestChunk(t *testing.T) {
	chunks := Chunk([]int{1, 2, 3, 4, 5}, 2)
	if len(chunks) != 3 {
		t.Fatalf("chunks = %d", len(chunks))
	}
	if len(chunks[0]) != 2 || len(chunks[1]) != 2 || len(chunks[2]) != 1 {
		t.Errorf("chunks = %v", chunks)
	}
}

func TestChunk_ExactDivision(t *testing.T) {
	chunks := Chunk([]int{1, 2, 3, 4}, 2)
	if len(chunks) != 2 {
		t.Errorf("chunks = %d", len(chunks))
	}
}

func TestChunk_ZeroSize(t *testing.T) {
	if Chunk([]int{1, 2}, 0) != nil {
		t.Error("zero size should return nil")
	}
}

func TestChunk_Empty(t *testing.T) {
	chunks := Chunk([]int{}, 5)
	if len(chunks) != 0 {
		t.Errorf("chunks = %d", len(chunks))
	}
}

func TestFlatten(t *testing.T) {
	result := Flatten([][]int{{1, 2}, {3}, {4, 5, 6}})
	if len(result) != 6 {
		t.Errorf("len = %d", len(result))
	}
}

func TestFirst(t *testing.T) {
	v, ok := First([]int{1, 2, 3, 4}, func(n int) bool { return n > 2 })
	if !ok || v != 3 {
		t.Errorf("First = %d, %v", v, ok)
	}
}

func TestFirst_NotFound(t *testing.T) {
	_, ok := First([]int{1, 2}, func(n int) bool { return n > 10 })
	if ok {
		t.Error("should not find")
	}
}

func TestLast(t *testing.T) {
	v, ok := Last([]int{1, 2, 3, 4}, func(n int) bool { return n < 4 })
	if !ok || v != 3 {
		t.Errorf("Last = %d, %v", v, ok)
	}
}

func TestReduce(t *testing.T) {
	sum := Reduce([]int{1, 2, 3, 4}, 0, func(acc, n int) int { return acc + n })
	if sum != 10 {
		t.Errorf("sum = %d", sum)
	}
}

func TestPartition(t *testing.T) {
	evens, odds := Partition([]int{1, 2, 3, 4, 5}, func(n int) bool { return n%2 == 0 })
	if len(evens) != 2 {
		t.Errorf("evens = %d", len(evens))
	}
	if len(odds) != 3 {
		t.Errorf("odds = %d", len(odds))
	}
}

func TestReverse(t *testing.T) {
	result := Reverse([]int{1, 2, 3})
	if result[0] != 3 || result[1] != 2 || result[2] != 1 {
		t.Errorf("Reverse = %v", result)
	}
}

func TestReverse_Empty(t *testing.T) {
	result := Reverse([]int{})
	if len(result) != 0 {
		t.Error("should be empty")
	}
}

func TestReverse_DoesNotMutate(t *testing.T) {
	original := []int{1, 2, 3}
	Reverse(original)
	if original[0] != 1 {
		t.Error("original mutated")
	}
}
