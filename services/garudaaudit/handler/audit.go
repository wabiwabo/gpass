package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/garudapass/gpass/services/garudaaudit/store"
)

// AuditHandler handles audit log HTTP endpoints.
type AuditHandler struct {
	store *store.InMemoryAuditStore
}

// NewAuditHandler creates a new AuditHandler.
func NewAuditHandler(s *store.InMemoryAuditStore) *AuditHandler {
	return &AuditHandler{store: s}
}

// ingestRequest is the JSON body for POST /api/v1/audit/events.
type ingestRequest struct {
	EventType    string            `json:"event_type"`
	ActorID      string            `json:"actor_id"`
	ActorType    string            `json:"actor_type"`
	ResourceID   string            `json:"resource_id"`
	ResourceType string            `json:"resource_type"`
	Action       string            `json:"action"`
	Metadata     map[string]string `json:"metadata"`
	IPAddress    string            `json:"ip_address"`
	UserAgent    string            `json:"user_agent"`
	ServiceName  string            `json:"service_name"`
	RequestID    string            `json:"request_id"`
	Status       string            `json:"status"`
}

// IngestEvent handles POST /api/v1/audit/events.
func (h *AuditHandler) IngestEvent(w http.ResponseWriter, r *http.Request) {
	var req ingestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	event := &store.AuditEvent{
		EventType:    req.EventType,
		ActorID:      req.ActorID,
		ActorType:    req.ActorType,
		ResourceID:   req.ResourceID,
		ResourceType: req.ResourceType,
		Action:       req.Action,
		Metadata:     req.Metadata,
		IPAddress:    req.IPAddress,
		UserAgent:    req.UserAgent,
		ServiceName:  req.ServiceName,
		RequestID:    req.RequestID,
		Status:       req.Status,
	}

	if err := h.store.Append(event); err != nil {
		slog.Warn("audit ingest validation failed", "error", err)
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	slog.Info("audit event ingested",
		"event_id", event.ID,
		"event_type", event.EventType,
		"actor_id", event.ActorID,
		"action", event.Action,
	)

	writeJSON(w, http.StatusCreated, event)
}

// queryResponse wraps paginated query results.
type queryResponse struct {
	Events []*store.AuditEvent `json:"events"`
	Total  int64               `json:"total"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
}

// QueryEvents handles GET /api/v1/audit/events.
func (h *AuditHandler) QueryEvents(w http.ResponseWriter, r *http.Request) {
	filter := store.AuditFilter{
		ActorID:      r.URL.Query().Get("actor_id"),
		ResourceID:   r.URL.Query().Get("resource_id"),
		ResourceType: r.URL.Query().Get("resource_type"),
		EventType:    r.URL.Query().Get("event_type"),
		Action:       r.URL.Query().Get("action"),
		ServiceName:  r.URL.Query().Get("service"),
		Status:       r.URL.Query().Get("status"),
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

	if limit := r.URL.Query().Get("limit"); limit != "" {
		if n, err := strconv.Atoi(limit); err == nil {
			filter.Limit = n
		}
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		if n, err := strconv.Atoi(offset); err == nil {
			filter.Offset = n
		}
	}

	events, err := h.store.Query(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query_error", err.Error())
		return
	}

	total, err := h.store.Count(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "count_error", err.Error())
		return
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	writeJSON(w, http.StatusOK, queryResponse{
		Events: events,
		Total:  total,
		Limit:  limit,
		Offset: filter.Offset,
	})
}

// GetEvent handles GET /api/v1/audit/events/{id}.
func (h *AuditHandler) GetEvent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "event ID is required")
		return
	}

	event, err := h.store.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "audit event not found")
		return
	}

	writeJSON(w, http.StatusOK, event)
}
