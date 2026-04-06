// Package difftrack tracks changes between old and new values
// for audit logging. Records which fields changed, their before/after
// values, enabling change history and rollback capabilities.
package difftrack

import (
	"encoding/json"
	"time"
)

// Change represents a single field change.
type Change struct {
	Field    string      `json:"field"`
	OldValue interface{} `json:"old_value"`
	NewValue interface{} `json:"new_value"`
}

// Diff is a collection of changes.
type Diff struct {
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id"`
	Changes      []Change  `json:"changes"`
	Timestamp    time.Time `json:"timestamp"`
	ActorID      string    `json:"actor_id,omitempty"`
}

// HasChanges returns true if there are any changes.
func (d Diff) HasChanges() bool {
	return len(d.Changes) > 0
}

// ChangedFields returns the names of changed fields.
func (d Diff) ChangedFields() []string {
	fields := make([]string, len(d.Changes))
	for i, c := range d.Changes {
		fields[i] = c.Field
	}
	return fields
}

// JSON returns the diff as JSON.
func (d Diff) JSON() ([]byte, error) {
	return json.Marshal(d)
}

// Builder constructs a diff by comparing field values.
type Builder struct {
	resourceType string
	resourceID   string
	changes      []Change
}

// NewBuilder creates a diff builder.
func NewBuilder(resourceType, resourceID string) *Builder {
	return &Builder{
		resourceType: resourceType,
		resourceID:   resourceID,
	}
}

// Compare adds a change if old != new.
func (b *Builder) Compare(field string, oldVal, newVal interface{}) *Builder {
	if !equal(oldVal, newVal) {
		b.changes = append(b.changes, Change{
			Field:    field,
			OldValue: oldVal,
			NewValue: newVal,
		})
	}
	return b
}

// CompareString compares string values.
func (b *Builder) CompareString(field, oldVal, newVal string) *Builder {
	if oldVal != newVal {
		b.changes = append(b.changes, Change{
			Field:    field,
			OldValue: oldVal,
			NewValue: newVal,
		})
	}
	return b
}

// CompareInt compares integer values.
func (b *Builder) CompareInt(field string, oldVal, newVal int) *Builder {
	if oldVal != newVal {
		b.changes = append(b.changes, Change{
			Field:    field,
			OldValue: oldVal,
			NewValue: newVal,
		})
	}
	return b
}

// CompareBool compares boolean values.
func (b *Builder) CompareBool(field string, oldVal, newVal bool) *Builder {
	if oldVal != newVal {
		b.changes = append(b.changes, Change{
			Field:    field,
			OldValue: oldVal,
			NewValue: newVal,
		})
	}
	return b
}

// Build finalizes the diff.
func (b *Builder) Build(actorID string) Diff {
	return Diff{
		ResourceType: b.resourceType,
		ResourceID:   b.resourceID,
		Changes:      b.changes,
		Timestamp:    time.Now().UTC(),
		ActorID:      actorID,
	}
}

func equal(a, b interface{}) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}

// CompareStringMaps computes changes between two string maps.
func CompareStringMaps(field string, old, new map[string]string) []Change {
	var changes []Change

	for k, ov := range old {
		if nv, ok := new[k]; !ok {
			changes = append(changes, Change{
				Field:    field + "." + k,
				OldValue: ov,
				NewValue: nil,
			})
		} else if ov != nv {
			changes = append(changes, Change{
				Field:    field + "." + k,
				OldValue: ov,
				NewValue: nv,
			})
		}
	}

	for k, nv := range new {
		if _, ok := old[k]; !ok {
			changes = append(changes, Change{
				Field:    field + "." + k,
				OldValue: nil,
				NewValue: nv,
			})
		}
	}

	return changes
}
