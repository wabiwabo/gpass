package handler

import (
	"encoding/json"
	"net/http"
)

// WebhookTemplate describes a webhook event type with example payload and field docs.
type WebhookTemplate struct {
	EventType      string         `json:"event_type"`
	Description    string         `json:"description"`
	ExamplePayload map[string]any `json:"example_payload"`
	Fields         []FieldDescription `json:"fields"`
}

// FieldDescription documents a single field within a webhook payload.
type FieldDescription struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// TemplateHandler provides webhook event payload templates.
type TemplateHandler struct{}

// NewTemplateHandler creates a new TemplateHandler.
func NewTemplateHandler() *TemplateHandler {
	return &TemplateHandler{}
}

// GetTemplates handles GET /api/v1/portal/webhooks/templates.
// Returns all available webhook event types with example payloads.
func (h *TemplateHandler) GetTemplates(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"templates": AllTemplates(),
	})
}

// GetTemplate handles GET /api/v1/portal/webhooks/templates/{event_type}.
// Returns a single template with full documentation.
func (h *TemplateHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	eventType := r.PathValue("event_type")

	for _, tmpl := range AllTemplates() {
		if tmpl.EventType == eventType {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(tmpl)
			return
		}
	}

	writeError(w, http.StatusNotFound, "not_found", "webhook template not found")
}

// AllTemplates returns all webhook event templates.
func AllTemplates() []WebhookTemplate {
	return []WebhookTemplate{
		{
			EventType:   "identity.verified",
			Description: "Fired when a user completes identity verification",
			ExamplePayload: map[string]any{
				"user_id":             "550e8400-e29b-41d4-a716-446655440000",
				"verification_status": "VERIFIED",
				"auth_level":          2,
				"timestamp":           "2026-04-06T10:30:00Z",
			},
			Fields: []FieldDescription{
				{Name: "user_id", Type: "string", Description: "User identifier", Required: true},
				{Name: "verification_status", Type: "string", Description: "Verification result: VERIFIED or REJECTED", Required: true},
				{Name: "auth_level", Type: "integer", Description: "Authentication assurance level (1-3)", Required: true},
				{Name: "timestamp", Type: "string", Description: "ISO 8601 timestamp of the event", Required: true},
			},
		},
		{
			EventType:   "identity.consent.granted",
			Description: "Fired when a user grants data access consent",
			ExamplePayload: map[string]any{
				"consent_id": "550e8400-e29b-41d4-a716-446655440001",
				"user_id":    "550e8400-e29b-41d4-a716-446655440000",
				"client_id":  "app-123",
				"fields":     []string{"name", "nik", "dob"},
				"expires_at": "2026-07-06T10:30:00Z",
				"timestamp":  "2026-04-06T10:30:00Z",
			},
			Fields: []FieldDescription{
				{Name: "consent_id", Type: "string", Description: "Unique consent identifier", Required: true},
				{Name: "user_id", Type: "string", Description: "User granting consent", Required: true},
				{Name: "client_id", Type: "string", Description: "Application receiving consent", Required: true},
				{Name: "fields", Type: "array", Description: "Data fields included in consent", Required: true},
				{Name: "expires_at", Type: "string", Description: "Consent expiration time", Required: true},
				{Name: "timestamp", Type: "string", Description: "ISO 8601 timestamp of the event", Required: true},
			},
		},
		{
			EventType:   "identity.consent.revoked",
			Description: "Fired when a user revokes data access consent",
			ExamplePayload: map[string]any{
				"consent_id": "550e8400-e29b-41d4-a716-446655440001",
				"user_id":    "550e8400-e29b-41d4-a716-446655440000",
				"client_id":  "app-123",
				"reason":     "user_requested",
				"timestamp":  "2026-04-06T10:30:00Z",
			},
			Fields: []FieldDescription{
				{Name: "consent_id", Type: "string", Description: "Revoked consent identifier", Required: true},
				{Name: "user_id", Type: "string", Description: "User revoking consent", Required: true},
				{Name: "client_id", Type: "string", Description: "Application whose consent was revoked", Required: true},
				{Name: "reason", Type: "string", Description: "Revocation reason", Required: false},
				{Name: "timestamp", Type: "string", Description: "ISO 8601 timestamp of the event", Required: true},
			},
		},
		{
			EventType:   "corp.entity.verified",
			Description: "Fired when a corporate entity passes verification",
			ExamplePayload: map[string]any{
				"entity_id":   "550e8400-e29b-41d4-a716-446655440002",
				"entity_name": "PT Maju Bersama",
				"entity_type": "PT",
				"nib":         "1234567890123",
				"status":      "VERIFIED",
				"timestamp":   "2026-04-06T10:30:00Z",
			},
			Fields: []FieldDescription{
				{Name: "entity_id", Type: "string", Description: "Corporate entity identifier", Required: true},
				{Name: "entity_name", Type: "string", Description: "Registered entity name", Required: true},
				{Name: "entity_type", Type: "string", Description: "Entity type (PT, CV, etc.)", Required: true},
				{Name: "nib", Type: "string", Description: "Nomor Induk Berusaha", Required: true},
				{Name: "status", Type: "string", Description: "Verification status", Required: true},
				{Name: "timestamp", Type: "string", Description: "ISO 8601 timestamp of the event", Required: true},
			},
		},
		{
			EventType:   "corp.role.assigned",
			Description: "Fired when a role is assigned to a user within a corporate entity",
			ExamplePayload: map[string]any{
				"entity_id": "550e8400-e29b-41d4-a716-446655440002",
				"user_id":   "550e8400-e29b-41d4-a716-446655440000",
				"role":      "DIRECTOR",
				"assigned_by": "550e8400-e29b-41d4-a716-446655440003",
				"timestamp":   "2026-04-06T10:30:00Z",
			},
			Fields: []FieldDescription{
				{Name: "entity_id", Type: "string", Description: "Corporate entity identifier", Required: true},
				{Name: "user_id", Type: "string", Description: "User receiving the role", Required: true},
				{Name: "role", Type: "string", Description: "Role name assigned", Required: true},
				{Name: "assigned_by", Type: "string", Description: "User who assigned the role", Required: true},
				{Name: "timestamp", Type: "string", Description: "ISO 8601 timestamp of the event", Required: true},
			},
		},
		{
			EventType:   "corp.role.revoked",
			Description: "Fired when a role is revoked from a user within a corporate entity",
			ExamplePayload: map[string]any{
				"entity_id":  "550e8400-e29b-41d4-a716-446655440002",
				"user_id":    "550e8400-e29b-41d4-a716-446655440000",
				"role":       "DIRECTOR",
				"revoked_by": "550e8400-e29b-41d4-a716-446655440003",
				"reason":     "role_change",
				"timestamp":  "2026-04-06T10:30:00Z",
			},
			Fields: []FieldDescription{
				{Name: "entity_id", Type: "string", Description: "Corporate entity identifier", Required: true},
				{Name: "user_id", Type: "string", Description: "User whose role was revoked", Required: true},
				{Name: "role", Type: "string", Description: "Role name revoked", Required: true},
				{Name: "revoked_by", Type: "string", Description: "User who revoked the role", Required: true},
				{Name: "reason", Type: "string", Description: "Revocation reason", Required: false},
				{Name: "timestamp", Type: "string", Description: "ISO 8601 timestamp of the event", Required: true},
			},
		},
		{
			EventType:   "sign.certificate.issued",
			Description: "Fired when a digital certificate is issued to a user",
			ExamplePayload: map[string]any{
				"certificate_id": "550e8400-e29b-41d4-a716-446655440004",
				"user_id":        "550e8400-e29b-41d4-a716-446655440000",
				"issuer":         "GarudaSign CA",
				"serial_number":  "ABC123DEF456",
				"valid_from":     "2026-04-06T00:00:00Z",
				"valid_until":    "2027-04-06T00:00:00Z",
				"timestamp":      "2026-04-06T10:30:00Z",
			},
			Fields: []FieldDescription{
				{Name: "certificate_id", Type: "string", Description: "Certificate identifier", Required: true},
				{Name: "user_id", Type: "string", Description: "User the certificate was issued to", Required: true},
				{Name: "issuer", Type: "string", Description: "Certificate authority name", Required: true},
				{Name: "serial_number", Type: "string", Description: "Certificate serial number", Required: true},
				{Name: "valid_from", Type: "string", Description: "Certificate validity start", Required: true},
				{Name: "valid_until", Type: "string", Description: "Certificate validity end", Required: true},
				{Name: "timestamp", Type: "string", Description: "ISO 8601 timestamp of the event", Required: true},
			},
		},
		{
			EventType:   "sign.document.signed",
			Description: "Fired when a document is successfully signed",
			ExamplePayload: map[string]any{
				"document_id":    "550e8400-e29b-41d4-a716-446655440005",
				"signer_id":     "550e8400-e29b-41d4-a716-446655440000",
				"certificate_id": "550e8400-e29b-41d4-a716-446655440004",
				"signature_type": "qualified",
				"hash":           "sha256:abc123def456",
				"timestamp":      "2026-04-06T10:30:00Z",
			},
			Fields: []FieldDescription{
				{Name: "document_id", Type: "string", Description: "Signed document identifier", Required: true},
				{Name: "signer_id", Type: "string", Description: "User who signed the document", Required: true},
				{Name: "certificate_id", Type: "string", Description: "Certificate used for signing", Required: true},
				{Name: "signature_type", Type: "string", Description: "Signature type (basic, advanced, qualified)", Required: true},
				{Name: "hash", Type: "string", Description: "Document hash with algorithm prefix", Required: true},
				{Name: "timestamp", Type: "string", Description: "ISO 8601 timestamp of the event", Required: true},
			},
		},
		{
			EventType:   "sign.document.failed",
			Description: "Fired when a document signing attempt fails",
			ExamplePayload: map[string]any{
				"document_id": "550e8400-e29b-41d4-a716-446655440005",
				"signer_id":   "550e8400-e29b-41d4-a716-446655440000",
				"error_code":  "CERTIFICATE_EXPIRED",
				"error_message": "The signing certificate has expired",
				"timestamp":    "2026-04-06T10:30:00Z",
			},
			Fields: []FieldDescription{
				{Name: "document_id", Type: "string", Description: "Document that failed to sign", Required: true},
				{Name: "signer_id", Type: "string", Description: "User who attempted signing", Required: true},
				{Name: "error_code", Type: "string", Description: "Machine-readable error code", Required: true},
				{Name: "error_message", Type: "string", Description: "Human-readable error description", Required: true},
				{Name: "timestamp", Type: "string", Description: "ISO 8601 timestamp of the event", Required: true},
			},
		},
	}
}
