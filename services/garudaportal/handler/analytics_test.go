package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

func setupAnalyticsTest(t *testing.T) (*AnalyticsHandler, *store.App) {
	t.Helper()
	appStore := store.NewInMemoryAppStore()
	usageStore := store.NewInMemoryUsageStore()
	app, err := appStore.Create(&store.App{
		OwnerUserID: "user-1",
		Name:        "Analytics Test App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  1000,
	})
	if err != nil {
		t.Fatal(err)
	}

	h := NewAnalyticsHandler(appStore, usageStore)
	return h, app
}

func TestAnalyticsHandler_GetAnalytics_Success(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	usageStore := store.NewInMemoryUsageStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-1",
		Name:        "My App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  1000,
	})

	// Add usage data
	for i := 0; i < 10; i++ {
		usageStore.Increment(app.ID, "/api/v1/verify", false)
	}
	for i := 0; i < 3; i++ {
		usageStore.Increment(app.ID, "/api/v1/verify", true)
	}
	for i := 0; i < 5; i++ {
		usageStore.Increment(app.ID, "/api/v1/sign", false)
	}

	h := NewAnalyticsHandler(appStore, usageStore)

	today := time.Now().UTC().Format("2006-01-02")
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/portal/apps/"+app.ID+"/analytics?from="+today+"&to="+today, nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.GetAnalytics(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp analyticsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.AppID != app.ID {
		t.Errorf("expected app_id %s, got %s", app.ID, resp.AppID)
	}
	if resp.Overview.TotalCalls != 18 {
		t.Errorf("expected 18 total calls, got %d", resp.Overview.TotalCalls)
	}
	if resp.Overview.TotalErrors != 3 {
		t.Errorf("expected 3 total errors, got %d", resp.Overview.TotalErrors)
	}
	if resp.Overview.ErrorRate <= 0 {
		t.Errorf("expected positive error rate, got %f", resp.Overview.ErrorRate)
	}
	if len(resp.TimeSeries) != 1 {
		t.Errorf("expected 1 time series entry, got %d", len(resp.TimeSeries))
	}
	if len(resp.TopEndpoints) != 2 {
		t.Errorf("expected 2 top endpoints, got %d", len(resp.TopEndpoints))
	}
	// Top endpoint should be /api/v1/verify (13 calls) before /api/v1/sign (5 calls)
	if len(resp.TopEndpoints) >= 2 {
		if resp.TopEndpoints[0].Path != "/api/v1/verify" {
			t.Errorf("expected top endpoint /api/v1/verify, got %s", resp.TopEndpoints[0].Path)
		}
	}
	if resp.StatusCodes == nil {
		t.Error("expected status_codes to be non-nil")
	}
}

func TestAnalyticsHandler_GetAnalytics_EmptyPeriod(t *testing.T) {
	h, app := setupAnalyticsTest(t)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/portal/apps/"+app.ID+"/analytics?from=2025-01-01&to=2025-01-31", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.GetAnalytics(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp analyticsResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Overview.TotalCalls != 0 {
		t.Errorf("expected 0 total calls, got %d", resp.Overview.TotalCalls)
	}
	if resp.Overview.TotalErrors != 0 {
		t.Errorf("expected 0 total errors, got %d", resp.Overview.TotalErrors)
	}
	if resp.Overview.ErrorRate != 0 {
		t.Errorf("expected 0 error rate, got %f", resp.Overview.ErrorRate)
	}
	if len(resp.TimeSeries) != 0 {
		t.Errorf("expected 0 time series entries, got %d", len(resp.TimeSeries))
	}
}

func TestAnalyticsHandler_GetAnalytics_MissingUserID(t *testing.T) {
	h, app := setupAnalyticsTest(t)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/portal/apps/"+app.ID+"/analytics", nil)
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.GetAnalytics(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAnalyticsHandler_GetAnalytics_NotOwner(t *testing.T) {
	h, app := setupAnalyticsTest(t)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/portal/apps/"+app.ID+"/analytics", nil)
	req.Header.Set("X-User-ID", "other-user")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.GetAnalytics(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestAnalyticsHandler_GetAnalytics_AppNotFound(t *testing.T) {
	h, _ := setupAnalyticsTest(t)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/portal/apps/nonexistent-app/analytics", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", "nonexistent-app")
	w := httptest.NewRecorder()

	h.GetAnalytics(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAnalyticsHandler_GetAnalytics_ResponseStructure(t *testing.T) {
	h, app := setupAnalyticsTest(t)

	today := time.Now().UTC().Format("2006-01-02")
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/portal/apps/"+app.ID+"/analytics?from="+today+"&to="+today, nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.GetAnalytics(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verify JSON structure by decoding into a generic map
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}

	requiredFields := []string{"app_id", "period", "overview", "time_series", "top_endpoints", "status_codes", "growth"}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing required field %q in response", field)
		}
	}

	// Verify period structure
	var period analyticsPeriod
	json.Unmarshal(raw["period"], &period)
	if period.From != today {
		t.Errorf("expected period.from=%s, got %s", today, period.From)
	}
	if period.To != today {
		t.Errorf("expected period.to=%s, got %s", today, period.To)
	}

	// Verify growth structure
	var growth analyticsGrowth
	json.Unmarshal(raw["growth"], &growth)
	// With no previous data, both should be 0
	if growth.CallsChangePct != 0 {
		t.Errorf("expected 0 calls change pct, got %f", growth.CallsChangePct)
	}
}
