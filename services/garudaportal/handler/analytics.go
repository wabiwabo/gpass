package handler

import (
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

// AnalyticsHandler provides detailed API analytics.
type AnalyticsHandler struct {
	usageStore store.UsageStore
	appStore   store.AppStore
}

// NewAnalyticsHandler creates a new analytics handler.
func NewAnalyticsHandler(appStore store.AppStore, usageStore store.UsageStore) *AnalyticsHandler {
	return &AnalyticsHandler{
		usageStore: usageStore,
		appStore:   appStore,
	}
}

type analyticsPeriod struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type analyticsOverview struct {
	TotalCalls   int64   `json:"total_calls"`
	TotalErrors  int64   `json:"total_errors"`
	ErrorRate    float64 `json:"error_rate"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

type timeSeriesEntry struct {
	Timestamp    string  `json:"timestamp"`
	Calls        int64   `json:"calls"`
	Errors       int64   `json:"errors"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

type topEndpointEntry struct {
	Path         string  `json:"path"`
	Calls        int64   `json:"calls"`
	Errors       int64   `json:"errors"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

type analyticsGrowth struct {
	CallsChangePct  float64 `json:"calls_change_pct"`
	ErrorsChangePct float64 `json:"errors_change_pct"`
}

type analyticsResponse struct {
	AppID        string             `json:"app_id"`
	Period       analyticsPeriod    `json:"period"`
	Overview     analyticsOverview  `json:"overview"`
	TimeSeries   []timeSeriesEntry  `json:"time_series"`
	TopEndpoints []topEndpointEntry `json:"top_endpoints"`
	StatusCodes  map[string]int64   `json:"status_codes"`
	Growth       analyticsGrowth    `json:"growth"`
}

// GetAnalytics handles GET /api/v1/portal/apps/{app_id}/analytics.
func (h *AnalyticsHandler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "X-User-ID header is required")
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
		from = time.Now().UTC().Add(-30 * 24 * time.Hour)
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

	// Get usage data for the requested period
	daily, err := h.usageStore.GetUsageRange(appID, from, to)
	if err != nil {
		slog.Error("failed to get usage range", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get analytics")
		return
	}

	byEndpoint, err := h.usageStore.GetUsageByEndpoint(appID, from, to)
	if err != nil {
		slog.Error("failed to get usage by endpoint", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get analytics")
		return
	}

	// Calculate overview
	var totalCalls, totalErrors int64
	for _, d := range daily {
		totalCalls += d.Calls
		totalErrors += d.Errors
	}

	var errorRate float64
	if totalCalls > 0 {
		errorRate = math.Round(float64(totalErrors)/float64(totalCalls)*10000) / 100 // percentage with 2 decimal places
	}

	overview := analyticsOverview{
		TotalCalls:   totalCalls,
		TotalErrors:  totalErrors,
		ErrorRate:    errorRate,
		AvgLatencyMs: 0, // latency tracking not yet in usage store
	}

	// Build time series
	timeSeries := make([]timeSeriesEntry, 0, len(daily))
	for _, d := range daily {
		timeSeries = append(timeSeries, timeSeriesEntry{
			Timestamp:    d.Date.Format("2006-01-02"),
			Calls:        d.Calls,
			Errors:       d.Errors,
			AvgLatencyMs: 0,
		})
	}

	// Build top endpoints sorted by calls descending
	topEndpoints := make([]topEndpointEntry, 0, len(byEndpoint))
	for _, e := range byEndpoint {
		topEndpoints = append(topEndpoints, topEndpointEntry{
			Path:         e.Endpoint,
			Calls:        e.Calls,
			Errors:       e.Errors,
			AvgLatencyMs: 0,
		})
	}
	sort.Slice(topEndpoints, func(i, j int) bool {
		return topEndpoints[i].Calls > topEndpoints[j].Calls
	})

	// Build status codes approximation from error data
	statusCodes := make(map[string]int64)
	if totalCalls > 0 {
		successCalls := totalCalls - totalErrors
		if successCalls > 0 {
			statusCodes["200"] = successCalls
		}
		if totalErrors > 0 {
			statusCodes["500"] = totalErrors
		}
	}

	// Calculate growth: compare current period vs previous period of same length
	periodDuration := to.Sub(from)
	prevFrom := from.Add(-periodDuration)
	prevTo := from.Add(-24 * time.Hour) // day before current period starts

	prevDaily, err := h.usageStore.GetUsageRange(appID, prevFrom, prevTo)
	if err != nil {
		slog.Error("failed to get previous usage range", "error", err)
		// Non-fatal: just report 0 growth
		prevDaily = nil
	}

	var prevCalls, prevErrors int64
	for _, d := range prevDaily {
		prevCalls += d.Calls
		prevErrors += d.Errors
	}

	growth := analyticsGrowth{}
	if prevCalls > 0 {
		growth.CallsChangePct = math.Round(float64(totalCalls-prevCalls)/float64(prevCalls)*10000) / 100
	}
	if prevErrors > 0 {
		growth.ErrorsChangePct = math.Round(float64(totalErrors-prevErrors)/float64(prevErrors)*10000) / 100
	}

	resp := analyticsResponse{
		AppID: appID,
		Period: analyticsPeriod{
			From: from.Format("2006-01-02"),
			To:   to.Format("2006-01-02"),
		},
		Overview:     overview,
		TimeSeries:   timeSeries,
		TopEndpoints: topEndpoints,
		StatusCodes:  statusCodes,
		Growth:       growth,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
