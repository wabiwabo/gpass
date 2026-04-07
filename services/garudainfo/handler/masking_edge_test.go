package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaskFieldNameEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"short_3", "Ali", "***"},
		{"short_5", "Budii", "*****"},
		{"exact_6", "Budiku", "Bu*iku"},
		{"long_name", "Budi Santoso Wijaya", "Bu** ******* ***aya"},
		{"single_char", "A", "*"},
		{"two_chars", "AB", "**"},
		{"unicode_name", "日本太郎さん", "日本*郎さん"}, // 6 runes
		{"empty", "", ""},
		{"spaces_only", "   ", "***"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskField("name", tt.value, "partial")
			if got != tt.want {
				t.Errorf("MaskField(name, %q, partial) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestMaskFieldNIKEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"standard_16", "3201012345678901", "****-****-****-8901"},
		{"short_4", "1234", "1234"},
		{"short_3", "123", "123"},
		{"single", "1", "1"},
		{"five_chars", "12345", "****-****-****-2345"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskField("nik", tt.value, "partial")
			if got != tt.want {
				t.Errorf("MaskField(nik, %q, partial) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestMaskFieldPhoneEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"indonesia", "+6281234567890", "+62****7890"},
		{"short_7", "1234567", "*******"},
		{"short_6", "123456", "******"},
		{"long", "+628123456789012", "+62****9012"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskField("phone", tt.value, "partial")
			if got != tt.want {
				t.Errorf("MaskField(phone, %q, partial) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestMaskFieldEmailEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"standard", "budi@example.com", "b***@e***.com"},
		{"short_local", "a@b.co", "a***@b***.co"},
		{"no_at", "invalidemail", "in********il"},
		{"no_dot_in_domain", "user@localhost", "u***@***"},
		{"multiple_dots", "user@sub.domain.co.id", "u***@s***.id"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskField("email", tt.value, "partial")
			if got != tt.want {
				t.Errorf("MaskField(email, %q, partial) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestMaskFieldDefaultEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value string
		want  string
	}{
		{"address", "address", "Jl. Sudirman No. 1", "Jl************** 1"},
		{"short_4", "address", "ABCD", "****"},
		{"short_3", "address", "ABC", "***"},
		{"exact_5", "custom", "ABCDE", "AB*DE"},
		{"unicode", "custom", "日本語テスト", "日本**スト"},
		{"empty", "custom", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskField(tt.field, tt.value, "partial")
			if got != tt.want {
				t.Errorf("MaskField(%q, %q, partial) = %q, want %q", tt.field, tt.value, got, tt.want)
			}
		})
	}
}

func TestMaskFieldFullLevel(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value string
		want  string
	}{
		{"name", "name", "Budi Santoso", "************"},
		{"nik", "nik", "3201012345678901", "****************"},
		{"phone", "phone", "+6281234567890", "**************"},
		{"email", "email", "budi@example.com", "***@***.***"},
		{"custom", "address", "Jakarta", "*******"},
		{"empty", "name", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskField(tt.field, tt.value, "full")
			if got != tt.want {
				t.Errorf("MaskField(%q, %q, full) = %q, want %q", tt.field, tt.value, got, tt.want)
			}
		})
	}
}

func TestMaskDataHandlerMissingFields(t *testing.T) {
	h := NewMaskingHandler()

	// No fields
	body := `{"fields":{},"mask_level":"partial"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.MaskData(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rr.Code)
	}
	var result map[string]string
	json.NewDecoder(rr.Body).Decode(&result)
	if len(result) != 0 {
		t.Errorf("should return empty map, got %v", result)
	}
}

func TestMaskDataHandlerInvalidJSON(t *testing.T) {
	h := NewMaskingHandler()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{invalid"))
	rr := httptest.NewRecorder()
	h.MaskData(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

func TestMaskDataHandlerEmptyBody(t *testing.T) {
	h := NewMaskingHandler()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	rr := httptest.NewRecorder()
	h.MaskData(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

func TestMaskDataHandlerMultipleFields(t *testing.T) {
	h := NewMaskingHandler()
	body := `{"fields":{"name":"Budi Santoso","email":"budi@example.com","nik":"3201012345678901","phone":"+6281234567890"},"mask_level":"partial"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.MaskData(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d", rr.Code)
	}

	var result map[string]string
	json.NewDecoder(rr.Body).Decode(&result)

	if result["name"] == "Budi Santoso" {
		t.Error("name should be masked")
	}
	if result["email"] == "budi@example.com" {
		t.Error("email should be masked")
	}
	if result["nik"] == "3201012345678901" {
		t.Error("nik should be masked")
	}
	if result["phone"] == "+6281234567890" {
		t.Error("phone should be masked")
	}
}

func TestMaskDataHandlerInvalidLevel(t *testing.T) {
	h := NewMaskingHandler()
	body := `{"fields":{"name":"test"},"mask_level":"invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.MaskData(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
	var resp errorResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "invalid_mask_level" {
		t.Errorf("error code: got %q", resp.Error)
	}
}
