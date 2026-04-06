package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestChangelogHandler() *ChangelogHandler {
	return NewChangelogHandler(DefaultChangelog())
}

func TestChangelogHandler_GetAll(t *testing.T) {
	h := newTestChangelogHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/changelog", nil)
	w := httptest.NewRecorder()

	h.GetChangelog(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp changelogResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Total != len(DefaultChangelog()) {
		t.Errorf("expected %d total entries, got %d", len(DefaultChangelog()), resp.Total)
	}
	if len(resp.Entries) == 0 {
		t.Error("expected non-empty entries")
	}
}

func TestChangelogHandler_FilterByVersion(t *testing.T) {
	h := newTestChangelogHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/changelog?version=v1.1", nil)
	w := httptest.NewRecorder()

	h.GetChangelog(w, req)

	var resp changelogResponse
	json.NewDecoder(w.Body).Decode(&resp)

	for _, e := range resp.Entries {
		if e.Version != "v1.1.0" {
			t.Errorf("expected v1.1.x entries, got version %s", e.Version)
		}
	}
	if resp.Total == 0 {
		t.Error("expected some v1.1 entries")
	}
	// v1.1 has 3 entries.
	if resp.Total != 3 {
		t.Errorf("expected 3 v1.1 entries, got %d", resp.Total)
	}
}

func TestChangelogHandler_FilterByTypeBreaking(t *testing.T) {
	h := newTestChangelogHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/changelog?type=breaking", nil)
	w := httptest.NewRecorder()

	h.GetChangelog(w, req)

	var resp changelogResponse
	json.NewDecoder(w.Body).Decode(&resp)

	for _, e := range resp.Entries {
		if !e.Breaking {
			t.Errorf("expected only breaking entries, got non-breaking: %s", e.Description)
		}
	}
	if resp.Total == 0 {
		t.Error("expected some breaking entries")
	}
}

func TestChangelogHandler_FilterByChangeType(t *testing.T) {
	h := newTestChangelogHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/changelog?type=security", nil)
	w := httptest.NewRecorder()

	h.GetChangelog(w, req)

	var resp changelogResponse
	json.NewDecoder(w.Body).Decode(&resp)

	for _, e := range resp.Entries {
		if e.Type != "security" {
			t.Errorf("expected security type, got %s", e.Type)
		}
	}
	if resp.Total != 1 {
		t.Errorf("expected 1 security entry, got %d", resp.Total)
	}
}

func TestChangelogHandler_Pagination(t *testing.T) {
	h := newTestChangelogHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/changelog?per_page=3&page=1", nil)
	w := httptest.NewRecorder()

	h.GetChangelog(w, req)

	var resp changelogResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Entries) != 3 {
		t.Errorf("expected 3 entries on page 1, got %d", len(resp.Entries))
	}
	if resp.PerPage != 3 {
		t.Errorf("expected per_page=3, got %d", resp.PerPage)
	}
	if resp.Page != 1 {
		t.Errorf("expected page=1, got %d", resp.Page)
	}
	if resp.TotalPages < 2 {
		t.Errorf("expected multiple pages, got %d", resp.TotalPages)
	}

	// Page 2.
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/portal/changelog?per_page=3&page=2", nil)
	w2 := httptest.NewRecorder()
	h.GetChangelog(w2, req2)

	var resp2 changelogResponse
	json.NewDecoder(w2.Body).Decode(&resp2)

	if len(resp2.Entries) != 3 {
		t.Errorf("expected 3 entries on page 2, got %d", len(resp2.Entries))
	}
	if resp2.Page != 2 {
		t.Errorf("expected page=2, got %d", resp2.Page)
	}

	// Entries should be different.
	if resp.Entries[0].Description == resp2.Entries[0].Description {
		t.Error("page 1 and page 2 should have different entries")
	}
}

func TestChangelogHandler_PaginationBeyondEnd(t *testing.T) {
	h := newTestChangelogHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/changelog?page=999", nil)
	w := httptest.NewRecorder()

	h.GetChangelog(w, req)

	var resp changelogResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Entries) != 0 {
		t.Errorf("expected 0 entries for page beyond end, got %d", len(resp.Entries))
	}
}

func TestChangelogHandler_DefaultChangelogHasEntries(t *testing.T) {
	entries := DefaultChangelog()
	if len(entries) == 0 {
		t.Fatal("DefaultChangelog should have entries")
	}

	// Verify we have entries from all three versions.
	versions := make(map[string]bool)
	for _, e := range entries {
		versions[e.Version] = true
	}
	for _, v := range []string{"v1.0.0", "v1.1.0", "v1.2.0"} {
		if !versions[v] {
			t.Errorf("missing version %s in default changelog", v)
		}
	}
}

func TestChangelogHandler_EmptyResult(t *testing.T) {
	h := newTestChangelogHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/changelog?version=v99", nil)
	w := httptest.NewRecorder()

	h.GetChangelog(w, req)

	var resp changelogResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Total != 0 {
		t.Errorf("expected 0 total for non-existent version, got %d", resp.Total)
	}
	if resp.Entries == nil {
		t.Error("entries should be empty array, not null")
	}
}
