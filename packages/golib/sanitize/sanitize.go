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

// PathTraversal checks if a path contains traversal attempts.
// Returns true if the path is suspicious.
func PathTraversal(path string) bool {
	dangerous := []string{
		"..",
		"%2e%2e",
		"%2E%2E",
		"%252e%252e",
		"..%2f",
		"..%5c",
		"%2e%2e%2f",
		"%2e%2e%5c",
	}
	lower := strings.ToLower(path)
	for _, d := range dangerous {
		if strings.Contains(lower, strings.ToLower(d)) {
			return true
		}
	}
	return false
}

// XSSPayload checks if a string contains common XSS patterns.
// This is a defense-in-depth check — proper output encoding is the primary defense.
func XSSPayload(s string) bool {
	lower := strings.ToLower(s)
	patterns := []string{
		"<script",
		"javascript:",
		"onerror=",
		"onload=",
		"onclick=",
		"onmouseover=",
		"onfocus=",
		"onblur=",
		"expression(",
		"vbscript:",
		"data:text/html",
		"<iframe",
		"<object",
		"<embed",
		"<svg",
		"document.cookie",
		"document.location",
		"window.location",
	}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// HeaderValue sanitizes HTTP header values by removing newlines
// and control characters that could enable header injection.
func HeaderValue(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\x00' {
			continue
		}
		if unicode.IsControl(r) && r != '\t' {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// JSONString escapes a string for safe embedding in JSON.
// Prevents JSON injection by escaping special characters.
func JSONString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '<':
			b.WriteString(`\u003c`)
		case '>':
			b.WriteString(`\u003e`)
		case '&':
			b.WriteString(`\u0026`)
		default:
			if unicode.IsControl(r) {
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}

// URL validates and sanitizes a URL to prevent SSRF.
// Returns empty string if the URL is dangerous.
func URL(s string) string {
	s = strings.TrimSpace(s)

	// Only allow http and https schemes.
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return ""
	}

	// Block common SSRF targets.
	lower := strings.ToLower(s)
	blockedHosts := []string{
		"localhost",
		"127.0.0.1",
		"0.0.0.0",
		"[::1]",
		"169.254.",    // AWS metadata.
		"metadata.google",
		"100.100.100.200", // Alibaba metadata.
	}
	for _, blocked := range blockedHosts {
		if strings.Contains(lower, blocked) {
			return ""
		}
	}

	return s
}

// NIK validates and normalizes an Indonesian NIK (National ID Number).
// Returns the cleaned NIK or empty string if invalid.
func NIK(s string) string {
	// Remove all non-digits.
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	nik := b.String()

	if len(nik) != 16 {
		return ""
	}

	// Validate province code (11-94).
	province := (nik[0]-'0')*10 + (nik[1] - '0')
	if province < 11 || province > 94 {
		return ""
	}

	return nik
}
