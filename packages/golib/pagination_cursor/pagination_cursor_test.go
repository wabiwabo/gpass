package pagination_cursor

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestEncodeDecode(t *testing.T) {
	tests := []struct {
		name   string
		cursor Cursor
	}{
		{"minimal", Cursor{ID: "abc123"}},
		{"with_sort", Cursor{ID: "x1", SortField: "created_at", SortValue: "2024-01-01"}},
		{"with_direction_next", Cursor{ID: "y2", Direction: "next"}},
		{"with_direction_prev", Cursor{ID: "z3", Direction: "prev"}},
		{"full", Cursor{ID: "full1", SortField: "name", SortValue: "alice", Direction: "next"}},
		{"unicode_id", Cursor{ID: "日本語テスト"}},
		{"special_chars", Cursor{ID: "id+with/special=chars"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := Encode(tt.cursor)
			if encoded == "" {
				t.Fatal("Encode returned empty string")
			}
			decoded, err := Decode(encoded)
			if err != nil {
				t.Fatalf("Decode error: %v", err)
			}
			if decoded.ID != tt.cursor.ID {
				t.Errorf("ID: got %q, want %q", decoded.ID, tt.cursor.ID)
			}
			if decoded.SortField != tt.cursor.SortField {
				t.Errorf("SortField: got %q, want %q", decoded.SortField, tt.cursor.SortField)
			}
			if decoded.SortValue != tt.cursor.SortValue {
				t.Errorf("SortValue: got %q, want %q", decoded.SortValue, tt.cursor.SortValue)
			}
			if decoded.Direction != tt.cursor.Direction {
				t.Errorf("Direction: got %q, want %q", decoded.Direction, tt.cursor.Direction)
			}
		})
	}
}

func TestDecodeEmpty(t *testing.T) {
	c, err := Decode("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ID != "" {
		t.Errorf("expected empty cursor, got ID=%q", c.ID)
	}
}

func TestDecodeInvalidBase64(t *testing.T) {
	_, err := Decode("!!!not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
	if !strings.Contains(err.Error(), "invalid cursor") {
		t.Errorf("error should mention invalid cursor: %v", err)
	}
}

func TestDecodeMalformedJSON(t *testing.T) {
	encoded := base64.RawURLEncoding.EncodeToString([]byte("{bad json"))
	_, err := Decode(encoded)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "malformed cursor") {
		t.Errorf("error should mention malformed cursor: %v", err)
	}
}

func TestEncodeIsURLSafe(t *testing.T) {
	c := Cursor{ID: "test/with+special=chars&more"}
	encoded := Encode(c)
	if strings.ContainsAny(encoded, "+/=") {
		t.Errorf("encoded cursor contains URL-unsafe chars: %q", encoded)
	}
}

func TestEncodeIsValidBase64(t *testing.T) {
	c := Cursor{ID: "abc", SortField: "created_at", SortValue: "2024-01-01", Direction: "next"}
	encoded := Encode(c)
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("not valid base64: %v", err)
	}
	if !json.Valid(data) {
		t.Fatal("decoded data is not valid JSON")
	}
}

func TestNewPageUnderLimit(t *testing.T) {
	items := []string{"a", "b", "c"}
	page := NewPage(items, 10, func(s string) string { return s })

	if page.Count != 3 {
		t.Errorf("Count: got %d, want 3", page.Count)
	}
	if page.HasMore {
		t.Error("HasMore should be false")
	}
	if len(page.Items) != 3 {
		t.Errorf("Items length: got %d, want 3", len(page.Items))
	}
	if page.NextCursor != "" {
		t.Errorf("NextCursor should be empty, got %q", page.NextCursor)
	}
	if page.PrevCursor == "" {
		t.Error("PrevCursor should be set for non-empty items")
	}
}

func TestNewPageOverLimit(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e", "f"}
	page := NewPage(items, 5, func(s string) string { return s })

	if page.Count != 5 {
		t.Errorf("Count: got %d, want 5", page.Count)
	}
	if !page.HasMore {
		t.Error("HasMore should be true")
	}
	if len(page.Items) != 5 {
		t.Errorf("Items length: got %d, want 5", len(page.Items))
	}
	if page.NextCursor == "" {
		t.Error("NextCursor should be set")
	}
	// Verify next cursor points to last item in page
	nc, err := Decode(page.NextCursor)
	if err != nil {
		t.Fatalf("Decode NextCursor error: %v", err)
	}
	if nc.ID != "e" {
		t.Errorf("NextCursor ID: got %q, want %q", nc.ID, "e")
	}
	if nc.Direction != "next" {
		t.Errorf("NextCursor Direction: got %q, want %q", nc.Direction, "next")
	}
}

func TestNewPageExactLimit(t *testing.T) {
	items := []string{"a", "b", "c"}
	page := NewPage(items, 3, func(s string) string { return s })

	if page.HasMore {
		t.Error("HasMore should be false when items == limit")
	}
	if page.Count != 3 {
		t.Errorf("Count: got %d, want 3", page.Count)
	}
}

func TestNewPageEmpty(t *testing.T) {
	var items []string
	page := NewPage(items, 10, func(s string) string { return s })

	if page.Count != 0 {
		t.Errorf("Count: got %d, want 0", page.Count)
	}
	if page.HasMore {
		t.Error("HasMore should be false for empty items")
	}
	if page.PrevCursor != "" {
		t.Errorf("PrevCursor should be empty for empty items, got %q", page.PrevCursor)
	}
}

func TestNewPagePrevCursor(t *testing.T) {
	items := []string{"x", "y", "z"}
	page := NewPage(items, 10, func(s string) string { return s })

	pc, err := Decode(page.PrevCursor)
	if err != nil {
		t.Fatalf("Decode PrevCursor error: %v", err)
	}
	if pc.ID != "x" {
		t.Errorf("PrevCursor ID: got %q, want %q", pc.ID, "x")
	}
	if pc.Direction != "prev" {
		t.Errorf("PrevCursor Direction: got %q, want %q", pc.Direction, "prev")
	}
}

type item struct {
	ID   string
	Name string
}

func TestNewPageWithStruct(t *testing.T) {
	items := []item{
		{ID: "1", Name: "Alice"},
		{ID: "2", Name: "Bob"},
		{ID: "3", Name: "Charlie"},
		{ID: "4", Name: "Dave"},
	}
	page := NewPage(items, 3, func(i item) string { return i.ID })

	if page.Count != 3 {
		t.Errorf("Count: got %d, want 3", page.Count)
	}
	if !page.HasMore {
		t.Error("HasMore should be true")
	}
	nc, err := Decode(page.NextCursor)
	if err != nil {
		t.Fatalf("Decode NextCursor error: %v", err)
	}
	if nc.ID != "3" {
		t.Errorf("NextCursor ID: got %q, want %q", nc.ID, "3")
	}
}

func TestCursorIsEmpty(t *testing.T) {
	tests := []struct {
		name   string
		cursor Cursor
		want   bool
	}{
		{"empty", Cursor{}, true},
		{"with_id", Cursor{ID: "abc"}, false},
		{"only_direction", Cursor{Direction: "next"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cursor.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCursorIsNext(t *testing.T) {
	tests := []struct {
		name   string
		cursor Cursor
		want   bool
	}{
		{"empty_direction", Cursor{ID: "a"}, true},
		{"next", Cursor{ID: "a", Direction: "next"}, true},
		{"prev", Cursor{ID: "a", Direction: "prev"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cursor.IsNext(); got != tt.want {
				t.Errorf("IsNext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCursorIsPrev(t *testing.T) {
	tests := []struct {
		name   string
		cursor Cursor
		want   bool
	}{
		{"empty_direction", Cursor{ID: "a"}, false},
		{"next", Cursor{ID: "a", Direction: "next"}, false},
		{"prev", Cursor{ID: "a", Direction: "prev"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cursor.IsPrev(); got != tt.want {
				t.Errorf("IsPrev() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPageJSONSerialization(t *testing.T) {
	items := []string{"a", "b"}
	page := NewPage(items, 10, func(s string) string { return s })

	data, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded Page[string]
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.Count != page.Count {
		t.Errorf("Count: got %d, want %d", decoded.Count, page.Count)
	}
	if decoded.HasMore != page.HasMore {
		t.Errorf("HasMore: got %v, want %v", decoded.HasMore, page.HasMore)
	}
	if len(decoded.Items) != len(page.Items) {
		t.Errorf("Items length: got %d, want %d", len(decoded.Items), len(page.Items))
	}
}

func TestRoundTripStability(t *testing.T) {
	original := Cursor{ID: "stable-test", SortField: "ts", SortValue: "12345", Direction: "next"}
	encoded1 := Encode(original)
	decoded1, err := Decode(encoded1)
	if err != nil {
		t.Fatalf("first decode: %v", err)
	}
	encoded2 := Encode(decoded1)
	if encoded1 != encoded2 {
		t.Errorf("round-trip unstable: %q != %q", encoded1, encoded2)
	}
}
