// Package negotiate implements HTTP content negotiation per RFC 7231.
// It supports Accept, Accept-Encoding, and Accept-Language header parsing
// with quality value handling, wildcard matching, and specificity ordering.
package negotiate

import (
	"sort"
	"strconv"
	"strings"
)

// AcceptItem represents a single entry from an Accept, Accept-Encoding,
// or Accept-Language header, including its quality value and parameters.
type AcceptItem struct {
	MediaType string
	Quality   float64
	Params    map[string]string
}

// specificity returns a score for sorting: more specific types rank higher.
// 3 = exact type (e.g. text/html), 2 = wildcard subtype (e.g. text/*), 1 = wildcard all (*/*).
func specificity(mediaType string) int {
	if mediaType == "*/*" || mediaType == "*" {
		return 1
	}
	if strings.HasSuffix(mediaType, "/*") {
		return 2
	}
	return 3
}

// parseItems parses a comma-separated header value into AcceptItems.
func parseItems(header string) []AcceptItem {
	header = strings.TrimSpace(header)
	if header == "" {
		return nil
	}

	parts := strings.Split(header, ",")
	items := make([]AcceptItem, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		item := AcceptItem{
			Quality: 1.0,
			Params:  make(map[string]string),
		}

		// Split on semicolons to get type and parameters.
		segments := strings.Split(part, ";")
		item.MediaType = strings.TrimSpace(segments[0])

		for _, seg := range segments[1:] {
			seg = strings.TrimSpace(seg)
			if seg == "" {
				continue
			}
			eqIdx := strings.IndexByte(seg, '=')
			if eqIdx < 0 {
				continue
			}
			key := strings.TrimSpace(seg[:eqIdx])
			val := strings.TrimSpace(seg[eqIdx+1:])

			if strings.EqualFold(key, "q") {
				if q, err := strconv.ParseFloat(val, 64); err == nil {
					item.Quality = q
				}
			} else {
				item.Params[key] = val
			}
		}

		items = append(items, item)
	}

	// Sort by quality descending, then by specificity descending.
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Quality != items[j].Quality {
			return items[i].Quality > items[j].Quality
		}
		return specificity(items[i].MediaType) > specificity(items[j].MediaType)
	})

	return items
}

// ParseAccept parses an HTTP Accept header into a sorted slice of AcceptItems.
// Items are sorted by quality descending, then by specificity descending.
func ParseAccept(header string) []AcceptItem {
	return parseItems(header)
}

// ParseAcceptEncoding parses an HTTP Accept-Encoding header.
func ParseAcceptEncoding(header string) []AcceptItem {
	return parseItems(header)
}

// ParseAcceptLanguage parses an HTTP Accept-Language header.
func ParseAcceptLanguage(header string) []AcceptItem {
	return parseItems(header)
}

// mediaTypeMatches returns true if the accept pattern matches the offered type.
// Supports exact match, wildcard subtype (text/*), and full wildcard (*/*).
func mediaTypeMatches(pattern, offered string) bool {
	if pattern == "*/*" || pattern == "*" {
		return true
	}
	if pattern == offered {
		return true
	}
	// Check wildcard subtype: text/* matches text/html.
	if strings.HasSuffix(pattern, "/*") {
		prefix := pattern[:len(pattern)-1] // "text/"
		if strings.HasPrefix(offered, prefix) {
			return true
		}
	}
	return false
}

// BestMatch returns the best matching media type from offered based on the
// Accept header. Returns "" if no match is found.
func BestMatch(header string, offered []string) string {
	items := ParseAccept(header)
	if len(items) == 0 || len(offered) == 0 {
		return ""
	}

	// For each accept item (already sorted by quality desc, specificity desc),
	// find the first offered type that matches.
	type candidate struct {
		offeredType string
		quality     float64
		spec        int
		position    int // position in offered list for stable tie-breaking
	}

	var best *candidate

	for _, item := range items {
		if item.Quality == 0 {
			continue
		}
		for idx, o := range offered {
			if mediaTypeMatches(item.MediaType, o) {
				c := &candidate{
					offeredType: o,
					quality:     item.Quality,
					spec:        specificity(item.MediaType),
					position:    idx,
				}
				if best == nil {
					best = c
				} else if c.quality > best.quality {
					best = c
				} else if c.quality == best.quality && c.spec > best.spec {
					best = c
				} else if c.quality == best.quality && c.spec == best.spec && c.position < best.position {
					best = c
				}
			}
		}
	}

	if best == nil {
		return ""
	}
	return best.offeredType
}

// BestEncoding returns the best matching encoding from supported based on
// the Accept-Encoding header. Returns "" if no match is found.
func BestEncoding(header string, supported []string) string {
	items := ParseAcceptEncoding(header)
	if len(items) == 0 || len(supported) == 0 {
		return ""
	}

	for _, item := range items {
		if item.Quality == 0 {
			continue
		}
		for _, s := range supported {
			if strings.EqualFold(item.MediaType, s) || item.MediaType == "*" {
				return s
			}
		}
	}

	return ""
}

// BestLanguage returns the best matching language from supported based on
// the Accept-Language header. Returns "" if no match is found.
func BestLanguage(header string, supported []string) string {
	items := ParseAcceptLanguage(header)
	if len(items) == 0 || len(supported) == 0 {
		return ""
	}

	for _, item := range items {
		if item.Quality == 0 {
			continue
		}
		for _, s := range supported {
			if strings.EqualFold(item.MediaType, s) || item.MediaType == "*" {
				return s
			}
			// Prefix matching: "en" matches "en-US".
			if strings.Contains(s, "-") {
				prefix := strings.SplitN(s, "-", 2)[0]
				if strings.EqualFold(item.MediaType, prefix) {
					return s
				}
			}
			if strings.Contains(item.MediaType, "-") {
				prefix := strings.SplitN(item.MediaType, "-", 2)[0]
				if strings.EqualFold(prefix, s) {
					return s
				}
			}
		}
	}

	return ""
}
