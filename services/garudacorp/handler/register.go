package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/garudapass/gpass/services/garudacorp/ahu"
	"github.com/garudapass/gpass/services/garudacorp/store"
)

// AHUVerifier abstracts the AHU client for testing.
type AHUVerifier interface {
	SearchCompany(ctx context.Context, sk string) (*ahu.CompanySearchResponse, error)
	GetOfficers(ctx context.Context, sk string) ([]ahu.Officer, error)
	GetShareholders(ctx context.Context, sk string) ([]ahu.Shareholder, error)
}

// RegisterDeps holds the dependencies for the registration handler.
type RegisterDeps struct {
	AHU         AHUVerifier
	EntityStore store.EntityStore
	RoleStore   store.RoleStore
	NIKKey      []byte
}

// RegisterHandler handles corporate registration flows.
type RegisterHandler struct {
	deps RegisterDeps
}

// NewRegisterHandler creates a new registration handler.
func NewRegisterHandler(deps RegisterDeps) *RegisterHandler {
	return &RegisterHandler{deps: deps}
}

type registerRequest struct {
	SKNumber     string `json:"sk_number"`
	CallerUserID string `json:"caller_user_id"`
	CallerNIK    string `json:"caller_nik"`
}

type registerResponse struct {
	EntityID string `json:"entity_id"`
	Role     string `json:"role"`
}

// Register handles POST /api/v1/corp/register.
func (h *RegisterHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if req.SKNumber == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "sk_number is required")
		return
	}
	if req.CallerUserID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "caller_user_id is required")
		return
	}
	if req.CallerNIK == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "caller_nik is required")
		return
	}

	ctx := r.Context()

	// 1. Search company by SK number
	company, err := h.deps.AHU.SearchCompany(ctx, req.SKNumber)
	if err != nil {
		slog.Error("AHU search company failed", "error", err)
		writeError(w, http.StatusBadGateway, "ahu_unavailable", "AHU service is unavailable")
		return
	}
	if !company.Found {
		writeError(w, http.StatusNotFound, "company_not_found", "Company with the given SK number was not found")
		return
	}

	// 2. Get officers from AHU
	officers, err := h.deps.AHU.GetOfficers(ctx, req.SKNumber)
	if err != nil {
		slog.Error("AHU get officers failed", "error", err)
		writeError(w, http.StatusBadGateway, "ahu_unavailable", "AHU service is unavailable")
		return
	}

	// 3. Tokenize each officer's NIK and the caller's NIK
	callerNIKToken := tokenizeNIK(req.CallerNIK, h.deps.NIKKey)

	// 4. Check if caller's NIK token matches any officer with position DIREKTUR_UTAMA
	isDirector := false
	for _, o := range officers {
		officerToken := tokenizeNIK(o.NIK, h.deps.NIKKey)
		if officerToken == callerNIKToken && o.Position == "DIREKTUR_UTAMA" {
			isDirector = true
			break
		}
	}

	if !isDirector {
		writeError(w, http.StatusForbidden, "not_authorized", "Caller is not a Direktur Utama of this company")
		return
	}

	// 5. Get shareholders from AHU
	shareholders, err := h.deps.AHU.GetShareholders(ctx, req.SKNumber)
	if err != nil {
		slog.Error("AHU get shareholders failed", "error", err)
		// Non-blocking: proceed without shareholders
		shareholders = nil
	}

	// 6. Create Entity
	entity := &store.Entity{
		AHUSKNumber:   company.SKNumber,
		Name:          company.Name,
		EntityType:    company.EntityType,
		Status:        company.Status,
		NPWP:          company.NPWP,
		Address:       company.Address,
		CapitalAuth:   company.CapitalAuth,
		CapitalPaid:   company.CapitalPaid,
		AHUVerifiedAt: time.Now().UTC(),
	}

	if err := h.deps.EntityStore.Create(ctx, entity); err != nil {
		slog.Error("failed to create entity", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create entity")
		return
	}

	// 7. Add Officers
	entityOfficers := make([]store.EntityOfficer, len(officers))
	for i, o := range officers {
		entityOfficers[i] = store.EntityOfficer{
			NIKToken:        tokenizeNIK(o.NIK, h.deps.NIKKey),
			Name:            o.Name,
			Position:        o.Position,
			AppointmentDate: o.AppointmentDate,
			Verified:        tokenizeNIK(o.NIK, h.deps.NIKKey) == callerNIKToken,
		}
	}
	if err := h.deps.EntityStore.AddOfficers(ctx, entity.ID, entityOfficers); err != nil {
		slog.Error("failed to add officers", "error", err)
	}

	// 8. Add Shareholders
	if len(shareholders) > 0 {
		entityShareholders := make([]store.EntityShareholder, len(shareholders))
		for i, s := range shareholders {
			entityShareholders[i] = store.EntityShareholder{
				Name:       s.Name,
				ShareType:  s.ShareType,
				Shares:     s.Shares,
				Percentage: s.Percentage,
			}
		}
		if err := h.deps.EntityStore.AddShareholders(ctx, entity.ID, entityShareholders); err != nil {
			slog.Error("failed to add shareholders", "error", err)
		}
	}

	// 9. Assign RO role
	role := &store.EntityRole{
		EntityID:      entity.ID,
		UserID:        req.CallerUserID,
		Role:          store.RoleRegisteredOfficer,
		GrantedBy:     "system",
		ServiceAccess: []string{"signing", "garudainfo"},
	}
	if err := h.deps.RoleStore.Assign(ctx, role); err != nil {
		slog.Error("failed to assign role", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to assign role")
		return
	}

	resp := registerResponse{
		EntityID: entity.ID,
		Role:     store.RoleRegisteredOfficer,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// tokenizeNIK produces a deterministic HMAC-SHA256 token from a NIK.
func tokenizeNIK(nik string, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(nik))
	return hex.EncodeToString(mac.Sum(nil))
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
