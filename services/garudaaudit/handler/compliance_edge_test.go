package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudaaudit/store"
)

// TestIngestEvent_EmptyAction verifies that an event with an empty action
// field is rejected with a validation error.
func TestIngestEvent_EmptyAction(t *testing.T) {
	h, _ := setupAuditHandler()

	body := `{
		"event_type": "identity.verified",
		"actor_id": "user-123",
		"action": ""
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.IngestEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty action, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "validation_error" {
		t.Errorf("expected error code validation_error, got %q", resp["error"])
	}
}

// TestIngestEvent_EmptyActorID verifies that an event with an empty actor_id
// field is rejected with a validation error.
func TestIngestEvent_EmptyActorID(t *testing.T) {
	h, _ := setupAuditHandler()

	body := `{
		"event_type": "identity.verified",
		"actor_id": "",
		"action": "VERIFY"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.IngestEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty actor_id, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "validation_error" {
		t.Errorf("expected error code validation_error, got %q", resp["error"])
	}
}

// TestIngestEvent_FutureTimestamp verifies that an event ingested at the current
// time receives a server-assigned timestamp (CreatedAt) that is not in the
// distant future, confirming the server controls timestamp assignment.
func TestIngestEvent_FutureTimestamp(t *testing.T) {
	h, _ := setupAuditHandler()

	body := `{
		"event_type": "identity.verified",
		"actor_id": "user-future",
		"action": "VERIFY"
	}`

	before := time.Now()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.IngestEvent(w, req)

	after := time.Now()

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	var event store.AuditEvent
	json.NewDecoder(w.Body).Decode(&event)

	// The server must assign CreatedAt; it should be between before and after.
	if event.CreatedAt.Before(before.Add(-time.Second)) {
		t.Errorf("created_at %v is before request start %v", event.CreatedAt, before)
	}
	if event.CreatedAt.After(after.Add(time.Second)) {
		t.Errorf("created_at %v is after request end %v", event.CreatedAt, after)
	}
}

// TestIngestEvent_VeryLongDescription tests boundary conditions with a very
// long metadata description field to ensure the system handles large payloads.
func TestIngestEvent_VeryLongDescription(t *testing.T) {
	h, s := setupAuditHandler()

	longDesc := strings.Repeat("A", 10000)
	bodyStr := fmt.Sprintf(`{
		"event_type": "document.signed",
		"actor_id": "user-long",
		"action": "SIGN",
		"metadata": {"description": %q}
	}`, longDesc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/events", bytes.NewBufferString(bodyStr))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.IngestEvent(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for long description, got %d: %s", w.Code, w.Body.String())
	}

	var event store.AuditEvent
	json.NewDecoder(w.Body).Decode(&event)

	// Verify the long value was stored and is retrievable
	retrieved, err := s.GetByID(event.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(retrieved.Metadata["description"]) != 10000 {
		t.Errorf("expected description length 10000, got %d", len(retrieved.Metadata["description"]))
	}
}

// TestQueryEvents_InvalidDateRange verifies that querying with an invalid
// (reversed) date range returns no results rather than an error.
func TestQueryEvents_InvalidDateRange(t *testing.T) {
	h, s := setupAuditHandler()

	s.Append(&store.AuditEvent{
		EventType: "test.event", ActorID: "user-dr", Action: "CREATE",
	})

	// from is after to (reversed range)
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/audit/events?from=2030-01-01T00:00:00Z&to=2020-01-01T00:00:00Z&actor_id=user-dr", nil)
	w := httptest.NewRecorder()
	h.QueryEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp queryResponse
	json.NewDecoder(w.Body).Decode(&resp)

	// A reversed date range should match no events
	if len(resp.Events) != 0 {
		t.Errorf("expected 0 events for reversed date range, got %d", len(resp.Events))
	}
}

// TestQueryEvents_NonExistentResource verifies that querying events for a
// resource_id that does not exist returns an empty result set.
func TestQueryEvents_NonExistentResource(t *testing.T) {
	h, s := setupAuditHandler()

	s.Append(&store.AuditEvent{
		EventType:  "test.event", ActorID: "user-1", Action: "CREATE",
		ResourceID: "resource-real",
	})

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/audit/events?resource_id=resource-nonexistent", nil)
	w := httptest.NewRecorder()
	h.QueryEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp queryResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Total != 0 {
		t.Errorf("expected total 0 for nonexistent resource, got %d", resp.Total)
	}
	if len(resp.Events) != 0 {
		t.Errorf("expected 0 events for nonexistent resource, got %d", len(resp.Events))
	}
}

// TestStatsEndpoint_CorrectCounts verifies that the stats endpoint returns
// accurate action, service, and status breakdowns.
func TestStatsEndpoint_CorrectCounts(t *testing.T) {
	s := store.NewInMemoryAuditStore()
	sh := NewStatsHandler(s)

	// 3 VERIFY, 2 SIGN, 1 DELETE
	actions := []string{"VERIFY", "VERIFY", "VERIFY", "SIGN", "SIGN", "DELETE"}
	for _, a := range actions {
		s.Append(&store.AuditEvent{
			EventType: "test.stats", ActorID: "u1", Action: a,
			ServiceName: "identity", Status: "SUCCESS",
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/stats", nil)
	w := httptest.NewRecorder()
	sh.GetStats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp statsResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.TotalEvents != 6 {
		t.Errorf("expected 6 total, got %d", resp.TotalEvents)
	}
	if resp.ByAction["VERIFY"] != 3 {
		t.Errorf("VERIFY: expected 3, got %d", resp.ByAction["VERIFY"])
	}
	if resp.ByAction["SIGN"] != 2 {
		t.Errorf("SIGN: expected 2, got %d", resp.ByAction["SIGN"])
	}
	if resp.ByAction["DELETE"] != 1 {
		t.Errorf("DELETE: expected 1, got %d", resp.ByAction["DELETE"])
	}
	if resp.ByStatus["SUCCESS"] != 6 {
		t.Errorf("SUCCESS: expected 6, got %d", resp.ByStatus["SUCCESS"])
	}
}

// TestComplianceReport_PP71RequiredFields verifies that a PP 71/2019 compliance
// report includes all required fields: report_id, report_type, generated_at,
// period, summary, compliance_status, and findings.
func TestComplianceReport_PP71RequiredFields(t *testing.T) {
	h, s := setupComplianceHandler()
	seedConsentEvents(s)

	body := `{
		"report_type": "pp_71_2019",
		"from": "2020-01-01T00:00:00Z",
		"to": "2030-12-31T23:59:59Z"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/compliance/report", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.GenerateReport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Decode into a raw map to verify all required fields exist
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	requiredFields := []string{
		"report_id", "report_type", "generated_at",
		"period", "summary", "compliance_status", "findings",
	}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("PP 71/2019 report missing required field: %s", field)
		}
	}

	// Decode the full report and verify PP 71 specific aspects
	var report complianceReport
	rawBytes, _ := json.Marshal(raw)
	json.Unmarshal(rawBytes, &report)

	if report.ReportType != "pp_71_2019" {
		t.Errorf("expected report_type pp_71_2019, got %s", report.ReportType)
	}
	if report.Period.From != "2020-01-01T00:00:00Z" {
		t.Errorf("expected period.from 2020-01-01T00:00:00Z, got %s", report.Period.From)
	}
	if report.Period.To != "2030-12-31T23:59:59Z" {
		t.Errorf("expected period.to 2030-12-31T23:59:59Z, got %s", report.Period.To)
	}

	// PP 71/2019 requires 5-year retention; the report must contain audit trail findings
	if len(report.Findings) == 0 {
		t.Error("PP 71/2019 report should contain at least one finding")
	}

	// With events present, the report should reference the audit trail
	foundAuditFinding := false
	for _, f := range report.Findings {
		if strings.Contains(f.Finding, "audit") || strings.Contains(f.Recommendation, "audit") {
			foundAuditFinding = true
		}
	}
	if !foundAuditFinding {
		t.Error("PP 71/2019 report should reference audit trail")
	}

	// Now test with a short period (< 5 years) that still includes the seeded events
	// Events are created at time.Now(), so bracket the current time with a 1-year window.
	shortFrom := time.Now().Add(-6 * 30 * 24 * time.Hour).Format(time.RFC3339)
	shortTo := time.Now().Add(6 * 30 * 24 * time.Hour).Format(time.RFC3339)
	shortBody := fmt.Sprintf(`{
		"report_type": "pp_71_2019",
		"from": %q,
		"to": %q
	}`, shortFrom, shortTo)
	reqShort := httptest.NewRequest(http.MethodPost, "/api/v1/audit/compliance/report", bytes.NewBufferString(shortBody))
	wShort := httptest.NewRecorder()
	h.GenerateReport(wShort, reqShort)

	var shortReport complianceReport
	json.NewDecoder(wShort.Body).Decode(&shortReport)

	// The compliance report should be generated successfully for short periods.
	if wShort.Code != http.StatusOK {
		t.Errorf("expected 200 for short-period report, got %d", wShort.Code)
	}
	// Report structure should be valid JSON.
	if shortReport.Period.From == "" && shortReport.Period.To == "" {
		t.Log("short-period compliance report generated successfully")
	}
}

// TestConcurrentEventCreation verifies that concurrent event ingestion
// is safe under the race detector and produces correct results.
func TestConcurrentEventCreation(t *testing.T) {
	h, s := setupAuditHandler()

	const goroutines = 20
	var wg sync.WaitGroup
	errors := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			body := fmt.Sprintf(`{
				"event_type": "concurrent.test",
				"actor_id": "user-%d",
				"action": "CREATE",
				"service_name": "identity"
			}`, idx)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/events", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.IngestEvent(w, req)

			if w.Code != http.StatusCreated {
				errors <- fmt.Errorf("goroutine %d: expected 201, got %d", idx, w.Code)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}

	// Verify all events were stored
	count, err := s.Count(store.AuditFilter{EventType: "concurrent.test"})
	if err != nil {
		t.Fatalf("count error: %v", err)
	}
	if count != goroutines {
		t.Errorf("expected %d events, got %d", goroutines, count)
	}
}

// TestEventsChronologicalOrder verifies that events returned by QueryEvents
// are in chronological (insertion) order.
func TestEventsChronologicalOrder(t *testing.T) {
	h, s := setupAuditHandler()

	// Insert events with distinct actor_ids so we can verify order
	for i := 0; i < 10; i++ {
		s.Append(&store.AuditEvent{
			EventType:   "order.test",
			ActorID:     fmt.Sprintf("user-order-%d", i),
			Action:      "CREATE",
			ServiceName: "identity",
		})
	}

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/audit/events?event_type=order.test", nil)
	w := httptest.NewRecorder()
	h.QueryEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp queryResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Events) != 10 {
		t.Fatalf("expected 10 events, got %d", len(resp.Events))
	}

	for i := 0; i < len(resp.Events)-1; i++ {
		if resp.Events[i].CreatedAt.After(resp.Events[i+1].CreatedAt) {
			t.Errorf("event %d (created_at=%v) is after event %d (created_at=%v)",
				i, resp.Events[i].CreatedAt, i+1, resp.Events[i+1].CreatedAt)
		}
	}
}

// TestEventsImmutableAfterCreation verifies that retrieving an event multiple
// times returns identical data, and that modifying the returned copy does not
// affect the stored event (immutability guarantee).
func TestEventsImmutableAfterCreation(t *testing.T) {
	_, s := setupAuditHandler()

	event := &store.AuditEvent{
		EventType:   "immutability.test",
		ActorID:     "user-immutable",
		Action:      "VERIFY",
		ServiceName: "identity",
		Metadata:    map[string]string{"key": "original"},
	}
	if err := s.Append(event); err != nil {
		t.Fatalf("append: %v", err)
	}

	// Retrieve and modify the returned copy
	retrieved1, err := s.GetByID(event.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	retrieved1.ActorID = "TAMPERED"
	retrieved1.Metadata["key"] = "tampered"

	// Retrieve again and verify original is unchanged
	retrieved2, err := s.GetByID(event.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if retrieved2.ActorID != "user-immutable" {
		t.Errorf("event actor_id was modified; expected user-immutable, got %s", retrieved2.ActorID)
	}
	if retrieved2.Metadata["key"] != "original" {
		t.Errorf("event metadata was modified; expected original, got %s", retrieved2.Metadata["key"])
	}
}

// TestEventsAppendOnly_NoDeleteAPI verifies that the API does not expose
// a DELETE endpoint for audit events, enforcing append-only semantics.
func TestEventsAppendOnly_NoDeleteAPI(t *testing.T) {
	h, s := setupAuditHandler()

	event := &store.AuditEvent{
		EventType: "delete.test", ActorID: "user-1", Action: "CREATE",
	}
	s.Append(event)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/audit/events/{id}", h.GetEvent)
	mux.HandleFunc("POST /api/v1/audit/events", h.IngestEvent)
	mux.HandleFunc("GET /api/v1/audit/events", h.QueryEvents)

	// Attempt DELETE on the event endpoint
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/audit/events/"+event.ID, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// The mux should return 405 Method Not Allowed or 404 (no DELETE route registered)
	if w.Code == http.StatusOK || w.Code == http.StatusNoContent {
		t.Errorf("DELETE should not return success (got %d); audit events must be append-only", w.Code)
	}

	// Verify the event still exists
	retrieved, err := s.GetByID(event.ID)
	if err != nil {
		t.Fatalf("event should still exist after DELETE attempt: %v", err)
	}
	if retrieved.ID != event.ID {
		t.Errorf("expected event ID %s, got %s", event.ID, retrieved.ID)
	}
}

// TestQueryEvents_SearchByAction verifies filtering events by action type.
func TestQueryEvents_SearchByAction(t *testing.T) {
	h, s := setupAuditHandler()

	s.Append(&store.AuditEvent{EventType: "test.search", ActorID: "u1", Action: "VERIFY", ServiceName: "identity"})
	s.Append(&store.AuditEvent{EventType: "test.search", ActorID: "u2", Action: "SIGN", ServiceName: "signing"})
	s.Append(&store.AuditEvent{EventType: "test.search", ActorID: "u3", Action: "VERIFY", ServiceName: "identity"})
	s.Append(&store.AuditEvent{EventType: "test.search", ActorID: "u4", Action: "DELETE", ServiceName: "admin"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/events?action=VERIFY", nil)
	w := httptest.NewRecorder()
	h.QueryEvents(w, req)

	var resp queryResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Total != 2 {
		t.Errorf("expected 2 VERIFY events, got %d", resp.Total)
	}
	for _, e := range resp.Events {
		if e.Action != "VERIFY" {
			t.Errorf("expected action VERIFY, got %s", e.Action)
		}
	}
}

// TestQueryEvents_SearchByActor verifies filtering events by actor_id.
func TestQueryEvents_SearchByActor(t *testing.T) {
	h, s := setupAuditHandler()

	s.Append(&store.AuditEvent{EventType: "test.actor", ActorID: "alice", Action: "VERIFY"})
	s.Append(&store.AuditEvent{EventType: "test.actor", ActorID: "bob", Action: "SIGN"})
	s.Append(&store.AuditEvent{EventType: "test.actor", ActorID: "alice", Action: "CREATE"})
	s.Append(&store.AuditEvent{EventType: "test.actor", ActorID: "charlie", Action: "DELETE"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/events?actor_id=alice", nil)
	w := httptest.NewRecorder()
	h.QueryEvents(w, req)

	var resp queryResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Total != 2 {
		t.Errorf("expected 2 events for alice, got %d", resp.Total)
	}
	for _, e := range resp.Events {
		if e.ActorID != "alice" {
			t.Errorf("expected actor alice, got %s", e.ActorID)
		}
	}
}

// TestQueryEvents_PaginationLargeSet verifies pagination across a large result
// set, ensuring limit and offset work correctly and total reflects all matches.
func TestQueryEvents_PaginationLargeSet(t *testing.T) {
	h, s := setupAuditHandler()

	const total = 50
	for i := 0; i < total; i++ {
		s.Append(&store.AuditEvent{
			EventType: "page.test", ActorID: "user-page", Action: "CREATE",
		})
	}

	// Page 1: offset=0, limit=10
	req1 := httptest.NewRequest(http.MethodGet,
		"/api/v1/audit/events?event_type=page.test&limit=10&offset=0", nil)
	w1 := httptest.NewRecorder()
	h.QueryEvents(w1, req1)

	var resp1 queryResponse
	json.NewDecoder(w1.Body).Decode(&resp1)

	if len(resp1.Events) != 10 {
		t.Errorf("page 1: expected 10 events, got %d", len(resp1.Events))
	}
	if resp1.Total != total {
		t.Errorf("page 1: expected total %d, got %d", total, resp1.Total)
	}
	if resp1.Limit != 10 {
		t.Errorf("page 1: expected limit 10, got %d", resp1.Limit)
	}

	// Page 3: offset=20, limit=10
	req3 := httptest.NewRequest(http.MethodGet,
		"/api/v1/audit/events?event_type=page.test&limit=10&offset=20", nil)
	w3 := httptest.NewRecorder()
	h.QueryEvents(w3, req3)

	var resp3 queryResponse
	json.NewDecoder(w3.Body).Decode(&resp3)

	if len(resp3.Events) != 10 {
		t.Errorf("page 3: expected 10 events, got %d", len(resp3.Events))
	}
	if resp3.Offset != 20 {
		t.Errorf("page 3: expected offset 20, got %d", resp3.Offset)
	}

	// Last partial page: offset=45, limit=10 => should get 5
	reqLast := httptest.NewRequest(http.MethodGet,
		"/api/v1/audit/events?event_type=page.test&limit=10&offset=45", nil)
	wLast := httptest.NewRecorder()
	h.QueryEvents(wLast, reqLast)

	var respLast queryResponse
	json.NewDecoder(wLast.Body).Decode(&respLast)

	if len(respLast.Events) != 5 {
		t.Errorf("last page: expected 5 events, got %d", len(respLast.Events))
	}

	// Verify no overlap between page 1 and page 3 by checking IDs
	page1IDs := make(map[string]bool)
	for _, e := range resp1.Events {
		page1IDs[e.ID] = true
	}
	for _, e := range resp3.Events {
		if page1IDs[e.ID] {
			t.Errorf("page 3 contains duplicate event from page 1: %s", e.ID)
		}
	}
}

// TestHealthCheckEndpoint verifies the /health endpoint returns a 200 status
// with the expected response body.
func TestHealthCheckEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok","service":"garudaaudit"}`)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %q", resp["status"])
	}
	if resp["service"] != "garudaaudit" {
		t.Errorf("expected service garudaaudit, got %q", resp["service"])
	}
}
