package pagination

import (
	"net/http/httptest"
	"testing"
)

func TestParse_Defaults(t *testing.T) {
	req := httptest.NewRequest("GET", "/items", nil)
	p := Parse(req, DefaultConfig())

	if p.Page != 1 {
		t.Errorf("page: got %d", p.Page)
	}
	if p.PerPage != 20 {
		t.Errorf("per_page: got %d", p.PerPage)
	}
	if p.Offset != 0 {
		t.Errorf("offset: got %d", p.Offset)
	}
}

func TestParse_CustomParams(t *testing.T) {
	req := httptest.NewRequest("GET", "/items?page=3&per_page=50", nil)
	p := Parse(req, DefaultConfig())

	if p.Page != 3 {
		t.Errorf("page: got %d", p.Page)
	}
	if p.PerPage != 50 {
		t.Errorf("per_page: got %d", p.PerPage)
	}
	if p.Offset != 100 {
		t.Errorf("offset: got %d, want 100", p.Offset)
	}
}

func TestParse_MaxPerPage(t *testing.T) {
	req := httptest.NewRequest("GET", "/items?per_page=500", nil)
	p := Parse(req, DefaultConfig())

	if p.PerPage != 100 {
		t.Errorf("per_page should be capped at 100: got %d", p.PerPage)
	}
}

func TestParse_InvalidPage(t *testing.T) {
	req := httptest.NewRequest("GET", "/items?page=-1", nil)
	p := Parse(req, DefaultConfig())

	if p.Page != 1 {
		t.Errorf("invalid page should default to 1: got %d", p.Page)
	}
}

func TestParse_NonNumeric(t *testing.T) {
	req := httptest.NewRequest("GET", "/items?page=abc&per_page=xyz", nil)
	p := Parse(req, DefaultConfig())

	if p.Page != 1 || p.PerPage != 20 {
		t.Errorf("non-numeric should use defaults: page=%d, per_page=%d", p.Page, p.PerPage)
	}
}

func TestNewMeta(t *testing.T) {
	m := NewMeta(2, 10, 25)

	if m.TotalPages != 3 {
		t.Errorf("totalPages: got %d, want 3", m.TotalPages)
	}
	if !m.HasNext {
		t.Error("page 2/3 should have next")
	}
	if !m.HasPrev {
		t.Error("page 2 should have prev")
	}
}

func TestNewMeta_FirstPage(t *testing.T) {
	m := NewMeta(1, 10, 25)
	if m.HasPrev {
		t.Error("first page should not have prev")
	}
	if !m.HasNext {
		t.Error("first page with more items should have next")
	}
}

func TestNewMeta_LastPage(t *testing.T) {
	m := NewMeta(3, 10, 25)
	if m.HasNext {
		t.Error("last page should not have next")
	}
	if !m.HasPrev {
		t.Error("last page should have prev")
	}
}

func TestNewMeta_SinglePage(t *testing.T) {
	m := NewMeta(1, 10, 5)
	if m.HasNext || m.HasPrev {
		t.Error("single page should have neither next nor prev")
	}
	if m.TotalPages != 1 {
		t.Errorf("totalPages: got %d", m.TotalPages)
	}
}

func TestNewMeta_Empty(t *testing.T) {
	m := NewMeta(1, 10, 0)
	if m.TotalPages != 0 {
		t.Errorf("empty: totalPages should be 0, got %d", m.TotalPages)
	}
}

func TestApply(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e", "f", "g"}

	// Page 1, 3 per page.
	page, total := Apply(items, Params{Page: 1, PerPage: 3, Offset: 0})
	if total != 7 {
		t.Errorf("total: got %d", total)
	}
	if len(page) != 3 {
		t.Errorf("page 1 items: got %d", len(page))
	}
	if page[0] != "a" || page[2] != "c" {
		t.Errorf("page 1: got %v", page)
	}

	// Page 3 (partial page).
	page, _ = Apply(items, Params{Page: 3, PerPage: 3, Offset: 6})
	if len(page) != 1 {
		t.Errorf("page 3 items: got %d", len(page))
	}
}

func TestApply_BeyondEnd(t *testing.T) {
	items := []string{"a", "b", "c"}
	page, total := Apply(items, Params{Page: 10, PerPage: 3, Offset: 27})

	if total != 3 {
		t.Errorf("total: got %d", total)
	}
	if len(page) != 0 {
		t.Error("beyond end should return empty")
	}
}

func TestApply_Empty(t *testing.T) {
	page, total := Apply([]int{}, Params{Page: 1, PerPage: 10, Offset: 0})
	if total != 0 || len(page) != 0 {
		t.Error("empty input should return empty")
	}
}
