// Package envelope provides a standardized event envelope for messaging
// systems (Kafka, event bus). Every event gets metadata for tracing,
// schema evolution, dead-letter handling, and replay.
package envelope

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Version is the current envelope schema version.
const Version = "1.0"

// Envelope wraps an event payload with metadata for reliable messaging.
type Envelope struct {
	// Routing.
	ID     string `json:"id"`              // Unique event ID.
	Type   string `json:"type"`            // Event type (e.g., "user.created").
	Source string `json:"source"`          // Originating service.
	Topic  string `json:"topic,omitempty"` // Target topic/channel.

	// Versioning.
	SchemaVersion string `json:"schema_version"` // Payload schema version.
	EnvVersion    string `json:"env_version"`    // Envelope format version.

	// Timing.
	Timestamp time.Time `json:"timestamp"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`

	// Tracing.
	CorrelationID string `json:"correlation_id,omitempty"`
	CausationID   string `json:"causation_id,omitempty"`
	TraceID       string `json:"trace_id,omitempty"`

	// Payload.
	ContentType string          `json:"content_type"` // MIME type of data.
	Data        json.RawMessage `json:"data"`         // The actual event payload.
	DataHash    string          `json:"data_hash"`    // SHA-256 of data for integrity.

	// Retry/DLQ.
	Attempt    int       `json:"attempt,omitempty"`
	MaxRetries int       `json:"max_retries,omitempty"`
	FirstError time.Time `json:"first_error,omitempty"`
	LastError  string    `json:"last_error,omitempty"`

	// Metadata.
	Headers map[string]string `json:"headers,omitempty"`
}

// Builder provides fluent envelope construction.
type Builder struct {
	env Envelope
}

// New creates a new envelope builder with required fields.
func New(id, eventType, source string) *Builder {
	return &Builder{
		env: Envelope{
			ID:          id,
			Type:        eventType,
			Source:      source,
			EnvVersion:  Version,
			Timestamp:   time.Now(),
			ContentType: "application/json",
			Headers:     make(map[string]string),
		},
	}
}

// Topic sets the target topic.
func (b *Builder) Topic(topic string) *Builder {
	b.env.Topic = topic
	return b
}

// SchemaVersion sets the payload schema version.
func (b *Builder) SchemaVersion(v string) *Builder {
	b.env.SchemaVersion = v
	return b
}

// Correlation sets correlation and causation IDs.
func (b *Builder) Correlation(correlationID, causationID string) *Builder {
	b.env.CorrelationID = correlationID
	b.env.CausationID = causationID
	return b
}

// TraceID sets the W3C trace ID.
func (b *Builder) TraceID(id string) *Builder {
	b.env.TraceID = id
	return b
}

// ExpiresIn sets the TTL from now.
func (b *Builder) ExpiresIn(d time.Duration) *Builder {
	b.env.ExpiresAt = time.Now().Add(d)
	return b
}

// ExpiresAt sets an absolute expiration time.
func (b *Builder) ExpiresAt(t time.Time) *Builder {
	b.env.ExpiresAt = t
	return b
}

// MaxRetries sets the retry limit for failed processing.
func (b *Builder) MaxRetries(n int) *Builder {
	b.env.MaxRetries = n
	return b
}

// Header adds a custom header.
func (b *Builder) Header(key, value string) *Builder {
	b.env.Headers[key] = value
	return b
}

// Data sets the payload and computes its hash.
func (b *Builder) Data(payload interface{}) (*Builder, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return b, fmt.Errorf("marshal payload: %w", err)
	}
	b.env.Data = data
	b.env.DataHash = computeHash(data)
	return b, nil
}

// DataRaw sets pre-serialized payload.
func (b *Builder) DataRaw(data json.RawMessage) *Builder {
	b.env.Data = data
	b.env.DataHash = computeHash(data)
	return b
}

// Build returns the completed envelope.
func (b *Builder) Build() Envelope {
	return b.env
}

func computeHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// Validate checks envelope integrity.
func (e *Envelope) Validate() error {
	if e.ID == "" {
		return fmt.Errorf("envelope: id is required")
	}
	if e.Type == "" {
		return fmt.Errorf("envelope: type is required")
	}
	if e.Source == "" {
		return fmt.Errorf("envelope: source is required")
	}
	if e.Timestamp.IsZero() {
		return fmt.Errorf("envelope: timestamp is required")
	}
	if len(e.Data) == 0 {
		return fmt.Errorf("envelope: data is required")
	}
	if e.DataHash != "" {
		expected := computeHash(e.Data)
		if e.DataHash != expected {
			return fmt.Errorf("envelope: data hash mismatch (integrity violation)")
		}
	}
	return nil
}

// IsExpired checks if the envelope has passed its TTL.
func (e *Envelope) IsExpired() bool {
	if e.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(e.ExpiresAt)
}

// CanRetry checks if the envelope has retries remaining.
func (e *Envelope) CanRetry() bool {
	if e.MaxRetries <= 0 {
		return true // No limit set.
	}
	return e.Attempt < e.MaxRetries
}

// RecordError marks a processing failure.
func (e *Envelope) RecordError(err error) {
	e.Attempt++
	e.LastError = err.Error()
	if e.FirstError.IsZero() {
		e.FirstError = time.Now()
	}
}

// Decode unmarshals the payload into the target.
func (e *Envelope) Decode(target interface{}) error {
	return json.Unmarshal(e.Data, target)
}

// Marshal serializes the full envelope to JSON.
func (e *Envelope) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// Unmarshal deserializes an envelope from JSON.
func Unmarshal(data []byte) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}
	return &env, nil
}

// Router routes envelopes to handlers by event type.
type Router struct {
	handlers map[string]Handler
	fallback Handler
}

// Handler processes an envelope.
type Handler func(env *Envelope) error

// NewRouter creates a new event router.
func NewRouter() *Router {
	return &Router{
		handlers: make(map[string]Handler),
	}
}

// On registers a handler for an event type.
func (r *Router) On(eventType string, h Handler) {
	r.handlers[eventType] = h
}

// Fallback sets the handler for unrecognized event types.
func (r *Router) Fallback(h Handler) {
	r.fallback = h
}

// Route dispatches an envelope to its handler.
func (r *Router) Route(env *Envelope) error {
	if err := env.Validate(); err != nil {
		return err
	}
	if env.IsExpired() {
		return fmt.Errorf("envelope: event %s has expired", env.ID)
	}

	h, ok := r.handlers[env.Type]
	if !ok {
		if r.fallback != nil {
			return r.fallback(env)
		}
		return fmt.Errorf("envelope: no handler for event type %q", env.Type)
	}
	return h(env)
}

// Types returns all registered event types.
func (r *Router) Types() []string {
	types := make([]string, 0, len(r.handlers))
	for t := range r.handlers {
		types = append(types, t)
	}
	return types
}
