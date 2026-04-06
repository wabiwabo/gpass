package cursor

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCursor_EncodeDecodeRoundtrip(t *testing.T) {
	ts := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	c := Cursor{
		ID:        "uuid-123",
		Timestamp: &ts,
		Extra:     map[string]string{"sort": "created_at"},
	}

	encoded := c.Encode()
	if encoded == "" {
		t.Fatal("encoded cursor should not be empty")
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if decoded.ID != "uuid-123" {
		t.Errorf("ID: got %q", decoded.ID)
	}
	if decoded.Timestamp == nil || !decoded.Timestamp.Equal(ts) {
		t.Errorf("Timestamp: got %v, want %v", decoded.Timestamp, ts)
	}
	if decoded.Extra["sort"] != "created_at" {
		t.Errorf("Extra: got %v", decoded.Extra)
	}
}

func TestCursor_EncodeDecodeOffset(t *testing.T) {
	c := Cursor{Offset: 42}
	encoded := c.Encode()

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Offset != 42 {
		t.Errorf("Offset: got %d, want 42", decoded.Offset)
	}
}

func TestDecode_EmptyString(t *testing.T) {
	c, err := Decode("")
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != "" || c.Offset != 0 {
		t.Error("empty string should return zero cursor")
	}
}

func TestDecode_InvalidBase64(t *testing.T) {
	_, err := Decode("not-valid!!!")
	if err == nil {
		t.Error("should fail on invalid base64")
	}
}

func TestDecode_InvalidJSON(t *testing.T) {
	_, err := Decode("bm90LWpzb24") // "not-json" in base64
	if err == nil {
		t.Error("should fail on invalid JSON")
	}
}

type testItem struct {
	ID   string
	Name string
}

func TestNewPage_NoMore(t *testing.T) {
	items := []testItem{
		{ID: "1", Name: "Alice"},
		{ID: "2", Name: "Bob"},
	}

	page := NewPage(items, 10, func(item testItem) Cursor {
		return Cursor{ID: item.ID}
	})

	if len(page.Items) != 2 {
		t.Errorf("items: got %d", len(page.Items))
	}
	if page.HasMore {
		t.Error("should not have more")
	}
	if page.NextCursor != "" {
		t.Error("next cursor should be empty")
	}
}

func TestNewPage_HasMore(t *testing.T) {
	items := make([]testItem, 11) // limit=10, 11 items means has_more
	for i := range items {
		items[i] = testItem{ID: string(rune('a' + i))}
	}

	page := NewPage(items, 10, func(item testItem) Cursor {
		return Cursor{ID: item.ID}
	})

	if len(page.Items) != 10 {
		t.Errorf("items should be trimmed to 10, got %d", len(page.Items))
	}
	if !page.HasMore {
		t.Error("should have more")
	}
	if page.NextCursor == "" {
		t.Error("next cursor should be set")
	}

	// Verify cursor points to last visible item.
	c, _ := Decode(page.NextCursor)
	lastItem := page.Items[9]
	if c.ID != lastItem.ID {
		t.Errorf("cursor ID: got %q, want %q", c.ID, lastItem.ID)
	}
}

func TestNewPage_EmptyItems(t *testing.T) {
	page := NewPage([]testItem{}, 10, func(item testItem) Cursor {
		return Cursor{ID: item.ID}
	})

	if page.Items == nil {
		t.Error("items should not be nil")
	}
	if len(page.Items) != 0 {
		t.Error("items should be empty")
	}
	if page.HasMore {
		t.Error("should not have more")
	}
}

func TestNewPage_NilItems(t *testing.T) {
	page := NewPage[testItem](nil, 10, func(item testItem) Cursor {
		return Cursor{ID: item.ID}
	})

	if page.Items == nil {
		t.Error("nil items should be converted to empty slice")
	}
}

func TestParseRequest_Defaults(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	params, err := ParseRequest(req, 20, 100)
	if err != nil {
		t.Fatal(err)
	}
	if params.Limit != 20 {
		t.Errorf("limit: got %d, want 20", params.Limit)
	}
}

func TestParseRequest_WithLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/items?limit=50", nil)
	params, err := ParseRequest(req, 20, 100)
	if err != nil {
		t.Fatal(err)
	}
	if params.Limit != 50 {
		t.Errorf("limit: got %d, want 50", params.Limit)
	}
}

func TestParseRequest_MaxLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/items?limit=500", nil)
	params, err := ParseRequest(req, 20, 100)
	if err != nil {
		t.Fatal(err)
	}
	if params.Limit != 100 {
		t.Errorf("limit should be capped at 100, got %d", params.Limit)
	}
}

func TestParseRequest_MinLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/items?limit=-5", nil)
	params, err := ParseRequest(req, 20, 100)
	if err != nil {
		t.Fatal(err)
	}
	if params.Limit != 1 {
		t.Errorf("limit should be at least 1, got %d", params.Limit)
	}
}

func TestParseRequest_WithCursor(t *testing.T) {
	c := Cursor{ID: "item-42"}.Encode()
	req := httptest.NewRequest(http.MethodGet, "/api/items?cursor="+c, nil)
	params, err := ParseRequest(req, 20, 100)
	if err != nil {
		t.Fatal(err)
	}
	if params.Cursor.ID != "item-42" {
		t.Errorf("cursor ID: got %q", params.Cursor.ID)
	}
}

func TestParseRequest_InvalidCursor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/items?cursor=invalid!!!", nil)
	_, err := ParseRequest(req, 20, 100)
	if err == nil {
		t.Error("should fail on invalid cursor")
	}
}

func TestParseRequest_InvalidLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/items?limit=abc", nil)
	_, err := ParseRequest(req, 20, 100)
	if err == nil {
		t.Error("should fail on non-numeric limit")
	}
}
