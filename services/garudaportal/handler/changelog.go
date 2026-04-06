package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// ChangelogEntry represents a single API changelog entry.
type ChangelogEntry struct {
	Version     string `json:"version"`
	Date        string `json:"date"`
	Type        string `json:"type"` // added, changed, deprecated, removed, security
	Endpoint    string `json:"endpoint,omitempty"`
	Description string `json:"description"`
	Breaking    bool   `json:"breaking"`
}

// ChangelogHandler provides API change history for developer tracking.
type ChangelogHandler struct {
	entries []ChangelogEntry
}

// NewChangelogHandler creates a new changelog handler with the given entries.
func NewChangelogHandler(entries []ChangelogEntry) *ChangelogHandler {
	return &ChangelogHandler{entries: entries}
}

type changelogResponse struct {
	Entries    []ChangelogEntry `json:"entries"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PerPage    int              `json:"per_page"`
	TotalPages int              `json:"total_pages"`
}

// GetChangelog handles GET /api/v1/portal/changelog.
// Query parameters:
//   - version: filter by version prefix (e.g., "v1.0")
//   - type: filter by change type; use "breaking" to filter breaking changes
//   - page: page number (default 1)
//   - per_page: entries per page (default 20, max 100)
func (h *ChangelogHandler) GetChangelog(w http.ResponseWriter, r *http.Request) {
	versionFilter := r.URL.Query().Get("version")
	typeFilter := r.URL.Query().Get("type")

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	// Filter entries.
	var filtered []ChangelogEntry
	for _, e := range h.entries {
		if versionFilter != "" && !strings.HasPrefix(e.Version, versionFilter) {
			continue
		}
		if typeFilter == "breaking" {
			if !e.Breaking {
				continue
			}
		} else if typeFilter != "" && e.Type != typeFilter {
			continue
		}
		filtered = append(filtered, e)
	}

	total := len(filtered)
	totalPages := (total + perPage - 1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}

	// Paginate.
	start := (page - 1) * perPage
	if start > total {
		start = total
	}
	end := start + perPage
	if end > total {
		end = total
	}

	resp := changelogResponse{
		Entries:    filtered[start:end],
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}

	if resp.Entries == nil {
		resp.Entries = []ChangelogEntry{}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// DefaultChangelog returns the GarudaPass API changelog entries.
func DefaultChangelog() []ChangelogEntry {
	return []ChangelogEntry{
		{
			Version:     "v1.0.0",
			Date:        "2026-01-15",
			Type:        "added",
			Description: "Initial release with identity verification, corporate entity, digital signing, and developer portal APIs",
			Breaking:    false,
		},
		{
			Version:     "v1.0.0",
			Date:        "2026-01-15",
			Type:        "added",
			Endpoint:    "/api/v1/identity/verify",
			Description: "Identity verification endpoint with Dukcapil integration",
			Breaking:    false,
		},
		{
			Version:     "v1.0.0",
			Date:        "2026-01-15",
			Type:        "added",
			Endpoint:    "/api/v1/corporate/nib",
			Description: "Corporate entity NIB lookup via OSS/BKPM integration",
			Breaking:    false,
		},
		{
			Version:     "v1.0.0",
			Date:        "2026-01-15",
			Type:        "added",
			Endpoint:    "/api/v1/sign/documents",
			Description: "Digital document signing with certificate management",
			Breaking:    false,
		},
		{
			Version:     "v1.0.0",
			Date:        "2026-01-15",
			Type:        "added",
			Endpoint:    "/api/v1/portal/apps",
			Description: "Developer portal application management",
			Breaking:    false,
		},
		{
			Version:     "v1.1.0",
			Date:        "2026-02-20",
			Type:        "added",
			Endpoint:    "/api/v1/portal/webhooks",
			Description: "Webhook v2 signing with Ed25519 for improved security",
			Breaking:    false,
		},
		{
			Version:     "v1.1.0",
			Date:        "2026-02-20",
			Type:        "added",
			Endpoint:    "/api/v1/portal/analytics",
			Description: "API usage analytics with time-series data and top endpoints",
			Breaking:    false,
		},
		{
			Version:     "v1.1.0",
			Date:        "2026-02-20",
			Type:        "changed",
			Endpoint:    "/api/v1/portal/webhooks",
			Description: "Deprecated HMAC-SHA256 webhook signing in favor of Ed25519",
			Breaking:    false,
		},
		{
			Version:     "v1.2.0",
			Date:        "2026-03-25",
			Type:        "added",
			Endpoint:    "/api/v1/certificates/revoke",
			Description: "Certificate revocation endpoint with CRL distribution",
			Breaking:    false,
		},
		{
			Version:     "v1.2.0",
			Date:        "2026-03-25",
			Type:        "added",
			Endpoint:    "/api/v1/keys/rotate",
			Description: "API key rotation with grace period for seamless migration",
			Breaking:    false,
		},
		{
			Version:     "v1.2.0",
			Date:        "2026-03-25",
			Type:        "added",
			Endpoint:    "/api/v1/batch",
			Description: "Batch API for executing multiple operations in a single request",
			Breaking:    false,
		},
		{
			Version:     "v1.2.0",
			Date:        "2026-03-25",
			Type:        "changed",
			Endpoint:    "/api/v1/sign/documents",
			Description: "Signing endpoint now requires auth_level >= 2 for all operations",
			Breaking:    true,
		},
		{
			Version:     "v1.2.0",
			Date:        "2026-03-25",
			Type:        "security",
			Description: "Enforced minimum TLS 1.3 for all API connections",
			Breaking:    true,
		},
	}
}
