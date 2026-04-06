package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/garudapass/gpass/services/identity/crypto"
	"github.com/garudapass/gpass/services/identity/dukcapil"
)

// DukcapilVerifier abstracts the Dukcapil client for testing.
type DukcapilVerifier interface {
	VerifyNIK(ctx context.Context, nik string) (*dukcapil.NIKVerifyResponse, error)
	VerifyBiometric(ctx context.Context, nik, selfieB64 string) (*dukcapil.BiometricResponse, error)
	VerifyDemographic(ctx context.Context, req *dukcapil.DemographicRequest) (*dukcapil.DemographicResponse, error)
}

// OTPGenerator abstracts the OTP service for testing.
type OTPGenerator interface {
	Generate(ctx context.Context, registrationID, channel string) (string, error)
	Verify(ctx context.Context, registrationID, channel, code string) error
}

// RegisterDeps holds the dependencies for the registration handler.
type RegisterDeps struct {
	Dukcapil DukcapilVerifier
	OTP      OTPGenerator
	NIKKey   []byte
}

// RegisterHandler handles user registration flows.
type RegisterHandler struct {
	deps RegisterDeps
}

// NewRegisterHandler creates a new registration handler.
func NewRegisterHandler(deps RegisterDeps) *RegisterHandler {
	return &RegisterHandler{deps: deps}
}

type initiateRequest struct {
	NIK   string `json:"nik"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

type initiateResponse struct {
	RegistrationID string    `json:"registration_id"`
	OTPExpiresAt   time.Time `json:"otp_expires_at"`
}

// Initiate handles POST /api/v1/register/initiate.
func (h *RegisterHandler) Initiate(w http.ResponseWriter, r *http.Request) {
	var req initiateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	// Validate NIK format
	if err := crypto.ValidateNIKFormat(req.NIK); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_nik", err.Error())
		return
	}

	// Verify NIK with Dukcapil
	nikResp, err := h.deps.Dukcapil.VerifyNIK(r.Context(), req.NIK)
	if err != nil {
		slog.Error("dukcapil verify failed", "error", err)
		writeError(w, http.StatusBadGateway, "dukcapil_unavailable", "Identity verification service is unavailable")
		return
	}

	if !nikResp.Valid {
		writeError(w, http.StatusBadRequest, "invalid_nik", "NIK is not valid")
		return
	}

	if !nikResp.Alive {
		writeError(w, http.StatusBadRequest, "deceased", "NIK belongs to a deceased individual")
		return
	}

	// Generate registration ID
	regID, err := generateRegistrationID()
	if err != nil {
		slog.Error("failed to generate registration ID", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to process registration")
		return
	}

	// Send OTPs
	if _, err := h.deps.OTP.Generate(r.Context(), regID, "phone"); err != nil {
		slog.Error("failed to generate phone OTP", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to send OTP")
		return
	}
	if _, err := h.deps.OTP.Generate(r.Context(), regID, "email"); err != nil {
		slog.Error("failed to generate email OTP", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to send OTP")
		return
	}

	resp := initiateResponse{
		RegistrationID: regID,
		OTPExpiresAt:   time.Now().Add(5 * time.Minute),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
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

func generateRegistrationID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
