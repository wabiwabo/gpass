package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudaaudit/store"
)

func TestGetStats_WithData(t *testing.T) {
	s := store.NewInMemoryAuditStore()
	h := NewStatsHandler(s)

	s.Append(&store.AuditEvent{
		EventType:   "identity.verified",
		ActorID:     "user-1",
		Action:      "CREATE",
		ServiceName: "identity",
		Status:      "SUCCESS",
	})
	s.Append(&store.AuditEvent{
		EventType:   "document.signed",
		ActorID:     "user-2",
		Action:      "SIGN",
		ServiceName: "signing",
		Status:      "SUCCESS",
	})
	s.Append(&store.AuditEvent{
		EventType:   "identity.failed",
		ActorID:     "user-3",
		Action:      "CREATE",
		ServiceName: "identity",
		Status:      "FAILURE",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/stats", nil)
	w := httptest.NewRecorder()

	h.GetStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp statsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.TotalEvents != 3 {
		t.Errorf("expected 3 total events, got %d", resp.TotalEvents)
	}
	if resp.ByAction["CREATE"] != 2 {
		t.Errorf("expected 2 CREATE actions, got %d", resp.ByAction["CREATE"])
	}
	if resp.ByAction["SIGN"] != 1 {
		t.Errorf("expected 1 SIGN action, got %d", resp.ByAction["SIGN"])
	}
	if resp.ByService["identity"] != 2 {
		t.Errorf("expected 2 identity events, got %d", resp.ByService["identity"])
	}
	if resp.ByStatus["SUCCESS"] != 2 {
		t.Errorf("expected 2 SUCCESS, got %d", resp.ByStatus["SUCCESS"])
	}
	if resp.ByStatus["FAILURE"] != 1 {
		t.Errorf("expected 1 FAILURE, got %d", resp.ByStatus["FAILURE"])
	}
}

func TestGetStats_Empty(t *testing.T) {
	s := store.NewInMemoryAuditStore()
	h := NewStatsHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/stats", nil)
	w := httptest.NewRecorder()

	h.GetStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp statsResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.TotalEvents != 0 {
		t.Errorf("expected 0 total events, got %d", resp.TotalEvents)
	}
}

func TestGetStats_WithServiceFilter(t *testing.T) {
	s := store.NewInMemoryAuditStore()
	h := NewStatsHandler(s)

	s.Append(&store.AuditEvent{
		EventType:   "identity.verified",
		ActorID:     "user-1",
		Action:      "CREATE",
		ServiceName: "identity",
	})
	s.Append(&store.AuditEvent{
		EventType:   "document.signed",
		ActorID:     "user-2",
		Action:      "SIGN",
		ServiceName: "signing",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/stats?service=identity", nil)
	w := httptest.NewRecorder()

	h.GetStats(w, req)

	var resp statsResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.TotalEvents != 1 {
		t.Errorf("expected 1 total event for identity service, got %d", resp.TotalEvents)
	}
	if resp.ByService["identity"] != 1 {
		t.Errorf("expected 1 identity event, got %d", resp.ByService["identity"])
	}
	if _, ok := resp.ByService["signing"]; ok {
		t.Error("expected no signing events in filtered stats")
	}
}
