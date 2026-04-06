package redactlog

import (
	"testing"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		key  string
		want FieldType
	}{
		{"nik", FieldNIK},
		{"NIK", FieldNIK},
		{"email", FieldEmail},
		{"phone", FieldPhone},
		{"phone_number", FieldPhone},
		{"token", FieldToken},
		{"password", FieldPassword},
		{"name", FieldName},
		{"address", FieldAddress},
		{"unknown_field", FieldGeneral},
		{"", FieldGeneral},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := Classify(tt.key); got != tt.want {
				t.Errorf("Classify(%q) = %d, want %d", tt.key, got, tt.want)
			}
		})
	}
}

func TestRedactNIK(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"full", "3201012345678901", "3201************"},
		{"short", "123", "****"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Redact(FieldNIK, tt.value); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactEmail(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"normal", "user@example.com", "us***@example.com"},
		{"short_local", "u@example.com", "u***@example.com"},
		{"no_at", "invalid", "***@***"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Redact(FieldEmail, tt.value); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactPhone(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"indonesia", "+6281234567890", "+62*******7890"},
		{"short", "123", "****"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Redact(FieldPhone, tt.value); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactToken(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"long", "eyJhbGciOiJSUzI1NiJ9.payload.signature", "eyJh...ture"},
		{"short", "abc", "****"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Redact(FieldToken, tt.value); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactPassword(t *testing.T) {
	if got := Redact(FieldPassword, "supersecret"); got != "***" {
		t.Errorf("got %q, want %q", got, "***")
	}
}

func TestRedactName(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"full", "Budi Santoso", "B*** S***"},
		{"single", "Budi", "B***"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Redact(FieldName, tt.value); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactAddress(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"long", "Jl. Sudirman No. 1, Jakarta", "Jl. S***"},
		{"short", "Jl. 1", "***"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Redact(FieldAddress, tt.value); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactByKey(t *testing.T) {
	tests := []struct {
		key   string
		value string
		redacted bool
	}{
		{"email", "user@example.com", true},
		{"nik", "3201012345678901", true},
		{"unknown", "some value", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := RedactByKey(tt.key, tt.value)
			if tt.redacted && got == tt.value {
				t.Errorf("expected redaction for key %q", tt.key)
			}
			if !tt.redacted && got != tt.value {
				t.Errorf("unexpected redaction for key %q: %q", tt.key, got)
			}
		})
	}
}

func TestIsSensitive(t *testing.T) {
	if !IsSensitive("email") {
		t.Error("email should be sensitive")
	}
	if !IsSensitive("NIK") {
		t.Error("NIK should be sensitive")
	}
	if IsSensitive("status") {
		t.Error("status should not be sensitive")
	}
}

func TestRegisterKey(t *testing.T) {
	RegisterKey("ktp_number", FieldNIK)
	if Classify("ktp_number") != FieldNIK {
		t.Error("registered key should be classified as NIK")
	}
	if !IsSensitive("ktp_number") {
		t.Error("registered key should be sensitive")
	}
}

func TestRedactGeneral(t *testing.T) {
	// General fields should not be redacted
	if got := Redact(FieldGeneral, "public data"); got != "public data" {
		t.Errorf("general field should pass through, got %q", got)
	}
}

func TestRedactEmptyValues(t *testing.T) {
	for _, ft := range []FieldType{FieldNIK, FieldEmail, FieldPhone, FieldToken, FieldName, FieldAddress} {
		if got := Redact(ft, ""); got != "" {
			t.Errorf("Redact(%d, '') should return empty, got %q", ft, got)
		}
	}
}
