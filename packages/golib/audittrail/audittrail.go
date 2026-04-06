package audittrail

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// Action represents a type of auditable action.
type Action string

const (
	ActionCreate      Action = "CREATE"
	ActionRead        Action = "READ"
	ActionUpdate      Action = "UPDATE"
	ActionDelete      Action = "DELETE"
	ActionLogin       Action = "LOGIN"
	ActionLogout      Action = "LOGOUT"
	ActionGrant       Action = "GRANT"
	ActionRevoke      Action = "REVOKE"
	ActionExport      Action = "EXPORT"
	ActionSign        Action = "SIGN"
	ActionVerify      Action = "VERIFY"
	ActionConsent     Action = "CONSENT"
	ActionAccessDeny  Action = "ACCESS_DENIED"
)

// Entry represents a single audit trail entry.
type Entry struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	Action       Action                 `json:"action"`
	ActorID      string                 `json:"actor_id"`
	ActorType    string                 `json:"actor_type"` // "user", "service", "system"
	ResourceType string                 `json:"resource_type"`
	ResourceID   string                 `json:"resource_id"`
	Description  string                 `json:"description"`
	IPAddress    string                 `json:"ip_address,omitempty"`
	UserAgent    string                 `json:"user_agent,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Result       string                 `json:"result"` // "success", "failure", "denied"
	Service      string                 `json:"service"`
}

// Builder provides a fluent API for constructing audit entries.
type Builder struct {
	entry Entry
}

// NewEntry starts building a new audit trail entry.
func NewEntry(action Action) *Builder {
	return &Builder{
		entry: Entry{
			Timestamp: time.Now().UTC(),
			Action:    action,
			Result:    "success",
			Metadata:  make(map[string]interface{}),
		},
	}
}

func (b *Builder) Actor(id, actorType string) *Builder {
	b.entry.ActorID = id
	b.entry.ActorType = actorType
	return b
}

func (b *Builder) Resource(resourceType, resourceID string) *Builder {
	b.entry.ResourceType = resourceType
	b.entry.ResourceID = resourceID
	return b
}

func (b *Builder) Description(desc string) *Builder {
	b.entry.Description = desc
	return b
}

func (b *Builder) IP(ip string) *Builder {
	b.entry.IPAddress = ip
	return b
}

func (b *Builder) UA(ua string) *Builder {
	b.entry.UserAgent = ua
	return b
}

func (b *Builder) Meta(key string, value interface{}) *Builder {
	b.entry.Metadata[key] = value
	return b
}

func (b *Builder) Success() *Builder {
	b.entry.Result = "success"
	return b
}

func (b *Builder) Failure() *Builder {
	b.entry.Result = "failure"
	return b
}

func (b *Builder) Denied() *Builder {
	b.entry.Result = "denied"
	return b
}

func (b *Builder) Service(name string) *Builder {
	b.entry.Service = name
	return b
}

func (b *Builder) Build() Entry {
	return b.entry
}

// Sink receives audit entries for storage/processing.
type Sink interface {
	Record(ctx context.Context, entry Entry) error
}

// MemorySink stores entries in memory for testing.
type MemorySink struct {
	mu      sync.RWMutex
	entries []Entry
}

func NewMemorySink() *MemorySink {
	return &MemorySink{}
}

func (s *MemorySink) Record(_ context.Context, entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entry)
	return nil
}

func (s *MemorySink) Entries() []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Entry, len(s.entries))
	copy(result, s.entries)
	return result
}

func (s *MemorySink) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

func (s *MemorySink) FindByAction(action Action) []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Entry
	for _, e := range s.entries {
		if e.Action == action {
			result = append(result, e)
		}
	}
	return result
}

func (s *MemorySink) FindByActor(actorID string) []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Entry
	for _, e := range s.entries {
		if e.ActorID == actorID {
			result = append(result, e)
		}
	}
	return result
}

// JSON returns the entry as a JSON string.
func (e Entry) JSON() string {
	data, _ := json.Marshal(e)
	return string(data)
}
