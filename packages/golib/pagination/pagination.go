package pagination

import (
	"math"
	"net/http"
	"strconv"
)

// Params holds pagination parameters from a request.
type Params struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Offset  int `json:"offset"`
}

// Meta holds pagination metadata for responses.
type Meta struct {
	Page       int  `json:"page"`
	PerPage    int  `json:"per_page"`
	TotalItems int  `json:"total_items"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// Config configures pagination defaults and limits.
type Config struct {
	DefaultPerPage int
	MaxPerPage     int
}

// DefaultConfig returns sensible pagination defaults.
func DefaultConfig() Config {
	return Config{
		DefaultPerPage: 20,
		MaxPerPage:     100,
	}
}

// Parse extracts pagination parameters from query string.
// ?page=1&per_page=20
func Parse(r *http.Request, cfg Config) Params {
	if cfg.DefaultPerPage <= 0 {
		cfg.DefaultPerPage = 20
	}
	if cfg.MaxPerPage <= 0 {
		cfg.MaxPerPage = 100
	}

	page := 1
	perPage := cfg.DefaultPerPage

	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}

	if v := r.URL.Query().Get("per_page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			perPage = n
		}
	}

	if perPage > cfg.MaxPerPage {
		perPage = cfg.MaxPerPage
	}

	return Params{
		Page:    page,
		PerPage: perPage,
		Offset:  (page - 1) * perPage,
	}
}

// NewMeta creates pagination metadata.
func NewMeta(page, perPage, totalItems int) Meta {
	totalPages := 0
	if totalItems > 0 && perPage > 0 {
		totalPages = int(math.Ceil(float64(totalItems) / float64(perPage)))
	}

	return Meta{
		Page:       page,
		PerPage:    perPage,
		TotalItems: totalItems,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
}

// Apply paginates a slice of items. Returns the page slice and total count.
func Apply[T any](items []T, params Params) ([]T, int) {
	total := len(items)
	if params.Offset >= total {
		return []T{}, total
	}

	end := params.Offset + params.PerPage
	if end > total {
		end = total
	}

	return items[params.Offset:end], total
}
