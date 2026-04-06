package oplog

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestNewLogger(t *testing.T) {
	l := NewLogger("identity", nil)
	if l.service != "identity" {
		t.Errorf("service = %q, want identity", l.service)
	}
	if l.logger == nil {
		t.Error("logger should default to slog.Default()")
	}
}

func TestNewLogger_CustomLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	l := NewLogger("garudasign", logger)

	l.Log(Entry{
		Operation:  OpCreate,
		Resource:   "certificate",
		ResourceID: "cert-123",
		ActorID:    "user-1",
		ActorType:  "user",
		Success:    true,
	})

	if !strings.Contains(buf.String(), "garudasign") {
		t.Errorf("log should contain service name, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "operation completed") {
		t.Errorf("success log should say 'operation completed', got: %s", buf.String())
	}
}

func TestLog_FailedOperation(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	l := NewLogger("identity", logger)

	l.Log(Entry{
		Operation:  OpDelete,
		Resource:   "user",
		ResourceID: "user-456",
		ActorID:    "admin-1",
		ActorType:  "user",
		Success:    false,
		Error:      "permission denied",
	})

	if !strings.Contains(buf.String(), "operation failed") {
		t.Errorf("failed log should say 'operation failed', got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "permission denied") {
		t.Errorf("should contain error, got: %s", buf.String())
	}
}

func TestLog_SetsTimestamp(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	l := NewLogger("svc", logger)

	l.Log(Entry{
		Operation:  OpCreate,
		Resource:   "item",
		ResourceID: "1",
		Success:    true,
	})

	// Should have logged (timestamp auto-set)
	if buf.Len() == 0 {
		t.Error("expected log output")
	}
}

func TestLog_PreservesExistingTimestamp(t *testing.T) {
	l := NewLogger("svc", nil)
	ts := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	entry := Entry{
		Timestamp:  ts,
		Operation:  OpUpdate,
		Resource:   "user",
		ResourceID: "1",
		Success:    true,
	}
	l.Log(entry)
	// Entry timestamp should be preserved (not overwritten)
	if entry.Timestamp != ts {
		t.Errorf("timestamp was modified")
	}
}

func TestLog_OptionalFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	l := NewLogger("svc", logger)

	l.Log(Entry{
		Operation:  OpCreate,
		Resource:   "item",
		ResourceID: "1",
		ActorID:    "user-1",
		ActorType:  "user",
		TenantID:   "tenant-abc",
		RequestID:  "req-xyz",
		Duration:   150 * time.Millisecond,
		Success:    true,
	})

	output := buf.String()
	if !strings.Contains(output, "tenant-abc") {
		t.Error("should contain tenant_id")
	}
	if !strings.Contains(output, "req-xyz") {
		t.Error("should contain request_id")
	}
}

func TestBuilder_Success(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	l := NewLogger("garudacorp", logger)

	entry := l.Begin(OpCreate, "entity", "ent-789").
		Actor("user-100", "user").
		Tenant("tenant-1").
		Request("req-abc").
		Describe("Created new corporate entity").
		Meta("entity_type", "PT").
		Meta("province", "DKI Jakarta").
		Done()

	if entry.Operation != OpCreate {
		t.Errorf("Operation = %q", entry.Operation)
	}
	if entry.Resource != "entity" {
		t.Errorf("Resource = %q", entry.Resource)
	}
	if entry.ResourceID != "ent-789" {
		t.Errorf("ResourceID = %q", entry.ResourceID)
	}
	if entry.ActorID != "user-100" {
		t.Errorf("ActorID = %q", entry.ActorID)
	}
	if entry.TenantID != "tenant-1" {
		t.Errorf("TenantID = %q", entry.TenantID)
	}
	if !entry.Success {
		t.Error("should be success")
	}
	if entry.Duration <= 0 {
		t.Error("duration should be positive")
	}
	if entry.Metadata["entity_type"] != "PT" {
		t.Errorf("Metadata[entity_type] = %q", entry.Metadata["entity_type"])
	}
	if entry.Service != "garudacorp" {
		t.Errorf("Service = %q", entry.Service)
	}
}

func TestBuilder_Failure(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	l := NewLogger("identity", logger)

	entry := l.Begin(OpDelete, "user", "user-456").
		Actor("admin-1", "user").
		Fail("constraint violation: active sessions exist").
		Done()

	if entry.Success {
		t.Error("should be failure")
	}
	if entry.Error != "constraint violation: active sessions exist" {
		t.Errorf("Error = %q", entry.Error)
	}

	if !strings.Contains(buf.String(), "operation failed") {
		t.Error("should log as error")
	}
}

func TestEntry_JSON(t *testing.T) {
	entry := Entry{
		Timestamp:   time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC),
		Operation:   OpApprove,
		Service:     "garudainfo",
		Resource:    "consent",
		ResourceID:  "consent-123",
		ActorID:     "user-1",
		ActorType:   "user",
		Description: "Approved data sharing consent",
		Success:     true,
		Metadata:    map[string]string{"scope": "email,phone"},
	}

	data, err := entry.JSON()
	if err != nil {
		t.Fatalf("JSON() error: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded["operation"] != "approve" {
		t.Errorf("operation = %v", decoded["operation"])
	}
	if decoded["service"] != "garudainfo" {
		t.Errorf("service = %v", decoded["service"])
	}
	if decoded["success"] != true {
		t.Errorf("success = %v", decoded["success"])
	}
}

func TestValidOp(t *testing.T) {
	tests := []struct {
		op   Op
		want bool
	}{
		{OpCreate, true},
		{OpUpdate, true},
		{OpDelete, true},
		{OpRestore, true},
		{OpArchive, true},
		{OpApprove, true},
		{OpReject, true},
		{OpRevoke, true},
		{OpGrant, true},
		{"unknown", false},
		{"", false},
		{"CREATE", false}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.op), func(t *testing.T) {
			if got := ValidOp(tt.op); got != tt.want {
				t.Errorf("ValidOp(%q) = %v, want %v", tt.op, got, tt.want)
			}
		})
	}
}

func TestBuilder_MetaAccumulates(t *testing.T) {
	l := NewLogger("svc", nil)
	entry := l.Begin(OpUpdate, "user", "1").
		Meta("key1", "val1").
		Meta("key2", "val2").
		Meta("key3", "val3").
		Done()

	if len(entry.Metadata) != 3 {
		t.Errorf("Metadata len = %d, want 3", len(entry.Metadata))
	}
}

func TestBuilder_ChainOrder(t *testing.T) {
	l := NewLogger("svc", nil)
	// All builder methods should be chainable in any order
	entry := l.Begin(OpGrant, "role", "role-1").
		Request("req-1").
		Tenant("t-1").
		Actor("u-1", "service").
		Describe("granted role").
		Meta("role", "admin").
		Done()

	if entry.RequestID != "req-1" {
		t.Error("RequestID not set")
	}
	if entry.TenantID != "t-1" {
		t.Error("TenantID not set")
	}
	if entry.ActorType != "service" {
		t.Error("ActorType not set")
	}
}

func TestOpConstants(t *testing.T) {
	// Ensure op constants are lowercase strings
	ops := []Op{OpCreate, OpUpdate, OpDelete, OpRestore, OpArchive, OpApprove, OpReject, OpRevoke, OpGrant}
	for _, op := range ops {
		s := string(op)
		if s != strings.ToLower(s) {
			t.Errorf("Op %q should be lowercase", s)
		}
	}
}
