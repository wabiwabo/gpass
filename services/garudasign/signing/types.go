package signing

import "time"

// CertificateIssueRequest represents a request to issue a new certificate.
type CertificateIssueRequest struct {
	SubjectCN    string `json:"subject_cn"`
	SubjectUID   string `json:"subject_uid"`
	ValidityDays int    `json:"validity_days"`
}

// CertificateIssueResponse represents the response from certificate issuance.
type CertificateIssueResponse struct {
	SerialNumber      string `json:"serial_number"`
	CertificatePEM    string `json:"certificate_pem"`
	IssuerDN          string `json:"issuer_dn"`
	SubjectDN         string `json:"subject_dn"`
	ValidFrom         string `json:"valid_from"`
	ValidTo           string `json:"valid_to"`
	FingerprintSHA256 string `json:"fingerprint_sha256"`
}

// SignRequest represents a request to sign a document.
type SignRequest struct {
	DocumentBase64 string `json:"document_base64"`
	CertificatePEM string `json:"certificate_pem"`
	SignatureLevel string `json:"signature_level"`
}

// SignResponse represents the response from document signing.
type SignResponse struct {
	SignedDocumentBase64 string `json:"signed_document_base64"`
	SignatureTimestamp    string `json:"signature_timestamp"`
	PAdESLevel           string `json:"pades_level"`
}

// Certificate represents a stored certificate.
type Certificate struct {
	ID                string     `json:"id"`
	UserID            string     `json:"user_id"`
	SerialNumber      string     `json:"serial_number"`
	IssuerDN          string     `json:"issuer_dn"`
	SubjectDN         string     `json:"subject_dn"`
	Status            string     `json:"status"`
	ValidFrom         time.Time  `json:"valid_from"`
	ValidTo           time.Time  `json:"valid_to"`
	CertificatePEM    string     `json:"certificate_pem"`
	FingerprintSHA256 string     `json:"fingerprint_sha256"`
	RevokedAt         *time.Time `json:"revoked_at,omitempty"`
	RevocationReason  string     `json:"revocation_reason,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// SigningRequest represents a document signing request.
type SigningRequest struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	CertificateID string    `json:"certificate_id,omitempty"`
	DocumentName  string    `json:"document_name"`
	DocumentSize  int64     `json:"document_size"`
	DocumentHash  string    `json:"document_hash"`
	DocumentPath  string    `json:"document_path"`
	Status        string    `json:"status"`
	ErrorMessage  string    `json:"error_message,omitempty"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SignedDocument represents a completed signed document.
type SignedDocument struct {
	ID                 string    `json:"id"`
	RequestID          string    `json:"request_id"`
	CertificateID      string    `json:"certificate_id"`
	SignedHash         string    `json:"signed_hash"`
	SignedPath         string    `json:"signed_path"`
	SignedSize         int64     `json:"signed_size"`
	PAdESLevel         string    `json:"pades_level"`
	SignatureTimestamp time.Time `json:"signature_timestamp"`
	CreatedAt          time.Time `json:"created_at"`
}
