package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/garudapass/gpass/services/garudacorp/oss"
	"github.com/garudapass/gpass/services/garudacorp/store"
)

// OSSSearcher abstracts the OSS client for testing.
type OSSSearcher interface {
	SearchByNPWP(ctx context.Context, npwp string) (*oss.NIBSearchResponse, error)
	SearchByNIB(ctx context.Context, nib string) (*oss.NIBSearchResponse, error)
}

// EntityHandler handles entity profile endpoints.
type EntityHandler struct {
	entityStore store.EntityStore
	oss         OSSSearcher
}

// NewEntityHandler creates a new entity handler.
func NewEntityHandler(entityStore store.EntityStore, ossClient OSSSearcher) *EntityHandler {
	return &EntityHandler{
		entityStore: entityStore,
		oss:         ossClient,
	}
}

type entityResponse struct {
	ID            string                  `json:"id"`
	AHUSKNumber   string                  `json:"ahu_sk_number"`
	Name          string                  `json:"name"`
	EntityType    string                  `json:"entity_type"`
	Status        string                  `json:"status"`
	NPWP          string                  `json:"npwp"`
	Address       string                  `json:"address"`
	CapitalAuth   int64                   `json:"capital_authorized"`
	CapitalPaid   int64                   `json:"capital_paid"`
	Officers      []store.EntityOfficer   `json:"officers"`
	Shareholders  []store.EntityShareholder `json:"shareholders"`
	OSSNIB        string                  `json:"oss_nib,omitempty"`
	OSSBusinesses []oss.Business          `json:"oss_businesses,omitempty"`
}

// GetEntity handles GET /api/v1/corp/entities/{id}.
func (h *EntityHandler) GetEntity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "entity id is required")
		return
	}

	entity, err := h.entityStore.GetByID(r.Context(), id)
	if err != nil {
		if err == store.ErrEntityNotFound {
			writeError(w, http.StatusNotFound, "entity_not_found", "Entity not found")
			return
		}
		slog.Error("failed to get entity", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve entity")
		return
	}

	resp := entityResponse{
		ID:           entity.ID,
		AHUSKNumber:  entity.AHUSKNumber,
		Name:         entity.Name,
		EntityType:   entity.EntityType,
		Status:       entity.Status,
		NPWP:         entity.NPWP,
		Address:      entity.Address,
		CapitalAuth:  entity.CapitalAuth,
		CapitalPaid:  entity.CapitalPaid,
		Officers:     entity.Officers,
		Shareholders: entity.Shareholders,
	}

	// Non-blocking OSS enrichment: if it fails, return entity without OSS data
	if entity.NPWP != "" && h.oss != nil {
		ossResp, err := h.oss.SearchByNPWP(r.Context(), entity.NPWP)
		if err != nil {
			slog.Warn("OSS enrichment failed", "error", err, "entity_id", id)
		} else if ossResp.Found {
			resp.OSSNIB = ossResp.NIB
			resp.OSSBusinesses = ossResp.Businesses
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
