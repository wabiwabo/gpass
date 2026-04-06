package sanitize

import (
	"testing"
)

func TestStripHTML_NestedTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"deeply nested", "<div><span><a href='x'><b>text</b></a></span></div>", "text"},
		{"nested with attributes", `<div class="outer"><p id="inner">hello</p></div>`, "hello"},
		{"mixed content and tags", "before<div>middle</div>after", "beforemiddleafter"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripHTML(tt.input)
			if got != tt.want {
				t.Errorf("StripHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripHTML_ScriptTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"script with JS", "<script>document.cookie</script>", "document.cookie"},
		{"script with src", `<script src="evil.js"></script>`, ""},
		{"script in middle", `hello<script>alert(1)</script>world`, "helloalert(1)world"},
		{"multiple scripts", `<script>a</script>b<script>c</script>`, "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripHTML(tt.input)
			if got != tt.want {
				t.Errorf("StripHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripHTML_EventHandlers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"onclick", `<div onclick="alert(1)">click</div>`, "click"},
		{"onerror on img", `<img onerror="alert(1)" src="x">`, ""},
		{"onload on body", `<body onload="steal()">content</body>`, "content"},
		{"onmouseover", `<span onmouseover="evil()">hover</span>`, "hover"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripHTML(tt.input)
			if got != tt.want {
				t.Errorf("StripHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFilename_UnicodePreserved(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"Indonesian", "dokumen_surat.pdf", 100, "dokumen_surat.pdf"},
		{"Chinese characters", "文件.pdf", 100, "文件.pdf"},
		{"Japanese", "ファイル.txt", 100, "ファイル.txt"},
		{"Korean", "파일.doc", 100, "파일.doc"},
		{"emoji in filename", "📄report.pdf", 100, "📄report.pdf"},
		{"Arabic", "ملف.xlsx", 100, "ملف.xlsx"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Filename(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("Filename(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestFilename_NullBytesRemoved(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"null at start", "\x00file.txt", 100, "file.txt"},
		{"null at end", "file.txt\x00", 100, "file.txt"},
		{"null in middle", "fi\x00le.txt", 100, "file.txt"},
		{"multiple nulls", "\x00fi\x00le\x00.txt\x00", 100, "file.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Filename(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("Filename(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestPhoneNumber_CountryCodePreserved(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"+62 already", "+6281234567890", "+6281234567890"},
		{"+62 with spaces", "+62 812 3456 7890", "+6281234567890"},
		{"+62 with dashes", "+62-812-3456-7890", "+6281234567890"},
		{"+62 with parens", "+62(812)34567890", "+6281234567890"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PhoneNumber(tt.input)
			if got != tt.want {
				t.Errorf("PhoneNumber(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPhoneNumber_SpacesAndDashesStripped(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"spaces", "0812 3456 7890", "+6281234567890"},
		{"dashes", "0812-3456-7890", "+6281234567890"},
		{"dots", "0812.3456.7890", "+6281234567890"},
		{"mixed separators", "0812-3456 7890", "+6281234567890"},
		{"leading spaces", "  081234567890  ", "+6281234567890"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PhoneNumber(tt.input)
			if got != tt.want {
				t.Errorf("PhoneNumber(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEmail_UppercaseDomainLowered(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"uppercase domain", "user@EXAMPLE.COM", "user@example.com"},
		{"mixed case", "User@Example.Com", "user@example.com"},
		{"uppercase local", "USER@example.com", "user@example.com"},
		{"all uppercase", "USER@EXAMPLE.COM", "user@example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Email(tt.input)
			if got != tt.want {
				t.Errorf("Email(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripSQLInjection_UnionAllSelect(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"UNION ALL SELECT", "1 UNION ALL SELECT username, password FROM users"},
		{"union all select lowercase", "1 union all select username, password from users"},
		{"UNION SELECT", "1 UNION SELECT * FROM users"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripSQLInjection(tt.input)
			// Should not contain "select " or "union " (case insensitive)
			if containsCaseInsensitive(got, "select ") {
				t.Errorf("StripSQLInjection(%q) still contains 'select': %q", tt.input, got)
			}
			if containsCaseInsensitive(got, "union ") {
				t.Errorf("StripSQLInjection(%q) still contains 'union': %q", tt.input, got)
			}
		})
	}
}

func TestStripSQLInjection_HexEncoding(t *testing.T) {
	// Hex-encoded attacks using CHAR() and CAST() should be stripped
	tests := []struct {
		name  string
		input string
	}{
		{"char function", "CHAR(0x41)"},
		{"cast function", "CAST(0x41 AS VARCHAR)"},
		{"nchar function", "NCHAR(0x41)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripSQLInjection(tt.input)
			if containsCaseInsensitive(got, "char(") || containsCaseInsensitive(got, "cast(") || containsCaseInsensitive(got, "nchar(") {
				t.Errorf("StripSQLInjection(%q) still contains injection pattern: %q", tt.input, got)
			}
		})
	}
}

func TestIsCleanString_TabCharacters(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"single tab", "hello\tworld", true},
		{"multiple tabs", "\thello\t\tworld\t", true},
		{"tab only", "\t", true},
		{"tab with null byte", "\t\x00", false},
		{"tab with control char", "\t\x01", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCleanString(tt.input)
			if got != tt.want {
				t.Errorf("IsCleanString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsCleanString_CarriageReturn(t *testing.T) {
	if !IsCleanString("hello\r\nworld") {
		t.Error("IsCleanString should accept carriage return + newline")
	}
}

func TestString_ControlCharVariants(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"bell char", "hello\x07world", 100, "helloworld"},
		{"backspace", "hello\x08world", 100, "helloworld"},
		{"escape", "hello\x1Bworld", 100, "helloworld"},
		{"DEL", "hello\x7Fworld", 100, "helloworld"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := String(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("String(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestName_IndonesianNames(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"common name", "Siti Nurhaliza", 100, "Siti Nurhaliza"},
		{"with title", "Dr. Ir. H. Joko Widodo", 100, "Dr. Ir. H. Joko Widodo"},
		{"long name", "Muhammad Rizky Pratama Putra Sanjaya", 20, "Muhammad Rizky Prata"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Name(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("Name(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

// helper
func containsCaseInsensitive(s, substr string) bool {
	return len(s) >= len(substr) &&
		containsLower(s, substr)
}

func containsLower(s, substr string) bool {
	sl := toLower(s)
	subl := toLower(substr)
	for i := 0; i <= len(sl)-len(subl); i++ {
		if sl[i:i+len(subl)] == subl {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
