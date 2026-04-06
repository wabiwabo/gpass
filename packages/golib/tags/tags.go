package tags

import (
	"context"
	"sort"
	"strings"
	"sync"
)

// Tags represents a set of key-value observability tags attached to a request.
// Used for metrics labeling, log enrichment, and trace attributes.
type Tags struct {
	mu   sync.RWMutex
	data map[string]string
}

// New creates a new Tags set.
func New() *Tags {
	return &Tags{data: make(map[string]string)}
}

// FromMap creates Tags from an existing map.
func FromMap(m map[string]string) *Tags {
	t := New()
	for k, v := range m {
		t.data[k] = v
	}
	return t
}

// Set adds or updates a tag.
func (t *Tags) Set(key, value string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.data[key] = value
}

// Get retrieves a tag value.
func (t *Tags) Get(key string) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	v, ok := t.data[key]
	return v, ok
}

// Delete removes a tag.
func (t *Tags) Delete(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.data, key)
}

// All returns a copy of all tags.
func (t *Tags) All() map[string]string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make(map[string]string, len(t.data))
	for k, v := range t.data {
		result[k] = v
	}
	return result
}

// Len returns the number of tags.
func (t *Tags) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.data)
}

// String returns tags as a sorted "key=value,key=value" string.
func (t *Tags) String() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	keys := make([]string, 0, len(t.data))
	for k := range t.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(t.data[k])
	}
	return b.String()
}

// Merge combines another Tags set into this one. Existing keys are overwritten.
func (t *Tags) Merge(other *Tags) {
	if other == nil {
		return
	}
	other.mu.RLock()
	defer other.mu.RUnlock()
	t.mu.Lock()
	defer t.mu.Unlock()
	for k, v := range other.data {
		t.data[k] = v
	}
}

// Context key for Tags.
type ctxKey struct{}

// ToContext stores Tags in the context.
func ToContext(ctx context.Context, tags *Tags) context.Context {
	return context.WithValue(ctx, ctxKey{}, tags)
}

// FromContext retrieves Tags from the context.
// Returns a new empty Tags if not found.
func FromContext(ctx context.Context) *Tags {
	if tags, ok := ctx.Value(ctxKey{}).(*Tags); ok {
		return tags
	}
	return New()
}

// WithTags returns middleware-style enrichment by adding standard tags.
func WithService(tags *Tags, service, version, env string) {
	tags.Set("service", service)
	tags.Set("version", version)
	tags.Set("environment", env)
}
