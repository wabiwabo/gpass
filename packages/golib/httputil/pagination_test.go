package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParsePagination_Defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items", nil)
	p := ParsePagination(r)

	if p.Page != 1 {
		t.Errorf("Page = %d, want 1", p.Page)
	}
	if p.PageSize != 20 {
		t.Errorf("PageSize = %d, want 20", p.PageSize)
	}
	if p.SortDir != "desc" {
		t.Errorf("SortDir = %q, want %q", p.SortDir, "desc")
	}
	if p.SortBy != "" {
		t.Errorf("SortBy = %q, want empty", p.SortBy)
	}
}

func TestParsePagination_CustomValues(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?page=3&page_size=50&sort_by=name&sort_dir=asc", nil)
	p := ParsePagination(r)

	if p.Page != 3 {
		t.Errorf("Page = %d, want 3", p.Page)
	}
	if p.PageSize != 50 {
		t.Errorf("PageSize = %d, want 50", p.PageSize)
	}
	if p.SortBy != "name" {
		t.Errorf("SortBy = %q, want %q", p.SortBy, "name")
	}
	if p.SortDir != "asc" {
		t.Errorf("SortDir = %q, want %q", p.SortDir, "asc")
	}
}

func TestParsePagination_PageClampedToMin1(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?page=0", nil)
	p := ParsePagination(r)

	if p.Page != 1 {
		t.Errorf("Page = %d, want 1 (clamped)", p.Page)
	}

	r = httptest.NewRequest(http.MethodGet, "/items?page=-5", nil)
	p = ParsePagination(r)

	if p.Page != 1 {
		t.Errorf("Page = %d, want 1 (clamped from negative)", p.Page)
	}
}

func TestParsePagination_PageSizeClampedToMax100(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?page_size=200", nil)
	p := ParsePagination(r)

	if p.PageSize != 100 {
		t.Errorf("PageSize = %d, want 100 (clamped)", p.PageSize)
	}
}

func TestParsePagination_InvalidPageSizeUsesDefault(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?page_size=abc", nil)
	p := ParsePagination(r)

	if p.PageSize != 20 {
		t.Errorf("PageSize = %d, want 20 (default)", p.PageSize)
	}

	r = httptest.NewRequest(http.MethodGet, "/items?page_size=0", nil)
	p = ParsePagination(r)

	if p.PageSize != 20 {
		t.Errorf("PageSize = %d, want 20 (default for zero)", p.PageSize)
	}

	r = httptest.NewRequest(http.MethodGet, "/items?page_size=-1", nil)
	p = ParsePagination(r)

	if p.PageSize != 20 {
		t.Errorf("PageSize = %d, want 20 (default for negative)", p.PageSize)
	}
}

func TestParsePagination_InvalidSortDirUsesDefault(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?sort_dir=invalid", nil)
	p := ParsePagination(r)

	if p.SortDir != "desc" {
		t.Errorf("SortDir = %q, want %q (default)", p.SortDir, "desc")
	}
}

func TestPaginationParams_Offset(t *testing.T) {
	tests := []struct {
		name     string
		page     int
		pageSize int
		want     int
	}{
		{"page 1, size 20", 1, 20, 0},
		{"page 3, size 20", 3, 20, 40},
		{"page 2, size 50", 2, 50, 50},
		{"page 1, size 10", 1, 10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := PaginationParams{Page: tt.page, PageSize: tt.pageSize}
			if got := p.Offset(); got != tt.want {
				t.Errorf("Offset() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNewPaginatedResponse_TotalPages(t *testing.T) {
	params := PaginationParams{Page: 1, PageSize: 10}
	resp := NewPaginatedResponse([]string{"a", "b"}, params, 25)

	if resp.Pagination.TotalPages != 3 {
		t.Errorf("TotalPages = %d, want 3", resp.Pagination.TotalPages)
	}
	if resp.Pagination.TotalItems != 25 {
		t.Errorf("TotalItems = %d, want 25", resp.Pagination.TotalItems)
	}
}

func TestNewPaginatedResponse_HasNext(t *testing.T) {
	params := PaginationParams{Page: 1, PageSize: 10}
	resp := NewPaginatedResponse(nil, params, 25)

	if !resp.Pagination.HasNext {
		t.Error("HasNext = false, want true (page 1 of 3)")
	}
	if resp.Pagination.HasPrev {
		t.Error("HasPrev = true, want false (page 1)")
	}
}

func TestNewPaginatedResponse_HasPrev(t *testing.T) {
	params := PaginationParams{Page: 2, PageSize: 10}
	resp := NewPaginatedResponse(nil, params, 25)

	if !resp.Pagination.HasPrev {
		t.Error("HasPrev = false, want true (page 2)")
	}
	if !resp.Pagination.HasNext {
		t.Error("HasNext = false, want true (page 2 of 3)")
	}
}

func TestNewPaginatedResponse_LastPage(t *testing.T) {
	params := PaginationParams{Page: 3, PageSize: 10}
	resp := NewPaginatedResponse(nil, params, 25)

	if resp.Pagination.HasNext {
		t.Error("HasNext = true, want false (last page)")
	}
	if !resp.Pagination.HasPrev {
		t.Error("HasPrev = false, want true (page 3)")
	}
}

func TestNewPaginatedResponse_SinglePage(t *testing.T) {
	params := PaginationParams{Page: 1, PageSize: 20}
	resp := NewPaginatedResponse(nil, params, 5)

	if resp.Pagination.TotalPages != 1 {
		t.Errorf("TotalPages = %d, want 1", resp.Pagination.TotalPages)
	}
	if resp.Pagination.HasNext {
		t.Error("HasNext = true, want false (single page)")
	}
	if resp.Pagination.HasPrev {
		t.Error("HasPrev = true, want false (single page)")
	}
}

func TestNewPaginatedResponse_ZeroItems(t *testing.T) {
	params := PaginationParams{Page: 1, PageSize: 20}
	resp := NewPaginatedResponse(nil, params, 0)

	if resp.Pagination.TotalPages != 0 {
		t.Errorf("TotalPages = %d, want 0", resp.Pagination.TotalPages)
	}
	if resp.Pagination.HasNext {
		t.Error("HasNext = true, want false (zero items)")
	}
	if resp.Pagination.HasPrev {
		t.Error("HasPrev = true, want false (zero items)")
	}
}
