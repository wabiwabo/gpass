package negotiate

import (
	"testing"
)

func TestParseAccept_SingleType(t *testing.T) {
	items := ParseAccept("application/json")
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].MediaType != "application/json" {
		t.Errorf("expected application/json, got %s", items[0].MediaType)
	}
	if items[0].Quality != 1.0 {
		t.Errorf("expected quality 1.0, got %f", items[0].Quality)
	}
}

func TestParseAccept_MultipleWithQuality(t *testing.T) {
	items := ParseAccept("text/html, application/json;q=0.9, */*;q=0.1")
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	// Should be sorted by quality desc.
	if items[0].MediaType != "text/html" {
		t.Errorf("expected text/html first, got %s", items[0].MediaType)
	}
	if items[0].Quality != 1.0 {
		t.Errorf("expected quality 1.0 for text/html, got %f", items[0].Quality)
	}
	if items[1].MediaType != "application/json" {
		t.Errorf("expected application/json second, got %s", items[1].MediaType)
	}
	if items[1].Quality != 0.9 {
		t.Errorf("expected quality 0.9 for application/json, got %f", items[1].Quality)
	}
	if items[2].MediaType != "*/*" {
		t.Errorf("expected */* third, got %s", items[2].MediaType)
	}
	if items[2].Quality != 0.1 {
		t.Errorf("expected quality 0.1 for */*, got %f", items[2].Quality)
	}
}

func TestParseAccept_DefaultQuality(t *testing.T) {
	items := ParseAccept("text/html, application/json")
	for _, item := range items {
		if item.Quality != 1.0 {
			t.Errorf("expected default quality 1.0 for %s, got %f", item.MediaType, item.Quality)
		}
	}
}

func TestParseAccept_Empty(t *testing.T) {
	items := ParseAccept("")
	if len(items) != 0 {
		t.Errorf("expected 0 items for empty header, got %d", len(items))
	}

	items = ParseAccept("   ")
	if len(items) != 0 {
		t.Errorf("expected 0 items for whitespace header, got %d", len(items))
	}
}

func TestBestMatch_ExactMatch(t *testing.T) {
	result := BestMatch("application/json", []string{"text/html", "application/json"})
	if result != "application/json" {
		t.Errorf("expected application/json, got %s", result)
	}
}

func TestBestMatch_QualityOrdering(t *testing.T) {
	result := BestMatch("text/html;q=0.5, application/json;q=0.9", []string{"text/html", "application/json"})
	if result != "application/json" {
		t.Errorf("expected application/json (higher quality), got %s", result)
	}
}

func TestBestMatch_WildcardSubtype(t *testing.T) {
	result := BestMatch("text/*", []string{"text/html", "application/json"})
	if result != "text/html" {
		t.Errorf("expected text/html via text/*, got %s", result)
	}
}

func TestBestMatch_WildcardAll(t *testing.T) {
	result := BestMatch("*/*", []string{"application/xml", "text/html"})
	if result == "" {
		t.Error("expected a match with */*, got empty string")
	}
}

func TestBestMatch_NoMatch(t *testing.T) {
	result := BestMatch("image/png", []string{"text/html", "application/json"})
	if result != "" {
		t.Errorf("expected empty string for no match, got %s", result)
	}
}

func TestBestEncoding_Gzip(t *testing.T) {
	result := BestEncoding("gzip, deflate, br", []string{"gzip", "br"})
	if result != "gzip" {
		t.Errorf("expected gzip, got %s", result)
	}
}

func TestBestEncoding_Identity(t *testing.T) {
	result := BestEncoding("gzip;q=0.5, identity;q=1.0", []string{"identity", "gzip"})
	if result != "identity" {
		t.Errorf("expected identity (higher quality), got %s", result)
	}
}

func TestBestLanguage(t *testing.T) {
	result := BestLanguage("en-US, id;q=0.8, *;q=0.1", []string{"id", "en-US"})
	if result != "en-US" {
		t.Errorf("expected en-US, got %s", result)
	}

	// Test prefix matching.
	result = BestLanguage("en", []string{"en-US", "id"})
	if result != "en-US" {
		t.Errorf("expected en-US via prefix match, got %s", result)
	}
}

func TestParseAccept_Parameters(t *testing.T) {
	items := ParseAccept("text/html;charset=utf-8;q=0.9")
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].MediaType != "text/html" {
		t.Errorf("expected text/html, got %s", items[0].MediaType)
	}
	if items[0].Quality != 0.9 {
		t.Errorf("expected quality 0.9, got %f", items[0].Quality)
	}
	charset, ok := items[0].Params["charset"]
	if !ok {
		t.Fatal("expected charset param to be present")
	}
	if charset != "utf-8" {
		t.Errorf("expected charset=utf-8, got charset=%s", charset)
	}
}
