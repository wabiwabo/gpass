// Package stringx provides string utility functions beyond
// the standard library. Includes truncation, padding, case
// conversion, and common string operations.
package stringx

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Truncate truncates a string to maxLen runes, adding suffix if truncated.
func Truncate(s string, maxLen int, suffix string) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	suffixLen := utf8.RuneCountInString(suffix)
	if maxLen <= suffixLen {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-suffixLen]) + suffix
}

// PadLeft pads a string on the left to the given length.
func PadLeft(s string, length int, pad rune) string {
	n := length - utf8.RuneCountInString(s)
	if n <= 0 {
		return s
	}
	return strings.Repeat(string(pad), n) + s
}

// PadRight pads a string on the right to the given length.
func PadRight(s string, length int, pad rune) string {
	n := length - utf8.RuneCountInString(s)
	if n <= 0 {
		return s
	}
	return s + strings.Repeat(string(pad), n)
}

// Snake converts a CamelCase string to snake_case.
func Snake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := rune(s[i-1])
				if unicode.IsLower(prev) || unicode.IsDigit(prev) {
					result.WriteByte('_')
				}
			}
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// Camel converts a snake_case string to CamelCase.
func Camel(s string) string {
	var result strings.Builder
	upper := true
	for _, r := range s {
		if r == '_' || r == '-' {
			upper = true
			continue
		}
		if upper {
			result.WriteRune(unicode.ToUpper(r))
			upper = false
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// LowerCamel converts to lowerCamelCase.
func LowerCamel(s string) string {
	c := Camel(s)
	if len(c) == 0 {
		return c
	}
	runes := []rune(c)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// ContainsAny checks if a string contains any of the substrings.
func ContainsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// DefaultIfEmpty returns def if s is empty or whitespace.
func DefaultIfEmpty(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// FirstNonEmpty returns the first non-empty string.
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// Mask replaces characters with a mask, keeping first/last n visible.
func Mask(s string, keep int, maskChar rune) string {
	runes := []rune(s)
	if len(runes) <= keep*2 {
		return s
	}
	for i := keep; i < len(runes)-keep; i++ {
		runes[i] = maskChar
	}
	return string(runes)
}

// Reverse reverses a string (rune-aware).
func Reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
