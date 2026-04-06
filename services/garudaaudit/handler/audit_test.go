package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudaaudit/store"
)

func setupAuditHandler() (*AuditHandler, *store.InMemoryAuditStore) {
	s := store.NewInMemoryAuditStore()
	h := NewAuditHandler(s)
	return h, s
}

func TestIngestEvent_Success(t *testing.T) {
	h, _ := setupAuditHandler()

	body := `{
		"event_type": "identity.verified",
		"actor_id": "user-123",
		"actor_type": "USER",
		"action": "VERIFY",
		"service_name": "identity",
		"status": "SUCCESS"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.IngestEvent(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}

	var event store.AuditEvent
	if err := json.NewDecoder(w.Body).Decode(&event); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if event.ID == "" {
		t.Error("expected event ID in response")
	}
	if event.EventType != "identity.verified" {
		t.Errorf("expected event_type identity.verified, got %s", event.EventType)
	}
}

func TestIngestEvent_MissingRequiredFields(t *testing.T) {
	h, _ := setupAuditHandler()

	body := `{"actor_id": "user-123"}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.IngestEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestIngestEvent_InvalidJSON(t *testing.T) {
	h, _ := setupAuditHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/events", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.IngestEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestQueryEvents_WithFilters(t *testing.T) {
	h, s := setupAuditHandler()

	s.Append(&store.AuditEvent{
		EventType:   "identity.verified",
		ActorID:     "alice",
		Action:      "VERIFY",
		ServiceName: "identity",
	})
	s.Append(&store.AuditEvent{
		EventType:   "document.signed",
		ActorID:     "bob",
		Action:      "SIGN",
		ServiceName: "signing",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/events?actor_id=alice", nil)
	w := httptest.NewRecorder()

	h.QueryEvents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp queryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(resp.Events))
	}
	if resp.Events[0].ActorID != "alice" {
		t.Errorf("expected alice, got %s", resp.Events[0].ActorID)
	}
	if resp.Total != 1 {
		t.Errorf("expected total 1, got %d", resp.Total)
	}
}

func TestQueryEvents_WithPagination(t *testing.T) {
	h, s := setupAuditHandler()

	for i := 0; i < 10; i++ {
		s.Append(&store.AuditEvent{
			EventType: "test.event",
			ActorID:   "user",
			Action:    "CREATE",
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/events?limit=3&offset=0", nil)
	w := httptest.NewRecorder()

	h.QueryEvents(w, req)

	var resp queryResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Events) != 3 {
		t.Errorf("expected 3 events, got %d", len(resp.Events))
	}
	if resp.Total != 10 {
		t.Errorf("expected total 10, got %d", resp.Total)
	}
	if resp.Limit != 3 {
		t.Errorf("expected limit 3, got %d", resp.Limit)
	}
}

func TestGetEvent_Success(t *testing.T) {
	h, s := setupAuditHandler()

	event := &store.AuditEvent{
		EventType:   "identity.verified",
		ActorID:     "user-123",
		Action:      "VERIFY",
		ServiceName: "identity",
	}
	s.Append(event)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/audit/events/{id}", h.GetEvent)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/events/"+event.ID, nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var got store.AuditEvent
	json.NewDecoder(w.Body).Decode(&got)
	if got.ID != event.ID {
		t.Errorf("expected ID %s, got %s", event.ID, got.ID)
	}
}

func TestGetEvent_NotFound(t *testing.T) {
	h, _ := setupAuditHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/audit/events/{id}", h.GetEvent)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/events/nonexistent", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
