package auditctx

import (
	"context"
	"testing"
)

func TestWithAuditInfo_FromContext(t *testing.T) {
	info := AuditInfo{
		ActorID:   "user-123",
		ActorType: "user",
		Action:    "CREATE",
		Resource:  "entity",
	}

	ctx := WithAuditInfo(context.Background(), info)
	got, ok := FromContext(ctx)

	if !ok {
		t.Fatal("should find audit info")
	}
	if got.ActorID != "user-123" {
		t.Errorf("actor: got %q", got.ActorID)
	}
	if got.Action != "CREATE" {
		t.Errorf("action: got %q", got.Action)
	}
	if got.Timestamp.IsZero() {
		t.Error("timestamp should be set")
	}
}

func TestFromContext_Missing(t *testing.T) {
	_, ok := FromContext(context.Background())
	if ok {
		t.Error("should return false")
	}
}

func TestBuilder(t *testing.T) {
	info := NewBuilder().
		Actor("user-456", "user").
		Tenant("tenant-1").
		Action("UPDATE").
		Resource("entity", "ent-789").
		Request("req-1", "10.0.0.1", "Mozilla").
		Session("sess-1").
		Meta("field", "name").
		Build()

	if info.ActorID != "user-456" {
		t.Errorf("actor: got %q", info.ActorID)
	}
	if info.TenantID != "tenant-1" {
		t.Errorf("tenant: got %q", info.TenantID)
	}
	if info.Action != "UPDATE" {
		t.Errorf("action: got %q", info.Action)
	}
	if info.Resource != "entity" {
		t.Errorf("resource: got %q", info.Resource)
	}
	if info.ResourceID != "ent-789" {
		t.Errorf("resource ID: got %q", info.ResourceID)
	}
	if info.IPAddress != "10.0.0.1" {
		t.Errorf("IP: got %q", info.IPAddress)
	}
	if info.SessionID != "sess-1" {
		t.Errorf("session: got %q", info.SessionID)
	}
	if info.Metadata["field"] != "name" {
		t.Errorf("meta: got %v", info.Metadata)
	}
}

func TestBuilder_IntoContext(t *testing.T) {
	ctx := NewBuilder().
		Actor("admin", "service").
		Action("DELETE").
		IntoContext(context.Background())

	info, ok := FromContext(ctx)
	if !ok {
		t.Fatal("should find info")
	}
	if info.ActorType != "service" {
		t.Errorf("actor type: got %q", info.ActorType)
	}
}

func TestBuilder_TimestampAutoSet(t *testing.T) {
	info := NewBuilder().Build()
	if info.Timestamp.IsZero() {
		t.Error("timestamp should be auto-set")
	}
}
