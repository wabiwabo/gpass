package stringx

import "testing"

func TestTruncate(t *testing.T) {
	tests := []struct {
		s, suffix string
		max       int
		want      string
	}{
		{"Hello World", "...", 8, "Hello..."},
		{"Hi", "...", 10, "Hi"},
		{"Hello", "...", 5, "Hello"},
		{"Hello", "...", 3, "Hel"},
		{"ABCDEF", "…", 4, "ABC…"},
	}
	for _, tt := range tests {
		got := Truncate(tt.s, tt.max, tt.suffix)
		if got != tt.want {
			t.Errorf("Truncate(%q, %d, %q) = %q, want %q", tt.s, tt.max, tt.suffix, got, tt.want)
		}
	}
}

func TestPadLeft(t *testing.T) {
	if got := PadLeft("42", 5, '0'); got != "00042" {
		t.Errorf("PadLeft = %q", got)
	}
	if got := PadLeft("hello", 3, ' '); got != "hello" {
		t.Errorf("no pad needed = %q", got)
	}
}

func TestPadRight(t *testing.T) {
	if got := PadRight("hi", 5, '.'); got != "hi..." {
		t.Errorf("PadRight = %q", got)
	}
}

func TestSnake(t *testing.T) {
	tests := []struct{ in, want string }{
		{"CamelCase", "camel_case"},
		{"userID", "user_id"},
		{"HTTPServer", "httpserver"},
		{"simple", "simple"},
		{"A", "a"},
	}
	for _, tt := range tests {
		if got := Snake(tt.in); got != tt.want {
			t.Errorf("Snake(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCamel(t *testing.T) {
	tests := []struct{ in, want string }{
		{"snake_case", "SnakeCase"},
		{"hello_world", "HelloWorld"},
		{"single", "Single"},
		{"kebab-case", "KebabCase"},
	}
	for _, tt := range tests {
		if got := Camel(tt.in); got != tt.want {
			t.Errorf("Camel(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestLowerCamel(t *testing.T) {
	if got := LowerCamel("snake_case"); got != "snakeCase" {
		t.Errorf("LowerCamel = %q", got)
	}
	if got := LowerCamel(""); got != "" {
		t.Errorf("empty = %q", got)
	}
}

func TestContainsAny(t *testing.T) {
	if !ContainsAny("hello world", "world", "test") {
		t.Error("should match")
	}
	if ContainsAny("hello", "world", "test") {
		t.Error("should not match")
	}
}

func TestDefaultIfEmpty(t *testing.T) {
	if DefaultIfEmpty("hello", "default") != "hello" {
		t.Error("non-empty should return value")
	}
	if DefaultIfEmpty("", "default") != "default" {
		t.Error("empty should return default")
	}
	if DefaultIfEmpty("  ", "default") != "default" {
		t.Error("whitespace should return default")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if FirstNonEmpty("", "  ", "hello", "world") != "hello" {
		t.Error("should return first non-empty")
	}
	if FirstNonEmpty("", "") != "" {
		t.Error("all empty should return empty")
	}
}

func TestMask(t *testing.T) {
	if got := Mask("1234567890", 2, '*'); got != "12******90" {
		t.Errorf("Mask = %q", got)
	}
	if got := Mask("ab", 2, '*'); got != "ab" {
		t.Errorf("short = %q", got)
	}
}

func TestReverse(t *testing.T) {
	if Reverse("hello") != "olleh" {
		t.Error("ascii reverse")
	}
	if Reverse("") != "" {
		t.Error("empty reverse")
	}
}
