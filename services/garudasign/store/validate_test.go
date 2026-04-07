package store

import (
	"strings"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

func validCert() *signing.Certificate {
	return &signing.Certificate{
		UserID:            "user-1",
		SerialNumber:      "SN-001",
		IssuerDN:          "CN=GarudaPass CA",
		SubjectDN:         "CN=Test User",
		Status:            "ACTIVE",
		ValidFrom:         time.Now(),
		ValidTo:           time.Now().Add(365 * 24 * time.Hour),
		CertificatePEM:    "-----BEGIN CERTIFICATE-----\nFAKE\n-----END CERTIFICATE-----",
		FingerprintSHA256: "abc123",
	}
}

func TestValidateCertificate_Valid(t *testing.T) {
	if err := ValidateCertificate(validCert()); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateCertificate_Required(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*signing.Certificate)
	}{
		{"no user_id", func(c *signing.Certificate) { c.UserID = "" }},
		{"no serial", func(c *signing.Certificate) { c.SerialNumber = "" }},
		{"no issuer", func(c *signing.Certificate) { c.IssuerDN = "" }},
		{"no subject", func(c *signing.Certificate) { c.SubjectDN = "" }},
		{"no pem", func(c *signing.Certificate) { c.CertificatePEM = "" }},
		{"no fingerprint", func(c *signing.Certificate) { c.FingerprintSHA256 = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validCert()
			tc.mut(c)
			if err := ValidateCertificate(c); err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestValidateCertificate_PEMHeader(t *testing.T) {
	c := validCert()
	c.CertificatePEM = "not pem"
	if err := ValidateCertificate(c); err == nil {
		t.Error("expected PEM header error")
	}
}

func TestValidateCertificate_BadValidity(t *testing.T) {
	c := validCert()
	c.ValidTo = c.ValidFrom.Add(-1 * time.Hour)
	if err := ValidateCertificate(c); err == nil {
		t.Error("expected validity ordering error")
	}

	c = validCert()
	c.ValidTo = c.ValidFrom.Add(30 * 365 * 24 * time.Hour)
	if err := ValidateCertificate(c); err == nil {
		t.Error("expected lifetime cap")
	}
}

func TestValidateCertificate_NilAndStatus(t *testing.T) {
	if err := ValidateCertificate(nil); err == nil {
		t.Error("expected nil rejection")
	}
	c := validCert()
	c.Status = "MAYBE"
	if err := ValidateCertificate(c); err == nil {
		t.Error("expected enum violation")
	}
}

func TestValidateUpdateCertStatus_RFC5280Reasons(t *testing.T) {
	good := []string{"key_compromise", "ca_compromise", "superseded", "cessation_of_operation"}
	for _, r := range good {
		if err := ValidateUpdateCertStatus("REVOKED", r); err != nil {
			t.Errorf("%s: %v", r, err)
		}
	}
	if err := ValidateUpdateCertStatus("REVOKED", "we_felt_like_it"); err == nil {
		t.Error("expected non-RFC reason rejected")
	}
	if err := ValidateUpdateCertStatus("MAYBE", ""); err == nil {
		t.Error("expected status enum violation")
	}
}

func validReq() *signing.SigningRequest {
	return &signing.SigningRequest{
		UserID:       "user-1",
		DocumentName: "doc.pdf",
		DocumentSize: 1024,
		DocumentHash: "hash",
		DocumentPath: "/tmp/doc.pdf",
		Status:       "PENDING",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
}

func TestValidateSigningRequest_Valid(t *testing.T) {
	if err := ValidateSigningRequest(validReq()); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateSigningRequest_SizeBounds(t *testing.T) {
	r := validReq()
	r.DocumentSize = 0
	if err := ValidateSigningRequest(r); err == nil {
		t.Error("expected min size error")
	}
	r = validReq()
	r.DocumentSize = MaxDocSizeBytes + 1
	if err := ValidateSigningRequest(r); err == nil {
		t.Error("expected max size error")
	}
}

func TestValidateSigningRequest_NoExpiry(t *testing.T) {
	r := validReq()
	r.ExpiresAt = time.Time{}
	if err := ValidateSigningRequest(r); err == nil {
		t.Error("expected expiry required")
	}
}

func TestValidateSignedDocument_PAdESLevels(t *testing.T) {
	d := &signing.SignedDocument{
		RequestID:          "r1",
		CertificateID:      "c1",
		SignedHash:         "h",
		SignedPath:         "/tmp/s.pdf",
		SignedSize:         1024,
		PAdESLevel:         "PAdES-B-LTA",
		SignatureTimestamp: time.Now(),
	}
	if err := ValidateSignedDocument(d); err != nil {
		t.Errorf("valid LTA: %v", err)
	}
	d.PAdESLevel = "PAdES-NEXT-GEN"
	if err := ValidateSignedDocument(d); err == nil {
		t.Error("expected pades_level rejection")
	}
}

func TestValidate_NullByteRejection(t *testing.T) {
	c := validCert()
	c.UserID = "evil\x00user"
	if err := ValidateCertificate(c); err == nil || !strings.Contains(err.Error(), "null") {
		t.Errorf("expected null byte error, got %v", err)
	}
}
