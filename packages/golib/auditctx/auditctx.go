// Package auditctx provides audit context propagation through the
// request lifecycle. Enriches audit events with actor, resource,
// and action information extracted from HTTP requests.
package auditctx

import (
	"context"
	"time"
)

type contextKey struct{}

// AuditInfo holds audit context information.
type AuditInfo struct {
	ActorID     string            `json:"actor_id"`
	ActorType   string            `json:"actor_type"` // "user", "service", "system"
	TenantID    string            `json:"tenant_id,omitempty"`
	Action      string            `json:"action"`
	Resource    string            `json:"resource"`
	ResourceID  string            `json:"resource_id,omitempty"`
	IPAddress   string            `json:"ip_address,omitempty"`
	UserAgent   string            `json:"user_agent,omitempty"`
	RequestID   string            `json:"request_id,omitempty"`
	SessionID   string            `json:"session_id,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// FromContext extracts audit info from context.
func FromContext(ctx context.Context) (AuditInfo, bool) {
	info, ok := ctx.Value(contextKey{}).(AuditInfo)
	return info, ok
}

// WithAuditInfo stores audit info in context.
func WithAuditInfo(ctx context.Context, info AuditInfo) context.Context {
	if info.Timestamp.IsZero() {
		info.Timestamp = time.Now()
	}
	return context.WithValue(ctx, contextKey{}, info)
}

// Builder provides fluent audit info construction.
type Builder struct {
	info AuditInfo
}

// NewBuilder creates an audit info builder.
func NewBuilder() *Builder {
	return &Builder{
		info: AuditInfo{
			Timestamp: time.Now(),
			Metadata:  make(map[string]string),
		},
	}
}

// Actor sets the actor identity.
func (b *Builder) Actor(id, actorType string) *Builder {
	b.info.ActorID = id
	b.info.ActorType = actorType
	return b
}

// Tenant sets the tenant context.
func (b *Builder) Tenant(id string) *Builder {
	b.info.TenantID = id
	return b
}

// Action sets the action being performed.
func (b *Builder) Action(action string) *Builder {
	b.info.Action = action
	return b
}

// Resource sets the target resource.
func (b *Builder) Resource(resource, resourceID string) *Builder {
	b.info.Resource = resource
	b.info.ResourceID = resourceID
	return b
}

// Request sets request metadata.
func (b *Builder) Request(requestID, ipAddress, userAgent string) *Builder {
	b.info.RequestID = requestID
	b.info.IPAddress = ipAddress
	b.info.UserAgent = userAgent
	return b
}

// Session sets the session ID.
func (b *Builder) Session(sessionID string) *Builder {
	b.info.SessionID = sessionID
	return b
}

// Meta adds a metadata key-value pair.
func (b *Builder) Meta(key, value string) *Builder {
	b.info.Metadata[key] = value
	return b
}

// Build returns the constructed audit info.
func (b *Builder) Build() AuditInfo {
	return b.info
}

// IntoContext stores the built audit info in context.
func (b *Builder) IntoContext(ctx context.Context) context.Context {
	return WithAuditInfo(ctx, b.info)
}
