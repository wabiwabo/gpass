package pades

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

func generateTestKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}
	return key
}

func TestSignPAdES_Success(t *testing.T) {
	key := generateTestKey(t)
	doc := []byte("%PDF-1.4 test document content")
	docB64 := base64.StdEncoding.EncodeToString(doc)
	certPEM := "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"

	result, err := SignPAdES(docB64, certPEM, key)
	if err != nil {
		t.Fatalf("SignPAdES() error: %v", err)
	}

	if result.SignedDocumentBase64 == "" {
		t.Error("SignedDocumentBase64 is empty")
	}

	// Signed document should differ from original
	if result.SignedDocumentBase64 == docB64 {
		t.Error("signed document should differ from original")
	}

	// Decode signed document and verify it starts with %PDF
	signedBytes, err := base64.StdEncoding.DecodeString(result.SignedDocumentBase64)
	if err != nil {
		t.Fatalf("failed to decode signed document: %v", err)
	}

	if !strings.HasPrefix(string(signedBytes), "%PDF") {
		t.Error("signed document should start with %PDF")
	}

	// Should contain signature block
	if !strings.Contains(string(signedBytes), "%PAdES-B-LTA-SIG%") {
		t.Error("signed document should contain PAdES signature block")
	}

	if result.PAdESLevel != "B_LTA" {
		t.Errorf("PAdESLevel = %q, want %q", result.PAdESLevel, "B_LTA")
	}

	if result.SignatureTimestamp == "" {
		t.Error("SignatureTimestamp is empty")
	}
}

func TestSignPAdES_InvalidBase64(t *testing.T) {
	key := generateTestKey(t)
	certPEM := "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"

	_, err := SignPAdES("not-valid-base64!!!", certPEM, key)
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestSignPAdES_EmptyDocument(t *testing.T) {
	key := generateTestKey(t)
	certPEM := "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"

	// Empty bytes encoded as base64
	emptyB64 := base64.StdEncoding.EncodeToString([]byte{})

	_, err := SignPAdES(emptyB64, certPEM, key)
	if err == nil {
		t.Error("expected error for empty document")
	}
}

func TestSignPAdES_EmptyDocumentBase64Field(t *testing.T) {
	key := generateTestKey(t)
	certPEM := "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"

	_, err := SignPAdES("", certPEM, key)
	if err == nil {
		t.Error("expected error for empty document_base64")
	}
}

func TestSignPAdES_EmptyCertificatePEM(t *testing.T) {
	key := generateTestKey(t)
	docB64 := base64.StdEncoding.EncodeToString([]byte("test"))

	_, err := SignPAdES(docB64, "", key)
	if err == nil {
		t.Error("expected error for empty certificate_pem")
	}
}

func TestSignPAdES_NilPrivateKey(t *testing.T) {
	docB64 := base64.StdEncoding.EncodeToString([]byte("test"))
	certPEM := "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"

	_, err := SignPAdES(docB64, certPEM, nil)
	if err == nil {
		t.Error("expected error for nil private key")
	}
}
