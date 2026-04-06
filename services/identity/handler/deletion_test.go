package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/garudapass/gpass/services/identity/store"
)

// mockAuditEmitter records emitted audit events for testing.
type mockAuditEmitter struct {
	events []auditEvent
}

type auditEvent struct {
	eventType  string
	userID     string
	resourceID string
	metadata   map[string]string
}

func (m *mockAuditEmitter) Emit(eventType, userID, resourceID string, metadata map[string]string) error {
	m.events = append(m.events, auditEvent{
		eventType:  eventType,
		userID:     userID,
		resourceID: resourceID,
		metadata:   metadata,
	})
	return nil
}

func newTestDeletionHandler() (*DeletionHandler, *store.InMemoryDeletionStore, *mockAuditEmitter) {
	s := store.NewInMemoryDeletionStore()
	ae := &mockAuditEmitter{}
	h := NewDeletionHandler(s, ae)
	return h, s, ae
}

func TestRequestDeletion_Success(t *testing.T) {
	h, _, ae := newTestDeletionHandler()

	body := `{"reason":"user_request"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/deletion", strings.NewReader(body))
	req.Header.Set("X-User-ID", "user-001")
	w := httptest.NewRecorder()

	h.RequestDeletion(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusAccepted)
	}

	var resp deletionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.ID == "" {
		t.Error("response ID is empty")
	}
	if resp.Status != "PENDING" {
		t.Errorf("status = %q, want %q", resp.Status, "PENDING")
	}
	if resp.RequestedAt.IsZero() {
		t.Error("RequestedAt is zero")
	}

	// Verify audit event was emitted (PP 71/2019 compliance).
	if len(ae.events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(ae.events))
	}
	if ae.events[0].eventType != "data_deletion.requested" {
		t.Errorf("audit event type = %q, want %q", ae.events[0].eventType, "data_deletion.requested")
	}
	if ae.events[0].userID != "user-001" {
		t.Errorf("audit event userID = %q, want %q", ae.events[0].userID, "user-001")
	}
	if ae.events[0].metadata["reason"] != "user_request" {
		t.Errorf("audit event reason = %q, want %q", ae.events[0].metadata["reason"], "user_request")
	}
}

func TestRequestDeletion_MissingUserID(t *testing.T) {
	h, _, _ := newTestDeletionHandler()

	body := `{"reason":"user_request"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/deletion", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.RequestDeletion(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "missing_user_id" {
		t.Errorf("error = %q, want %q", errResp["error"], "missing_user_id")
	}
}

func TestRequestDeletion_InvalidReason(t *testing.T) {
	h, _, _ := newTestDeletionHandler()

	body := `{"reason":"because_i_want_to"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/deletion", strings.NewReader(body))
	req.Header.Set("X-User-ID", "user-001")
	w := httptest.NewRecorder()

	h.RequestDeletion(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "invalid_reason" {
		t.Errorf("error = %q, want %q", errResp["error"], "invalid_reason")
	}
}

func TestGetDeletionStatus_Success(t *testing.T) {
	h, s, _ := newTestDeletionHandler()

	// Create a deletion request directly in the store.
	dr := &store.DeletionRequest{UserID: "user-001", Reason: "user_request"}
	if err := s.Create(dr); err != nil {
		t.Fatalf("store.Create() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/deletion/"+dr.ID, nil)
	req.Header.Set("X-User-ID", "user-001")
	req.SetPathValue("id", dr.ID)
	w := httptest.NewRecorder()

	h.GetDeletionStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp deletionStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.ID != dr.ID {
		t.Errorf("ID = %q, want %q", resp.ID, dr.ID)
	}
	if resp.UserID != "user-001" {
		t.Errorf("UserID = %q, want %q", resp.UserID, "user-001")
	}
	if resp.Status != "PENDING" {
		t.Errorf("Status = %q, want %q", resp.Status, "PENDING")
	}
}

func TestGetDeletionStatus_NotFound(t *testing.T) {
	h, _, _ := newTestDeletionHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/deletion/nonexistent", nil)
	req.Header.Set("X-User-ID", "user-001")
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.GetDeletionStatus(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestGetDeletionStatus_NotOwner(t *testing.T) {
	h, s, _ := newTestDeletionHandler()

	dr := &store.DeletionRequest{UserID: "user-001", Reason: "user_request"}
	if err := s.Create(dr); err != nil {
		t.Fatalf("store.Create() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/deletion/"+dr.ID, nil)
	req.Header.Set("X-User-ID", "user-002") // different user
	req.SetPathValue("id", dr.ID)
	w := httptest.NewRecorder()

	h.GetDeletionStatus(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "forbidden" {
		t.Errorf("error = %q, want %q", errResp["error"], "forbidden")
	}
}

func TestGetDeletionStatus_MissingUserID(t *testing.T) {
	h, _, _ := newTestDeletionHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/deletion/some-id", nil)
	req.SetPathValue("id", "some-id")
	w := httptest.NewRecorder()

	h.GetDeletionStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRequestDeletion_InvalidJSON(t *testing.T) {
	h, _, _ := newTestDeletionHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/deletion", strings.NewReader("not json"))
	req.Header.Set("X-User-ID", "user-001")
	w := httptest.NewRecorder()

	h.RequestDeletion(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
