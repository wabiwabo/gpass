// Package pdpaudit provides UU PDP (Personal Data Protection)
// compliance audit logging. Records all personal data access,
// processing, sharing, and deletion events per UU PDP No. 27/2022
// with 5-year retention per PP 71/2019.
package pdpaudit

import (
	"encoding/json"
	"log/slog"
	"time"
)

// EventType classifies the PDP audit event.
type EventType string

const (
	EventAccess    EventType = "data_access"
	EventProcess   EventType = "data_processing"
	EventShare     EventType = "data_sharing"
	EventDelete    EventType = "data_deletion"
	EventExport    EventType = "data_export"
	EventConsent   EventType = "consent_change"
	EventBreach    EventType = "data_breach"
	EventRetention EventType = "retention_check"
)

// LawfulBasis documents the legal basis for processing.
type LawfulBasis string

const (
	BasisConsent          LawfulBasis = "consent"
	BasisContract         LawfulBasis = "contract"
	BasisLegalObligation  LawfulBasis = "legal_obligation"
	BasisVitalInterest    LawfulBasis = "vital_interest"
	BasisPublicInterest   LawfulBasis = "public_interest"
	BasisLegitimateInterest LawfulBasis = "legitimate_interest"
)

// Event is a PDP audit record.
type Event struct {
	Timestamp   time.Time         `json:"timestamp"`
	EventType   EventType         `json:"event_type"`
	DataSubject string            `json:"data_subject"` // user whose data is affected
	Controller  string            `json:"controller"`   // entity controlling data
	Processor   string            `json:"processor"`    // entity processing data
	LawfulBasis LawfulBasis       `json:"lawful_basis"`
	DataFields  []string          `json:"data_fields"`  // which PII fields
	Purpose     string            `json:"purpose"`
	ConsentID   string            `json:"consent_id,omitempty"`
	RequestID   string            `json:"request_id,omitempty"`
	ActorID     string            `json:"actor_id"`
	ActorType   string            `json:"actor_type"` // "user", "service", "admin"
	Recipient   string            `json:"recipient,omitempty"` // for data sharing
	Outcome     string            `json:"outcome"` // "success", "denied", "error"
	Reason      string            `json:"reason,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	RetainUntil time.Time         `json:"retain_until"` // PP 71/2019 5-year minimum
}

// Logger writes PDP audit events.
type Logger struct {
	controller string
	logger     *slog.Logger
}

// NewLogger creates a PDP audit logger.
func NewLogger(controller string, logger *slog.Logger) *Logger {
	if logger == nil {
		logger = slog.Default()
	}
	return &Logger{controller: controller, logger: logger}
}

// Record logs a PDP audit event.
func (l *Logger) Record(event Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	event.Controller = l.controller
	if event.RetainUntil.IsZero() {
		// PP 71/2019: 5-year minimum retention
		event.RetainUntil = event.Timestamp.Add(5 * 365 * 24 * time.Hour)
	}

	l.logger.Info("pdp_audit",
		slog.String("event_type", string(event.EventType)),
		slog.String("data_subject", event.DataSubject),
		slog.String("controller", event.Controller),
		slog.String("processor", event.Processor),
		slog.String("lawful_basis", string(event.LawfulBasis)),
		slog.String("purpose", event.Purpose),
		slog.String("actor_id", event.ActorID),
		slog.String("outcome", event.Outcome),
	)
}

// Builder provides fluent audit event construction.
type Builder struct {
	logger *Logger
	event  Event
}

// Begin starts building a PDP audit event.
func (l *Logger) Begin(eventType EventType, dataSubject string) *Builder {
	return &Builder{
		logger: l,
		event: Event{
			EventType:   eventType,
			DataSubject: dataSubject,
			Outcome:     "success",
		},
	}
}

// Processor sets the data processor.
func (b *Builder) Processor(p string) *Builder {
	b.event.Processor = p
	return b
}

// Basis sets the lawful basis.
func (b *Builder) Basis(lb LawfulBasis) *Builder {
	b.event.LawfulBasis = lb
	return b
}

// Fields sets which PII data fields are involved.
func (b *Builder) Fields(fields ...string) *Builder {
	b.event.DataFields = fields
	return b
}

// Purpose sets the processing purpose.
func (b *Builder) Purpose(p string) *Builder {
	b.event.Purpose = p
	return b
}

// Consent sets the consent ID.
func (b *Builder) Consent(id string) *Builder {
	b.event.ConsentID = id
	return b
}

// Actor sets who performed the action.
func (b *Builder) Actor(id, actorType string) *Builder {
	b.event.ActorID = id
	b.event.ActorType = actorType
	return b
}

// Request sets the request ID.
func (b *Builder) Request(id string) *Builder {
	b.event.RequestID = id
	return b
}

// Recipient sets the data recipient (for sharing events).
func (b *Builder) Recipient(r string) *Builder {
	b.event.Recipient = r
	return b
}

// Denied marks the event as denied.
func (b *Builder) Denied(reason string) *Builder {
	b.event.Outcome = "denied"
	b.event.Reason = reason
	return b
}

// Failed marks the event as failed.
func (b *Builder) Failed(reason string) *Builder {
	b.event.Outcome = "error"
	b.event.Reason = reason
	return b
}

// Meta adds metadata.
func (b *Builder) Meta(key, value string) *Builder {
	if b.event.Metadata == nil {
		b.event.Metadata = make(map[string]string)
	}
	b.event.Metadata[key] = value
	return b
}

// Done records the audit event.
func (b *Builder) Done() Event {
	b.event.Controller = b.logger.controller
	if b.event.Timestamp.IsZero() {
		b.event.Timestamp = time.Now().UTC()
	}
	if b.event.RetainUntil.IsZero() {
		b.event.RetainUntil = b.event.Timestamp.Add(5 * 365 * 24 * time.Hour)
	}
	b.logger.Record(b.event)
	return b.event
}

// JSON returns the event as JSON bytes.
func (e Event) JSON() ([]byte, error) {
	return json.Marshal(e)
}

// ValidEventType checks if the event type is predefined.
func ValidEventType(et EventType) bool {
	switch et {
	case EventAccess, EventProcess, EventShare, EventDelete,
		EventExport, EventConsent, EventBreach, EventRetention:
		return true
	}
	return false
}

// ValidLawfulBasis checks if the lawful basis is predefined.
func ValidLawfulBasis(lb LawfulBasis) bool {
	switch lb {
	case BasisConsent, BasisContract, BasisLegalObligation,
		BasisVitalInterest, BasisPublicInterest, BasisLegitimateInterest:
		return true
	}
	return false
}
