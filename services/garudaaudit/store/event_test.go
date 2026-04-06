package store

import (
	"testing"
	"time"
)

func newTestEvent() *AuditEvent {
	return &AuditEvent{
		EventType:    "identity.verified",
		ActorID:      "user-123",
		ActorType:    "USER",
		ResourceID:   "res-456",
		ResourceType: "USER",
		Action:       "VERIFY",
		ServiceName:  "identity",
		Status:       "SUCCESS",
		IPAddress:    "10.0.0.1",
		UserAgent:    "test-agent",
		RequestID:    "req-789",
		Metadata:     map[string]string{"key": "value"},
	}
}

func TestAppend_Success(t *testing.T) {
	s := NewInMemoryAuditStore()
	event := newTestEvent()

	if err := s.Append(event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.ID == "" {
		t.Error("expected ID to be assigned")
	}
	if event.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestAppend_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(e *AuditEvent)
		wantErr string
	}{
		{
			name:    "missing event_type",
			modify:  func(e *AuditEvent) { e.EventType = "" },
			wantErr: "event_type is required",
		},
		{
			name:    "missing actor_id",
			modify:  func(e *AuditEvent) { e.ActorID = "" },
			wantErr: "actor_id is required",
		},
		{
			name:    "missing action",
			modify:  func(e *AuditEvent) { e.Action = "" },
			wantErr: "action is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewInMemoryAuditStore()
			event := newTestEvent()
			tt.modify(event)

			err := s.Append(event)
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Errorf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestAppend_DefaultValues(t *testing.T) {
	s := NewInMemoryAuditStore()
	event := &AuditEvent{
		EventType: "test.event",
		ActorID:   "user-1",
		Action:    "CREATE",
	}

	if err := s.Append(event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := s.GetByID(event.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ActorType != "USER" {
		t.Errorf("expected default actor_type USER, got %s", got.ActorType)
	}
	if got.Status != "SUCCESS" {
		t.Errorf("expected default status SUCCESS, got %s", got.Status)
	}
	if got.Metadata == nil {
		t.Error("expected metadata to be initialized")
	}
}

func TestQueryByActorID(t *testing.T) {
	s := NewInMemoryAuditStore()

	e1 := newTestEvent()
	e1.ActorID = "alice"
	e2 := newTestEvent()
	e2.ActorID = "bob"

	s.Append(e1)
	s.Append(e2)

	results, err := s.Query(AuditFilter{ActorID: "alice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ActorID != "alice" {
		t.Errorf("expected actor_id alice, got %s", results[0].ActorID)
	}
}

func TestQueryByResourceType(t *testing.T) {
	s := NewInMemoryAuditStore()

	e1 := newTestEvent()
	e1.ResourceType = "DOCUMENT"
	e2 := newTestEvent()
	e2.ResourceType = "USER"

	s.Append(e1)
	s.Append(e2)

	results, err := s.Query(AuditFilter{ResourceType: "DOCUMENT"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ResourceType != "DOCUMENT" {
		t.Errorf("expected resource_type DOCUMENT, got %s", results[0].ResourceType)
	}
}

func TestQueryByDateRange(t *testing.T) {
	s := NewInMemoryAuditStore()

	// Append events with known ordering
	for i := 0; i < 5; i++ {
		e := newTestEvent()
		s.Append(e)
	}

	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	// All events should be within past-future range
	results, err := s.Query(AuditFilter{From: past, To: future})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("expected 5 results, got %d", len(results))
	}

	// No events should be in the future
	results, err = s.Query(AuditFilter{From: future})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestQueryMultipleFilters(t *testing.T) {
	s := NewInMemoryAuditStore()

	e1 := newTestEvent()
	e1.ActorID = "alice"
	e1.Action = "CREATE"
	e1.ServiceName = "identity"

	e2 := newTestEvent()
	e2.ActorID = "alice"
	e2.Action = "DELETE"
	e2.ServiceName = "identity"

	e3 := newTestEvent()
	e3.ActorID = "bob"
	e3.Action = "CREATE"
	e3.ServiceName = "identity"

	s.Append(e1)
	s.Append(e2)
	s.Append(e3)

	results, err := s.Query(AuditFilter{
		ActorID:     "alice",
		Action:      "CREATE",
		ServiceName: "identity",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ActorID != "alice" || results[0].Action != "CREATE" {
		t.Error("unexpected result")
	}
}

func TestQueryWithLimitOffset(t *testing.T) {
	s := NewInMemoryAuditStore()

	for i := 0; i < 10; i++ {
		e := newTestEvent()
		s.Append(e)
	}

	// Limit
	results, err := s.Query(AuditFilter{Limit: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// Offset
	results, err = s.Query(AuditFilter{Limit: 3, Offset: 8})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// Default limit (100)
	results, err = s.Query(AuditFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 10 {
		t.Errorf("expected 10 results, got %d", len(results))
	}

	// Max limit capped at 1000
	results, err = s.Query(AuditFilter{Limit: 5000})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 10 {
		t.Errorf("expected 10 results, got %d", len(results))
	}
}

func TestCount(t *testing.T) {
	s := NewInMemoryAuditStore()

	for i := 0; i < 5; i++ {
		e := newTestEvent()
		if i < 3 {
			e.Action = "CREATE"
		} else {
			e.Action = "DELETE"
		}
		s.Append(e)
	}

	count, err := s.Count(AuditFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5, got %d", count)
	}

	count, err = s.Count(AuditFilter{Action: "CREATE"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}

func TestEventsAreImmutable(t *testing.T) {
	s := NewInMemoryAuditStore()
	event := newTestEvent()
	s.Append(event)

	id := event.ID

	// Modify the original event
	event.ActorID = "modified"
	event.Metadata["new_key"] = "new_value"

	// Retrieve from store — should not reflect modifications
	got, err := s.GetByID(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ActorID == "modified" {
		t.Error("stored event should not be affected by external modification")
	}
	if _, ok := got.Metadata["new_key"]; ok {
		t.Error("stored metadata should not be affected by external modification")
	}

	// Modify the retrieved copy
	got.ActorID = "also-modified"

	// Retrieve again — should not reflect modifications to the copy
	got2, err := s.GetByID(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got2.ActorID == "also-modified" {
		t.Error("stored event should not be affected by modifying a retrieved copy")
	}
}

func TestGetByID_NotFound(t *testing.T) {
	s := NewInMemoryAuditStore()

	_, err := s.GetByID("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent ID")
	}
}
