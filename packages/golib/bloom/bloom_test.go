package bloom

import (
	"fmt"
	"testing"
)

func TestFilter_AddContains(t *testing.T) {
	f := New(1000, 0.01)

	f.AddString("hello")
	f.AddString("world")

	if !f.ContainsString("hello") {
		t.Error("should contain 'hello'")
	}
	if !f.ContainsString("world") {
		t.Error("should contain 'world'")
	}
}

func TestFilter_NotContains(t *testing.T) {
	f := New(1000, 0.01)
	f.AddString("hello")

	// Not 100% guaranteed but very unlikely with 1% FP rate.
	falsePositives := 0
	for i := 0; i < 1000; i++ {
		if f.ContainsString(fmt.Sprintf("not-added-%d", i)) {
			falsePositives++
		}
	}
	// With 1 item and 1000 tests, should have very few false positives.
	if falsePositives > 50 {
		t.Errorf("too many false positives: %d", falsePositives)
	}
}

func TestFilter_Count(t *testing.T) {
	f := New(1000, 0.01)
	f.AddString("a")
	f.AddString("b")
	f.AddString("c")

	if f.Count() != 3 {
		t.Errorf("count: got %d", f.Count())
	}
}

func TestFilter_FalsePositiveRate(t *testing.T) {
	f := New(100, 0.01)

	// Add 100 items.
	for i := 0; i < 100; i++ {
		f.AddString(fmt.Sprintf("item-%d", i))
	}

	rate := f.EstimatedFPRate()
	if rate <= 0 {
		t.Error("FP rate should be positive after adding items")
	}
	// Should be near the configured 1%.
	if rate > 0.1 {
		t.Errorf("FP rate too high: %f", rate)
	}
}

func TestFilter_FillRatio(t *testing.T) {
	f := New(1000, 0.01)

	if f.FillRatio() != 0 {
		t.Error("empty filter should have 0 fill ratio")
	}

	for i := 0; i < 100; i++ {
		f.AddString(fmt.Sprintf("item-%d", i))
	}

	ratio := f.FillRatio()
	if ratio <= 0 || ratio >= 1 {
		t.Errorf("fill ratio: got %f", ratio)
	}
}

func TestFilter_Clear(t *testing.T) {
	f := New(1000, 0.01)
	f.AddString("item")
	f.Clear()

	if f.Count() != 0 {
		t.Error("count should be 0 after clear")
	}
	if f.ContainsString("item") {
		t.Error("should not contain after clear")
	}
}

func TestFilter_Defaults(t *testing.T) {
	f := New(0, 0) // Should use defaults.
	f.AddString("test")
	if !f.ContainsString("test") {
		t.Error("defaults should work")
	}
}

func TestFilter_InvalidFPRate(t *testing.T) {
	f := New(100, 2.0) // Invalid, should default.
	f.AddString("test")
	if !f.ContainsString("test") {
		t.Error("should work with corrected FP rate")
	}
}

func TestFilter_LargeScale(t *testing.T) {
	f := New(10000, 0.001) // 0.1% FP rate.

	for i := 0; i < 10000; i++ {
		f.AddString(fmt.Sprintf("key-%d", i))
	}

	// Verify all added items are found.
	for i := 0; i < 10000; i++ {
		if !f.ContainsString(fmt.Sprintf("key-%d", i)) {
			t.Fatalf("false negative at %d", i)
		}
	}

	// Test false positive rate.
	fp := 0
	tests := 10000
	for i := 0; i < tests; i++ {
		if f.ContainsString(fmt.Sprintf("other-%d", i)) {
			fp++
		}
	}
	fpRate := float64(fp) / float64(tests)
	// Should be close to 0.1%, allow some margin.
	if fpRate > 0.01 {
		t.Errorf("FP rate too high: %.4f (expected ~0.001)", fpRate)
	}
}

func TestFilter_BytesAndStrings(t *testing.T) {
	f := New(100, 0.01)

	f.Add([]byte("bytes"))
	if !f.Contains([]byte("bytes")) {
		t.Error("should find bytes")
	}
	if !f.ContainsString("bytes") {
		t.Error("string and bytes should be equivalent")
	}
}

func TestPopcount(t *testing.T) {
	tests := []struct {
		x    uint64
		want int
	}{
		{0, 0},
		{1, 1},
		{0xFF, 8},
		{0xFFFFFFFFFFFFFFFF, 64},
		{0xAAAAAAAAAAAAAAAA, 32},
	}
	for _, tt := range tests {
		if got := popcount(tt.x); got != tt.want {
			t.Errorf("popcount(%x): got %d, want %d", tt.x, got, tt.want)
		}
	}
}
