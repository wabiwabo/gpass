package handler

import (
	"net/http"
	"time"

	"github.com/garudapass/gpass/services/garudaaudit/store"
)

// StatsHandler provides audit statistics.
type StatsHandler struct {
	store *store.InMemoryAuditStore
}

// NewStatsHandler creates a new StatsHandler.
func NewStatsHandler(s *store.InMemoryAuditStore) *StatsHandler {
	return &StatsHandler{store: s}
}

// statsResponse is the JSON response for GET /api/v1/audit/stats.
type statsResponse struct {
	TotalEvents int64            `json:"total_events"`
	ByAction    map[string]int64 `json:"by_action"`
	ByService   map[string]int64 `json:"by_service"`
	ByStatus    map[string]int64 `json:"by_status"`
}

// GetStats handles GET /api/v1/audit/stats.
func (h *StatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	filter := store.AuditFilter{
		ServiceName: r.URL.Query().Get("service"),
	}

	if from := r.URL.Query().Get("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			filter.From = t
		}
	}
	if to := r.URL.Query().Get("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			filter.To = t
		}
	}

	// Get all matching events to compute stats
	allFilter := filter
	allFilter.Limit = 0 // will default to 100 in Query, so we use a large number
	// We need all events for accurate stats, use Count for total and iterate
	events, err := h.store.Query(store.AuditFilter{
		ServiceName: filter.ServiceName,
		From:        filter.From,
		To:          filter.To,
		Limit:       1000000, // get all
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "stats_error", err.Error())
		return
	}

	total, err := h.store.Count(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "stats_error", err.Error())
		return
	}

	byAction := make(map[string]int64)
	byService := make(map[string]int64)
	byStatus := make(map[string]int64)

	for _, e := range events {
		byAction[e.Action]++
		byService[e.ServiceName]++
		byStatus[e.Status]++
	}

	writeJSON(w, http.StatusOK, statsResponse{
		TotalEvents: total,
		ByAction:    byAction,
		ByService:   byService,
		ByStatus:    byStatus,
	})
}
