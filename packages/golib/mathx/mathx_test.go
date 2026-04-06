package mathx

import (
	"math"
	"testing"
)

func TestMin(t *testing.T) {
	if Min(3, 5) != 3 {
		t.Error("Min(3,5)")
	}
	if Min(5, 3) != 3 {
		t.Error("Min(5,3)")
	}
	if Min(3, 3) != 3 {
		t.Error("Min(3,3)")
	}
	if Min(-1, 1) != -1 {
		t.Error("Min(-1,1)")
	}
	if Min(1.5, 2.5) != 1.5 {
		t.Error("Min float")
	}
	if Min("abc", "xyz") != "abc" {
		t.Error("Min string")
	}
}

func TestMax(t *testing.T) {
	if Max(3, 5) != 5 {
		t.Error("Max(3,5)")
	}
	if Max(5, 3) != 5 {
		t.Error("Max(5,3)")
	}
	if Max(-1, 1) != 1 {
		t.Error("Max(-1,1)")
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		name     string
		v, lo, hi int
		want     int
	}{
		{"in_range", 5, 0, 10, 5},
		{"below", -5, 0, 10, 0},
		{"above", 15, 0, 10, 10},
		{"at_lo", 0, 0, 10, 0},
		{"at_hi", 10, 0, 10, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Clamp(tt.v, tt.lo, tt.hi); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestClampFloat(t *testing.T) {
	if got := Clamp(1.5, 0.0, 1.0); got != 1.0 {
		t.Errorf("got %f, want 1.0", got)
	}
}

func TestAbs(t *testing.T) {
	if Abs(-5) != 5 {
		t.Error("Abs(-5)")
	}
	if Abs(5) != 5 {
		t.Error("Abs(5)")
	}
	if Abs(0) != 0 {
		t.Error("Abs(0)")
	}
	if Abs(-3.14) != 3.14 {
		t.Error("Abs(-3.14)")
	}
}

func TestSum(t *testing.T) {
	if Sum([]int{1, 2, 3, 4, 5}) != 15 {
		t.Error("Sum ints")
	}
	if Sum([]float64{1.1, 2.2, 3.3}) != 6.6 {
		t.Error("Sum floats")
	}
	if Sum([]int{}) != 0 {
		t.Error("Sum empty")
	}
}

func TestAverage(t *testing.T) {
	if got := Average([]int{2, 4, 6}); got != 4.0 {
		t.Errorf("Average: got %f, want 4.0", got)
	}
	if got := Average([]int{}); got != 0 {
		t.Errorf("Average empty: got %f", got)
	}
	if got := Average([]float64{1.0, 2.0, 3.0}); math.Abs(got-2.0) > 0.001 {
		t.Errorf("Average float: got %f, want 2.0", got)
	}
}

func TestPercent(t *testing.T) {
	if got := Percent(25, 100); got != 25.0 {
		t.Errorf("got %f, want 25.0", got)
	}
	if got := Percent(1, 3); math.Abs(got-33.333) > 0.01 {
		t.Errorf("got %f", got)
	}
	if got := Percent(0, 0); got != 0 {
		t.Errorf("zero total: got %f", got)
	}
}

func TestDivCeil(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{10, 3, 4},
		{9, 3, 3},
		{1, 10, 1},
		{0, 5, 0},
		{100, 10, 10},
		{5, 0, 0},
	}
	for _, tt := range tests {
		if got := DivCeil(tt.a, tt.b); got != tt.want {
			t.Errorf("DivCeil(%d,%d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestInRange(t *testing.T) {
	tests := []struct {
		name     string
		v, lo, hi int
		want     bool
	}{
		{"in", 5, 0, 10, true},
		{"below", -1, 0, 10, false},
		{"above", 11, 0, 10, false},
		{"at_lo", 0, 0, 10, true},
		{"at_hi", 10, 0, 10, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InRange(tt.v, tt.lo, tt.hi); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInRangeString(t *testing.T) {
	if !InRange("m", "a", "z") {
		t.Error("m should be in [a, z]")
	}
	if InRange("A", "a", "z") {
		t.Error("A should not be in [a, z]")
	}
}
