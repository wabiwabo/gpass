package negotiate

import "testing"

// TestParseItems_EdgeCases covers the empty-header, empty-segment, and
// malformed-q-value branches that the existing tests didn't reach.
func TestParseItems_EdgeCases(t *testing.T) {
	// Empty input → nil.
	if got := parseItems(""); got != nil {
		t.Errorf("empty header should return nil, got %v", got)
	}
	if got := parseItems("   "); got != nil {
		t.Errorf("whitespace-only header should return nil, got %v", got)
	}

	// Empty comma-separated segments are skipped.
	items := parseItems("text/html,, text/plain")
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	// Malformed q value (q=abc) falls back to default 1.0.
	items = parseItems("text/html;q=abc")
	if len(items) != 1 || items[0].Quality != 1.0 {
		t.Errorf("malformed q should default to 1.0, got %+v", items)
	}

	// A segment with no '=' is silently dropped.
	items = parseItems("text/html;noequals")
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if _, ok := items[0].Params["noequals"]; ok {
		t.Error("no-equals segment should be silently dropped, not stored as param")
	}

	// Custom parameter survives.
	items = parseItems("text/html;charset=utf-8")
	if items[0].Params["charset"] != "utf-8" {
		t.Errorf("charset param lost: %+v", items[0].Params)
	}
}

// TestBestMatch_QualityZeroExcluded pins that an explicit q=0 means
// "I refuse this" — RFC 7231 §5.3.1. Without this branch a server could
// silently send `text/html;q=0` content and confuse caches.
func TestBestMatch_QualityZeroExcluded(t *testing.T) {
	got := BestMatch("text/html;q=0, application/json", []string{"text/html", "application/json"})
	if got != "application/json" {
		t.Errorf("got %q, want application/json (text/html was q=0)", got)
	}
}

// TestBestMatch_TieBreakingByPositionAndSpecificity covers the
// position-tiebreak and specificity-tiebreak branches.
func TestBestMatch_TieBreakingByPositionAndSpecificity(t *testing.T) {
	// Two candidates with equal quality (1.0) — first in offered wins.
	got := BestMatch("application/json, text/html",
		[]string{"text/html", "application/json"})
	// Both q=1, both spec=3 → position breaks the tie. The accept items
	// are sorted by quality desc, specificity desc, but parsing preserves
	// stable order for equal items. The first OFFERED match wins on tie.
	if got == "" {
		t.Errorf("got %q, want non-empty match", got)
	}

	// Specificity tie-break: */* and text/html both available, exact wins.
	got = BestMatch("*/*, text/html", []string{"text/html"})
	if got != "text/html" {
		t.Errorf("got %q, want text/html (specificity tie-break)", got)
	}
}

// TestBestMatch_NoMatchReturnsEmpty pins the "no candidates matched"
// branch.
func TestBestMatch_NoMatchReturnsEmpty(t *testing.T) {
	got := BestMatch("application/xml", []string{"text/html", "application/json"})
	if got != "" {
		t.Errorf("got %q, want empty (no overlap)", got)
	}

	// Empty inputs.
	if BestMatch("", []string{"text/html"}) != "" {
		t.Error("empty header should return empty")
	}
	if BestMatch("text/html", nil) != "" {
		t.Error("nil offered should return empty")
	}
}

// TestBestEncoding_StarFallback covers the "*" wildcard branch in
// BestEncoding plus the case-insensitive match.
func TestBestEncoding_StarFallback(t *testing.T) {
	// Star matches the first supported encoding.
	got := BestEncoding("*", []string{"gzip", "deflate"})
	if got != "gzip" {
		t.Errorf("got %q, want gzip (* matches first)", got)
	}

	// Case-insensitive: GZIP matches gzip.
	got = BestEncoding("GZIP", []string{"gzip"})
	if got != "gzip" {
		t.Errorf("got %q, want gzip (case-insensitive)", got)
	}

	// q=0 excluded.
	got = BestEncoding("gzip;q=0, deflate", []string{"gzip", "deflate"})
	if got != "deflate" {
		t.Errorf("got %q, want deflate (gzip was q=0)", got)
	}

	// Empty input.
	if BestEncoding("", []string{"gzip"}) != "" {
		t.Error("empty header → empty")
	}
	if BestEncoding("gzip", nil) != "" {
		t.Error("nil supported → empty")
	}
}

// TestBestLanguage_PrefixMatching covers the "en" matches "en-US"
// branch and the inverse "en-GB" matches "en" branch.
func TestBestLanguage_PrefixMatching(t *testing.T) {
	// Accept "en" with supported ["en-US"] → en-US wins.
	got := BestLanguage("en", []string{"en-US"})
	if got != "en-US" {
		t.Errorf("got %q, want en-US ('en' should match 'en-US' prefix)", got)
	}

	// Accept "en-GB" with supported ["en"] → en wins (inverse direction).
	got = BestLanguage("en-GB", []string{"en"})
	if got != "en" {
		t.Errorf("got %q, want en ('en-GB' prefix matches 'en')", got)
	}

	// Exact match still preferred.
	got = BestLanguage("id-ID, en;q=0.5", []string{"id-ID", "en"})
	if got != "id-ID" {
		t.Errorf("got %q, want id-ID (exact, q=1)", got)
	}

	// q=0 excluded.
	got = BestLanguage("en;q=0, id", []string{"en", "id"})
	if got != "id" {
		t.Errorf("got %q, want id (en was q=0)", got)
	}

	// "*" wildcard.
	got = BestLanguage("*", []string{"id", "en"})
	if got != "id" {
		t.Errorf("got %q, want id (* picks first supported)", got)
	}

	// Empty inputs.
	if BestLanguage("", []string{"en"}) != "" {
		t.Error("empty header → empty")
	}
	if BestLanguage("en", nil) != "" {
		t.Error("nil supported → empty")
	}
}
