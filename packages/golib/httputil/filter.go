package httputil

import (
	"net/http"
	"strings"
	"time"
)

// reservedParams are query parameters used by pagination and should be
// excluded from the Filters map.
var reservedParams = map[string]bool{
	"page":      true,
	"page_size": true,
	"sort_by":   true,
	"sort_dir":  true,
	"search":    true,
	"from":      true,
	"to":        true,
}

// FilterParams for search and filtering across list endpoints.
type FilterParams struct {
	Search  string            // free-text search term
	Filters map[string]string // key=value filters from query
	DateFrom *time.Time       // ?from= parsed
	DateTo   *time.Time       // ?to= parsed
}

// ParseFilters extracts filter params from query string.
// Recognized params: search, from, to. All other params become Filters.
// Reserved params (page, page_size, sort_by, sort_dir) are excluded from Filters.
func ParseFilters(r *http.Request) FilterParams {
	q := r.URL.Query()

	fp := FilterParams{
		Search:  q.Get("search"),
		Filters: make(map[string]string),
	}

	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			fp.DateFrom = &t
		} else if t, err := time.Parse("2006-01-02", v); err == nil {
			fp.DateFrom = &t
		}
	}

	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			fp.DateTo = &t
		} else if t, err := time.Parse("2006-01-02", v); err == nil {
			fp.DateTo = &t
		}
	}

	for key, values := range q {
		if reservedParams[key] {
			continue
		}
		if len(values) > 0 && values[0] != "" {
			fp.Filters[key] = values[0]
		}
	}

	return fp
}

// MatchesSearch returns true if any of the given fields contain the search
// term (case-insensitive). Returns true if the search term is empty.
func (f FilterParams) MatchesSearch(fields ...string) bool {
	if f.Search == "" {
		return true
	}
	lower := strings.ToLower(f.Search)
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), lower) {
			return true
		}
	}
	return false
}

// InDateRange returns true if t is within the from/to range (inclusive).
// If DateFrom is nil, there is no lower bound. If DateTo is nil, there is no upper bound.
func (f FilterParams) InDateRange(t time.Time) bool {
	if f.DateFrom != nil && t.Before(*f.DateFrom) {
		return false
	}
	if f.DateTo != nil && t.After(*f.DateTo) {
		return false
	}
	return true
}
