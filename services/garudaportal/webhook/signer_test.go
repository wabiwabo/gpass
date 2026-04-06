package webhook

import (
	"testing"
	"time"
)

func TestSign_Format(t *testing.T) {
	payload := []byte(`{"event":"test"}`)
	secret := "whsec_test_secret_123"
	ts := time.Now().Unix()

	sig := Sign(payload, secret, ts)

	if sig == "" {
		t.Fatal("expected non-empty signature")
	}
	if len(sig) < 10 {
		t.Errorf("signature too short: %s", sig)
	}
}

func TestSignAndVerify_RoundTrip(t *testing.T) {
	payload := []byte(`{"event":"identity.verified","user_id":"123"}`)
	secret := "whsec_test_secret_abc"
	ts := time.Now().Unix()

	sig := Sign(payload, secret, ts)
	valid := Verify(payload, secret, sig, 5*time.Minute)

	if !valid {
		t.Error("expected valid signature after round trip")
	}
}

func TestVerify_TamperedPayload(t *testing.T) {
	payload := []byte(`{"event":"identity.verified"}`)
	secret := "whsec_test_secret_abc"
	ts := time.Now().Unix()

	sig := Sign(payload, secret, ts)

	tampered := []byte(`{"event":"identity.verified","hacked":true}`)
	valid := Verify(tampered, secret, sig, 5*time.Minute)

	if valid {
		t.Error("expected invalid signature for tampered payload")
	}
}

func TestVerify_ExpiredTimestamp(t *testing.T) {
	payload := []byte(`{"event":"test"}`)
	secret := "whsec_test_secret_abc"
	// Timestamp from 10 minutes ago
	ts := time.Now().Add(-10 * time.Minute).Unix()

	sig := Sign(payload, secret, ts)
	valid := Verify(payload, secret, sig, 5*time.Minute)

	if valid {
		t.Error("expected invalid signature for expired timestamp")
	}
}

func TestVerify_InvalidFormat(t *testing.T) {
	payload := []byte(`{"event":"test"}`)
	secret := "whsec_test_secret_abc"

	tests := []struct {
		name string
		sig  string
	}{
		{"empty", ""},
		{"no comma", "t=123v1=abc"},
		{"missing t=", "x=123,v1=abc"},
		{"missing v1=", "t=123,x=abc"},
		{"invalid timestamp", "t=notanumber,v1=abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := Verify(payload, secret, tt.sig, 5*time.Minute)
			if valid {
				t.Errorf("expected invalid for signature %q", tt.sig)
			}
		})
	}
}

func TestVerify_WrongSecret(t *testing.T) {
	payload := []byte(`{"event":"test"}`)
	ts := time.Now().Unix()

	sig := Sign(payload, "secret1", ts)
	valid := Verify(payload, "secret2", sig, 5*time.Minute)

	if valid {
		t.Error("expected invalid signature with wrong secret")
	}
}
