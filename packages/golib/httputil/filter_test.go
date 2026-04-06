package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseFilters_SearchFromTo(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?search=hello&from=2024-01-01&to=2024-12-31", nil)
	f := ParseFilters(r)

	if f.Search != "hello" {
		t.Errorf("Search = %q, want %q", f.Search, "hello")
	}
	if f.DateFrom == nil {
		t.Fatal("DateFrom is nil, want parsed date")
	}
	if f.DateFrom.Year() != 2024 || f.DateFrom.Month() != 1 || f.DateFrom.Day() != 1 {
		t.Errorf("DateFrom = %v, want 2024-01-01", f.DateFrom)
	}
	if f.DateTo == nil {
		t.Fatal("DateTo is nil, want parsed date")
	}
	if f.DateTo.Year() != 2024 || f.DateTo.Month() != 12 || f.DateTo.Day() != 31 {
		t.Errorf("DateTo = %v, want 2024-12-31", f.DateTo)
	}
}

func TestParseFilters_RFC3339Dates(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?from=2024-06-15T10:30:00Z&to=2024-06-20T18:00:00Z", nil)
	f := ParseFilters(r)

	if f.DateFrom == nil {
		t.Fatal("DateFrom is nil")
	}
	if f.DateFrom.Hour() != 10 || f.DateFrom.Minute() != 30 {
		t.Errorf("DateFrom = %v, want 10:30", f.DateFrom)
	}
}

func TestParseFilters_CustomFilters(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?status=ACTIVE&region=jakarta", nil)
	f := ParseFilters(r)

	if f.Filters["status"] != "ACTIVE" {
		t.Errorf("Filters[status] = %q, want %q", f.Filters["status"], "ACTIVE")
	}
	if f.Filters["region"] != "jakarta" {
		t.Errorf("Filters[region] = %q, want %q", f.Filters["region"], "jakarta")
	}
}

func TestParseFilters_ReservedParamsExcluded(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?page=2&page_size=10&sort_by=name&sort_dir=asc&search=test&from=2024-01-01&to=2024-12-31&status=ACTIVE", nil)
	f := ParseFilters(r)

	reserved := []string{"page", "page_size", "sort_by", "sort_dir", "search", "from", "to"}
	for _, key := range reserved {
		if _, ok := f.Filters[key]; ok {
			t.Errorf("Filters should not contain reserved param %q", key)
		}
	}
	if f.Filters["status"] != "ACTIVE" {
		t.Errorf("Filters[status] = %q, want %q", f.Filters["status"], "ACTIVE")
	}
}

func TestParseFilters_InvalidDateIgnored(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?from=not-a-date&to=also-bad", nil)
	f := ParseFilters(r)

	if f.DateFrom != nil {
		t.Errorf("DateFrom = %v, want nil (invalid date)", f.DateFrom)
	}
	if f.DateTo != nil {
		t.Errorf("DateTo = %v, want nil (invalid date)", f.DateTo)
	}
}

func TestFilterParams_MatchesSearch_CaseInsensitive(t *testing.T) {
	f := FilterParams{Search: "HELLO"}

	if !f.MatchesSearch("say hello world") {
		t.Error("MatchesSearch should match case-insensitively")
	}
	if !f.MatchesSearch("no match", "HELLO there") {
		t.Error("MatchesSearch should match any field")
	}
	if f.MatchesSearch("no match here", "nothing") {
		t.Error("MatchesSearch should not match when search term absent")
	}
}

func TestFilterParams_MatchesSearch_EmptyMatchesAll(t *testing.T) {
	f := FilterParams{Search: ""}

	if !f.MatchesSearch("anything") {
		t.Error("MatchesSearch with empty search should match all")
	}
	if !f.MatchesSearch() {
		t.Error("MatchesSearch with empty search and no fields should match")
	}
}

func TestFilterParams_InDateRange_Within(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	f := FilterParams{DateFrom: &from, DateTo: &to}

	mid := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	if !f.InDateRange(mid) {
		t.Error("InDateRange should return true for date within range")
	}

	// Inclusive boundaries
	if !f.InDateRange(from) {
		t.Error("InDateRange should return true for from boundary")
	}
	if !f.InDateRange(to) {
		t.Error("InDateRange should return true for to boundary")
	}
}

func TestFilterParams_InDateRange_Outside(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	f := FilterParams{DateFrom: &from, DateTo: &to}

	before := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)
	if f.InDateRange(before) {
		t.Error("InDateRange should return false for date before range")
	}

	after := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if f.InDateRange(after) {
		t.Error("InDateRange should return false for date after range")
	}
}

func TestFilterParams_InDateRange_OnlyFrom(t *testing.T) {
	from := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	f := FilterParams{DateFrom: &from}

	before := time.Date(2024, 5, 31, 0, 0, 0, 0, time.UTC)
	if f.InDateRange(before) {
		t.Error("InDateRange should return false for date before from")
	}

	after := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if !f.InDateRange(after) {
		t.Error("InDateRange with only from should return true for any date after from")
	}
}

func TestFilterParams_InDateRange_NoBounds(t *testing.T) {
	f := FilterParams{}

	any := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	if !f.InDateRange(any) {
		t.Error("InDateRange with no bounds should return true")
	}
}
