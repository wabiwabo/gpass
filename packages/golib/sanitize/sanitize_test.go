package sanitize

import (
	"testing"
)

func TestString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"trims whitespace", "  hello  ", 100, "hello"},
		{"limits length", "abcdefghij", 5, "abcde"},
		{"removes control chars", "hello\x00world", 100, "helloworld"},
		{"preserves tabs and newlines", "hello\tworld\n", 100, "hello\tworld"},
		{"empty string", "", 100, ""},
		{"zero maxLen means no limit", "hello", 0, "hello"},
		{"unicode length", "привет мир", 6, "привет"},
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

func TestName(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"simple name", "John Doe", 100, "John Doe"},
		{"normalizes multiple spaces", "John   Doe", 100, "John Doe"},
		{"preserves Indonesian Unicode", "Budi Santoso", 100, "Budi Santoso"},
		{"preserves Unicode letters", "Müller Straße", 100, "Müller Straße"},
		{"removes non-printable", "John\x00Doe", 100, "JohnDoe"},
		{"limits length", "Muhammad Rizky Pratama", 10, "Muhammad R"},
		{"trims whitespace", "  Siti Aminah  ", 100, "Siti Aminah"},
		{"normalizes tabs", "John\tDoe", 100, "John Doe"},
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

func TestFilename(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"simple filename", "document.pdf", 100, "document.pdf"},
		{"removes forward slash", "path/to/file.txt", 100, "pathtofile.txt"},
		{"removes backslash", "path\\to\\file.txt", 100, "pathtofile.txt"},
		{"removes double dots", "../../../etc/passwd", 100, "etcpasswd"},
		{"removes null bytes", "file\x00name.txt", 100, "filename.txt"},
		{"limits length", "verylongfilename.txt", 10, "verylongfi"},
		{"empty string", "", 100, ""},
		{"just dots single", ".", 100, ""},
		{"just dots double", "..", 100, ""},
		{"just dots triple", "...", 100, ""},
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

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"removes simple tags", "<b>bold</b>", "bold"},
		{"removes script tags", "<script>alert('xss')</script>", "alert('xss')"},
		{"preserves text", "no tags here", "no tags here"},
		{"removes nested tags", "<div><p>hello</p></div>", "hello"},
		{"removes self-closing tags", "text<br/>more", "textmore"},
		{"empty string", "", ""},
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

func TestStripSQLInjection(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"removes DROP TABLE", "'; DROP TABLE users", "'; users"},
		{"removes UNION SELECT", "1 UNION SELECT * FROM users", "1 * FROM users"},
		{"removes comment dashes", "admin'--", "admin'"},
		{"preserves normal text", "hello world", "hello world"},
		{"case insensitive", "'; drop TABLE users", "'; users"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripSQLInjection(tt.input)
			if got != tt.want {
				t.Errorf("StripSQLInjection(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsCleanString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"clean string", "hello world", true},
		{"with tab", "hello\tworld", true},
		{"with newline", "hello\nworld", true},
		{"with null byte", "hello\x00world", false},
		{"with control char", "hello\x01world", false},
		{"with HTML tag", "hello <b>world</b>", false},
		{"empty string", "", true},
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

func TestPhoneNumber(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"08 format", "081234567890", "+6281234567890"},
		{"62 format", "6281234567890", "+6281234567890"},
		{"+62 format", "+6281234567890", "+6281234567890"},
		{"with spaces", "0812 3456 7890", "+6281234567890"},
		{"with dashes", "0812-3456-7890", "+6281234567890"},
		{"empty string", "", ""},
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

func TestEmail(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"lowercases", "User@Example.COM", "user@example.com"},
		{"trims whitespace", "  user@example.com  ", "user@example.com"},
		{"already lowercase", "user@example.com", "user@example.com"},
		{"empty string", "", ""},
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
