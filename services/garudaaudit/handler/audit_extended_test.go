package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudaaudit/store"
)

// TestQueryEvents_DateRangeFilters verifies that queries with from/to
// date range parameters correctly filter events by creation time.
func TestQueryEvents_DateRangeFilters(t *testing.T) {
	h, s := setupAuditHandler()

	// Insert events with controlled timestamps via sleep-free approach:
	// We'll append events and then verify the filter works with known boundaries.
	now := time.Now()

	// Append several events
	for i := 0; i < 5; i++ {
		s.Append(&store.AuditEvent{
			EventType:   "test.event",
			ActorID:     "user-time",
			Action:      "CREATE",
			ServiceName: "identity",
		})
	}

	// Query with from=1 minute ago, to=1 minute from now (should include all)
	from := now.Add(-1 * time.Minute).Format(time.RFC3339)
	to := now.Add(1 * time.Minute).Format(time.RFC3339)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/audit/events?from="+from+"&to="+to+"&actor_id=user-time", nil)
	w := httptest.NewRecorder()
	h.QueryEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp queryResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Events) != 5 {
		t.Errorf("expected 5 events in range, got %d", len(resp.Events))
	}

	// Query with from=1 hour from now (should return 0)
	futureFrom := now.Add(1 * time.Hour).Format(time.RFC3339)
	req2 := httptest.NewRequest(http.MethodGet,
		"/api/v1/audit/events?from="+futureFrom+"&actor_id=user-time", nil)
	w2 := httptest.NewRecorder()
	h.QueryEvents(w2, req2)

	var resp2 queryResponse
	json.NewDecoder(w2.Body).Decode(&resp2)
	if len(resp2.Events) != 0 {
		t.Errorf("expected 0 events after future from, got %d", len(resp2.Events))
	}

	// Query with to=1 hour ago (should return 0)
	pastTo := now.Add(-1 * time.Hour).Format(time.RFC3339)
	req3 := httptest.NewRequest(http.MethodGet,
		"/api/v1/audit/events?to="+pastTo+"&actor_id=user-time", nil)
	w3 := httptest.NewRecorder()
	h.QueryEvents(w3, req3)

	var resp3 queryResponse
	json.NewDecoder(w3.Body).Decode(&resp3)
	if len(resp3.Events) != 0 {
		t.Errorf("expected 0 events before past to, got %d", len(resp3.Events))
	}
}

// TestQueryEvents_CombinedFilters verifies that combining actor_id,
// action, and date range filters produces correct AND-logic results.
func TestQueryEvents_CombinedFilters(t *testing.T) {
	h, s := setupAuditHandler()

	now := time.Now()

	// Seed diverse events
	events := []struct {
		actorID string
		action  string
		service string
	}{
		{"alice", "VERIFY", "identity"},
		{"alice", "SIGN", "signing"},
		{"alice", "VERIFY", "identity"},
		{"bob", "VERIFY", "identity"},
		{"bob", "SIGN", "signing"},
		{"charlie", "DELETE", "admin"},
	}

	for _, e := range events {
		s.Append(&store.AuditEvent{
			EventType:   "test.combined",
			ActorID:     e.actorID,
			Action:      e.action,
			ServiceName: e.service,
		})
	}

	from := now.Add(-1 * time.Minute).Format(time.RFC3339)
	to := now.Add(1 * time.Minute).Format(time.RFC3339)

	// Query: alice + VERIFY + date range
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/audit/events?actor_id=alice&action=VERIFY&from="+from+"&to="+to, nil)
	w := httptest.NewRecorder()
	h.QueryEvents(w, req)

	var resp queryResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Events) != 2 {
		t.Errorf("expected 2 events for alice+VERIFY, got %d", len(resp.Events))
	}
	for _, e := range resp.Events {
		if e.ActorID != "alice" {
			t.Errorf("expected actor alice, got %s", e.ActorID)
		}
		if e.Action != "VERIFY" {
			t.Errorf("expected action VERIFY, got %s", e.Action)
		}
	}

	// Query: bob + SIGN
	req2 := httptest.NewRequest(http.MethodGet,
		"/api/v1/audit/events?actor_id=bob&action=SIGN", nil)
	w2 := httptest.NewRecorder()
	h.QueryEvents(w2, req2)

	var resp2 queryResponse
	json.NewDecoder(w2.Body).Decode(&resp2)

	if len(resp2.Events) != 1 {
		t.Errorf("expected 1 event for bob+SIGN, got %d", len(resp2.Events))
	}

	// Query: charlie + nonexistent action
	req3 := httptest.NewRequest(http.MethodGet,
		"/api/v1/audit/events?actor_id=charlie&action=NONEXISTENT", nil)
	w3 := httptest.NewRecorder()
	h.QueryEvents(w3, req3)

	var resp3 queryResponse
	json.NewDecoder(w3.Body).Decode(&resp3)

	if len(resp3.Events) != 0 {
		t.Errorf("expected 0 events for charlie+NONEXISTENT, got %d", len(resp3.Events))
	}
}

// TestStats_LargeDataset verifies that the stats endpoint correctly
// aggregates counts across a large number of events.
func TestStats_LargeDataset(t *testing.T) {
	s := store.NewInMemoryAuditStore()
	sh := NewStatsHandler(s)

	// Insert 200 events across multiple services and actions
	actions := []string{"VERIFY", "SIGN", "CREATE", "DELETE", "UPDATE"}
	services := []string{"identity", "signing", "garudacorp", "garudaportal"}
	statuses := []string{"SUCCESS", "FAILURE"}

	totalExpected := 0
	for i := 0; i < 200; i++ {
		s.Append(&store.AuditEvent{
			EventType:   "load.test",
			ActorID:     "loadtest-user",
			Action:      actions[i%len(actions)],
			ServiceName: services[i%len(services)],
			Status:      statuses[i%len(statuses)],
		})
		totalExpected++
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/stats", nil)
	w := httptest.NewRecorder()
	sh.GetStats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp statsResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.TotalEvents != int64(totalExpected) {
		t.Errorf("total = %d, want %d", resp.TotalEvents, totalExpected)
	}

	// Verify action distribution (200/5 = 40 each)
	for _, action := range actions {
		if resp.ByAction[action] != 40 {
			t.Errorf("action %s: got %d, want 40", action, resp.ByAction[action])
		}
	}

	// Verify service distribution (200/4 = 50 each)
	for _, svc := range services {
		if resp.ByService[svc] != 50 {
			t.Errorf("service %s: got %d, want 50", svc, resp.ByService[svc])
		}
	}

	// Verify status distribution (200/2 = 100 each)
	if resp.ByStatus["SUCCESS"] != 100 {
		t.Errorf("SUCCESS count: got %d, want 100", resp.ByStatus["SUCCESS"])
	}
	if resp.ByStatus["FAILURE"] != 100 {
		t.Errorf("FAILURE count: got %d, want 100", resp.ByStatus["FAILURE"])
	}
}

// TestIngestEvent_AllOptionalFields verifies that an event with all
// optional fields populated is stored and returned correctly.
func TestIngestEvent_AllOptionalFields(t *testing.T) {
	h, _ := setupAuditHandler()

	body := `{
		"event_type": "document.signed",
		"actor_id": "user-456",
		"actor_type": "SERVICE_ACCOUNT",
		"resource_id": "doc-789",
		"resource_type": "DOCUMENT",
		"action": "SIGN",
		"ip_address": "192.168.1.100",
		"user_agent": "GarudaPass-CLI/1.0",
		"service_name": "garudasign",
		"request_id": "req-abc-123",
		"status": "SUCCESS"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.IngestEvent(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var event store.AuditEvent
	json.NewDecoder(w.Body).Decode(&event)

	if event.ActorType != "SERVICE_ACCOUNT" {
		t.Errorf("actor_type = %q, want SERVICE_ACCOUNT", event.ActorType)
	}
	if event.ResourceID != "doc-789" {
		t.Errorf("resource_id = %q, want doc-789", event.ResourceID)
	}
	if event.ResourceType != "DOCUMENT" {
		t.Errorf("resource_type = %q, want DOCUMENT", event.ResourceType)
	}
	if event.IPAddress != "192.168.1.100" {
		t.Errorf("ip_address = %q, want 192.168.1.100", event.IPAddress)
	}
	if event.UserAgent != "GarudaPass-CLI/1.0" {
		t.Errorf("user_agent = %q, want GarudaPass-CLI/1.0", event.UserAgent)
	}
	if event.ServiceName != "garudasign" {
		t.Errorf("service_name = %q, want garudasign", event.ServiceName)
	}
	if event.RequestID != "req-abc-123" {
		t.Errorf("request_id = %q, want req-abc-123", event.RequestID)
	}
}

// TestIngestEvent_WithMetadata verifies that events with custom metadata
// fields are stored and retrievable.
func TestIngestEvent_WithMetadata(t *testing.T) {
	h, s := setupAuditHandler()

	body := `{
		"event_type": "signing.request.created",
		"actor_id": "user-meta",
		"action": "CREATE",
		"metadata": {
			"document_name": "contract.pdf",
			"document_size": "1048576",
			"signing_level": "PAdES-B-LTA",
			"request_source": "web-portal",
			"ip_geo": "ID-JK"
		}
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.IngestEvent(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var event store.AuditEvent
	json.NewDecoder(w.Body).Decode(&event)

	// Verify metadata was stored
	if event.Metadata["document_name"] != "contract.pdf" {
		t.Errorf("metadata[document_name] = %q, want contract.pdf", event.Metadata["document_name"])
	}
	if event.Metadata["signing_level"] != "PAdES-B-LTA" {
		t.Errorf("metadata[signing_level] = %q, want PAdES-B-LTA", event.Metadata["signing_level"])
	}
	if event.Metadata["ip_geo"] != "ID-JK" {
		t.Errorf("metadata[ip_geo] = %q, want ID-JK", event.Metadata["ip_geo"])
	}

	// Verify metadata survives retrieval from store
	retrieved, err := s.GetByID(event.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(retrieved.Metadata) != 5 {
		t.Errorf("retrieved metadata has %d entries, want 5", len(retrieved.Metadata))
	}
	if retrieved.Metadata["request_source"] != "web-portal" {
		t.Errorf("retrieved metadata[request_source] = %q, want web-portal", retrieved.Metadata["request_source"])
	}
}

// TestQueryEvents_ServiceFilter verifies filtering by service name.
func TestQueryEvents_ServiceFilter(t *testing.T) {
	h, s := setupAuditHandler()

	// Seed events from different services
	s.Append(&store.AuditEvent{
		EventType: "test.svc", ActorID: "u1", Action: "A", ServiceName: "identity",
	})
	s.Append(&store.AuditEvent{
		EventType: "test.svc", ActorID: "u2", Action: "B", ServiceName: "signing",
	})
	s.Append(&store.AuditEvent{
		EventType: "test.svc", ActorID: "u3", Action: "C", ServiceName: "identity",
	})

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/audit/events?service=identity", nil)
	w := httptest.NewRecorder()
	h.QueryEvents(w, req)

	var resp queryResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Events) != 2 {
		t.Errorf("expected 2 identity events, got %d", len(resp.Events))
	}
	if resp.Total != 2 {
		t.Errorf("expected total 2, got %d", resp.Total)
	}
}

// TestQueryEvents_EmptyResults verifies that queries with no matches
// return an empty array (not null) and total of 0.
func TestQueryEvents_EmptyResults(t *testing.T) {
	h, _ := setupAuditHandler()

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/audit/events?actor_id=nonexistent-actor", nil)
	w := httptest.NewRecorder()
	h.QueryEvents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp queryResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Total != 0 {
		t.Errorf("expected total 0, got %d", resp.Total)
	}
}

// TestStats_FilterByService verifies that stats can be filtered
// by service name.
func TestStats_FilterByService(t *testing.T) {
	s := store.NewInMemoryAuditStore()
	sh := NewStatsHandler(s)

	for i := 0; i < 10; i++ {
		s.Append(&store.AuditEvent{
			EventType: "test.stats", ActorID: "u1", Action: "VERIFY",
			ServiceName: "identity", Status: "SUCCESS",
		})
	}
	for i := 0; i < 5; i++ {
		s.Append(&store.AuditEvent{
			EventType: "test.stats", ActorID: "u2", Action: "SIGN",
			ServiceName: "signing", Status: "SUCCESS",
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/stats?service=identity", nil)
	w := httptest.NewRecorder()
	sh.GetStats(w, req)

	var resp statsResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.TotalEvents != 10 {
		t.Errorf("total = %d, want 10 (identity only)", resp.TotalEvents)
	}
	if resp.ByAction["VERIFY"] != 10 {
		t.Errorf("VERIFY count = %d, want 10", resp.ByAction["VERIFY"])
	}
	if resp.ByAction["SIGN"] != 0 {
		t.Errorf("SIGN count = %d, want 0 (filtered out)", resp.ByAction["SIGN"])
	}
}

// TestIngestEvent_DefaultsApplied verifies that default values are set
// for actor_type and status when not provided.
func TestIngestEvent_DefaultsApplied(t *testing.T) {
	h, _ := setupAuditHandler()

	body := `{
		"event_type": "test.defaults",
		"actor_id": "user-defaults",
		"action": "READ"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.IngestEvent(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	var event store.AuditEvent
	json.NewDecoder(w.Body).Decode(&event)

	if event.ActorType != "USER" {
		t.Errorf("default actor_type = %q, want USER", event.ActorType)
	}
	if event.Status != "SUCCESS" {
		t.Errorf("default status = %q, want SUCCESS", event.Status)
	}
	if event.Metadata == nil {
		t.Error("expected metadata to be initialized, got nil")
	}
}
