package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

func TestUsageHandler_GetUsage(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	usageStore := store.NewInMemoryUsageStore()
	app, _ := appStore.Create(&store.App{OwnerUserID: "user-1", Name: "My App", Environment: "sandbox", Tier: "free", DailyLimit: 100})

	// Add some usage
	usageStore.Increment(app.ID, "/api/v1/verify", false)
	usageStore.Increment(app.ID, "/api/v1/verify", true)
	usageStore.Increment(app.ID, "/api/v1/sign", false)

	h := NewUsageHandler(appStore, usageStore)

	today := time.Now().UTC().Format("2006-01-02")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps/"+app.ID+"/usage?from="+today+"&to="+today, nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.GetUsage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp usageResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.TotalCalls != 3 {
		t.Errorf("expected 3 total calls, got %d", resp.TotalCalls)
	}
	if resp.TotalErrors != 1 {
		t.Errorf("expected 1 total error, got %d", resp.TotalErrors)
	}
}

func TestUsageHandler_GetUsage_MissingUserID(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	usageStore := store.NewInMemoryUsageStore()

	h := NewUsageHandler(appStore, usageStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps/app-1/usage", nil)
	req.SetPathValue("app_id", "app-1")
	w := httptest.NewRecorder()

	h.GetUsage(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestUsageHandler_GetUsage_DefaultDateRange(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	usageStore := store.NewInMemoryUsageStore()
	app, _ := appStore.Create(&store.App{OwnerUserID: "user-1", Name: "My App", Environment: "sandbox", Tier: "free", DailyLimit: 100})

	h := NewUsageHandler(appStore, usageStore)

	// No from/to params - should default to last 30 days
	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps/"+app.ID+"/usage", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.GetUsage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
