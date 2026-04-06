package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMaskField_NamePartial(t *testing.T) {
	got := MaskField("name", "Budi Santoso", "partial")
	want := "Bu** ****oso"
	if got != want {
		t.Errorf("MaskField(name, partial) = %q, want %q", got, want)
	}
}

func TestMaskField_NIKPartial(t *testing.T) {
	got := MaskField("nik", "3201234567890001", "partial")
	want := "****-****-****-0001"
	if got != want {
		t.Errorf("MaskField(nik, partial) = %q, want %q", got, want)
	}
}

func TestMaskField_PhonePartial(t *testing.T) {
	got := MaskField("phone", "+6281234567890", "partial")
	want := "+62****7890"
	if got != want {
		t.Errorf("MaskField(phone, partial) = %q, want %q", got, want)
	}
}

func TestMaskField_EmailPartial(t *testing.T) {
	got := MaskField("email", "budi@example.com", "partial")
	want := "b***@e***.com"
	if got != want {
		t.Errorf("MaskField(email, partial) = %q, want %q", got, want)
	}
}

func TestMaskField_FullMasking(t *testing.T) {
	tests := []struct {
		field string
		value string
		want  string
	}{
		{"name", "Budi Santoso", "************"},
		{"nik", "3201234567890001", "****************"},
		{"phone", "+6281234567890", "**************"},
		{"email", "budi@example.com", "***@***.***"},
	}

	for _, tt := range tests {
		got := MaskField(tt.field, tt.value, "full")
		if got != tt.want {
			t.Errorf("MaskField(%s, full) = %q, want %q", tt.field, got, tt.want)
		}
	}
}

func TestMaskField_UnknownFieldUsesDefault(t *testing.T) {
	got := MaskField("address", "Jakarta Selatan", "partial")
	want := "Ja***********an"
	if got != want {
		t.Errorf("MaskField(address, partial) = %q, want %q", got, want)
	}
}

func TestMaskField_EmptyString(t *testing.T) {
	got := MaskField("name", "", "partial")
	if got != "" {
		t.Errorf("MaskField(name, empty) = %q, want empty", got)
	}

	got = MaskField("name", "", "full")
	if got != "" {
		t.Errorf("MaskField(name, empty, full) = %q, want empty", got)
	}
}

func TestMaskData_PostSuccess(t *testing.T) {
	h := NewMaskingHandler()

	body := `{"fields":{"name":"Budi Santoso","nik":"3201234567890001"},"mask_level":"partial"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/data/mask", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.MaskData(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["name"] != "Bu** ****oso" {
		t.Errorf("masked name = %q, want %q", resp["name"], "Bu** ****oso")
	}
	if resp["nik"] != "****-****-****-0001" {
		t.Errorf("masked nik = %q, want %q", resp["nik"], "****-****-****-0001")
	}
}

func TestMaskData_InvalidMaskLevel(t *testing.T) {
	h := NewMaskingHandler()

	body := `{"fields":{"name":"Budi"},"mask_level":"invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/data/mask", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.MaskData(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
