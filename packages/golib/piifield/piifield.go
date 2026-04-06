// Package piifield provides PII field classification and handling
// per UU PDP No. 27/2022. Classifies data fields by sensitivity
// level and provides masking, consent requirements, and retention rules.
package piifield

import "strings"

// Sensitivity level for PII fields.
type Sensitivity int

const (
	// Public data, no special handling needed.
	Public Sensitivity = iota
	// Internal data, basic access controls.
	Internal
	// Sensitive PII requiring consent and encryption.
	Sensitive
	// HighlySensitive PII with strict controls (biometrics, health, religion).
	HighlySensitive
)

// String returns the sensitivity level name.
func (s Sensitivity) String() string {
	switch s {
	case Public:
		return "public"
	case Internal:
		return "internal"
	case Sensitive:
		return "sensitive"
	case HighlySensitive:
		return "highly_sensitive"
	default:
		return "unknown"
	}
}

// Field describes a PII data field.
type Field struct {
	Name           string      `json:"name"`
	Sensitivity    Sensitivity `json:"sensitivity"`
	ConsentScope   string      `json:"consent_scope"`
	RequiresConsent bool       `json:"requires_consent"`
	Encrypted      bool        `json:"encrypted"`
	MaskPattern    string      `json:"mask_pattern,omitempty"`
	RetentionDays  int         `json:"retention_days,omitempty"`
}

// Registry holds PII field definitions.
type Registry struct {
	fields map[string]Field
}

// NewRegistry creates a PII field registry.
func NewRegistry() *Registry {
	return &Registry{fields: make(map[string]Field)}
}

// Register adds a field definition.
func (r *Registry) Register(f Field) {
	r.fields[f.Name] = f
}

// Get returns a field definition.
func (r *Registry) Get(name string) (Field, bool) {
	f, ok := r.fields[name]
	return f, ok
}

// IsPII checks if a field is classified as PII.
func (r *Registry) IsPII(name string) bool {
	f, ok := r.fields[name]
	return ok && f.Sensitivity >= Sensitive
}

// RequiresConsent checks if a field requires user consent.
func (r *Registry) RequiresConsent(name string) bool {
	f, ok := r.fields[name]
	return ok && f.RequiresConsent
}

// FieldsRequiringConsent returns all fields that need consent for a scope.
func (r *Registry) FieldsRequiringConsent(scope string) []Field {
	var result []Field
	for _, f := range r.fields {
		if f.RequiresConsent && f.ConsentScope == scope {
			result = append(result, f)
		}
	}
	return result
}

// Classify returns all field names grouped by sensitivity.
func (r *Registry) Classify() map[Sensitivity][]string {
	result := make(map[Sensitivity][]string)
	for _, f := range r.fields {
		result[f.Sensitivity] = append(result[f.Sensitivity], f.Name)
	}
	return result
}

// DefaultIndonesianRegistry returns PII field classifications per UU PDP.
func DefaultIndonesianRegistry() *Registry {
	r := NewRegistry()

	r.Register(Field{Name: "nik", Sensitivity: HighlySensitive, ConsentScope: "nik", RequiresConsent: true, Encrypted: true, MaskPattern: "****-****-****-####"})
	r.Register(Field{Name: "name", Sensitivity: Sensitive, ConsentScope: "name", RequiresConsent: true, Encrypted: false})
	r.Register(Field{Name: "email", Sensitivity: Sensitive, ConsentScope: "email", RequiresConsent: true, Encrypted: false})
	r.Register(Field{Name: "phone", Sensitivity: Sensitive, ConsentScope: "phone", RequiresConsent: true, Encrypted: false})
	r.Register(Field{Name: "address", Sensitivity: Sensitive, ConsentScope: "address", RequiresConsent: true, Encrypted: false})
	r.Register(Field{Name: "birth_date", Sensitivity: Sensitive, ConsentScope: "birth_date", RequiresConsent: true, Encrypted: false})
	r.Register(Field{Name: "birth_place", Sensitivity: Sensitive, ConsentScope: "birth_date", RequiresConsent: true, Encrypted: false})
	r.Register(Field{Name: "religion", Sensitivity: HighlySensitive, ConsentScope: "religion", RequiresConsent: true, Encrypted: true})
	r.Register(Field{Name: "blood_type", Sensitivity: HighlySensitive, ConsentScope: "blood_type", RequiresConsent: true, Encrypted: true})
	r.Register(Field{Name: "marital_status", Sensitivity: Sensitive, ConsentScope: "marital_status", RequiresConsent: true, Encrypted: false})
	r.Register(Field{Name: "photo", Sensitivity: HighlySensitive, ConsentScope: "photo", RequiresConsent: true, Encrypted: true})
	r.Register(Field{Name: "family_members", Sensitivity: Sensitive, ConsentScope: "family", RequiresConsent: true, Encrypted: false})

	return r
}

// MaskField masks a PII value based on its field name.
func MaskField(fieldName, value string) string {
	if value == "" {
		return ""
	}
	switch fieldName {
	case "nik":
		if len(value) >= 4 {
			return strings.Repeat("*", len(value)-4) + value[len(value)-4:]
		}
	case "email":
		parts := strings.SplitN(value, "@", 2)
		if len(parts) == 2 {
			local := parts[0]
			if len(local) > 2 {
				return local[:2] + strings.Repeat("*", len(local)-2) + "@" + parts[1]
			}
		}
	case "phone":
		if len(value) > 4 {
			return strings.Repeat("*", len(value)-4) + value[len(value)-4:]
		}
	case "name":
		words := strings.Fields(value)
		if len(words) > 0 {
			masked := make([]string, len(words))
			for i, w := range words {
				if len(w) > 1 {
					masked[i] = string(w[0]) + strings.Repeat("*", len(w)-1)
				} else {
					masked[i] = "*"
				}
			}
			return strings.Join(masked, " ")
		}
	}
	// Default: mask middle
	if len(value) > 4 {
		return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
	}
	return strings.Repeat("*", len(value))
}
