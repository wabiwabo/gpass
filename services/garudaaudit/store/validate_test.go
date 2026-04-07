package store

import (
	"strings"
	"testing"
)

func validBase() *AuditEvent {
	return &AuditEvent{
		EventType:   "USER_LOGIN",
		ActorID:     "user-1",
		Action:      "LOGIN",
		ServiceName: "test",
	}
}

func TestValidateEvent_Valid(t *testing.T) {
	if err := ValidateEvent(validBase()); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateEvent_Required(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*AuditEvent)
		want string
	}{
		{"nil", func(*AuditEvent) {}, ""},
		{"missing event_type", func(e *AuditEvent) { e.EventType = "" }, "event_type is required"},
		{"missing actor_id", func(e *AuditEvent) { e.ActorID = "" }, "actor_id is required"},
		{"missing action", func(e *AuditEvent) { e.Action = "" }, "action is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "nil" {
				if err := ValidateEvent(nil); err == nil {
					t.Error("expected error for nil event")
				}
				return
			}
			e := validBase()
			tt.mut(e)
			err := ValidateEvent(e)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("got %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestValidateEvent_LengthLimits(t *testing.T) {
	e := validBase()
	e.EventType = strings.Repeat("a", MaxEventTypeLen+1)
	if err := ValidateEvent(e); err == nil {
		t.Error("expected length error")
	}

	e = validBase()
	e.UserAgent = strings.Repeat("u", MaxUserAgentLen+1)
	if err := ValidateEvent(e); err == nil {
		t.Error("expected user_agent length error")
	}
}

func TestValidateEvent_NullByte(t *testing.T) {
	e := validBase()
	e.ActorID = "evil\x00user"
	if err := ValidateEvent(e); err == nil {
		t.Error("expected null byte rejection")
	}
}

func TestValidateEvent_BadActorType(t *testing.T) {
	e := validBase()
	e.ActorType = "ROBOT"
	if err := ValidateEvent(e); err == nil {
		t.Error("expected enum violation")
	}
}

func TestValidateEvent_BadStatus(t *testing.T) {
	e := validBase()
	e.Status = "MAYBE"
	if err := ValidateEvent(e); err == nil {
		t.Error("expected enum violation")
	}
}

func TestValidateEvent_MetadataLimits(t *testing.T) {
	e := validBase()
	e.Metadata = make(map[string]string, MaxMetadataKeys+1)
	for i := 0; i <= MaxMetadataKeys; i++ {
		e.Metadata[string(rune('a'+i%26))+strings.Repeat("x", i)] = "v"
	}
	if err := ValidateEvent(e); err == nil {
		t.Error("expected too-many-keys error")
	}

	e = validBase()
	e.Metadata = map[string]string{"k": strings.Repeat("v", MaxMetadataValueLen+1)}
	if err := ValidateEvent(e); err == nil {
		t.Error("expected value-too-long error")
	}

	e = validBase()
	e.Metadata = map[string]string{"": "v"}
	if err := ValidateEvent(e); err == nil {
		t.Error("expected empty-key error")
	}

	e = validBase()
	e.Metadata = map[string]string{"k\nbad": "v"}
	if err := ValidateEvent(e); err == nil {
		t.Error("expected control-char rejection")
	}
}
