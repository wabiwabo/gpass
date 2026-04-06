package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/garudapass/gpass/services/dukcapil-sim/data"
)

// --- Request / Response types ---

type VerifyNIKRequest struct {
	NIK string `json:"nik"`
}

type VerifyNIKResponse struct {
	Valid    bool   `json:"valid"`
	Alive    bool   `json:"alive"`
	Province string `json:"province"`
}

type VerifyDemographicRequest struct {
	NIK    string `json:"nik"`
	Name   string `json:"name"`
	DOB    string `json:"dob"`
	Gender string `json:"gender"`
}

type VerifyDemographicResponse struct {
	Match      bool    `json:"match"`
	Confidence float64 `json:"confidence"`
}

type VerifyBiometricRequest struct {
	NIK          string `json:"nik"`
	SelfieBase64 string `json:"selfie_base64"`
}

type VerifyBiometricResponse struct {
	Match bool    `json:"match"`
	Score float64 `json:"score"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// --- Handlers ---

// VerifyNIK handles POST /api/v1/verify/nik
func VerifyNIK(w http.ResponseWriter, r *http.Request) {
	var req VerifyNIKRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("invalid request body", "error", err)
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "Invalid JSON body",
		})
		return
	}

	if req.NIK == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "NIK is required",
		})
		return
	}

	person := data.Lookup(req.NIK)
	if person == nil {
		writeJSON(w, http.StatusOK, VerifyNIKResponse{
			Valid:    false,
			Alive:    false,
			Province: "",
		})
		return
	}

	writeJSON(w, http.StatusOK, VerifyNIKResponse{
		Valid:    true,
		Alive:    person.Alive,
		Province: person.Province,
	})
}

// VerifyDemographic handles POST /api/v1/verify/demographic
func VerifyDemographic(w http.ResponseWriter, r *http.Request) {
	var req VerifyDemographicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("invalid request body", "error", err)
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "Invalid JSON body",
		})
		return
	}

	if req.NIK == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "NIK is required",
		})
		return
	}

	person := data.Lookup(req.NIK)
	if person == nil {
		writeJSON(w, http.StatusOK, VerifyDemographicResponse{
			Match:      false,
			Confidence: 0,
		})
		return
	}

	// Calculate confidence based on field matching
	matched := 0
	total := 3 // name, dob, gender

	if strings.EqualFold(strings.TrimSpace(req.Name), person.Name) {
		matched++
	}
	if strings.TrimSpace(req.DOB) == person.DOB {
		matched++
	}
	if strings.EqualFold(strings.TrimSpace(req.Gender), person.Gender) {
		matched++
	}

	confidence := float64(matched) / float64(total)
	match := matched == total

	writeJSON(w, http.StatusOK, VerifyDemographicResponse{
		Match:      match,
		Confidence: confidence,
	})
}

// VerifyBiometric handles POST /api/v1/verify/biometric
func VerifyBiometric(w http.ResponseWriter, r *http.Request) {
	var req VerifyBiometricRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("invalid request body", "error", err)
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "Invalid JSON body",
		})
		return
	}

	if req.NIK == "" || req.SelfieBase64 == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "NIK and selfie_base64 are required",
		})
		return
	}

	person := data.Lookup(req.NIK)
	if person == nil {
		writeJSON(w, http.StatusOK, VerifyBiometricResponse{
			Match: false,
			Score: 0,
		})
		return
	}

	// Exact photo match = 0.92, mismatch = 0.21
	if req.SelfieBase64 == person.PhotoB64 {
		writeJSON(w, http.StatusOK, VerifyBiometricResponse{
			Match: true,
			Score: 0.92,
		})
		return
	}

	writeJSON(w, http.StatusOK, VerifyBiometricResponse{
		Match: false,
		Score: 0.21,
	})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
