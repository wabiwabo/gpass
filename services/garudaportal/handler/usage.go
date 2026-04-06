package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

// UsageHandler handles usage stats endpoints.
type UsageHandler struct {
	appStore   store.AppStore
	usageStore store.UsageStore
}

// NewUsageHandler creates a new usage handler.
func NewUsageHandler(appStore store.AppStore, usageStore store.UsageStore) *UsageHandler {
	return &UsageHandler{
		appStore:   appStore,
		usageStore: usageStore,
	}
}

type usageResponse struct {
	TotalCalls  int64               `json:"total_calls"`
	TotalErrors int64               `json:"total_errors"`
	Daily       []dailyUsageResp    `json:"daily"`
	ByEndpoint  []endpointUsageResp `json:"by_endpoint"`
}

type dailyUsageResp struct {
	Date   string `json:"date"`
	Calls  int64  `json:"calls"`
	Errors int64  `json:"errors"`
}

type endpointUsageResp struct {
	Endpoint string `json:"endpoint"`
	Calls    int64  `json:"calls"`
	Errors   int64  `json:"errors"`
}

// GetUsage handles GET /api/v1/portal/apps/{app_id}/usage.
func (h *UsageHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "X-User-ID header is required")
		return
	}

	appID := r.PathValue("app_id")
	if appID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "app_id is required")
		return
	}

	// Verify ownership
	app, err := h.appStore.GetByID(appID)
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

	// Parse date range
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	var from, to time.Time
	if fromStr != "" {
		from, err = time.Parse("2006-01-02", fromStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid from date format (use YYYY-MM-DD)")
			return
		}
	} else {
		from = time.Now().UTC().Add(-30 * 24 * time.Hour) // default last 30 days
	}

	if toStr != "" {
		to, err = time.Parse("2006-01-02", toStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid to date format (use YYYY-MM-DD)")
			return
		}
	} else {
		to = time.Now().UTC()
	}

	daily, err := h.usageStore.GetUsageRange(appID, from, to)
	if err != nil {
		slog.Error("failed to get usage range", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get usage")
		return
	}

	byEndpoint, err := h.usageStore.GetUsageByEndpoint(appID, from, to)
	if err != nil {
		slog.Error("failed to get usage by endpoint", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get usage")
		return
	}

	var totalCalls, totalErrors int64
	dailyResp := make([]dailyUsageResp, 0, len(daily))
	for _, d := range daily {
		totalCalls += d.Calls
		totalErrors += d.Errors
		dailyResp = append(dailyResp, dailyUsageResp{
			Date:   d.Date.Format("2006-01-02"),
			Calls:  d.Calls,
			Errors: d.Errors,
		})
	}

	endpointResp := make([]endpointUsageResp, 0, len(byEndpoint))
	for _, e := range byEndpoint {
		endpointResp = append(endpointResp, endpointUsageResp{
			Endpoint: e.Endpoint,
			Calls:    e.Calls,
			Errors:   e.Errors,
		})
	}

	resp := usageResponse{
		TotalCalls:  totalCalls,
		TotalErrors: totalErrors,
		Daily:       dailyResp,
		ByEndpoint:  endpointResp,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
