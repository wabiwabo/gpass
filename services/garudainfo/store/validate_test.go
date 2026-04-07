package store

import (
	"strings"
	"testing"
)

func validBaseConsent() *Consent {
	return &Consent{
		UserID:          "user-1",
		ClientID:        "client-1",
		ClientName:      "Test App",
		Purpose:         "KYC",
		Fields:          map[string]bool{"name": true, "dob": false},
		DurationSeconds: 3600,
	}
}

func TestValidateConsent_Valid(t *testing.T) {
	if err := ValidateConsent(validBaseConsent()); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateConsent_NilRejected(t *testing.T) {
	if err := ValidateConsent(nil); err == nil {
		t.Error("expected error for nil consent")
	}
}

func TestValidateConsent_RequiredFields(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Consent)
		want string
	}{
		{"no user_id", func(c *Consent) { c.UserID = "" }, "user_id is required"},
		{"no client_id", func(c *Consent) { c.ClientID = "" }, "client_id is required"},
		{"no client_name", func(c *Consent) { c.ClientName = "" }, "client_name is required"},
		{"no purpose", func(c *Consent) { c.Purpose = "" }, "purpose is required"},
		{"no fields", func(c *Consent) { c.Fields = nil }, "fields is required"},
		{"empty fields", func(c *Consent) { c.Fields = map[string]bool{} }, "fields is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validBaseConsent()
			tc.mut(c)
			err := ValidateConsent(c)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("got %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestValidateConsent_DurationLimits(t *testing.T) {
	c := validBaseConsent()
	c.DurationSeconds = 0
	if err := ValidateConsent(c); err == nil {
		t.Error("expected duration too small")
	}

	c = validBaseConsent()
	c.DurationSeconds = MaxDuration + 1
	if err := ValidateConsent(c); err == nil {
		t.Error("expected duration too big")
	}
}

func TestValidateConsent_FieldKeyAllowList(t *testing.T) {
	c := validBaseConsent()
	c.Fields = map[string]bool{"hacker_field": true}
	if err := ValidateConsent(c); err == nil {
		t.Error("expected unknown field rejected")
	}
}

func TestValidateConsent_AllValuesFalse(t *testing.T) {
	c := validBaseConsent()
	c.Fields = map[string]bool{"name": false, "dob": false}
	if err := ValidateConsent(c); err == nil {
		t.Error("expected at-least-one-true rule")
	}
}

func TestValidateConsent_LengthBounds(t *testing.T) {
	c := validBaseConsent()
	c.ClientName = strings.Repeat("a", MaxClientNameLen+1)
	if err := ValidateConsent(c); err == nil {
		t.Error("expected client_name length error")
	}
}

func TestValidateConsent_NullByte(t *testing.T) {
	c := validBaseConsent()
	c.Purpose = "evil\x00purpose"
	if err := ValidateConsent(c); err == nil {
		t.Error("expected null byte rejected")
	}
}
