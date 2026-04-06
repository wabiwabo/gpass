// Package oplog provides structured operation logging for tracking
// service mutations (create, update, delete) with actor, target,
// and metadata for audit trails and debugging.
package oplog

import (
	"encoding/json"
	"log/slog"
	"time"
)

// Op represents an operation type.
type Op string

const (
	OpCreate  Op = "create"
	OpUpdate  Op = "update"
	OpDelete  Op = "delete"
	OpRestore Op = "restore"
	OpArchive Op = "archive"
	OpApprove Op = "approve"
	OpReject  Op = "reject"
	OpRevoke  Op = "revoke"
	OpGrant   Op = "grant"
)

// Entry is a single operation log entry.
type Entry struct {
	Timestamp   time.Time         `json:"timestamp"`
	Operation   Op                `json:"operation"`
	Service     string            `json:"service"`
	Resource    string            `json:"resource"`
	ResourceID  string            `json:"resource_id"`
	ActorID     string            `json:"actor_id"`
	ActorType   string            `json:"actor_type"` // "user", "service", "system"
	TenantID    string            `json:"tenant_id,omitempty"`
	RequestID   string            `json:"request_id,omitempty"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Duration    time.Duration     `json:"duration,omitempty"`
	Success     bool              `json:"success"`
	Error       string            `json:"error,omitempty"`
}

// Logger writes operation log entries.
type Logger struct {
	service string
	logger  *slog.Logger
}

// NewLogger creates an operation logger for a service.
func NewLogger(service string, logger *slog.Logger) *Logger {
	if logger == nil {
		logger = slog.Default()
	}
	return &Logger{service: service, logger: logger}
}

// Log writes an operation log entry.
func (l *Logger) Log(entry Entry) {
	entry.Service = l.service
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	attrs := []slog.Attr{
		slog.String("operation", string(entry.Operation)),
		slog.String("service", entry.Service),
		slog.String("resource", entry.Resource),
		slog.String("resource_id", entry.ResourceID),
		slog.String("actor_id", entry.ActorID),
		slog.String("actor_type", entry.ActorType),
		slog.Bool("success", entry.Success),
	}

	if entry.TenantID != "" {
		attrs = append(attrs, slog.String("tenant_id", entry.TenantID))
	}
	if entry.RequestID != "" {
		attrs = append(attrs, slog.String("request_id", entry.RequestID))
	}
	if entry.Duration > 0 {
		attrs = append(attrs, slog.Duration("duration", entry.Duration))
	}
	if entry.Error != "" {
		attrs = append(attrs, slog.String("error", entry.Error))
	}

	args := make([]any, len(attrs))
	for i, a := range attrs {
		args[i] = a
	}

	if entry.Success {
		l.logger.Info("operation completed", args...)
	} else {
		l.logger.Error("operation failed", args...)
	}
}

// Builder provides a fluent API for constructing log entries.
type Builder struct {
	logger *Logger
	entry  Entry
	start  time.Time
}

// Begin starts building an operation log entry.
func (l *Logger) Begin(op Op, resource, resourceID string) *Builder {
	return &Builder{
		logger: l,
		start:  time.Now(),
		entry: Entry{
			Operation:  op,
			Resource:   resource,
			ResourceID: resourceID,
			Success:    true,
		},
	}
}

// Actor sets who performed the operation.
func (b *Builder) Actor(id, actorType string) *Builder {
	b.entry.ActorID = id
	b.entry.ActorType = actorType
	return b
}

// Tenant sets the tenant context.
func (b *Builder) Tenant(id string) *Builder {
	b.entry.TenantID = id
	return b
}

// Request sets the request ID.
func (b *Builder) Request(id string) *Builder {
	b.entry.RequestID = id
	return b
}

// Describe sets a human-readable description.
func (b *Builder) Describe(desc string) *Builder {
	b.entry.Description = desc
	return b
}

// Meta adds a metadata key-value pair.
func (b *Builder) Meta(key, value string) *Builder {
	if b.entry.Metadata == nil {
		b.entry.Metadata = make(map[string]string)
	}
	b.entry.Metadata[key] = value
	return b
}

// Fail marks the operation as failed with an error message.
func (b *Builder) Fail(err string) *Builder {
	b.entry.Success = false
	b.entry.Error = err
	return b
}

// Done completes and logs the entry.
func (b *Builder) Done() Entry {
	b.entry.Service = b.logger.service
	b.entry.Duration = time.Since(b.start)
	b.logger.Log(b.entry)
	return b.entry
}

// JSON returns the entry as a JSON byte slice.
func (e Entry) JSON() ([]byte, error) {
	return json.Marshal(e)
}

// ValidOp checks if an operation type is one of the predefined values.
func ValidOp(op Op) bool {
	switch op {
	case OpCreate, OpUpdate, OpDelete, OpRestore, OpArchive,
		OpApprove, OpReject, OpRevoke, OpGrant:
		return true
	}
	return false
}
