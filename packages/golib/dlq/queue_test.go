package dlq

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAddAndGet(t *testing.T) {
	q := New()

	id := q.Add(Entry{
		OriginalID: "evt-123",
		Type:       "webhook",
		Payload:    []byte(`{"url":"https://example.com"}`),
		Error:      "connection refused",
		Attempts:   3,
		Source:     "payment-service",
	})

	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	entry, err := q.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry.OriginalID != "evt-123" {
		t.Errorf("expected evt-123, got %s", entry.OriginalID)
	}
	if entry.Status != StatusPending {
		t.Errorf("expected PENDING, got %s", entry.Status)
	}
	if entry.Type != "webhook" {
		t.Errorf("expected webhook, got %s", entry.Type)
	}
}

func TestGetNotFound(t *testing.T) {
	q := New()
	_, err := q.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent entry")
	}
}

func TestListByStatus(t *testing.T) {
	q := New()

	q.Add(Entry{Type: "webhook", Error: "err1"})
	id2 := q.Add(Entry{Type: "event", Error: "err2"})
	q.Add(Entry{Type: "webhook", Error: "err3"})

	// Mark one as reviewed
	q.MarkReviewed(id2)

	pending := q.List(StatusPending, "", 0)
	if len(pending) != 2 {
		t.Errorf("expected 2 pending, got %d", len(pending))
	}

	reviewed := q.List(StatusReviewed, "", 0)
	if len(reviewed) != 1 {
		t.Errorf("expected 1 reviewed, got %d", len(reviewed))
	}
}

func TestListByType(t *testing.T) {
	q := New()

	q.Add(Entry{Type: "webhook", Error: "err1"})
	q.Add(Entry{Type: "event", Error: "err2"})
	q.Add(Entry{Type: "webhook", Error: "err3"})

	webhooks := q.List("", "webhook", 0)
	if len(webhooks) != 2 {
		t.Errorf("expected 2 webhooks, got %d", len(webhooks))
	}

	events := q.List("", "event", 0)
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
}

func TestMarkReviewed(t *testing.T) {
	q := New()
	id := q.Add(Entry{Type: "webhook", Error: "err"})

	if err := q.MarkReviewed(id); err != nil {
		t.Fatalf("MarkReviewed: %v", err)
	}

	entry, _ := q.Get(id)
	if entry.Status != StatusReviewed {
		t.Errorf("expected REVIEWED, got %s", entry.Status)
	}
	if entry.ReviewedAt == nil {
		t.Error("expected ReviewedAt to be set")
	}
}

func TestReplay(t *testing.T) {
	q := New()
	id := q.Add(Entry{Type: "event", Error: "timeout"})

	if err := q.Replay(id); err != nil {
		t.Fatalf("Replay: %v", err)
	}

	entry, _ := q.Get(id)
	if entry.Status != StatusReplayed {
		t.Errorf("expected REPLAYED, got %s", entry.Status)
	}
	if entry.ReplayedAt == nil {
		t.Error("expected ReplayedAt to be set")
	}
}

func TestDiscard(t *testing.T) {
	q := New()
	id := q.Add(Entry{Type: "notification", Error: "invalid payload"})

	if err := q.Discard(id); err != nil {
		t.Fatalf("Discard: %v", err)
	}

	entry, _ := q.Get(id)
	if entry.Status != StatusDiscarded {
		t.Errorf("expected DISCARDED, got %s", entry.Status)
	}
}

func TestCount(t *testing.T) {
	q := New()

	id1 := q.Add(Entry{Type: "webhook", Error: "err1"})
	q.Add(Entry{Type: "event", Error: "err2"})
	id3 := q.Add(Entry{Type: "webhook", Error: "err3"})

	q.MarkReviewed(id1)
	q.Discard(id3)

	counts := q.Count()
	if counts[StatusPending] != 1 {
		t.Errorf("expected 1 pending, got %d", counts[StatusPending])
	}
	if counts[StatusReviewed] != 1 {
		t.Errorf("expected 1 reviewed, got %d", counts[StatusReviewed])
	}
	if counts[StatusDiscarded] != 1 {
		t.Errorf("expected 1 discarded, got %d", counts[StatusDiscarded])
	}
}

func TestHandlerListJSON(t *testing.T) {
	q := New()
	q.Add(Entry{Type: "webhook", Error: "connection refused", Source: "svc-a"})
	q.Add(Entry{Type: "event", Error: "timeout", Source: "svc-b"})

	handler := q.Handler()

	req := httptest.NewRequest(http.MethodGet, "/internal/dlq", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}

	var entries []*Entry
	if err := json.Unmarshal(w.Body.Bytes(), &entries); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestHandlerGetEntry(t *testing.T) {
	q := New()
	id := q.Add(Entry{Type: "webhook", Error: "fail"})

	handler := q.Handler()

	req := httptest.NewRequest(http.MethodGet, "/internal/dlq/"+id, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var entry Entry
	if err := json.Unmarshal(w.Body.Bytes(), &entry); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if entry.ID != id {
		t.Errorf("expected ID %s, got %s", id, entry.ID)
	}
}

func TestHandlerGetNotFound(t *testing.T) {
	q := New()
	handler := q.Handler()

	req := httptest.NewRequest(http.MethodGet, "/internal/dlq/nonexistent", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandlerStats(t *testing.T) {
	q := New()
	q.Add(Entry{Type: "webhook", Error: "err1"})
	id := q.Add(Entry{Type: "event", Error: "err2"})
	q.MarkReviewed(id)

	handler := q.Handler()

	req := httptest.NewRequest(http.MethodGet, "/internal/dlq/stats", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var counts map[string]int
	if err := json.Unmarshal(w.Body.Bytes(), &counts); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if counts[StatusPending] != 1 {
		t.Errorf("expected 1 pending, got %d", counts[StatusPending])
	}
	if counts[StatusReviewed] != 1 {
		t.Errorf("expected 1 reviewed, got %d", counts[StatusReviewed])
	}
}

func TestHandlerReview(t *testing.T) {
	q := New()
	id := q.Add(Entry{Type: "webhook", Error: "err"})

	handler := q.Handler()

	req := httptest.NewRequest(http.MethodPost, "/internal/dlq/"+id+"/review", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	entry, _ := q.Get(id)
	if entry.Status != StatusReviewed {
		t.Errorf("expected REVIEWED, got %s", entry.Status)
	}
}
