package sanitize

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	htmlTagRegex = regexp.MustCompile(`<[^>]*>`)
	sqlPatterns  = []string{
		"--",
		";--",
		"/*",
		"*/",
		"@@",
		"char(",
		"nchar(",
		"varchar(",
		"nvarchar(",
		"alter ",
		"begin ",
		"cast(",
		"create ",
		"cursor ",
		"declare ",
		"delete ",
		"drop ",
		"end ",
		"exec(",
		"exec ",
		"execute(",
		"execute ",
		"fetch ",
		"insert ",
		"kill ",
		"select ",
		"sys.",
		"sysobjects",
		"syscolumns",
		"table ",
		"update ",
		"union ",
		"' or ",
		"'or ",
		"' and ",
		"'and ",
	}
	multiSpaceRegex = regexp.MustCompile(`\s+`)
)

// String removes control characters, trims whitespace, and limits length.
func String(s string, maxLen int) string {
	s = removeControlChars(s)
	s = strings.TrimSpace(s)
	if maxLen > 0 && len([]rune(s)) > maxLen {
		s = string([]rune(s)[:maxLen])
	}
	return s
}

// Name sanitizes a person/company name: removes non-printable chars,
// normalizes whitespace, limits to maxLen. Preserves Unicode letters.
func Name(s string, maxLen int) string {
	s = removeNonPrintable(s)
	s = multiSpaceRegex.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	if maxLen > 0 && len([]rune(s)) > maxLen {
		s = string([]rune(s)[:maxLen])
	}
	return s
}

// Filename sanitizes a filename: removes path separators, null bytes,
// double dots, and limits to maxLen. Returns empty string if result is
// empty or just dots.
func Filename(s string, maxLen int) string {
	// Remove null bytes.
	s = strings.ReplaceAll(s, "\x00", "")
	// Remove path separators.
	s = strings.ReplaceAll(s, "/", "")
	s = strings.ReplaceAll(s, "\\", "")
	// Remove double dots (path traversal).
	s = strings.ReplaceAll(s, "..", "")
	s = strings.TrimSpace(s)

	if maxLen > 0 && len([]rune(s)) > maxLen {
		s = string([]rune(s)[:maxLen])
	}

	// Reject if empty or only dots.
	trimmed := strings.TrimRight(s, ".")
	if trimmed == "" {
		return ""
	}

	return s
}

// StripHTML removes all HTML tags from a string.
func StripHTML(s string) string {
	return htmlTagRegex.ReplaceAllString(s, "")
}

// StripSQLInjection removes common SQL injection patterns.
// This is a defense-in-depth measure -- parameterized queries are the primary defense.
func StripSQLInjection(s string) string {
	result := s
	for _, pattern := range sqlPatterns {
		for {
			idx := strings.Index(strings.ToLower(result), pattern)
			if idx == -1 {
				break
			}
			result = result[:idx] + result[idx+len(pattern):]
		}
	}
	return result
}

// IsCleanString returns true if the string contains no control characters,
// null bytes, or HTML tags.
func IsCleanString(s string) bool {
	for _, r := range s {
		if r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		if unicode.IsControl(r) {
			return false
		}
	}
	if strings.Contains(s, "\x00") {
		return false
	}
	if htmlTagRegex.MatchString(s) {
		return false
	}
	return true
}

// PhoneNumber normalizes Indonesian phone numbers to +62 format.
// Accepts: 08xxx, 628xxx, +628xxx -> +628xxx
func PhoneNumber(s string) string {
	s = strings.TrimSpace(s)
	// Remove all non-digit and non-plus characters.
	var b strings.Builder
	for _, r := range s {
		if r == '+' || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	s = b.String()

	if strings.HasPrefix(s, "+62") {
		return s
	}
	if strings.HasPrefix(s, "62") {
		return "+" + s
	}
	if strings.HasPrefix(s, "0") {
		return "+62" + s[1:]
	}
	return s
}

// Email normalizes an email address: lowercase, trim whitespace.
func Email(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// removeControlChars removes control characters except tab, newline, carriage return.
func removeControlChars(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\t' || r == '\n' || r == '\r' {
			b.WriteRune(r)
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// removeNonPrintable removes non-printable characters.
func removeNonPrintable(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsPrint(r) || r == '\t' || r == '\n' || r == '\r' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
