package ca

import (
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

func TestNewCA(t *testing.T) {
	c, err := NewCA()
	if err != nil {
		t.Fatalf("NewCA() error: %v", err)
	}

	if c.RootCN() != "GarudaPass Dev Root CA" {
		t.Errorf("RootCN() = %q, want %q", c.RootCN(), "GarudaPass Dev Root CA")
	}

	if c.RootSerial() == "" {
		t.Error("RootSerial() is empty")
	}

	rootPEM := c.RootCertificate()
	if !strings.Contains(rootPEM, "BEGIN CERTIFICATE") {
		t.Error("RootCertificate() does not contain PEM header")
	}

	// Parse the root certificate
	block, _ := pem.Decode([]byte(rootPEM))
	if block == nil {
		t.Fatal("failed to decode root PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse root certificate: %v", err)
	}

	if !cert.IsCA {
		t.Error("root certificate IsCA = false, want true")
	}

	if cert.Subject.CommonName != "GarudaPass Dev Root CA" {
		t.Errorf("root CN = %q, want %q", cert.Subject.CommonName, "GarudaPass Dev Root CA")
	}

	if len(cert.Subject.Organization) == 0 || cert.Subject.Organization[0] != "GarudaPass" {
		t.Errorf("root Organization = %v, want [GarudaPass]", cert.Subject.Organization)
	}

	if len(cert.Subject.Country) == 0 || cert.Subject.Country[0] != "ID" {
		t.Errorf("root Country = %v, want [ID]", cert.Subject.Country)
	}
}

func TestIssueCertificate(t *testing.T) {
	c, err := NewCA()
	if err != nil {
		t.Fatalf("NewCA() error: %v", err)
	}

	cert, err := c.IssueCertificate("Test User", "user-123", 365)
	if err != nil {
		t.Fatalf("IssueCertificate() error: %v", err)
	}

	if cert.SerialNumber == "" {
		t.Error("SerialNumber is empty")
	}

	if cert.IssuerDN == "" {
		t.Error("IssuerDN is empty")
	}

	if cert.SubjectDN == "" {
		t.Error("SubjectDN is empty")
	}

	if cert.FingerprintSHA256 == "" {
		t.Error("FingerprintSHA256 is empty")
	}

	if cert.PrivateKey == nil {
		t.Error("PrivateKey is nil")
	}

	if cert.ValidFrom.IsZero() {
		t.Error("ValidFrom is zero")
	}

	if cert.ValidTo.IsZero() {
		t.Error("ValidTo is zero")
	}

	// Parse the certificate PEM
	block, _ := pem.Decode([]byte(cert.CertificatePEM))
	if block == nil {
		t.Fatal("failed to decode certificate PEM")
	}

	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	if x509Cert.Subject.CommonName != "Test User" {
		t.Errorf("CN = %q, want %q", x509Cert.Subject.CommonName, "Test User")
	}

	if x509Cert.IsCA {
		t.Error("issued certificate IsCA = true, want false")
	}
}

func TestIssueCertificate_UniqueSerials(t *testing.T) {
	c, err := NewCA()
	if err != nil {
		t.Fatalf("NewCA() error: %v", err)
	}

	cert1, err := c.IssueCertificate("User One", "uid-1", 365)
	if err != nil {
		t.Fatalf("IssueCertificate(1) error: %v", err)
	}

	cert2, err := c.IssueCertificate("User Two", "uid-2", 365)
	if err != nil {
		t.Fatalf("IssueCertificate(2) error: %v", err)
	}

	if cert1.SerialNumber == cert2.SerialNumber {
		t.Error("serial numbers should be unique")
	}

	if cert1.FingerprintSHA256 == cert2.FingerprintSHA256 {
		t.Error("fingerprints should be unique")
	}
}

func TestIssueCertificate_EmptyCommonName(t *testing.T) {
	c, err := NewCA()
	if err != nil {
		t.Fatalf("NewCA() error: %v", err)
	}

	_, err = c.IssueCertificate("", "uid-1", 365)
	if err == nil {
		t.Error("expected error for empty commonName")
	}
}

func TestIssueCertificate_InvalidValidityDays(t *testing.T) {
	c, err := NewCA()
	if err != nil {
		t.Fatalf("NewCA() error: %v", err)
	}

	tests := []struct {
		name string
		days int
	}{
		{"zero", 0},
		{"negative", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.IssueCertificate("Test", "uid", tt.days)
			if err == nil {
				t.Errorf("expected error for validityDays=%d", tt.days)
			}
		})
	}
}

func TestGetCertificate(t *testing.T) {
	c, err := NewCA()
	if err != nil {
		t.Fatalf("NewCA() error: %v", err)
	}

	issued, err := c.IssueCertificate("Test User", "uid-1", 365)
	if err != nil {
		t.Fatalf("IssueCertificate() error: %v", err)
	}

	got, ok := c.GetCertificate(issued.SerialNumber)
	if !ok {
		t.Fatal("GetCertificate() not found")
	}

	if got.SerialNumber != issued.SerialNumber {
		t.Errorf("SerialNumber = %q, want %q", got.SerialNumber, issued.SerialNumber)
	}

	// Non-existent serial
	_, ok = c.GetCertificate("nonexistent")
	if ok {
		t.Error("GetCertificate() found non-existent certificate")
	}
}

func TestListSerials(t *testing.T) {
	c, err := NewCA()
	if err != nil {
		t.Fatalf("NewCA() error: %v", err)
	}

	if len(c.ListSerials()) != 0 {
		t.Error("ListSerials() should be empty initially")
	}

	cert1, _ := c.IssueCertificate("User 1", "uid-1", 365)
	cert2, _ := c.IssueCertificate("User 2", "uid-2", 365)

	serials := c.ListSerials()
	if len(serials) != 2 {
		t.Fatalf("ListSerials() len = %d, want 2", len(serials))
	}

	serialSet := make(map[string]bool)
	for _, s := range serials {
		serialSet[s] = true
	}

	if !serialSet[cert1.SerialNumber] {
		t.Errorf("ListSerials() missing serial %q", cert1.SerialNumber)
	}
	if !serialSet[cert2.SerialNumber] {
		t.Errorf("ListSerials() missing serial %q", cert2.SerialNumber)
	}
}
