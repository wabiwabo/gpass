package dlq

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// seedQueue creates a queue with three entries in distinct states so the
// handler tests can exercise filtering, lookup, and state transitions
// against a realistic fixture.
func seedQueue(t *testing.T) (*Queue, []string) {
	t.Helper()
	q := New()
	ids := []string{
		q.Add(Entry{Type: "webhook", Source: "garudaportal", Error: "503", Payload: []byte(`{}`)}),
		q.Add(Entry{Type: "webhook", Source: "garudaportal", Error: "timeout", Payload: []byte(`{}`)}),
		q.Add(Entry{Type: "notification", Source: "garudanotify", Error: "smtp", Payload: []byte(`{}`)}),
	}
	return q, ids
}

func doReq(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// TestHandler_ListAndFilter exercises GET /internal/dlq with and without
// query-string filters. Closes the bulk of the 50.9% gap on Handler.
func TestHandler_ListAndFilter(t *testing.T) {
	q, _ := seedQueue(t)
	h := q.Handler()

	// No filter → 3 entries.
	w := doReq(t, h, http.MethodGet, "/internal/dlq")
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var all []*Entry
	if err := json.Unmarshal(w.Body.Bytes(), &all); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("got %d entries, want 3", len(all))
	}

	// type=webhook → 2 entries.
	w = doReq(t, h, http.MethodGet, "/internal/dlq?type=webhook")
	var webhooks []*Entry
	_ = json.Unmarshal(w.Body.Bytes(), &webhooks)
	if len(webhooks) != 2 {
		t.Errorf("type=webhook: got %d, want 2", len(webhooks))
	}

	// status=DISCARDED → 0 entries (none discarded yet), but the response
	// must be `[]`, not `null`.
	w = doReq(t, h, http.MethodGet, "/internal/dlq?status=DISCARDED")
	body := strings.TrimSpace(w.Body.String())
	if body != `[]` {
		t.Errorf("empty list = %q, want []", body)
	}
}

// TestHandler_StatsCountsByStatus covers GET /internal/dlq/stats.
func TestHandler_StatsCountsByStatus(t *testing.T) {
	q, ids := seedQueue(t)
	if err := q.MarkReviewed(ids[0]); err != nil {
		t.Fatal(err)
	}
	h := q.Handler()
	w := doReq(t, h, http.MethodGet, "/internal/dlq/stats")
	var counts map[string]int
	if err := json.Unmarshal(w.Body.Bytes(), &counts); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if counts[StatusPending] != 2 {
		t.Errorf("PENDING count = %d, want 2", counts[StatusPending])
	}
	if counts[StatusReviewed] != 1 {
		t.Errorf("REVIEWED count = %d, want 1", counts[StatusReviewed])
	}
}

// TestHandler_GetByID covers the path-value lookup for a single entry,
// including the 404 branch.
func TestHandler_GetByID(t *testing.T) {
	q, ids := seedQueue(t)
	h := q.Handler()

	w := doReq(t, h, http.MethodGet, "/internal/dlq/"+ids[0])
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var got Entry
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != ids[0] || got.Type != "webhook" {
		t.Errorf("wrong entry: %+v", got)
	}

	w = doReq(t, h, http.MethodGet, "/internal/dlq/dlq-999")
	if w.Code != http.StatusNotFound {
		t.Errorf("missing id status = %d, want 404", w.Code)
	}
}

// TestHandler_StateTransitions runs each POST endpoint and asserts the
// underlying entry status changes accordingly.
func TestHandler_StateTransitions(t *testing.T) {
	q, ids := seedQueue(t)
	h := q.Handler()

	cases := []struct {
		path   string
		id     string
		want   string
	}{
		{"/internal/dlq/" + ids[0] + "/review", ids[0], StatusReviewed},
		{"/internal/dlq/" + ids[1] + "/replay", ids[1], StatusReplayed},
		{"/internal/dlq/" + ids[2] + "/discard", ids[2], StatusDiscarded},
	}
	for _, tc := range cases {
		w := doReq(t, h, http.MethodPost, tc.path)
		if w.Code != 200 {
			t.Errorf("%s: status = %d", tc.path, w.Code)
		}
		entry, err := q.Get(tc.id)
		if err != nil {
			t.Fatalf("%s: get after transition: %v", tc.path, err)
		}
		if entry.Status != tc.want {
			t.Errorf("%s: status = %q, want %q", tc.path, entry.Status, tc.want)
		}
	}

	// 404 branches: each transition handler must reject unknown IDs.
	for _, suffix := range []string{"/review", "/replay", "/discard"} {
		w := doReq(t, h, http.MethodPost, "/internal/dlq/dlq-9999"+suffix)
		if w.Code != http.StatusNotFound {
			t.Errorf("POST %s on missing id: status = %d, want 404", suffix, w.Code)
		}
	}
}
