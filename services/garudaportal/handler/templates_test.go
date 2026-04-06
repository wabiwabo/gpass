package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetTemplates_ReturnsAll9(t *testing.T) {
	h := NewTemplateHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/webhooks/templates", nil)
	rec := httptest.NewRecorder()

	h.GetTemplates(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Templates []WebhookTemplate `json:"templates"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Templates) != 9 {
		t.Errorf("got %d templates, want 9", len(resp.Templates))
	}
}

func TestGetTemplate_ByEventType(t *testing.T) {
	h := NewTemplateHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/portal/webhooks/templates/{event_type}", h.GetTemplate)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/webhooks/templates/identity.verified", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var tmpl WebhookTemplate
	if err := json.NewDecoder(rec.Body).Decode(&tmpl); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if tmpl.EventType != "identity.verified" {
		t.Errorf("event_type = %q, want %q", tmpl.EventType, "identity.verified")
	}
	if tmpl.Description == "" {
		t.Error("description should not be empty")
	}
}

func TestGetTemplate_UnknownEvent404(t *testing.T) {
	h := NewTemplateHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/portal/webhooks/templates/{event_type}", h.GetTemplate)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/webhooks/templates/nonexistent.event", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestTemplates_HaveExamplePayload(t *testing.T) {
	for _, tmpl := range AllTemplates() {
		if tmpl.ExamplePayload == nil || len(tmpl.ExamplePayload) == 0 {
			t.Errorf("template %q has no example_payload", tmpl.EventType)
		}
	}
}

func TestTemplates_HaveFieldDescriptions(t *testing.T) {
	for _, tmpl := range AllTemplates() {
		if len(tmpl.Fields) == 0 {
			t.Errorf("template %q has no field descriptions", tmpl.EventType)
		}
		for _, f := range tmpl.Fields {
			if f.Name == "" {
				t.Errorf("template %q has a field with empty name", tmpl.EventType)
			}
			if f.Type == "" {
				t.Errorf("template %q field %q has empty type", tmpl.EventType, f.Name)
			}
			if f.Description == "" {
				t.Errorf("template %q field %q has empty description", tmpl.EventType, f.Name)
			}
		}
	}
}

func TestAllTemplates_ReturnsCompleteList(t *testing.T) {
	expected := []string{
		"identity.verified",
		"identity.consent.granted",
		"identity.consent.revoked",
		"corp.entity.verified",
		"corp.role.assigned",
		"corp.role.revoked",
		"sign.certificate.issued",
		"sign.document.signed",
		"sign.document.failed",
	}

	templates := AllTemplates()
	if len(templates) != len(expected) {
		t.Fatalf("got %d templates, want %d", len(templates), len(expected))
	}

	eventTypes := make(map[string]bool)
	for _, tmpl := range templates {
		eventTypes[tmpl.EventType] = true
	}

	for _, et := range expected {
		if !eventTypes[et] {
			t.Errorf("missing template for event type %q", et)
		}
	}
}
