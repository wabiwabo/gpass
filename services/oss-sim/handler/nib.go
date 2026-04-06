package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/garudapass/gpass/services/oss-sim/data"
)

// --- Request / Response types ---

type SearchRequest struct {
	NPWP string `json:"npwp"`
	NIB  string `json:"nib"`
}

type SearchResponse struct {
	Businesses []data.Business `json:"businesses"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// --- Handlers ---

// SearchNIB handles POST /api/v1/oss/nib/search
func SearchNIB(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("invalid request body", "error", err)
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "Invalid JSON body",
		})
		return
	}

	if req.NPWP == "" && req.NIB == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "Either npwp or nib is required",
		})
		return
	}

	var businesses []data.Business

	if req.NIB != "" {
		b := data.SearchByNIB(req.NIB)
		if b != nil {
			businesses = append(businesses, *b)
		}
	} else {
		businesses = data.SearchByNPWP(req.NPWP)
	}

	if businesses == nil {
		businesses = []data.Business{}
	}

	writeJSON(w, http.StatusOK, SearchResponse{Businesses: businesses})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
