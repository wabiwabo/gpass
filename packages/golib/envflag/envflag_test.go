package envflag

import (
	"strings"
	"testing"
	"time"
)

func TestString(t *testing.T) {
	t.Setenv("TEST_STRING", "hello")
	if got := String("TEST_STRING", "default"); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestStringFallback(t *testing.T) {
	if got := String("ENVFLAG_UNSET_STRING", "default"); got != "default" {
		t.Errorf("got %q, want %q", got, "default")
	}
}

func TestInt(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fallback int
		want     int
	}{
		{"valid", "42", 0, 42},
		{"negative", "-10", 0, -10},
		{"invalid", "abc", 99, 99},
		{"empty", "", 99, 99},
		{"float", "3.14", 99, 99},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				t.Setenv("TEST_INT", tt.value)
			}
			key := "TEST_INT"
			if tt.value == "" {
				key = "ENVFLAG_UNSET_INT"
			}
			if got := Int(key, tt.fallback); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestInt64(t *testing.T) {
	t.Setenv("TEST_INT64", "9223372036854775807")
	if got := Int64("TEST_INT64", 0); got != 9223372036854775807 {
		t.Errorf("got %d, want max int64", got)
	}
}

func TestInt64Fallback(t *testing.T) {
	if got := Int64("ENVFLAG_UNSET_INT64", 100); got != 100 {
		t.Errorf("got %d, want 100", got)
	}
}

func TestInt64Invalid(t *testing.T) {
	t.Setenv("TEST_INT64_BAD", "not-a-number")
	if got := Int64("TEST_INT64_BAD", 50); got != 50 {
		t.Errorf("got %d, want 50 (fallback)", got)
	}
}

func TestBool(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fallback bool
		want     bool
	}{
		{"true_1", "1", false, true},
		{"true_true", "true", false, true},
		{"true_TRUE", "TRUE", false, true},
		{"true_yes", "yes", false, true},
		{"true_on", "ON", false, true},
		{"false_0", "0", true, false},
		{"false_false", "false", true, false},
		{"false_no", "no", true, false},
		{"false_off", "off", true, false},
		{"invalid", "maybe", false, false},
		{"invalid_true_fallback", "maybe", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TEST_BOOL", tt.value)
			if got := Bool("TEST_BOOL", tt.fallback); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBoolFallback(t *testing.T) {
	if got := Bool("ENVFLAG_UNSET_BOOL", true); got != true {
		t.Errorf("got %v, want true", got)
	}
}

func TestDuration(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fallback time.Duration
		want     time.Duration
	}{
		{"milliseconds", "100ms", 0, 100 * time.Millisecond},
		{"seconds", "5s", 0, 5 * time.Second},
		{"minutes", "2m", 0, 2 * time.Minute},
		{"invalid", "bad", time.Second, time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TEST_DUR", tt.value)
			if got := Duration("TEST_DUR", tt.fallback); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDurationFallback(t *testing.T) {
	if got := Duration("ENVFLAG_UNSET_DUR", 30*time.Second); got != 30*time.Second {
		t.Errorf("got %v, want 30s", got)
	}
}

func TestFloat64(t *testing.T) {
	t.Setenv("TEST_FLOAT", "3.14")
	if got := Float64("TEST_FLOAT", 0); got != 3.14 {
		t.Errorf("got %f, want 3.14", got)
	}
}

func TestFloat64Fallback(t *testing.T) {
	if got := Float64("ENVFLAG_UNSET_FLOAT", 2.71); got != 2.71 {
		t.Errorf("got %f, want 2.71", got)
	}
}

func TestFloat64Invalid(t *testing.T) {
	t.Setenv("TEST_FLOAT_BAD", "not-a-float")
	if got := Float64("TEST_FLOAT_BAD", 1.0); got != 1.0 {
		t.Errorf("got %f, want 1.0", got)
	}
}

func TestStringSlice(t *testing.T) {
	t.Setenv("TEST_SLICE", "a,b,c")
	got := StringSlice("TEST_SLICE", ",", nil)
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("got %v, want [a b c]", got)
	}
}

func TestStringSliceWithSpaces(t *testing.T) {
	t.Setenv("TEST_SLICE_SP", " a , b , c ")
	got := StringSlice("TEST_SLICE_SP", ",", nil)
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("got %v, want [a b c]", got)
	}
}

func TestStringSliceFallback(t *testing.T) {
	fb := []string{"x", "y"}
	got := StringSlice("ENVFLAG_UNSET_SLICE", ",", fb)
	if len(got) != 2 || got[0] != "x" || got[1] != "y" {
		t.Errorf("got %v, want %v", got, fb)
	}
}

func TestStringSliceCustomSep(t *testing.T) {
	t.Setenv("TEST_SLICE_PIPE", "a|b|c")
	got := StringSlice("TEST_SLICE_PIPE", "|", nil)
	if len(got) != 3 {
		t.Errorf("got %v, want 3 items", got)
	}
}

func TestStringSliceEmpty(t *testing.T) {
	t.Setenv("TEST_SLICE_EMPTY", ",,,,")
	fb := []string{"default"}
	got := StringSlice("TEST_SLICE_EMPTY", ",", fb)
	if len(got) != 1 || got[0] != "default" {
		t.Errorf("all-empty should return fallback, got %v", got)
	}
}

func TestMustString(t *testing.T) {
	t.Setenv("TEST_MUST", "required_value")
	if got := MustString("TEST_MUST"); got != "required_value" {
		t.Errorf("got %q, want %q", got, "required_value")
	}
}

func TestMustStringPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for unset required var")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "envflag:") {
			t.Errorf("panic should contain 'envflag:': %v", r)
		}
	}()
	MustString("ENVFLAG_DEFINITELY_UNSET_" + "MUST")
}

func TestNewPrefix(t *testing.T) {
	p := NewPrefix("APP")
	if p.prefix != "APP_" {
		t.Errorf("prefix: got %q, want %q", p.prefix, "APP_")
	}
}

func TestNewPrefixWithUnderscore(t *testing.T) {
	p := NewPrefix("APP_")
	if p.prefix != "APP_" {
		t.Errorf("prefix: got %q, want %q", p.prefix, "APP_")
	}
}

func TestPrefixString(t *testing.T) {
	t.Setenv("MYAPP_HOST", "localhost")
	p := NewPrefix("MYAPP")
	if got := p.String("HOST", "0.0.0.0"); got != "localhost" {
		t.Errorf("got %q, want %q", got, "localhost")
	}
}

func TestPrefixInt(t *testing.T) {
	t.Setenv("MYAPP_PORT", "8080")
	p := NewPrefix("MYAPP")
	if got := p.Int("PORT", 3000); got != 8080 {
		t.Errorf("got %d, want 8080", got)
	}
}

func TestPrefixBool(t *testing.T) {
	t.Setenv("MYAPP_DEBUG", "true")
	p := NewPrefix("MYAPP")
	if got := p.Bool("DEBUG", false); !got {
		t.Error("got false, want true")
	}
}

func TestPrefixDuration(t *testing.T) {
	t.Setenv("MYAPP_TIMEOUT", "30s")
	p := NewPrefix("MYAPP")
	if got := p.Duration("TIMEOUT", time.Second); got != 30*time.Second {
		t.Errorf("got %v, want 30s", got)
	}
}

func TestPrefixFallback(t *testing.T) {
	p := NewPrefix("ENVFLAG_TEST")
	if got := p.String("UNSET", "fallback"); got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
}
