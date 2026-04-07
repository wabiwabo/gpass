package store

import (
	"testing"
	"time"
)

func TestAppendValidation(t *testing.T) {
	s := NewInMemoryAuditStore()

	tests := []struct {
		name  string
		event *AuditEvent
	}{
		{"missing_event_type", &AuditEvent{ActorID: "u1", Action: "read"}},
		{"missing_actor_id", &AuditEvent{EventType: "LOGIN", Action: "read"}},
		{"missing_action", &AuditEvent{EventType: "LOGIN", ActorID: "u1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.Append(tt.event)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestAppendDefaults(t *testing.T) {
	s := NewInMemoryAuditStore()
	e := &AuditEvent{
		EventType: "LOGIN",
		ActorID:   "user-1",
		Action:    "authenticate",
	}
	_ = s.Append(e)

	got, _ := s.GetByID(e.ID)
	if got.ActorType != "USER" {
		t.Errorf("ActorType default: got %q, want USER", got.ActorType)
	}
	if got.Status != "SUCCESS" {
		t.Errorf("Status default: got %q, want SUCCESS", got.Status)
	}
	if got.Metadata == nil {
		t.Error("Metadata should be initialized")
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestAppendPreservesCustomValues(t *testing.T) {
	s := NewInMemoryAuditStore()
	e := &AuditEvent{
		EventType:   "DATA_ACCESS",
		ActorID:     "service-1",
		ActorType:   "SERVICE",
		Action:      "read",
		Status:      "FAILURE",
		ServiceName: "identity",
		IPAddress:   "10.0.0.1",
		UserAgent:   "GarudaPass/1.0",
		RequestID:   "req-123",
		Metadata:    map[string]string{"reason": "unauthorized"},
	}
	_ = s.Append(e)

	got, _ := s.GetByID(e.ID)
	if got.ActorType != "SERVICE" {
		t.Errorf("ActorType: got %q", got.ActorType)
	}
	if got.Status != "FAILURE" {
		t.Errorf("Status: got %q", got.Status)
	}
	if got.ServiceName != "identity" {
		t.Errorf("ServiceName: got %q", got.ServiceName)
	}
	if got.RequestID != "req-123" {
		t.Errorf("RequestID: got %q", got.RequestID)
	}
	if got.Metadata["reason"] != "unauthorized" {
		t.Errorf("Metadata: got %v", got.Metadata)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	s := NewInMemoryAuditStore()
	_, err := s.GetByID("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestQueryPagination(t *testing.T) {
	s := NewInMemoryAuditStore()
	for i := 0; i < 25; i++ {
		_ = s.Append(&AuditEvent{
			EventType: "TEST",
			ActorID:   "paginate-user",
			Action:    "action",
		})
	}

	// Page 1
	page1, _ := s.Query(AuditFilter{ActorID: "paginate-user", Limit: 10})
	if len(page1) != 10 {
		t.Errorf("page1: got %d, want 10", len(page1))
	}

	// Page 2
	page2, _ := s.Query(AuditFilter{ActorID: "paginate-user", Limit: 10, Offset: 10})
	if len(page2) != 10 {
		t.Errorf("page2: got %d, want 10", len(page2))
	}

	// Page 3
	page3, _ := s.Query(AuditFilter{ActorID: "paginate-user", Limit: 10, Offset: 20})
	if len(page3) != 5 {
		t.Errorf("page3: got %d, want 5", len(page3))
	}

	// No overlap
	if page1[0].ID == page2[0].ID {
		t.Error("pages should not overlap")
	}
}

func TestQueryDefaultLimit(t *testing.T) {
	s := NewInMemoryAuditStore()
	for i := 0; i < 150; i++ {
		_ = s.Append(&AuditEvent{
			EventType: "BULK", ActorID: "user-dl", Action: "test",
		})
	}

	results, _ := s.Query(AuditFilter{ActorID: "user-dl"})
	if len(results) != 100 {
		t.Errorf("default limit: got %d, want 100", len(results))
	}
}

func TestQueryMaxLimit(t *testing.T) {
	s := NewInMemoryAuditStore()
	for i := 0; i < 5; i++ {
		_ = s.Append(&AuditEvent{
			EventType: "BULK", ActorID: "user-ml", Action: "test",
		})
	}

	results, _ := s.Query(AuditFilter{ActorID: "user-ml", Limit: 5000})
	if len(results) != 5 {
		t.Errorf("got %d", len(results))
	}
}

func TestQueryTimeRange(t *testing.T) {
	s := NewInMemoryAuditStore()

	// Create events
	for i := 0; i < 5; i++ {
		_ = s.Append(&AuditEvent{
			EventType: "TIME_TEST", ActorID: "user-tr", Action: "test",
		})
	}

	now := time.Now()
	future := now.Add(1 * time.Hour)
	past := now.Add(-1 * time.Hour)

	// All events should be within past-future range
	results, _ := s.Query(AuditFilter{ActorID: "user-tr", From: past, To: future})
	if len(results) != 5 {
		t.Errorf("time range: got %d, want 5", len(results))
	}

	// No events in future range
	results, _ = s.Query(AuditFilter{ActorID: "user-tr", From: future})
	if len(results) != 0 {
		t.Errorf("future range: got %d, want 0", len(results))
	}
}

func TestQueryCombinedFilters(t *testing.T) {
	s := NewInMemoryAuditStore()

	_ = s.Append(&AuditEvent{EventType: "LOGIN", ActorID: "user-1", Action: "auth", ServiceName: "bff", Status: "SUCCESS"})
	_ = s.Append(&AuditEvent{EventType: "LOGIN", ActorID: "user-1", Action: "auth", ServiceName: "bff", Status: "FAILURE"})
	_ = s.Append(&AuditEvent{EventType: "DATA_ACCESS", ActorID: "user-1", Action: "read", ServiceName: "identity"})
	_ = s.Append(&AuditEvent{EventType: "LOGIN", ActorID: "user-2", Action: "auth", ServiceName: "bff"})

	results, _ := s.Query(AuditFilter{
		ActorID:     "user-1",
		EventType:   "LOGIN",
		ServiceName: "bff",
		Status:      "SUCCESS",
	})
	if len(results) != 1 {
		t.Errorf("multi-filter: got %d, want 1", len(results))
	}
}

func TestQueryEmptyResults(t *testing.T) {
	s := NewInMemoryAuditStore()
	results, err := s.Query(AuditFilter{ActorID: "nobody"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d, want 0", len(results))
	}
}

func TestCountMatchesQuery(t *testing.T) {
	s := NewInMemoryAuditStore()
	for i := 0; i < 30; i++ {
		actorID := "user-a"
		if i%3 == 0 {
			actorID = "user-b"
		}
		_ = s.Append(&AuditEvent{EventType: "TEST", ActorID: actorID, Action: "act"})
	}

	filter := AuditFilter{ActorID: "user-a"}
	count, _ := s.Count(filter)
	results, _ := s.Query(AuditFilter{ActorID: "user-a", Limit: 1000})

	if count != int64(len(results)) {
		t.Errorf("count=%d but query returned %d", count, len(results))
	}
}

func TestQueryResourceFilters(t *testing.T) {
	s := NewInMemoryAuditStore()
	_ = s.Append(&AuditEvent{EventType: "UPDATE", ActorID: "u1", Action: "update", ResourceID: "doc-1", ResourceType: "document"})
	_ = s.Append(&AuditEvent{EventType: "UPDATE", ActorID: "u1", Action: "update", ResourceID: "doc-2", ResourceType: "document"})
	_ = s.Append(&AuditEvent{EventType: "UPDATE", ActorID: "u1", Action: "update", ResourceID: "user-1", ResourceType: "user"})

	// Filter by resource type
	results, _ := s.Query(AuditFilter{ResourceType: "document"})
	if len(results) != 2 {
		t.Errorf("resource type filter: got %d, want 2", len(results))
	}

	// Filter by resource ID
	results, _ = s.Query(AuditFilter{ResourceID: "doc-1"})
	if len(results) != 1 {
		t.Errorf("resource ID filter: got %d, want 1", len(results))
	}
}
