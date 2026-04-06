package cursor

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Cursor represents an opaque pagination cursor.
// It encodes position information without exposing internal IDs.
type Cursor struct {
	// ID is the last item's identifier (UUID, auto-increment, etc).
	ID string `json:"i,omitempty"`
	// Timestamp is the last item's sort timestamp.
	Timestamp *time.Time `json:"t,omitempty"`
	// Offset is a numeric position (for offset-based fallback).
	Offset int `json:"o,omitempty"`
	// Extra holds custom position data.
	Extra map[string]string `json:"x,omitempty"`
}

// Encode serializes the cursor to an opaque base64url string.
func (c Cursor) Encode() string {
	data, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(data)
}

// Decode parses an opaque cursor string back into a Cursor.
func Decode(s string) (Cursor, error) {
	if s == "" {
		return Cursor{}, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, fmt.Errorf("cursor: invalid encoding: %w", err)
	}
	var c Cursor
	if err := json.Unmarshal(data, &c); err != nil {
		return Cursor{}, fmt.Errorf("cursor: invalid format: %w", err)
	}
	return c, nil
}

// Page represents a paginated response with cursor information.
type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	PrevCursor string `json:"prev_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	TotalCount *int   `json:"total_count,omitempty"` // optional, expensive query
}

// NewPage creates a Page from items. If len(items) > limit, trims to limit and sets HasMore.
// cursorFn extracts a cursor from the last item.
func NewPage[T any](items []T, limit int, cursorFn func(T) Cursor) Page[T] {
	page := Page[T]{
		Items: items,
	}

	if len(items) > limit {
		page.Items = items[:limit]
		page.HasMore = true
	}

	if page.HasMore && len(page.Items) > 0 {
		last := page.Items[len(page.Items)-1]
		c := cursorFn(last)
		page.NextCursor = c.Encode()
	}

	if page.Items == nil {
		page.Items = make([]T, 0)
	}

	return page
}

// Params holds pagination parameters extracted from an HTTP request.
type Params struct {
	Cursor Cursor
	Limit  int
}

// ParseRequest extracts pagination parameters from query string.
// ?cursor=<opaque>&limit=20
func ParseRequest(r *http.Request, defaultLimit, maxLimit int) (Params, error) {
	params := Params{
		Limit: defaultLimit,
	}

	if cs := r.URL.Query().Get("cursor"); cs != "" {
		c, err := Decode(cs)
		if err != nil {
			return params, err
		}
		params.Cursor = c
	}

	if ls := r.URL.Query().Get("limit"); ls != "" {
		l, err := strconv.Atoi(ls)
		if err != nil {
			return params, fmt.Errorf("cursor: invalid limit: %w", err)
		}
		if l < 1 {
			l = 1
		}
		if l > maxLimit {
			l = maxLimit
		}
		params.Limit = l
	}

	return params, nil
}
