package paginatedb

import (
	"strings"
	"testing"
)

func TestPageRequest_Normalize(t *testing.T) {
	p := PageRequest{Page: -1, PerPage: 500}
	p.Normalize(100)

	if p.Page != 1 {
		t.Errorf("page: got %d", p.Page)
	}
	if p.PerPage != 100 {
		t.Errorf("per_page: got %d", p.PerPage)
	}
	if p.SortDir != "ASC" {
		t.Errorf("sort_dir: got %q", p.SortDir)
	}
}

func TestPageRequest_NormalizeDefaults(t *testing.T) {
	p := PageRequest{}
	p.Normalize(0)

	if p.Page != 1 {
		t.Errorf("page: got %d", p.Page)
	}
	if p.PerPage != 20 {
		t.Errorf("per_page: got %d", p.PerPage)
	}
}

func TestPageRequest_NormalizeSortDir(t *testing.T) {
	p := PageRequest{SortDir: "desc"}
	p.Normalize(0)
	if p.SortDir != "DESC" {
		t.Errorf("sort_dir: got %q", p.SortDir)
	}

	p.SortDir = "invalid"
	p.Normalize(0)
	if p.SortDir != "ASC" {
		t.Errorf("invalid sort_dir: got %q", p.SortDir)
	}
}

func TestPageRequest_Offset(t *testing.T) {
	p := PageRequest{Page: 3, PerPage: 20}
	if p.Offset() != 40 {
		t.Errorf("offset: got %d", p.Offset())
	}
}

func TestPageRequest_SQLClause(t *testing.T) {
	p := PageRequest{Page: 2, PerPage: 10, SortDir: "ASC"}
	p.Normalize(0)

	clause := p.SQLClause("created_at")
	if !strings.Contains(clause, "ORDER BY created_at ASC") {
		t.Errorf("clause: got %q", clause)
	}
	if !strings.Contains(clause, "LIMIT 10") {
		t.Errorf("clause: got %q", clause)
	}
	if !strings.Contains(clause, "OFFSET 10") {
		t.Errorf("clause: got %q", clause)
	}
}

func TestPageRequest_SQLClause_NoSort(t *testing.T) {
	p := PageRequest{Page: 1, PerPage: 20}
	p.Normalize(0)

	clause := p.SQLClause("")
	if strings.Contains(clause, "ORDER BY") {
		t.Error("should not have ORDER BY without sort column")
	}
	if !strings.Contains(clause, "LIMIT 20") {
		t.Errorf("clause: got %q", clause)
	}
}

func TestNewPageResult(t *testing.T) {
	r := NewPageResult(2, 10, 25)

	if r.TotalPages != 3 {
		t.Errorf("total pages: got %d", r.TotalPages)
	}
	if !r.HasNext {
		t.Error("page 2/3 should have next")
	}
	if !r.HasPrev {
		t.Error("page 2 should have prev")
	}
}

func TestNewPageResult_FirstPage(t *testing.T) {
	r := NewPageResult(1, 10, 25)
	if r.HasPrev {
		t.Error("first page should not have prev")
	}
	if !r.HasNext {
		t.Error("should have next")
	}
}

func TestNewPageResult_LastPage(t *testing.T) {
	r := NewPageResult(3, 10, 25)
	if r.HasNext {
		t.Error("last page should not have next")
	}
	if !r.HasPrev {
		t.Error("should have prev")
	}
}

func TestNewPageResult_Empty(t *testing.T) {
	r := NewPageResult(1, 10, 0)
	if r.TotalPages != 0 {
		t.Errorf("total pages: got %d", r.TotalPages)
	}
	if r.HasNext || r.HasPrev {
		t.Error("empty should have neither")
	}
}

func TestKeysetRequest_Normalize(t *testing.T) {
	k := KeysetRequest{Limit: 500}
	k.Normalize(100)
	if k.Limit != 100 {
		t.Errorf("limit: got %d", k.Limit)
	}
	if k.SortDir != "ASC" {
		t.Errorf("sort_dir: got %q", k.SortDir)
	}
}

func TestKeysetRequest_WhereClause_After(t *testing.T) {
	k := KeysetRequest{After: "2024-01-01", SortDir: "ASC"}
	clause, arg := k.WhereClause("created_at")

	if !strings.Contains(clause, ">") {
		t.Errorf("ASC after should use >: got %q", clause)
	}
	if arg != "2024-01-01" {
		t.Errorf("arg: got %q", arg)
	}
}

func TestKeysetRequest_WhereClause_Before_DESC(t *testing.T) {
	k := KeysetRequest{Before: "2024-12-31", SortDir: "DESC"}
	clause, _ := k.WhereClause("created_at")

	if !strings.Contains(clause, ">") {
		t.Errorf("DESC before should use >: got %q", clause)
	}
}

func TestKeysetRequest_WhereClause_NoCursor(t *testing.T) {
	k := KeysetRequest{}
	clause, arg := k.WhereClause("id")
	if clause != "" || arg != "" {
		t.Error("no cursor should return empty")
	}
}
