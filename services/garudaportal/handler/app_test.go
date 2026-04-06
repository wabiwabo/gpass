package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

func TestAppHandler_CreateApp(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	h := NewAppHandler(appStore)

	body := `{"name":"My App","description":"Test app"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps", bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()

	h.CreateApp(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp appResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Name != "My App" {
		t.Errorf("expected name My App, got %s", resp.Name)
	}
	if resp.Environment != "sandbox" {
		t.Errorf("expected sandbox, got %s", resp.Environment)
	}
	if resp.Tier != "free" {
		t.Errorf("expected free tier, got %s", resp.Tier)
	}
	if resp.DailyLimit != 100 {
		t.Errorf("expected daily limit 100, got %d", resp.DailyLimit)
	}
}

func TestAppHandler_CreateApp_MissingName(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	h := NewAppHandler(appStore)

	body := `{"description":"No name"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps", bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()

	h.CreateApp(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAppHandler_CreateApp_MissingUserID(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	h := NewAppHandler(appStore)

	body := `{"name":"My App"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.CreateApp(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAppHandler_ListApps(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	appStore.Create(&store.App{OwnerUserID: "user-1", Name: "App 1", Environment: "sandbox", Tier: "free", DailyLimit: 100})
	appStore.Create(&store.App{OwnerUserID: "user-1", Name: "App 2", Environment: "sandbox", Tier: "free", DailyLimit: 100})
	appStore.Create(&store.App{OwnerUserID: "user-2", Name: "App 3", Environment: "sandbox", Tier: "free", DailyLimit: 100})

	h := NewAppHandler(appStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()

	h.ListApps(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Apps []appResponse `json:"apps"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(resp.Apps))
	}
}

func TestAppHandler_GetApp(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	app, _ := appStore.Create(&store.App{OwnerUserID: "user-1", Name: "My App", Environment: "sandbox", Tier: "free", DailyLimit: 100})

	h := NewAppHandler(appStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps/"+app.ID, nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", app.ID)
	w := httptest.NewRecorder()

	h.GetApp(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAppHandler_GetApp_NotOwner(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	app, _ := appStore.Create(&store.App{OwnerUserID: "user-1", Name: "My App", Environment: "sandbox", Tier: "free", DailyLimit: 100})

	h := NewAppHandler(appStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps/"+app.ID, nil)
	req.Header.Set("X-User-ID", "user-2")
	req.SetPathValue("id", app.ID)
	w := httptest.NewRecorder()

	h.GetApp(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestAppHandler_UpdateApp(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	app, _ := appStore.Create(&store.App{OwnerUserID: "user-1", Name: "Old Name", Environment: "sandbox", Tier: "free", DailyLimit: 100})

	h := NewAppHandler(appStore)

	body := `{"name":"New Name","description":"Updated"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/portal/apps/"+app.ID, bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", app.ID)
	w := httptest.NewRecorder()

	h.UpdateApp(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp appResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "New Name" {
		t.Errorf("expected New Name, got %s", resp.Name)
	}
}
