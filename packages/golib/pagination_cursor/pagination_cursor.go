// Package pagination_cursor provides cursor-based pagination for
// APIs. More efficient than offset pagination for large datasets,
// using encoded cursors for stable page references.
package pagination_cursor

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// Cursor holds pagination state.
type Cursor struct {
	ID        string `json:"id"`
	SortField string `json:"sf,omitempty"`
	SortValue string `json:"sv,omitempty"`
	Direction string `json:"d,omitempty"` // "next" or "prev"
}

// Encode serializes a cursor to a URL-safe string.
func Encode(c Cursor) string {
	data, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(data)
}

// Decode deserializes a cursor string.
func Decode(s string) (Cursor, error) {
	if s == "" {
		return Cursor{}, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, fmt.Errorf("pagination_cursor: invalid cursor: %w", err)
	}
	var c Cursor
	if err := json.Unmarshal(data, &c); err != nil {
		return Cursor{}, fmt.Errorf("pagination_cursor: malformed cursor: %w", err)
	}
	return c, nil
}

// Page represents a page of results.
type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	PrevCursor string `json:"prev_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	Count      int    `json:"count"`
}

// NewPage creates a page from items with cursor generation.
func NewPage[T any](items []T, limit int, idFunc func(T) string) Page[T] {
	page := Page[T]{
		Items: items,
		Count: len(items),
	}

	if len(items) > limit {
		page.Items = items[:limit]
		page.Count = limit
		page.HasMore = true
		last := items[limit-1]
		page.NextCursor = Encode(Cursor{ID: idFunc(last), Direction: "next"})
	}

	if len(items) > 0 {
		first := items[0]
		page.PrevCursor = Encode(Cursor{ID: idFunc(first), Direction: "prev"})
	}

	return page
}

// IsEmpty checks if the cursor is empty (first page request).
func (c Cursor) IsEmpty() bool {
	return c.ID == ""
}

// IsNext checks if this is a forward pagination request.
func (c Cursor) IsNext() bool {
	return c.Direction == "next" || c.Direction == ""
}

// IsPrev checks if this is a backward pagination request.
func (c Cursor) IsPrev() bool {
	return c.Direction == "prev"
}
