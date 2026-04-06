package handler

import (
	"testing"
)

func TestRenderTemplate_OTP(t *testing.T) {
	store := NewTemplateStore()
	subject, body, err := store.RenderTemplate("otp", map[string]string{
		"code":   "123456",
		"expiry": "5",
	})
	if err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}
	if subject != "Your GarudaPass Verification Code" {
		t.Errorf("unexpected subject: %s", subject)
	}
	if body == "" {
		t.Error("body should not be empty")
	}
	if !contains(body, "123456") {
		t.Errorf("body should contain OTP code, got: %s", body)
	}
	if !contains(body, "5 minutes") {
		t.Errorf("body should contain expiry, got: %s", body)
	}
}

func TestRenderTemplate_Welcome(t *testing.T) {
	store := NewTemplateStore()
	subject, body, err := store.RenderTemplate("welcome", map[string]string{
		"name": "Budi",
	})
	if err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}
	if subject != "Welcome to GarudaPass" {
		t.Errorf("unexpected subject: %s", subject)
	}
	if !contains(body, "Budi") {
		t.Errorf("body should contain name, got: %s", body)
	}
}

func TestRenderTemplate_PasswordReset(t *testing.T) {
	store := NewTemplateStore()
	subject, body, err := store.RenderTemplate("password_reset", map[string]string{
		"name":   "Siti",
		"link":   "https://garudapass.id/reset/abc123",
		"expiry": "15",
	})
	if err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}
	if subject != "GarudaPass Password Reset" {
		t.Errorf("unexpected subject: %s", subject)
	}
	if !contains(body, "Siti") {
		t.Errorf("body should contain name, got: %s", body)
	}
	if !contains(body, "https://garudapass.id/reset/abc123") {
		t.Errorf("body should contain link, got: %s", body)
	}
}

func TestRenderTemplate_SecurityAlert(t *testing.T) {
	store := NewTemplateStore()
	subject, body, err := store.RenderTemplate("security_alert", map[string]string{
		"event":    "login",
		"location": "Jakarta, Indonesia",
		"time":     "2026-04-06 10:00 WIB",
	})
	if err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}
	if subject != "GarudaPass Security Alert" {
		t.Errorf("unexpected subject: %s", subject)
	}
	if !contains(body, "login") {
		t.Errorf("body should contain event, got: %s", body)
	}
	if !contains(body, "Jakarta") {
		t.Errorf("body should contain location, got: %s", body)
	}
}

func TestRenderTemplate_ConsentGranted(t *testing.T) {
	store := NewTemplateStore()
	subject, body, err := store.RenderTemplate("consent_granted", map[string]string{
		"service":   "BankXYZ",
		"data_type": "identity",
		"expiry":    "2027-01-01",
	})
	if err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}
	if subject != "GarudaPass Consent Confirmation" {
		t.Errorf("unexpected subject: %s", subject)
	}
	if !contains(body, "BankXYZ") {
		t.Errorf("body should contain service, got: %s", body)
	}
}

func TestRenderTemplate_DataDeletion(t *testing.T) {
	store := NewTemplateStore()
	subject, body, err := store.RenderTemplate("data_deletion", map[string]string{
		"data_type":    "personal",
		"date":         "2026-04-06",
		"reference_id": "DEL-001",
	})
	if err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}
	if subject != "GarudaPass Data Deletion Confirmation" {
		t.Errorf("unexpected subject: %s", subject)
	}
	if !contains(body, "DEL-001") {
		t.Errorf("body should contain reference_id, got: %s", body)
	}
}

func TestRenderTemplate_UnknownTemplate(t *testing.T) {
	store := NewTemplateStore()
	_, _, err := store.RenderTemplate("nonexistent", map[string]string{})
	if err == nil {
		t.Fatal("expected error for unknown template, got nil")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %s", err.Error())
	}
}

func TestRenderTemplate_MissingVariable(t *testing.T) {
	store := NewTemplateStore()
	_, _, err := store.RenderTemplate("otp", map[string]string{
		"code": "123456",
		// missing "expiry"
	})
	if err == nil {
		t.Fatal("expected error for missing variable, got nil")
	}
	if !contains(err.Error(), "missing required variable") {
		t.Errorf("error should mention 'missing required variable', got: %s", err.Error())
	}
}

func TestRenderTemplate_MissingAllVariables(t *testing.T) {
	store := NewTemplateStore()
	_, _, err := store.RenderTemplate("welcome", map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing variable, got nil")
	}
}

func TestRenderTemplate_IndonesianOTP(t *testing.T) {
	store := NewTemplateStore()
	subject, body, err := store.RenderTemplateWithLang("otp", "id", map[string]string{
		"code":   "999888",
		"expiry": "5",
	})
	if err != nil {
		t.Fatalf("RenderTemplateWithLang() error = %v", err)
	}
	if subject != "Kode Verifikasi GarudaPass Anda" {
		t.Errorf("unexpected Indonesian subject: %s", subject)
	}
	if !contains(body, "999888") {
		t.Errorf("body should contain OTP code, got: %s", body)
	}
	if !contains(body, "Jangan bagikan") {
		t.Errorf("body should be in Indonesian, got: %s", body)
	}
}

func TestRenderTemplate_IndonesianWelcome(t *testing.T) {
	store := NewTemplateStore()
	subject, body, err := store.RenderTemplateWithLang("welcome", "id", map[string]string{
		"name": "Andi",
	})
	if err != nil {
		t.Fatalf("RenderTemplateWithLang() error = %v", err)
	}
	if subject != "Selamat Datang di GarudaPass" {
		t.Errorf("unexpected Indonesian subject: %s", subject)
	}
	if !contains(body, "Andi") {
		t.Errorf("body should contain name, got: %s", body)
	}
}

func TestRenderTemplate_IndonesianDataDeletion(t *testing.T) {
	store := NewTemplateStore()
	subject, _, err := store.RenderTemplateWithLang("data_deletion", "id", map[string]string{
		"data_type":    "pribadi",
		"date":         "2026-04-06",
		"reference_id": "DEL-002",
	})
	if err != nil {
		t.Fatalf("RenderTemplateWithLang() error = %v", err)
	}
	if subject != "Konfirmasi Penghapusan Data GarudaPass" {
		t.Errorf("unexpected Indonesian subject: %s", subject)
	}
}

func TestListTemplates(t *testing.T) {
	store := NewTemplateStore()
	templates := store.ListTemplates()

	if len(templates) != 6 {
		t.Fatalf("expected 6 templates, got %d", len(templates))
	}

	names := make(map[string]bool)
	for _, tmpl := range templates {
		names[tmpl.Name] = true
	}

	expected := []string{"otp", "welcome", "password_reset", "security_alert", "consent_granted", "data_deletion"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected template %q in list", name)
		}
	}
}

func TestListTemplates_HasVariables(t *testing.T) {
	store := NewTemplateStore()
	templates := store.ListTemplates()

	for _, tmpl := range templates {
		if len(tmpl.Variables) == 0 {
			t.Errorf("template %q should have variables", tmpl.Name)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
