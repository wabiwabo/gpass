package handler

import (
	"encoding/json"
	"net/http"
	"strings"
)

// MaskingHandler provides data masking for PII fields.
type MaskingHandler struct{}

// NewMaskingHandler creates a new MaskingHandler.
func NewMaskingHandler() *MaskingHandler {
	return &MaskingHandler{}
}

type maskRequest struct {
	Fields    map[string]string `json:"fields"`
	MaskLevel string           `json:"mask_level"`
}

// MaskData handles POST /api/v1/data/mask.
// It applies field-type-aware masking based on the requested mask level.
func (h *MaskingHandler) MaskData(w http.ResponseWriter, r *http.Request) {
	var req maskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if req.MaskLevel != "partial" && req.MaskLevel != "full" {
		writeError(w, http.StatusBadRequest, "invalid_mask_level", "mask_level must be 'partial' or 'full'")
		return
	}

	masked := make(map[string]string, len(req.Fields))
	for name, value := range req.Fields {
		masked[name] = MaskField(name, value, req.MaskLevel)
	}

	writeJSON(w, http.StatusOK, masked)
}

// MaskField applies field-type-aware masking to a value.
//
// Rules:
//
//	name:    partial=show first 2 + last 3, full=all stars
//	nik:     partial=show last 4 with dash formatting, full=all stars
//	phone:   partial=show +62 + last 4, full=all stars
//	email:   partial=first char + *** + @domain first char + ***.tld, full=all stars
//	default: partial=show first 2 + last 2, full=all stars
func MaskField(fieldName, value, level string) string {
	if value == "" {
		return ""
	}

	if level == "full" {
		return maskFull(fieldName, value)
	}

	switch fieldName {
	case "name":
		return maskNamePartial(value)
	case "nik":
		return maskNIKPartial(value)
	case "phone":
		return maskPhonePartial(value)
	case "email":
		return maskEmailPartial(value)
	default:
		return maskDefaultPartial(value)
	}
}

func maskFull(fieldName, value string) string {
	if fieldName == "email" {
		return "***@***.***"
	}
	return strings.Repeat("*", len(value))
}

// maskNamePartial shows first 2 and last 3 characters with stars in between.
func maskNamePartial(value string) string {
	runes := []rune(value)
	if len(runes) <= 5 {
		return strings.Repeat("*", len(runes))
	}

	var b strings.Builder
	for i, r := range runes {
		if i < 2 || i >= len(runes)-3 {
			b.WriteRune(r)
		} else if r == ' ' {
			b.WriteRune(' ')
		} else {
			b.WriteRune('*')
		}
	}
	return b.String()
}

// maskNIKPartial shows last 4 digits with dash-separated star groups.
func maskNIKPartial(value string) string {
	if len(value) <= 4 {
		return value
	}
	last4 := value[len(value)-4:]
	return "****-****-****-" + last4
}

// maskPhonePartial shows +62 prefix and last 4 digits with fixed mask.
func maskPhonePartial(value string) string {
	if len(value) <= 7 {
		return strings.Repeat("*", len(value))
	}
	last4 := value[len(value)-4:]
	return "+62****" + last4
}

// maskEmailPartial shows first char, ***, @, first domain char, ***.tld.
func maskEmailPartial(value string) string {
	parts := strings.SplitN(value, "@", 2)
	if len(parts) != 2 {
		return maskDefaultPartial(value)
	}

	local := parts[0]
	domain := parts[1]

	maskedLocal := string(local[0]) + "***"

	dotIdx := strings.LastIndex(domain, ".")
	if dotIdx <= 0 {
		return maskedLocal + "@" + "***"
	}

	tld := domain[dotIdx:]
	maskedDomain := string(domain[0]) + "***" + tld

	return maskedLocal + "@" + maskedDomain
}

// maskDefaultPartial shows first 2 and last 2 characters.
func maskDefaultPartial(value string) string {
	runes := []rune(value)
	if len(runes) <= 4 {
		return strings.Repeat("*", len(runes))
	}

	var b strings.Builder
	for i, r := range runes {
		if i < 2 || i >= len(runes)-2 {
			b.WriteRune(r)
		} else {
			b.WriteRune('*')
		}
	}
	return b.String()
}
