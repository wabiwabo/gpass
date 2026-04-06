package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/garudapass/gpass/services/garudacorp/store"
	"github.com/garudapass/gpass/services/garudacorp/ubo"
)

// UBOHandler handles beneficial ownership endpoints.
type UBOHandler struct {
	entityStore store.EntityStore
	uboStore    store.UBOStore
	analyzer    *ubo.Analyzer
}

// NewUBOHandler creates a new UBO handler.
func NewUBOHandler(entityStore store.EntityStore, uboStore store.UBOStore) *UBOHandler {
	return &UBOHandler{
		entityStore: entityStore,
		uboStore:    uboStore,
		analyzer:    ubo.NewAnalyzer(),
	}
}

// AnalyzeUBO handles POST /api/v1/corp/entities/{entity_id}/ubo/analyze.
func (h *UBOHandler) AnalyzeUBO(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entity_id")
	if entityID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "entity_id is required")
		return
	}

	entity, err := h.entityStore.GetByID(r.Context(), entityID)
	if err != nil {
		if err == store.ErrEntityNotFound {
			writeError(w, http.StatusNotFound, "entity_not_found", "Entity not found")
			return
		}
		slog.Error("failed to get entity for UBO analysis", "error", err, "entity_id", entityID)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve entity")
		return
	}

	// Convert entity shareholders to UBO input.
	shareholders := make([]ubo.Shareholder, len(entity.Shareholders))
	for i, sh := range entity.Shareholders {
		shareholders[i] = ubo.Shareholder{
			Name:       sh.Name,
			NIKToken:   "",
			ShareType:  sh.ShareType,
			Shares:     sh.Shares,
			Percentage: sh.Percentage,
		}
	}

	// Convert entity officers to UBO input.
	officers := make([]ubo.Officer, len(entity.Officers))
	for i, off := range entity.Officers {
		officers[i] = ubo.Officer{
			Name:     off.Name,
			NIKToken: off.NIKToken,
			Position: off.Position,
		}
	}

	result := h.analyzer.Analyze(entity.ID, entity.Name, shareholders, officers)

	if err := h.uboStore.Save(result); err != nil {
		slog.Error("failed to save UBO analysis", "error", err, "entity_id", entityID)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to save analysis result")
		return
	}

	slog.Info("UBO analysis completed",
		"entity_id", entityID,
		"status", result.Status,
		"ubo_count", len(result.BeneficialOwners),
	)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// GetUBO handles GET /api/v1/corp/entities/{entity_id}/ubo.
func (h *UBOHandler) GetUBO(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entity_id")
	if entityID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "entity_id is required")
		return
	}

	result, err := h.uboStore.GetByEntityID(entityID)
	if err != nil {
		if err == store.ErrUBONotFound {
			writeError(w, http.StatusNotFound, "ubo_not_found", "UBO analysis not found for this entity")
			return
		}
		slog.Error("failed to get UBO analysis", "error", err, "entity_id", entityID)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve UBO analysis")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}
