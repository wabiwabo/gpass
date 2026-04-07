package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

func newAppSetup(t *testing.T) (*AppHandler, *store.App) {
	t.Helper()
	appStore := store.NewInMemoryAppStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-1", Name: "App", Environment: "sandbox", Tier: "free", DailyLimit: 100,
	})
	return NewAppHandler(appStore), app
}

func reqApp(method, body string, hdr map[string]string, h http.HandlerFunc, pv map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/", bytes.NewBufferString(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	for k, v := range pv {
		req.SetPathValue(k, v)
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestGetApp_FullMatrix(t *testing.T) {
	t.Run("missing user id", func(t *testing.T) {
		h, app := newAppSetup(t)
		rec := reqApp("GET", "", nil, h.GetApp, map[string]string{"id": app.ID})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("missing id", func(t *testing.T) {
		h, _ := newAppSetup(t)
		rec := reqApp("GET", "", map[string]string{"X-User-ID": "user-1"}, h.GetApp, nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("not found", func(t *testing.T) {
		h, _ := newAppSetup(t)
		rec := reqApp("GET", "", map[string]string{"X-User-ID": "user-1"}, h.GetApp, map[string]string{"id": "missing"})
		if rec.Code != http.StatusNotFound {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("not owner", func(t *testing.T) {
		h, app := newAppSetup(t)
		rec := reqApp("GET", "", map[string]string{"X-User-ID": "intruder"}, h.GetApp, map[string]string{"id": app.ID})
		if rec.Code != http.StatusForbidden {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("happy", func(t *testing.T) {
		h, app := newAppSetup(t)
		rec := reqApp("GET", "", map[string]string{"X-User-ID": "user-1"}, h.GetApp, map[string]string{"id": app.ID})
		if rec.Code != 200 {
			t.Errorf("code = %d", rec.Code)
		}
	})
}

func TestUpdateApp_FullMatrix(t *testing.T) {
	t.Run("missing user id", func(t *testing.T) {
		h, app := newAppSetup(t)
		rec := reqApp("PATCH", `{}`, nil, h.UpdateApp, map[string]string{"id": app.ID})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("missing id", func(t *testing.T) {
		h, _ := newAppSetup(t)
		rec := reqApp("PATCH", `{}`, map[string]string{"X-User-ID": "user-1"}, h.UpdateApp, nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("not found", func(t *testing.T) {
		h, _ := newAppSetup(t)
		rec := reqApp("PATCH", `{}`, map[string]string{"X-User-ID": "user-1"}, h.UpdateApp, map[string]string{"id": "missing"})
		if rec.Code != http.StatusNotFound {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("not owner", func(t *testing.T) {
		h, app := newAppSetup(t)
		rec := reqApp("PATCH", `{}`, map[string]string{"X-User-ID": "intruder"}, h.UpdateApp, map[string]string{"id": app.ID})
		if rec.Code != http.StatusForbidden {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("bad json", func(t *testing.T) {
		h, app := newAppSetup(t)
		rec := reqApp("PATCH", "{not json", map[string]string{"X-User-ID": "user-1"}, h.UpdateApp, map[string]string{"id": app.ID})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("happy", func(t *testing.T) {
		h, app := newAppSetup(t)
		name := "new"
		body := `{"name":"` + name + `"}`
		rec := reqApp("PATCH", body, map[string]string{"X-User-ID": "user-1"}, h.UpdateApp, map[string]string{"id": app.ID})
		if rec.Code != 200 {
			t.Errorf("code = %d", rec.Code)
		}
	})
}

func TestListApps_Branches(t *testing.T) {
	t.Run("missing user id", func(t *testing.T) {
		h, _ := newAppSetup(t)
		rec := reqApp("GET", "", nil, h.ListApps, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("happy", func(t *testing.T) {
		h, _ := newAppSetup(t)
		rec := reqApp("GET", "", map[string]string{"X-User-ID": "user-1"}, h.ListApps, nil)
		if rec.Code != 200 {
			t.Errorf("code = %d", rec.Code)
		}
	})
}

func TestCreateApp_Branches(t *testing.T) {
	t.Run("missing user id", func(t *testing.T) {
		h, _ := newAppSetup(t)
		rec := reqApp("POST", `{}`, nil, h.CreateApp, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("bad json", func(t *testing.T) {
		h, _ := newAppSetup(t)
		rec := reqApp("POST", "{x", map[string]string{"X-User-ID": "user-1"}, h.CreateApp, nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("missing name", func(t *testing.T) {
		h, _ := newAppSetup(t)
		rec := reqApp("POST", `{}`, map[string]string{"X-User-ID": "user-1"}, h.CreateApp, nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("code = %d", rec.Code)
		}
	})
}
