package sortx

import "testing"

// TestBottomN_AllBranches covers the three previously-uncovered paths in
// BottomN: n<=0 (returns nil), n>=len (returns full sorted slice), and
// n<len (returns truncated slice).
func TestBottomN_AllBranches(t *testing.T) {
	scores := []int{50, 90, 70, 30, 100, 60}
	id := func(x int) int { return x }

	// n <= 0 → nil.
	if got := BottomN(scores, 0, id); got != nil {
		t.Errorf("BottomN n=0 = %v, want nil", got)
	}
	if got := BottomN(scores, -3, id); got != nil {
		t.Errorf("BottomN n=-3 = %v, want nil", got)
	}

	// n >= len → all elements, descending.
	full := BottomN(scores, 100, id)
	wantFull := []int{100, 90, 70, 60, 50, 30}
	if !equalIntSlice(full, wantFull) {
		t.Errorf("BottomN n=100 = %v, want %v", full, wantFull)
	}

	// n < len → top-N descending (= bottom-N from worst).
	top3 := BottomN(scores, 3, id)
	want3 := []int{100, 90, 70}
	if !equalIntSlice(top3, want3) {
		t.Errorf("BottomN n=3 = %v, want %v", top3, want3)
	}
}

// TestBottomN_DoesNotMutateInput pins a critical contract: BottomN must
// not reorder the caller's slice. The function copies internally — this
// test catches any future "let me skip the copy for performance" PR.
func TestBottomN_DoesNotMutateInput(t *testing.T) {
	scores := []int{1, 2, 3, 4, 5}
	original := append([]int{}, scores...)
	_ = BottomN(scores, 3, func(x int) int { return x })
	if !equalIntSlice(scores, original) {
		t.Errorf("BottomN mutated input: %v vs %v", scores, original)
	}
}

func equalIntSlice(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
