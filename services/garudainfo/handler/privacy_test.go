package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetDataProcessingInfo(t *testing.T) {
	h := NewPrivacyHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/privacy/processing", nil)
	w := httptest.NewRecorder()

	h.GetDataProcessingInfo(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp dataProcessingResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Controller info
	if resp.Controller.Name == "" {
		t.Error("expected controller name")
	}
	if resp.Controller.Contact == "" {
		t.Error("expected controller contact")
	}
	if resp.Controller.DPOEmail == "" {
		t.Error("expected controller dpo_email")
	}

	// Processing activities
	if len(resp.ProcessingActivities) == 0 {
		t.Fatal("expected at least one processing activity")
	}

	// Last updated
	if resp.LastUpdated == "" {
		t.Error("expected last_updated")
	}
}

func TestDataProcessingInfoIncludesUUPDPFields(t *testing.T) {
	h := NewPrivacyHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/privacy/processing", nil)
	w := httptest.NewRecorder()

	h.GetDataProcessingInfo(w, req)

	var resp dataProcessingResponse
	json.NewDecoder(w.Body).Decode(&resp)

	// UU PDP requires: purpose, legal basis, retention, recipients for each activity
	for _, activity := range resp.ProcessingActivities {
		if activity.Activity == "" {
			t.Error("activity name is required per UU PDP")
		}
		if len(activity.DataCategories) == 0 {
			t.Errorf("data_categories required for activity %s", activity.Activity)
		}
		if activity.Purpose == "" {
			t.Errorf("purpose required for activity %s per UU PDP", activity.Activity)
		}
		if activity.LegalBasis == "" {
			t.Errorf("legal_basis required for activity %s per UU PDP", activity.Activity)
		}
		if activity.Retention == "" {
			t.Errorf("retention required for activity %s per UU PDP", activity.Activity)
		}
		if len(activity.Recipients) == 0 {
			t.Errorf("recipients required for activity %s per UU PDP", activity.Activity)
		}
	}
}

func TestDataSubjectRightsEndpoints(t *testing.T) {
	h := NewPrivacyHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/privacy/processing", nil)
	w := httptest.NewRecorder()

	h.GetDataProcessingInfo(w, req)

	var resp dataProcessingResponse
	json.NewDecoder(w.Body).Decode(&resp)

	rights := resp.DataSubjectRights
	if !strings.HasPrefix(rights.Access, "/api/") {
		t.Errorf("access endpoint should be a valid API path, got %s", rights.Access)
	}
	if !strings.HasPrefix(rights.Deletion, "/api/") {
		t.Errorf("deletion endpoint should be a valid API path, got %s", rights.Deletion)
	}
	if !strings.HasPrefix(rights.ConsentManagement, "/api/") {
		t.Errorf("consent_management endpoint should be a valid API path, got %s", rights.ConsentManagement)
	}
}

func TestGetRetentionPolicy(t *testing.T) {
	h := NewPrivacyHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/privacy/retention", nil)
	w := httptest.NewRecorder()

	h.GetRetentionPolicy(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp retentionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Policies) == 0 {
		t.Fatal("expected at least one retention policy")
	}

	for _, p := range resp.Policies {
		if p.DataCategory == "" {
			t.Error("data_category is required")
		}
		if p.Retention == "" {
			t.Errorf("retention is required for %s", p.DataCategory)
		}
		if p.LegalBasis == "" {
			t.Errorf("legal_basis is required for %s", p.DataCategory)
		}
	}
}

func TestProcessingActivitiesIncludeAllServices(t *testing.T) {
	h := NewPrivacyHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/privacy/processing", nil)
	w := httptest.NewRecorder()

	h.GetDataProcessingInfo(w, req)

	var resp dataProcessingResponse
	json.NewDecoder(w.Body).Decode(&resp)

	expectedActivities := map[string]bool{
		"identity_verification":  false,
		"consent_management":     false,
		"document_signing":       false,
		"audit_logging":          false,
		"business_registration":  false,
	}

	for _, a := range resp.ProcessingActivities {
		if _, ok := expectedActivities[a.Activity]; ok {
			expectedActivities[a.Activity] = true
		}
	}

	for activity, found := range expectedActivities {
		if !found {
			t.Errorf("expected processing activity %s not found", activity)
		}
	}
}
