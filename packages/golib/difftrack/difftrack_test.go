package difftrack

import (
	"encoding/json"
	"testing"
)

func TestBuilder_NoChanges(t *testing.T) {
	d := NewBuilder("user", "u-1").
		CompareString("name", "John", "John").
		CompareInt("age", 30, 30).
		Build("admin-1")

	if d.HasChanges() {
		t.Error("should have no changes")
	}
}

func TestBuilder_StringChange(t *testing.T) {
	d := NewBuilder("user", "u-1").
		CompareString("name", "John", "Jane").
		CompareString("email", "a@b.com", "a@b.com").
		Build("admin-1")

	if !d.HasChanges() {
		t.Fatal("should have changes")
	}
	if len(d.Changes) != 1 {
		t.Fatalf("changes = %d", len(d.Changes))
	}
	if d.Changes[0].Field != "name" {
		t.Errorf("field = %q", d.Changes[0].Field)
	}
	if d.Changes[0].OldValue != "John" {
		t.Errorf("old = %v", d.Changes[0].OldValue)
	}
	if d.Changes[0].NewValue != "Jane" {
		t.Errorf("new = %v", d.Changes[0].NewValue)
	}
}

func TestBuilder_IntChange(t *testing.T) {
	d := NewBuilder("config", "c-1").
		CompareInt("max_retries", 3, 5).
		Build("")

	if len(d.Changes) != 1 {
		t.Fatalf("changes = %d", len(d.Changes))
	}
}

func TestBuilder_BoolChange(t *testing.T) {
	d := NewBuilder("user", "u-1").
		CompareBool("active", true, false).
		Build("")

	if len(d.Changes) != 1 {
		t.Fatalf("changes = %d", len(d.Changes))
	}
}

func TestBuilder_Compare_Interface(t *testing.T) {
	d := NewBuilder("entity", "e-1").
		Compare("metadata", map[string]string{"a": "1"}, map[string]string{"a": "2"}).
		Build("")

	if !d.HasChanges() {
		t.Error("should detect map change")
	}
}

func TestBuilder_MultipleChanges(t *testing.T) {
	d := NewBuilder("user", "u-1").
		CompareString("name", "A", "B").
		CompareString("email", "old@x.com", "new@x.com").
		CompareInt("age", 25, 26).
		CompareBool("active", true, false).
		Build("admin")

	if len(d.Changes) != 4 {
		t.Errorf("changes = %d, want 4", len(d.Changes))
	}
}

func TestDiff_Metadata(t *testing.T) {
	d := NewBuilder("user", "u-123").
		CompareString("name", "A", "B").
		Build("admin-1")

	if d.ResourceType != "user" {
		t.Errorf("ResourceType = %q", d.ResourceType)
	}
	if d.ResourceID != "u-123" {
		t.Errorf("ResourceID = %q", d.ResourceID)
	}
	if d.ActorID != "admin-1" {
		t.Errorf("ActorID = %q", d.ActorID)
	}
	if d.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestDiff_ChangedFields(t *testing.T) {
	d := NewBuilder("user", "u-1").
		CompareString("name", "A", "B").
		CompareString("email", "a@b", "c@d").
		Build("")

	fields := d.ChangedFields()
	if len(fields) != 2 {
		t.Fatalf("fields = %d", len(fields))
	}
	if fields[0] != "name" || fields[1] != "email" {
		t.Errorf("fields = %v", fields)
	}
}

func TestDiff_JSON(t *testing.T) {
	d := NewBuilder("user", "u-1").
		CompareString("name", "Old", "New").
		Build("actor")

	data, err := d.JSON()
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["resource_type"] != "user" {
		t.Errorf("resource_type = %v", decoded["resource_type"])
	}
}

func TestCompareStringMaps(t *testing.T) {
	old := map[string]string{"a": "1", "b": "2", "c": "3"}
	new := map[string]string{"a": "1", "b": "changed", "d": "4"}

	changes := CompareStringMaps("meta", old, new)

	// b changed, c removed, d added = 3 changes
	if len(changes) != 3 {
		t.Errorf("changes = %d, want 3", len(changes))
	}
}

func TestCompareStringMaps_NoChanges(t *testing.T) {
	m := map[string]string{"a": "1"}
	changes := CompareStringMaps("meta", m, m)
	if len(changes) != 0 {
		t.Errorf("changes = %d, want 0", len(changes))
	}
}
