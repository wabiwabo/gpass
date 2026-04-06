package csrftoken

import (
	"strings"
	"testing"
	"time"
)

var testCfg = Config{Secret: []byte("test-csrf-secret-32bytes-long!!!")}

func TestGenerate(t *testing.T) {
	token, err := Generate(testCfg)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if token == "" {
		t.Error("should not be empty")
	}
	parts := strings.SplitN(token, ":", 3)
	if len(parts) != 3 {
		t.Errorf("should have 3 parts, got %d", len(parts))
	}
}

func TestGenerate_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		tok, _ := Generate(testCfg)
		if seen[tok] {
			t.Fatal("duplicate token")
		}
		seen[tok] = true
	}
}

func TestValidate_Valid(t *testing.T) {
	token, _ := Generate(testCfg)
	if err := Validate(testCfg, token); err != nil {
		t.Errorf("valid token rejected: %v", err)
	}
}

func TestValidate_InvalidFormat(t *testing.T) {
	if err := Validate(testCfg, "invalid"); err == nil {
		t.Error("should reject invalid format")
	}
	if err := Validate(testCfg, "a:b"); err == nil {
		t.Error("should reject 2-part format")
	}
}

func TestValidate_TamperedSignature(t *testing.T) {
	token, _ := Generate(testCfg)
	parts := strings.SplitN(token, ":", 3)
	tampered := parts[0] + ":" + parts[1] + ":deadbeef"

	if err := Validate(testCfg, tampered); err == nil {
		t.Error("should reject tampered signature")
	}
}

func TestValidate_WrongSecret(t *testing.T) {
	token, _ := Generate(testCfg)
	wrongCfg := Config{Secret: []byte("wrong-secret")}

	if err := Validate(wrongCfg, token); err == nil {
		t.Error("should reject with wrong secret")
	}
}

func TestValidate_Expired(t *testing.T) {
	cfg := Config{Secret: testCfg.Secret, TTL: 1 * time.Nanosecond}
	token, _ := Generate(cfg)

	time.Sleep(1 * time.Millisecond)

	if err := Validate(cfg, token); err == nil {
		t.Error("should reject expired token")
	}
}

func TestValidate_DefaultTTL(t *testing.T) {
	// Without TTL set, should use 1 hour default
	token, _ := Generate(testCfg)
	if err := Validate(testCfg, token); err != nil {
		t.Errorf("should be valid within default TTL: %v", err)
	}
}

func TestValidate_TamperedPayload(t *testing.T) {
	token, _ := Generate(testCfg)
	parts := strings.SplitN(token, ":", 3)
	// Change nonce but keep signature
	tampered := "AAAA" + parts[0][4:] + ":" + parts[1] + ":" + parts[2]

	if err := Validate(testCfg, tampered); err == nil {
		t.Error("should reject tampered payload")
	}
}
