// Package security provides structured security event logging for SOC/SIEM integration.
package security

import (
	"log/slog"
	"sync"
	"time"
)

// EventType for security events.
type EventType string

const (
	EventAuthSuccess    EventType = "AUTH_SUCCESS"
	EventAuthFailure    EventType = "AUTH_FAILURE"
	EventAuthRateLimit  EventType = "AUTH_RATE_LIMITED"
	EventTokenInvalid   EventType = "TOKEN_INVALID"
	EventTokenExpired   EventType = "TOKEN_EXPIRED"
	EventAccessDenied   EventType = "ACCESS_DENIED"
	EventKeyCreated     EventType = "API_KEY_CREATED"
	EventKeyRevoked     EventType = "API_KEY_REVOKED"
	EventCertRevoked    EventType = "CERT_REVOKED"
	EventDataExport     EventType = "DATA_EXPORT"
	EventDataDeletion   EventType = "DATA_DELETION"
	EventSuspiciousIP   EventType = "SUSPICIOUS_IP"
	EventBruteForce     EventType = "BRUTE_FORCE_DETECTED"
	EventCSRFViolation  EventType = "CSRF_VIOLATION"
	EventCORSViolation  EventType = "CORS_VIOLATION"
	EventInputSanitized EventType = "INPUT_SANITIZED"
)

// Event is a structured security event for SIEM integration.
type Event struct {
	Type      EventType
	Severity  string // INFO, WARNING, CRITICAL
	ActorID   string
	ActorIP   string
	Resource  string
	Action    string
	Outcome   string // SUCCESS, FAILURE, BLOCKED
	Details   map[string]string
	Timestamp time.Time
}

// Logger logs security events in a structured format suitable for SIEM consumption.
type Logger struct {
	serviceName string
	collector   *InMemoryCollector
}

// NewLogger creates a new security event logger for the given service.
func NewLogger(serviceName string) *Logger {
	return &Logger{serviceName: serviceName}
}

// SetCollector attaches an in-memory collector for testing or event aggregation.
func (l *Logger) SetCollector(c *InMemoryCollector) {
	l.collector = c
}

// Log emits a structured security event via slog and optionally to a collector.
func (l *Logger) Log(event Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	attrs := []any{
		slog.String("service", l.serviceName),
		slog.String("event_type", string(event.Type)),
		slog.String("severity", event.Severity),
		slog.String("actor_id", event.ActorID),
		slog.String("actor_ip", event.ActorIP),
		slog.String("resource", event.Resource),
		slog.String("action", event.Action),
		slog.String("outcome", event.Outcome),
		slog.Time("event_time", event.Timestamp),
	}

	for k, v := range event.Details {
		attrs = append(attrs, slog.String("detail."+k, v))
	}

	switch event.Severity {
	case "CRITICAL":
		slog.Error("security event", attrs...)
	case "WARNING":
		slog.Warn("security event", attrs...)
	default:
		slog.Info("security event", attrs...)
	}

	if l.collector != nil {
		l.collector.Collect(event)
	}
}

// LogAuth logs an authentication event.
func (l *Logger) LogAuth(eventType EventType, actorID, actorIP, outcome string) {
	severity := "INFO"
	if outcome != "SUCCESS" {
		severity = "WARNING"
	}
	l.Log(Event{
		Type:     eventType,
		Severity: severity,
		ActorID:  actorID,
		ActorIP:  actorIP,
		Action:   "authenticate",
		Outcome:  outcome,
	})
}

// LogAccess logs an access control event.
func (l *Logger) LogAccess(eventType EventType, actorID, resource, outcome string) {
	severity := "WARNING"
	if outcome == "SUCCESS" {
		severity = "INFO"
	}
	l.Log(Event{
		Type:     eventType,
		Severity: severity,
		ActorID:  actorID,
		Resource: resource,
		Action:   "access",
		Outcome:  outcome,
	})
}

// LogDataEvent logs a data lifecycle event (export, deletion).
func (l *Logger) LogDataEvent(eventType EventType, actorID, resource string, details map[string]string) {
	l.Log(Event{
		Type:     eventType,
		Severity: "INFO",
		ActorID:  actorID,
		Resource: resource,
		Action:   "data_operation",
		Outcome:  "SUCCESS",
		Details:  details,
	})
}

// InMemoryCollector collects events for testing and aggregation.
type InMemoryCollector struct {
	Events []Event
	mu     sync.Mutex
}

// NewCollector creates a new in-memory event collector.
func NewCollector() *InMemoryCollector {
	return &InMemoryCollector{}
}

// Collect adds an event to the collector.
func (c *InMemoryCollector) Collect(event Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Events = append(c.Events, event)
}

// Count returns the number of events matching the given type.
func (c *InMemoryCollector) Count(eventType EventType) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for _, e := range c.Events {
		if e.Type == eventType {
			count++
		}
	}
	return count
}

// Last returns the most recently collected event, or nil if empty.
func (c *InMemoryCollector) Last() *Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.Events) == 0 {
		return nil
	}
	e := c.Events[len(c.Events)-1]
	return &e
}
