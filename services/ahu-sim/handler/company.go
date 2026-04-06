package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/garudapass/gpass/services/ahu-sim/data"
)

// --- Request / Response types ---

type SearchRequest struct {
	SKNumber string `json:"sk_number"`
	Name     string `json:"name"`
}

type SearchResponse struct {
	Companies []data.Company `json:"companies"`
}

type OfficersResponse struct {
	Officers []data.Officer `json:"officers"`
}

type ShareholdersResponse struct {
	Shareholders []data.Shareholder `json:"shareholders"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// --- Handlers ---

// SearchCompany handles POST /api/v1/ahu/company/search
func SearchCompany(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("invalid request body", "error", err)
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "Invalid JSON body",
		})
		return
	}

	if req.SKNumber == "" && req.Name == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "Either sk_number or name is required",
		})
		return
	}

	var companies []data.Company

	if req.SKNumber != "" {
		c := data.LookupBySK(req.SKNumber)
		if c != nil {
			companies = append(companies, *c)
		}
	} else {
		companies = data.SearchByName(req.Name)
	}

	if companies == nil {
		companies = []data.Company{}
	}

	writeJSON(w, http.StatusOK, SearchResponse{Companies: companies})
}

// GetOfficers handles GET /api/v1/ahu/company/{sk}/officers
func GetOfficers(w http.ResponseWriter, r *http.Request) {
	sk := r.PathValue("sk")
	if sk == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "SK number is required",
		})
		return
	}

	c := data.LookupBySK(sk)
	if c == nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Company not found",
		})
		return
	}

	officers := c.Officers
	if officers == nil {
		officers = []data.Officer{}
	}

	writeJSON(w, http.StatusOK, OfficersResponse{Officers: officers})
}

// GetShareholders handles GET /api/v1/ahu/company/{sk}/shareholders
func GetShareholders(w http.ResponseWriter, r *http.Request) {
	sk := r.PathValue("sk")
	if sk == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Message: "SK number is required",
		})
		return
	}

	c := data.LookupBySK(sk)
	if c == nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Company not found",
		})
		return
	}

	shareholders := c.Shareholders
	if shareholders == nil {
		shareholders = []data.Shareholder{}
	}

	writeJSON(w, http.StatusOK, ShareholdersResponse{Shareholders: shareholders})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
