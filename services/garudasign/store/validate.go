package store

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

// Validators in this file are called by the Postgres-backed stores (production
// data path) to enforce ETSI EN 319 142 / RFC 5280 conformance. The InMemory
// stores are deliberately permissive — they are test fakes whose job is to
// support flexible fixtures without forcing every test to populate every
// field. Enterprise correctness is enforced where it matters: the persistence
// layer that survives process restart.

// Enterprise validation limits for signing artifacts (ETSI EN 319 142, RFC 5280).
const (
	MaxUserIDLen        = 128
	MaxSerialLen        = 64
	MaxDNLen            = 500
	MaxStatusLen        = 32
	MaxCertPEMLen       = 16384 // 16KB — 2x RSA-4096 cert PEM
	MaxFingerprintLen   = 128
	MaxRevokeReasonLen  = 50

	MaxDocNameLen   = 255
	MaxDocPathLen   = 500
	MaxHashLen      = 128
	MaxErrorMsgLen  = 1024
	MaxDocSizeBytes = int64(100 * 1024 * 1024) // 100MB hard ceiling
	MinDocSizeBytes = int64(1)

	MaxPAdESLevelLen = 32
)

var allowedCertStatuses = map[string]bool{
	"":        true, // empty defaults handled by caller
	"ACTIVE":  true,
	"REVOKED": true,
	"EXPIRED": true,
}

var allowedRequestStatuses = map[string]bool{
	"":           true,
	"PENDING":    true,
	"PROCESSING": true,
	"COMPLETED":  true,
	"FAILED":     true,
	"EXPIRED":    true,
}

// allowedRevocationReasons follows RFC 5280 §5.3.1 CRLReason values.
var allowedRevocationReasons = map[string]bool{
	"":                       true,
	"unspecified":            true,
	"key_compromise":         true,
	"ca_compromise":          true,
	"affiliation_changed":    true,
	"superseded":             true,
	"cessation_of_operation": true,
	"certificate_hold":       true,
	"privilege_withdrawn":    true,
	"aa_compromise":          true,
}

// allowedPAdESLevels per ETSI EN 319 142.
var allowedPAdESLevels = map[string]bool{
	"":          true,
	"PAdES-B":   true,
	"PAdES-B-B": true,
	"PAdES-B-T": true,
	"PAdES-B-LT":  true,
	"PAdES-B-LTA": true,
}

// ValidateCertificate enforces cert field requirements + RFC 5280 conformance.
func ValidateCertificate(c *signing.Certificate) error {
	if c == nil {
		return fmt.Errorf("certificate is nil")
	}
	if err := requireBounded("user_id", c.UserID, MaxUserIDLen); err != nil {
		return err
	}
	if err := requireBounded("serial_number", c.SerialNumber, MaxSerialLen); err != nil {
		return err
	}
	if err := requireBounded("issuer_dn", c.IssuerDN, MaxDNLen); err != nil {
		return err
	}
	if err := requireBounded("subject_dn", c.SubjectDN, MaxDNLen); err != nil {
		return err
	}
	if err := requireBounded("certificate_pem", c.CertificatePEM, MaxCertPEMLen); err != nil {
		return err
	}
	if !strings.Contains(c.CertificatePEM, "-----BEGIN CERTIFICATE-----") {
		return fmt.Errorf("certificate_pem missing PEM header")
	}
	if err := requireBounded("fingerprint_sha256", c.FingerprintSHA256, MaxFingerprintLen); err != nil {
		return err
	}
	if !allowedCertStatuses[c.Status] {
		return fmt.Errorf("status %q not in allowed set", c.Status)
	}
	if c.ValidFrom.IsZero() || c.ValidTo.IsZero() {
		return fmt.Errorf("valid_from and valid_to are required")
	}
	if !c.ValidTo.After(c.ValidFrom) {
		return fmt.Errorf("valid_to must be after valid_from")
	}
	// Sanity: cert lifetime ≤ 25y (browser CABF baseline + slack for eIDAS QSCDs)
	if c.ValidTo.Sub(c.ValidFrom) > 25*365*24*time.Hour {
		return fmt.Errorf("certificate lifetime exceeds 25 years")
	}
	return nil
}

// ValidateUpdateCertStatus enforces revoke reason allow-list.
func ValidateUpdateCertStatus(status, reason string) error {
	if !allowedCertStatuses[status] {
		return fmt.Errorf("status %q not in allowed set", status)
	}
	if utf8.RuneCountInString(reason) > MaxRevokeReasonLen {
		return fmt.Errorf("revocation_reason exceeds %d chars", MaxRevokeReasonLen)
	}
	if !allowedRevocationReasons[reason] {
		return fmt.Errorf("revocation_reason %q not RFC 5280 conformant", reason)
	}
	return nil
}

// ValidateSigningRequest enforces document field requirements.
func ValidateSigningRequest(r *signing.SigningRequest) error {
	if r == nil {
		return fmt.Errorf("signing request is nil")
	}
	if err := requireBounded("user_id", r.UserID, MaxUserIDLen); err != nil {
		return err
	}
	if err := requireBounded("document_name", r.DocumentName, MaxDocNameLen); err != nil {
		return err
	}
	if err := requireBounded("document_hash", r.DocumentHash, MaxHashLen); err != nil {
		return err
	}
	if err := requireBounded("document_path", r.DocumentPath, MaxDocPathLen); err != nil {
		return err
	}
	if r.DocumentSize < MinDocSizeBytes {
		return fmt.Errorf("document_size %d below minimum %d", r.DocumentSize, MinDocSizeBytes)
	}
	if r.DocumentSize > MaxDocSizeBytes {
		return fmt.Errorf("document_size %d exceeds %d (100MB)", r.DocumentSize, MaxDocSizeBytes)
	}
	if !allowedRequestStatuses[r.Status] {
		return fmt.Errorf("status %q not in allowed set", r.Status)
	}
	if r.ExpiresAt.IsZero() {
		return fmt.Errorf("expires_at is required")
	}
	return nil
}

// ValidateUpdateRequestStatus enforces enum + length bounds.
func ValidateUpdateRequestStatus(status, errorMsg string) error {
	if !allowedRequestStatuses[status] {
		return fmt.Errorf("status %q not in allowed set", status)
	}
	if utf8.RuneCountInString(errorMsg) > MaxErrorMsgLen {
		return fmt.Errorf("error_message exceeds %d chars", MaxErrorMsgLen)
	}
	return nil
}

// ValidateSignedDocument enforces signed-doc invariants per ETSI EN 319 142.
func ValidateSignedDocument(d *signing.SignedDocument) error {
	if d == nil {
		return fmt.Errorf("signed document is nil")
	}
	if err := requireBounded("request_id", d.RequestID, MaxUserIDLen); err != nil {
		return err
	}
	if err := requireBounded("certificate_id", d.CertificateID, MaxUserIDLen); err != nil {
		return err
	}
	if err := requireBounded("signed_hash", d.SignedHash, MaxHashLen); err != nil {
		return err
	}
	if err := requireBounded("signed_path", d.SignedPath, MaxDocPathLen); err != nil {
		return err
	}
	if d.SignedSize < MinDocSizeBytes || d.SignedSize > MaxDocSizeBytes {
		return fmt.Errorf("signed_size %d outside allowed range", d.SignedSize)
	}
	if !allowedPAdESLevels[d.PAdESLevel] {
		return fmt.Errorf("pades_level %q not ETSI EN 319 142 conformant", d.PAdESLevel)
	}
	if d.SignatureTimestamp.IsZero() {
		return fmt.Errorf("signature_timestamp is required")
	}
	return nil
}

func requireBounded(name, v string, max int) error {
	if v == "" {
		return fmt.Errorf("%s is required", name)
	}
	if utf8.RuneCountInString(v) > max {
		return fmt.Errorf("%s exceeds %d chars", name, max)
	}
	if strings.ContainsAny(v, "\x00") {
		return fmt.Errorf("%s contains null bytes", name)
	}
	return nil
}
