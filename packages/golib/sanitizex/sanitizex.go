// Package sanitizex provides advanced input sanitization for
// enterprise applications. Strips control characters, normalizes
// whitespace, and prevents injection through Unicode tricks.
package sanitizex

import (
	"strings"
	"unicode"
)

// StripControl removes all control characters except newline and tab.
func StripControl(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\n' || r == '\t' || !unicode.IsControl(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// NormalizeWhitespace collapses multiple whitespace to single space.
func NormalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// StripNullBytes removes null bytes from string.
func StripNullBytes(s string) string {
	return strings.ReplaceAll(s, "\x00", "")
}

// TrimAndNormalize trims, strips control chars, and normalizes whitespace.
func TrimAndNormalize(s string) string {
	return NormalizeWhitespace(StripControl(strings.TrimSpace(s)))
}

// StripBIDI removes Unicode bidirectional override characters
// which can be used for text reordering attacks.
func StripBIDI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\u200E', '\u200F', // LRM, RLM
			'\u202A', '\u202B', '\u202C', '\u202D', '\u202E', // LRE, RLE, PDF, LRO, RLO
			'\u2066', '\u2067', '\u2068', '\u2069': // LRI, RLI, FSI, PDI
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// StripZeroWidth removes zero-width characters that can create
// visually identical but technically different strings.
func StripZeroWidth(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\u200B', '\u200C', '\u200D', '\uFEFF': // ZWSP, ZWNJ, ZWJ, BOM
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SafeString applies all sanitizations for user-facing text.
func SafeString(s string) string {
	s = StripNullBytes(s)
	s = StripControl(s)
	s = StripBIDI(s)
	s = StripZeroWidth(s)
	s = NormalizeWhitespace(s)
	return strings.TrimSpace(s)
}

// SafeFilename sanitizes a filename, keeping only safe characters.
func SafeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	result := b.String()
	if result == "" || result == "." || result == ".." {
		return "unnamed"
	}
	return result
}
