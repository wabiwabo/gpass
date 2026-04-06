package security

import (
	"sync"
	"testing"
	"time"
)

func TestLogEmitsEventWithAllFields(t *testing.T) {
	collector := NewCollector()
	logger := NewLogger("test-service")
	logger.SetCollector(collector)

	now := time.Now()
	event := Event{
		Type:      EventAuthSuccess,
		Severity:  "INFO",
		ActorID:   "user-123",
		ActorIP:   "192.168.1.1",
		Resource:  "/api/login",
		Action:    "authenticate",
		Outcome:   "SUCCESS",
		Details:   map[string]string{"method": "password"},
		Timestamp: now,
	}
	logger.Log(event)

	last := collector.Last()
	if last == nil {
		t.Fatal("expected event to be collected")
	}
	if last.Type != EventAuthSuccess {
		t.Errorf("expected AUTH_SUCCESS, got %s", last.Type)
	}
	if last.Severity != "INFO" {
		t.Errorf("expected INFO, got %s", last.Severity)
	}
	if last.ActorID != "user-123" {
		t.Errorf("expected user-123, got %s", last.ActorID)
	}
	if last.ActorIP != "192.168.1.1" {
		t.Errorf("expected 192.168.1.1, got %s", last.ActorIP)
	}
	if last.Resource != "/api/login" {
		t.Errorf("expected /api/login, got %s", last.Resource)
	}
	if last.Action != "authenticate" {
		t.Errorf("expected authenticate, got %s", last.Action)
	}
	if last.Outcome != "SUCCESS" {
		t.Errorf("expected SUCCESS, got %s", last.Outcome)
	}
	if last.Details["method"] != "password" {
		t.Error("details not preserved")
	}
	if !last.Timestamp.Equal(now) {
		t.Error("timestamp not preserved")
	}
}

func TestLogAuthCreatesCorrectEventType(t *testing.T) {
	collector := NewCollector()
	logger := NewLogger("auth-svc")
	logger.SetCollector(collector)

	logger.LogAuth(EventAuthSuccess, "user-1", "10.0.0.1", "SUCCESS")

	last := collector.Last()
	if last == nil {
		t.Fatal("expected event")
	}
	if last.Type != EventAuthSuccess {
		t.Errorf("expected AUTH_SUCCESS, got %s", last.Type)
	}
	if last.ActorID != "user-1" {
		t.Errorf("expected user-1, got %s", last.ActorID)
	}
	if last.ActorIP != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1, got %s", last.ActorIP)
	}
	if last.Outcome != "SUCCESS" {
		t.Errorf("expected SUCCESS, got %s", last.Outcome)
	}
	if last.Severity != "INFO" {
		t.Errorf("expected INFO severity for success, got %s", last.Severity)
	}
}

func TestLogAuthFailureSeverity(t *testing.T) {
	collector := NewCollector()
	logger := NewLogger("auth-svc")
	logger.SetCollector(collector)

	logger.LogAuth(EventAuthFailure, "user-1", "10.0.0.1", "FAILURE")

	last := collector.Last()
	if last.Severity != "WARNING" {
		t.Errorf("expected WARNING severity for failure, got %s", last.Severity)
	}
}

func TestLogAccessCreatesAccessEvent(t *testing.T) {
	collector := NewCollector()
	logger := NewLogger("api-gw")
	logger.SetCollector(collector)

	logger.LogAccess(EventAccessDenied, "user-2", "/admin/settings", "FAILURE")

	last := collector.Last()
	if last == nil {
		t.Fatal("expected event")
	}
	if last.Type != EventAccessDenied {
		t.Errorf("expected ACCESS_DENIED, got %s", last.Type)
	}
	if last.Resource != "/admin/settings" {
		t.Errorf("expected /admin/settings, got %s", last.Resource)
	}
	if last.Action != "access" {
		t.Errorf("expected access action, got %s", last.Action)
	}
}

func TestLogDataEventIncludesDetails(t *testing.T) {
	collector := NewCollector()
	logger := NewLogger("data-svc")
	logger.SetCollector(collector)

	details := map[string]string{
		"format":      "csv",
		"record_count": "1500",
	}
	logger.LogDataEvent(EventDataExport, "admin-1", "citizens_table", details)

	last := collector.Last()
	if last == nil {
		t.Fatal("expected event")
	}
	if last.Type != EventDataExport {
		t.Errorf("expected DATA_EXPORT, got %s", last.Type)
	}
	if last.Details["format"] != "csv" {
		t.Error("format detail missing")
	}
	if last.Details["record_count"] != "1500" {
		t.Error("record_count detail missing")
	}
	if last.Resource != "citizens_table" {
		t.Errorf("expected citizens_table, got %s", last.Resource)
	}
}

func TestInMemoryCollectorCountsByType(t *testing.T) {
	collector := NewCollector()
	logger := NewLogger("test")
	logger.SetCollector(collector)

	logger.LogAuth(EventAuthSuccess, "u1", "1.1.1.1", "SUCCESS")
	logger.LogAuth(EventAuthSuccess, "u2", "2.2.2.2", "SUCCESS")
	logger.LogAuth(EventAuthFailure, "u3", "3.3.3.3", "FAILURE")

	if collector.Count(EventAuthSuccess) != 2 {
		t.Errorf("expected 2 success events, got %d", collector.Count(EventAuthSuccess))
	}
	if collector.Count(EventAuthFailure) != 1 {
		t.Errorf("expected 1 failure event, got %d", collector.Count(EventAuthFailure))
	}
	if collector.Count(EventAccessDenied) != 0 {
		t.Errorf("expected 0 access denied events, got %d", collector.Count(EventAccessDenied))
	}
}

func TestInMemoryCollectorLastReturnsMostRecent(t *testing.T) {
	collector := NewCollector()
	logger := NewLogger("test")
	logger.SetCollector(collector)

	logger.LogAuth(EventAuthSuccess, "first", "1.1.1.1", "SUCCESS")
	logger.LogAuth(EventAuthFailure, "second", "2.2.2.2", "FAILURE")

	last := collector.Last()
	if last == nil {
		t.Fatal("expected event")
	}
	if last.ActorID != "second" {
		t.Errorf("expected 'second', got %s", last.ActorID)
	}
}

func TestInMemoryCollectorLastReturnsNilWhenEmpty(t *testing.T) {
	collector := NewCollector()
	if collector.Last() != nil {
		t.Error("expected nil for empty collector")
	}
}

func TestAllEventTypesAreUniqueStrings(t *testing.T) {
	types := []EventType{
		EventAuthSuccess, EventAuthFailure, EventAuthRateLimit,
		EventTokenInvalid, EventTokenExpired, EventAccessDenied,
		EventKeyCreated, EventKeyRevoked, EventCertRevoked,
		EventDataExport, EventDataDeletion, EventSuspiciousIP,
		EventBruteForce, EventCSRFViolation, EventCORSViolation,
		EventInputSanitized,
	}

	seen := make(map[EventType]bool)
	for _, et := range types {
		if string(et) == "" {
			t.Errorf("event type must not be empty")
		}
		if seen[et] {
			t.Errorf("duplicate event type: %s", et)
		}
		seen[et] = true
	}

	if len(seen) != 16 {
		t.Errorf("expected 16 unique event types, got %d", len(seen))
	}
}

func TestSeverityLevelsValid(t *testing.T) {
	collector := NewCollector()
	logger := NewLogger("test")
	logger.SetCollector(collector)

	// INFO via success auth
	logger.LogAuth(EventAuthSuccess, "u", "1.1.1.1", "SUCCESS")
	if collector.Last().Severity != "INFO" {
		t.Errorf("expected INFO, got %s", collector.Last().Severity)
	}

	// WARNING via failure auth
	logger.LogAuth(EventAuthFailure, "u", "1.1.1.1", "FAILURE")
	if collector.Last().Severity != "WARNING" {
		t.Errorf("expected WARNING, got %s", collector.Last().Severity)
	}

	// CRITICAL via direct log
	logger.Log(Event{
		Type:     EventBruteForce,
		Severity: "CRITICAL",
		ActorIP:  "1.1.1.1",
		Action:   "brute_force",
		Outcome:  "BLOCKED",
	})
	if collector.Last().Severity != "CRITICAL" {
		t.Errorf("expected CRITICAL, got %s", collector.Last().Severity)
	}
}

func TestConcurrentLoggingSafe(t *testing.T) {
	collector := NewCollector()
	logger := NewLogger("concurrent-test")
	logger.SetCollector(collector)

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			logger.LogAuth(EventAuthSuccess, "user", "1.1.1.1", "SUCCESS")
			_ = collector.Count(EventAuthSuccess)
			_ = collector.Last()
		}(i)
	}
	wg.Wait()

	if collector.Count(EventAuthSuccess) != 100 {
		t.Errorf("expected 100 events, got %d", collector.Count(EventAuthSuccess))
	}
}
