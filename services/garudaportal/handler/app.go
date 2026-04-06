package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

// AppHandler handles developer app CRUD endpoints.
type AppHandler struct {
	appStore store.AppStore
}

// NewAppHandler creates a new app handler.
func NewAppHandler(appStore store.AppStore) *AppHandler {
	return &AppHandler{appStore: appStore}
}

type createAppRequest struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	CallbackURLs []string `json:"callback_urls"`
}

type appResponse struct {
	ID            string   `json:"id"`
	OwnerUserID   string   `json:"owner_user_id"`
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Environment   string   `json:"environment"`
	Tier          string   `json:"tier"`
	DailyLimit    int      `json:"daily_limit"`
	CallbackURLs  []string `json:"callback_urls,omitempty"`
	OAuthClientID string   `json:"oauth_client_id,omitempty"`
	Status        string   `json:"status"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
}

func toAppResponse(a *store.App) appResponse {
	return appResponse{
		ID:            a.ID,
		OwnerUserID:   a.OwnerUserID,
		Name:          a.Name,
		Description:   a.Description,
		Environment:   a.Environment,
		Tier:          a.Tier,
		DailyLimit:    a.DailyLimit,
		CallbackURLs:  a.CallbackURLs,
		OAuthClientID: a.OAuthClientID,
		Status:        a.Status,
		CreatedAt:     a.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     a.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// CreateApp handles POST /api/v1/portal/apps.
func (h *AppHandler) CreateApp(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "X-User-ID header is required")
		return
	}

	var req createAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "name is required")
		return
	}

	app, err := h.appStore.Create(&store.App{
		OwnerUserID:  userID,
		Name:         req.Name,
		Description:  req.Description,
		Environment:  "sandbox",
		Tier:         "free",
		DailyLimit:   100,
		CallbackURLs: req.CallbackURLs,
	})
	if err != nil {
		slog.Error("failed to create app", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create app")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toAppResponse(app))
}

// ListApps handles GET /api/v1/portal/apps.
func (h *AppHandler) ListApps(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "X-User-ID header is required")
		return
	}

	apps, err := h.appStore.ListByOwner(userID)
	if err != nil {
		slog.Error("failed to list apps", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list apps")
		return
	}

	resp := make([]appResponse, 0, len(apps))
	for _, a := range apps {
		resp = append(resp, toAppResponse(a))
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"apps": resp})
}

// GetApp handles GET /api/v1/portal/apps/{id}.
func (h *AppHandler) GetApp(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "X-User-ID header is required")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "app id is required")
		return
	}

	app, err := h.appStore.GetByID(id)
	if err != nil {
		if err == store.ErrAppNotFound {
			writeError(w, http.StatusNotFound, "not_found", "App not found")
			return
		}
		slog.Error("failed to get app", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get app")
		return
	}

	if app.OwnerUserID != userID {
		writeError(w, http.StatusForbidden, "forbidden", "You do not own this app")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(toAppResponse(app))
}

// UpdateApp handles PATCH /api/v1/portal/apps/{id}.
func (h *AppHandler) UpdateApp(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "X-User-ID header is required")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "app id is required")
		return
	}

	// Verify ownership
	app, err := h.appStore.GetByID(id)
	if err != nil {
		if err == store.ErrAppNotFound {
			writeError(w, http.StatusNotFound, "not_found", "App not found")
			return
		}
		slog.Error("failed to get app", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get app")
		return
	}

	if app.OwnerUserID != userID {
		writeError(w, http.StatusForbidden, "forbidden", "You do not own this app")
		return
	}

	var body struct {
		Name         *string  `json:"name"`
		Description  *string  `json:"description"`
		CallbackURLs []string `json:"callback_urls"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	updated, err := h.appStore.Update(id, store.AppUpdate{
		Name:         body.Name,
		Description:  body.Description,
		CallbackURLs: body.CallbackURLs,
	})
	if err != nil {
		slog.Error("failed to update app", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update app")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(toAppResponse(updated))
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	})
}
