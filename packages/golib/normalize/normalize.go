// Package normalize provides text normalization for Indonesian names,
// addresses, and identifiers. Handles common Indonesian naming patterns,
// diacritics, and abbreviations.
package normalize

import (
	"strings"
	"unicode"
)

// Name normalizes an Indonesian name: trims, collapses whitespace,
// title-cases, and handles common prefixes/suffixes.
func Name(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// Collapse whitespace.
	var b strings.Builder
	b.Grow(len(s))
	lastSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !lastSpace {
				b.WriteRune(' ')
				lastSpace = true
			}
			continue
		}
		lastSpace = false
		b.WriteRune(r)
	}
	s = b.String()

	// Title case each word, respecting Indonesian prefixes.
	words := strings.Fields(s)
	for i, w := range words {
		words[i] = titleWord(w)
		_ = i
	}

	return strings.Join(words, " ")
}

func titleWord(w string) string {
	if len(w) == 0 {
		return w
	}
	// Keep common Indonesian lowercase words in context.
	lower := strings.ToLower(w)
	lowerWords := map[string]bool{
		"bin": true, "binti": true, "van": true, "de": true, "di": true,
	}
	if lowerWords[lower] {
		return lower
	}

	runes := []rune(strings.ToLower(w))
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// Phone normalizes an Indonesian phone number to E.164 format (+62...).
func Phone(s string) string {
	s = strings.TrimSpace(s)

	// Extract digits and leading +.
	var b strings.Builder
	for i, r := range s {
		if r == '+' && i == 0 {
			b.WriteRune(r)
		} else if r >= '0' && r <= '9' {
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

// Email normalizes an email: lowercase, trim, remove dots from local
// part of gmail addresses (common dedup pattern).
func Email(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}

	at := strings.LastIndex(s, "@")
	if at < 0 {
		return s
	}

	local := s[:at]
	domain := s[at+1:]

	// Remove + suffixes (sub-addressing).
	if plus := strings.Index(local, "+"); plus >= 0 {
		local = local[:plus]
	}

	return local + "@" + domain
}

// NIK normalizes a NIK by removing non-digit characters.
func NIK(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	result := b.String()
	if len(result) != 16 {
		return "" // Invalid NIK.
	}
	return result
}

// NPWP normalizes an NPWP by removing separators, returning 15 digits.
func NPWP(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	result := b.String()
	if len(result) != 15 {
		return "" // Invalid NPWP.
	}
	return result
}

// Address normalizes an Indonesian address: collapses whitespace,
// standardizes common abbreviations.
func Address(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// Collapse whitespace.
	fields := strings.Fields(s)
	s = strings.Join(fields, " ")

	// Standardize common Indonesian address abbreviations.
	replacements := map[string]string{
		"jl.":  "Jalan",
		"jl ":  "Jalan ",
		"gg.":  "Gang",
		"gg ":  "Gang ",
		"rt.":  "RT",
		"rw.":  "RW",
		"kel.": "Kelurahan",
		"kec.": "Kecamatan",
		"kab.": "Kabupaten",
		"prov.": "Provinsi",
		"no.":  "No.",
	}

	lower := strings.ToLower(s)
	for abbr, full := range replacements {
		if idx := strings.Index(lower, abbr); idx != -1 {
			s = s[:idx] + full + s[idx+len(abbr):]
			lower = strings.ToLower(s)
		}
	}

	return s
}

// CompanyName normalizes an Indonesian company name.
func CompanyName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// Collapse whitespace.
	fields := strings.Fields(s)

	// Uppercase common Indonesian company type suffixes.
	suffixes := map[string]string{
		"pt":     "PT",
		"pt.":    "PT",
		"cv":     "CV",
		"cv.":    "CV",
		"tbk":    "Tbk",
		"tbk.":   "Tbk",
	}

	for i, f := range fields {
		if replacement, ok := suffixes[strings.ToLower(f)]; ok {
			fields[i] = replacement
		} else {
			fields[i] = titleWord(f)
		}
	}

	return strings.Join(fields, " ")
}
