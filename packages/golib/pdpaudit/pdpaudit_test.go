package pdpaudit

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestNewLogger(t *testing.T) {
	l := NewLogger("GarudaPass", nil)
	if l.controller != "GarudaPass" {
		t.Errorf("controller = %q", l.controller)
	}
}

func TestRecord(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	l := NewLogger("GarudaPass", logger)

	l.Record(Event{
		EventType:   EventAccess,
		DataSubject: "user-123",
		Processor:   "identity-service",
		LawfulBasis: BasisConsent,
		Purpose:     "identity verification",
		ActorID:     "admin-1",
		Outcome:     "success",
	})

	output := buf.String()
	if !strings.Contains(output, "pdp_audit") {
		t.Errorf("should contain pdp_audit: %s", output)
	}
	if !strings.Contains(output, "data_access") {
		t.Errorf("should contain event_type: %s", output)
	}
	if !strings.Contains(output, "GarudaPass") {
		t.Errorf("should contain controller: %s", output)
	}
}

func TestRecord_SetsDefaults(t *testing.T) {
	l := NewLogger("Controller", nil)

	event := Event{
		EventType:   EventProcess,
		DataSubject: "user-1",
		Outcome:     "success",
	}
	l.Record(event)

	// RetainUntil should be set (5 years)
	// We can't check the event directly since Record takes a copy,
	// so we test via the builder instead.
}

func TestBuilder_Success(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	l := NewLogger("GarudaPass", logger)

	event := l.Begin(EventAccess, "user-123").
		Processor("identity-service").
		Basis(BasisConsent).
		Fields(ScopeNIK, ScopeEmail).
		Purpose("identity verification").
		Consent("consent-456").
		Actor("admin-1", "admin").
		Request("req-789").
		Meta("ip", "203.0.113.50").
		Done()

	if event.EventType != EventAccess {
		t.Errorf("EventType = %q", event.EventType)
	}
	if event.DataSubject != "user-123" {
		t.Errorf("DataSubject = %q", event.DataSubject)
	}
	if event.Controller != "GarudaPass" {
		t.Errorf("Controller = %q", event.Controller)
	}
	if event.LawfulBasis != BasisConsent {
		t.Errorf("LawfulBasis = %q", event.LawfulBasis)
	}
	if len(event.DataFields) != 2 {
		t.Errorf("DataFields = %v", event.DataFields)
	}
	if event.ConsentID != "consent-456" {
		t.Errorf("ConsentID = %q", event.ConsentID)
	}
	if event.Outcome != "success" {
		t.Errorf("Outcome = %q", event.Outcome)
	}
	if event.RetainUntil.IsZero() {
		t.Error("RetainUntil should be set")
	}

	// Check 5-year retention
	retentionYears := event.RetainUntil.Sub(event.Timestamp).Hours() / 24 / 365
	if retentionYears < 4.9 || retentionYears > 5.1 {
		t.Errorf("retention = %.1f years, want ~5", retentionYears)
	}
}

func TestBuilder_Denied(t *testing.T) {
	l := NewLogger("Controller", nil)

	event := l.Begin(EventAccess, "user-123").
		Denied("insufficient consent").
		Done()

	if event.Outcome != "denied" {
		t.Errorf("Outcome = %q, want denied", event.Outcome)
	}
	if event.Reason != "insufficient consent" {
		t.Errorf("Reason = %q", event.Reason)
	}
}

func TestBuilder_Failed(t *testing.T) {
	l := NewLogger("Controller", nil)

	event := l.Begin(EventProcess, "user-123").
		Failed("database error").
		Done()

	if event.Outcome != "error" {
		t.Errorf("Outcome = %q, want error", event.Outcome)
	}
}

func TestBuilder_DataSharing(t *testing.T) {
	l := NewLogger("GarudaPass", nil)

	event := l.Begin(EventShare, "user-123").
		Processor("garudainfo").
		Recipient("third-party-corp").
		Basis(BasisConsent).
		Fields("name", "email").
		Purpose("KYC verification").
		Done()

	if event.Recipient != "third-party-corp" {
		t.Errorf("Recipient = %q", event.Recipient)
	}
}

func TestEvent_JSON(t *testing.T) {
	event := Event{
		Timestamp:   time.Now().UTC(),
		EventType:   EventConsent,
		DataSubject: "user-1",
		Controller:  "GarudaPass",
		LawfulBasis: BasisConsent,
		Outcome:     "success",
		RetainUntil: time.Now().Add(5 * 365 * 24 * time.Hour),
	}

	data, err := event.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded["event_type"] != "consent_change" {
		t.Errorf("event_type = %v", decoded["event_type"])
	}
}

func TestValidEventType(t *testing.T) {
	valid := []EventType{EventAccess, EventProcess, EventShare, EventDelete,
		EventExport, EventConsent, EventBreach, EventRetention}

	for _, et := range valid {
		if !ValidEventType(et) {
			t.Errorf("%q should be valid", et)
		}
	}

	if ValidEventType("unknown") {
		t.Error("unknown should not be valid")
	}
}

func TestValidLawfulBasis(t *testing.T) {
	valid := []LawfulBasis{BasisConsent, BasisContract, BasisLegalObligation,
		BasisVitalInterest, BasisPublicInterest, BasisLegitimateInterest}

	for _, lb := range valid {
		if !ValidLawfulBasis(lb) {
			t.Errorf("%q should be valid", lb)
		}
	}

	if ValidLawfulBasis("unknown") {
		t.Error("unknown should not be valid")
	}
}

func TestBuilder_MetaAccumulates(t *testing.T) {
	l := NewLogger("C", nil)
	event := l.Begin(EventAccess, "user").
		Meta("key1", "val1").
		Meta("key2", "val2").
		Done()

	if len(event.Metadata) != 2 {
		t.Errorf("Metadata len = %d", len(event.Metadata))
	}
}

// Sentinel scope constants for test readability
const (
	ScopeNIK   = "nik"
	ScopeEmail = "email"
)
