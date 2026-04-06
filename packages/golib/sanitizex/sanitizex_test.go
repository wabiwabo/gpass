package sanitizex

import "testing"

func TestStripControl(t *testing.T) {
	if StripControl("hello\x00world") != "helloworld" { t.Error("null") }
	if StripControl("line1\nline2") != "line1\nline2" { t.Error("newline preserved") }
	if StripControl("tab\there") != "tab\there" { t.Error("tab preserved") }
	if StripControl("bell\x07ring") != "bellring" { t.Error("bell") }
}

func TestNormalizeWhitespace(t *testing.T) {
	if NormalizeWhitespace("  hello   world  ") != "hello world" { t.Error("spaces") }
	if NormalizeWhitespace("a\t\nb") != "a b" { t.Error("tabs/newlines") }
}

func TestStripNullBytes(t *testing.T) {
	if StripNullBytes("a\x00b\x00c") != "abc" { t.Error("nulls") }
}

func TestTrimAndNormalize(t *testing.T) {
	got := TrimAndNormalize("  hello\x00   world  ")
	if got != "hello world" { t.Errorf("got %q", got) }
}

func TestStripBIDI(t *testing.T) {
	// Text with RLO (right-to-left override)
	got := StripBIDI("hello\u202Eworld")
	if got != "helloworld" { t.Errorf("got %q", got) }

	// Normal text unchanged
	if StripBIDI("normal") != "normal" { t.Error("normal") }
}

func TestStripZeroWidth(t *testing.T) {
	got := StripZeroWidth("he\u200Bllo")
	if got != "hello" { t.Errorf("got %q", got) }

	got2 := StripZeroWidth("\uFEFFBOM")
	if got2 != "BOM" { t.Errorf("BOM: %q", got2) }
}

func TestSafeString(t *testing.T) {
	got := SafeString("  \x00hello\u200B   \u202Eworld\x07  ")
	if got != "hello world" { t.Errorf("got %q", got) }
}

func TestSafeFilename(t *testing.T) {
	tests := []struct{ in, want string }{
		{"report.pdf", "report.pdf"},
		{"my file (1).pdf", "myfile1.pdf"},
		{"../../../etc/passwd", "......etcpasswd"},
		{"hello world.txt", "helloworld.txt"},
		{"", "unnamed"},
		{".", "unnamed"},
		{"..", "unnamed"},
	}
	for _, tt := range tests {
		got := SafeFilename(tt.in)
		if got != tt.want { t.Errorf("SafeFilename(%q) = %q, want %q", tt.in, got, tt.want) }
	}
}

func TestSafeString_Empty(t *testing.T) {
	if SafeString("") != "" { t.Error("empty") }
	if SafeString("   ") != "" { t.Error("spaces") }
}
