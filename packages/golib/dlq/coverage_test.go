package dlq

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func mustQueue(t *testing.T) (*Queue, string, string) {
	t.Helper()
	q := New()
	id1 := q.Add(Entry{Type: "webhook", Error: "5xx"})
	id2 := q.Add(Entry{Type: "email", Error: "bounce"})
	return q, id1, id2
}

// TestList_LimitAndFilters pins the limit short-circuit AND the
// status/type filter branches all in one table.
func TestList_LimitAndFilters(t *testing.T) {
	q := New()
	q.Add(Entry{Type: "a", Error: "x"})
	q.Add(Entry{Type: "a", Error: "y"})
	q.Add(Entry{Type: "b", Error: "z"})

	if got := q.List("", "", 1); len(got) != 1 {
		t.Errorf("limit=1: got %d, want 1", len(got))
	}
	if got := q.List("", "a", 0); len(got) != 2 {
		t.Errorf("type=a: got %d, want 2", len(got))
	}
	if got := q.List("", "nonexistent", 0); len(got) != 0 {
		t.Errorf("type=nonexistent: got %d, want 0", len(got))
	}
	if got := q.List(StatusPending, "", 0); len(got) != 3 {
		t.Errorf("pending: got %d, want 3", len(got))
	}
	if got := q.List(StatusReviewed, "", 0); len(got) != 0 {
		t.Errorf("reviewed: got %d, want 0", len(got))
	}
}

// TestHandler_AllRoutes pins every HTTP route in the DLQ admin handler:
// list, stats, get-by-id, review, replay, discard, plus 404 paths.
func TestHandler_AllRoutes(t *testing.T) {
	q, id1, _ := mustQueue(t)
	h := q.Handler()

	do := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	// GET list
	if rec := do("GET", "/internal/dlq"); rec.Code != 200 {
		t.Errorf("list: %d", rec.Code)
	}
	// GET list with query filters
	if rec := do("GET", "/internal/dlq?status=PENDING&type=webhook"); rec.Code != 200 {
		t.Errorf("filtered list: %d", rec.Code)
	}
	// GET stats
	if rec := do("GET", "/internal/dlq/stats"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "PENDING") {
		t.Errorf("stats: code=%d body=%s", rec.Code, rec.Body)
	}
	// GET by id
	if rec := do("GET", "/internal/dlq/"+id1); rec.Code != 200 {
		t.Errorf("get id: %d", rec.Code)
	}
	// GET unknown id → 404
	if rec := do("GET", "/internal/dlq/does-not-exist"); rec.Code != http.StatusNotFound {
		t.Errorf("get missing: %d", rec.Code)
	}
	// POST review
	if rec := do("POST", "/internal/dlq/"+id1+"/review"); rec.Code != 200 {
		t.Errorf("review: %d", rec.Code)
	}
	if rec := do("POST", "/internal/dlq/missing/review"); rec.Code != http.StatusNotFound {
		t.Errorf("review missing: %d", rec.Code)
	}
	// POST replay
	if rec := do("POST", "/internal/dlq/"+id1+"/replay"); rec.Code != 200 {
		t.Errorf("replay: %d", rec.Code)
	}
	if rec := do("POST", "/internal/dlq/missing/replay"); rec.Code != http.StatusNotFound {
		t.Errorf("replay missing: %d", rec.Code)
	}
	// POST discard
	if rec := do("POST", "/internal/dlq/"+id1+"/discard"); rec.Code != 200 {
		t.Errorf("discard: %d", rec.Code)
	}
	if rec := do("POST", "/internal/dlq/missing/discard"); rec.Code != http.StatusNotFound {
		t.Errorf("discard missing: %d", rec.Code)
	}
}

// TestHandler_EmptyListReturnsArray pins that List nil → empty JSON array
// (not "null"), avoiding a known bug class with JSON consumers.
func TestHandler_EmptyListReturnsArray(t *testing.T) {
	q := New()
	rec := httptest.NewRecorder()
	q.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/internal/dlq?type=nope", nil))
	body := strings.TrimSpace(rec.Body.String())
	if body != "[]" {
		t.Errorf("empty list body = %q, want []", body)
	}
}
