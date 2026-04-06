package audit

import (
	"fmt"
	"log/slog"
)

// Audit action constants.
const (
	ActionCertRequested  = "CERT_REQUESTED"
	ActionCertIssued     = "CERT_ISSUED"
	ActionCertRevoked    = "CERT_REVOKED"
	ActionDocUploaded    = "DOC_UPLOADED"
	ActionDocSigned      = "DOC_SIGNED"
	ActionDocDownloaded  = "DOC_DOWNLOADED"
	ActionSignFailed     = "SIGN_FAILED"
)

// Event represents an audit event.
type Event struct {
	UserID   string
	Action   string
	Metadata map[string]string
}

// Emitter defines the interface for emitting audit events.
type Emitter interface {
	Emit(event Event) error
}

// LogEmitter emits audit events using slog.
type LogEmitter struct{}

// NewLogEmitter creates a new LogEmitter.
func NewLogEmitter() *LogEmitter {
	return &LogEmitter{}
}

// Emit logs the audit event.
func (e *LogEmitter) Emit(event Event) error {
	if event.UserID == "" {
		return fmt.Errorf("audit event requires user_id")
	}
	if event.Action == "" {
		return fmt.Errorf("audit event requires action")
	}

	attrs := []any{
		"user_id", event.UserID,
		"action", event.Action,
	}
	for k, v := range event.Metadata {
		attrs = append(attrs, k, v)
	}

	slog.Info("audit_event", attrs...)
	return nil
}
