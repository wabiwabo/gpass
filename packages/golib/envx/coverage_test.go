package envx

import (
	"testing"
	"time"
)

// TestParsers_BadValuesFallToDefault pins the strconv/time.ParseDuration
// error branches in Int64/Duration/Float64 — they all silently return the
// default rather than erroring.
func TestParsers_BadValuesFallToDefault(t *testing.T) {
	t.Setenv("BAD_INT", "not-a-number")
	if got := Int64("BAD_INT", 42); got != 42 {
		t.Errorf("Int64 bad: got %d, want 42", got)
	}

	t.Setenv("BAD_DUR", "12parsecs")
	if got := Duration("BAD_DUR", time.Second); got != time.Second {
		t.Errorf("Duration bad: got %v, want 1s", got)
	}

	t.Setenv("BAD_FLOAT", "pi")
	if got := Float64("BAD_FLOAT", 3.14); got != 3.14 {
		t.Errorf("Float64 bad: got %v, want 3.14", got)
	}
}

// TestParsers_GoodValues pins the happy paths.
func TestParsers_GoodValues(t *testing.T) {
	t.Setenv("GOOD_INT", "9223372036854775807")
	if got := Int64("GOOD_INT", 0); got != 9223372036854775807 {
		t.Errorf("Int64 max: got %d", got)
	}

	t.Setenv("GOOD_DUR", "5m30s")
	if got := Duration("GOOD_DUR", 0); got != 5*time.Minute+30*time.Second {
		t.Errorf("Duration: got %v", got)
	}

	t.Setenv("GOOD_FLOAT", "2.718")
	if got := Float64("GOOD_FLOAT", 0); got != 2.718 {
		t.Errorf("Float64: got %v", got)
	}
}

// TestList_DefaultSeparatorAndTrimAndEmpties pins the empty-separator
// fallback to "," AND that whitespace-only / empty fragments are dropped.
func TestList_DefaultSeparatorAndTrimAndEmpties(t *testing.T) {
	t.Setenv("LIST_VAL", " a , b ,, c , ")
	got := List("LIST_VAL", "")
	want := []string{"a", "b", "c"}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3 (%v)", len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("[%d] = %q, want %q", i, got[i], w)
		}
	}

	// Custom separator.
	t.Setenv("LIST_PIPE", "x|y|z")
	if got := List("LIST_PIPE", "|"); len(got) != 3 || got[2] != "z" {
		t.Errorf("pipe sep: %v", got)
	}
}

// TestOneOf_RejectsDisallowedReturnsDefault pins the not-in-allowlist
// branch.
func TestOneOf_RejectsDisallowedReturnsDefault(t *testing.T) {
	t.Setenv("ENV_TIER", "platinum")
	got := OneOf("ENV_TIER", []string{"free", "pro", "enterprise"}, "free")
	if got != "free" {
		t.Errorf("disallowed: got %q, want free", got)
	}

	t.Setenv("ENV_TIER", "pro")
	if got := OneOf("ENV_TIER", []string{"free", "pro", "enterprise"}, "free"); got != "pro" {
		t.Errorf("allowed: got %q, want pro", got)
	}
}
