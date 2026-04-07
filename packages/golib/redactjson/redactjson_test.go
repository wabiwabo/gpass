package redactjson

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactSimple(t *testing.T) {
	r := New()
	input := map[string]interface{}{
		"name":     "Alice",
		"password": "secret123",
	}
	out := r.Redact(input).(map[string]interface{})

	if out["name"] != "Alice" {
		t.Errorf("name should not be redacted: %v", out["name"])
	}
	if out["password"] != "[REDACTED]" {
		t.Errorf("password should be redacted: %v", out["password"])
	}
}

func TestRedactCaseInsensitive(t *testing.T) {
	r := New()
	input := map[string]interface{}{
		"Password": "secret",
		"PASSWORD": "secret2",
		"password": "secret3",
	}
	out := r.Redact(input).(map[string]interface{})

	for k, v := range out {
		if v != "[REDACTED]" {
			t.Errorf("%s not redacted: %v", k, v)
		}
	}
}

func TestRedactNested(t *testing.T) {
	r := New()
	input := map[string]interface{}{
		"user": map[string]interface{}{
			"name":     "Alice",
			"password": "secret",
		},
	}
	out := r.Redact(input).(map[string]interface{})
	user := out["user"].(map[string]interface{})

	if user["password"] != "[REDACTED]" {
		t.Errorf("nested password should be redacted")
	}
	if user["name"] != "Alice" {
		t.Error("nested name should be preserved")
	}
}

func TestRedactArray(t *testing.T) {
	r := New()
	input := []interface{}{
		map[string]interface{}{"name": "a", "token": "tok1"},
		map[string]interface{}{"name": "b", "token": "tok2"},
	}
	out := r.Redact(input).([]interface{})

	for _, item := range out {
		m := item.(map[string]interface{})
		if m["token"] != "[REDACTED]" {
			t.Errorf("token should be redacted: %v", m["token"])
		}
	}
}

func TestRedactBytes(t *testing.T) {
	r := New()
	input := []byte(`{"name":"alice","password":"secret","nested":{"api_key":"abc123"}}`)
	out, err := r.RedactBytes(input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(out, &result)

	if result["password"] != "[REDACTED]" {
		t.Error("password should be redacted")
	}
	nested := result["nested"].(map[string]interface{})
	if nested["api_key"] != "[REDACTED]" {
		t.Error("api_key should be redacted")
	}
}

func TestRedactBytesInvalid(t *testing.T) {
	r := New()
	_, err := r.RedactBytes([]byte("{invalid"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestRedactString(t *testing.T) {
	r := New()
	input := `{"user":"alice","secret":"hush"}`
	out, err := r.RedactString(input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Errorf("should contain redaction: %s", out)
	}
	if strings.Contains(out, "hush") {
		t.Errorf("should not contain secret: %s", out)
	}
}

func TestAddField(t *testing.T) {
	r := New()
	r.AddField("custom_secret")

	input := map[string]interface{}{"custom_secret": "value"}
	out := r.Redact(input).(map[string]interface{})

	if out["custom_secret"] != "[REDACTED]" {
		t.Error("custom field should be redacted")
	}
}

func TestRedactPrimitive(t *testing.T) {
	r := New()
	if got := r.Redact("string"); got != "string" {
		t.Errorf("string passthrough: %v", got)
	}
	if got := r.Redact(42); got != 42 {
		t.Errorf("int passthrough: %v", got)
	}
	if got := r.Redact(nil); got != nil {
		t.Error("nil should pass through")
	}
}

func TestCustomReplacement(t *testing.T) {
	r := New()
	r.Replacement = "***"
	input := map[string]interface{}{"password": "secret"}
	out := r.Redact(input).(map[string]interface{})
	if out["password"] != "***" {
		t.Errorf("custom replacement: %v", out["password"])
	}
}

func TestDefaultSensitiveFieldsCount(t *testing.T) {
	if len(DefaultSensitiveFields) < 10 {
		t.Errorf("should have at least 10 default fields, got %d", len(DefaultSensitiveFields))
	}
}

func TestNestedArrayInMap(t *testing.T) {
	r := New()
	input := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{"name": "a", "pin": "1234"},
		},
	}
	out := r.Redact(input).(map[string]interface{})
	users := out["users"].([]interface{})
	user := users[0].(map[string]interface{})
	if user["pin"] != "[REDACTED]" {
		t.Error("pin in nested array should be redacted")
	}
}

func TestNewIsolation(t *testing.T) {
	r1 := New()
	r2 := New()
	r1.AddField("custom1")

	if r2.SensitiveFields["custom1"] {
		t.Error("redactors should have isolated field maps")
	}
}
