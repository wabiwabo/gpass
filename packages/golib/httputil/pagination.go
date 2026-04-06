package httputil

import (
	"math"
	"net/http"
	"strconv"
)

const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100
)

// PaginationParams parsed from query string.
type PaginationParams struct {
	Page     int    // 1-based (default 1)
	PageSize int    // default 20, max 100
	SortBy   string // field to sort by
	SortDir  string // "asc" or "desc" (default "desc")
}

// PaginatedResponse wraps list responses with pagination metadata.
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Pagination Pagination  `json:"pagination"`
}

// Pagination contains pagination metadata.
type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// ParsePagination extracts pagination params from the request query string.
// Enforces: page >= 1, page_size 1-100, sort_dir must be "asc" or "desc".
func ParsePagination(r *http.Request) PaginationParams {
	q := r.URL.Query()

	page := defaultPage
	if v, err := strconv.Atoi(q.Get("page")); err == nil {
		page = v
	}
	if page < 1 {
		page = 1
	}

	pageSize := defaultPageSize
	if v, err := strconv.Atoi(q.Get("page_size")); err == nil && v >= 1 {
		pageSize = v
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	sortDir := "desc"
	if d := q.Get("sort_dir"); d == "asc" || d == "desc" {
		sortDir = d
	}

	return PaginationParams{
		Page:     page,
		PageSize: pageSize,
		SortBy:   q.Get("sort_by"),
		SortDir:  sortDir,
	}
}

// NewPaginatedResponse creates a paginated response.
func NewPaginatedResponse(data interface{}, params PaginationParams, totalItems int64) PaginatedResponse {
	totalPages := 0
	if totalItems > 0 {
		totalPages = int(math.Ceil(float64(totalItems) / float64(params.PageSize)))
	}

	return PaginatedResponse{
		Data: data,
		Pagination: Pagination{
			Page:       params.Page,
			PageSize:   params.PageSize,
			TotalItems: totalItems,
			TotalPages: totalPages,
			HasNext:    params.Page < totalPages,
			HasPrev:    params.Page > 1,
		},
	}
}

// Offset returns the SQL-compatible offset for the current page.
func (p PaginationParams) Offset() int {
	return (p.Page - 1) * p.PageSize
}
