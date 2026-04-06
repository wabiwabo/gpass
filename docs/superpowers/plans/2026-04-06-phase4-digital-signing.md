# Phase 4: Digital Signing — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add digital signing to GarudaPass — a GarudaSign Go service orchestrating certificate issuance and PAdES-B-LTA PDF signing, backed by a Go signing-sim for dev/CI.

**Architecture:** GarudaSign (Go) calls signing-sim (dev) or EJBCA+EU DSS (prod) for certificate issuance and PAdES signing. Documents are hashed SHA-256 before signing. All actions audited to Kafka. File storage on local disk (MVP).

**Tech Stack:** Go 1.25, crypto/x509, crypto/ecdsa (P-256), crypto/sha256, encoding/pem, net/http, slog, miniredis (test)

---

## File Structure

```
services/
├── signing-sim/
│   ├── go.mod
│   ├── main.go
│   ├── Dockerfile
│   ├── ca/
│   │   ├── ca.go              # In-memory self-signed CA + cert issuance (crypto/x509)
│   │   └── ca_test.go
│   ├── pades/
│   │   ├── mock.go            # Mock PAdES-B-LTA signing (appends fake sig to PDF)
│   │   └── mock_test.go
│   └── handler/
│       ├── certificate.go     # POST /certificates/issue
│       ├── certificate_test.go
│       ├── sign.go            # POST /sign/pades
│       └── sign_test.go
│
├── garudasign/
│   ├── go.mod
│   ├── main.go
│   ├── Dockerfile
│   ├── config/
│   │   ├── config.go          # Env config with SIGNING_MODE, URL validation
│   │   └── config_test.go
│   ├── signing/
│   │   ├── client.go          # HTTP client for signing-sim / EJBCA+DSS with circuit breaker
│   │   ├── client_test.go
│   │   └── types.go           # Shared types: CertificateIssueRequest/Response, SignRequest/Response
│   ├── store/
│   │   ├── certificate.go     # CertificateStore interface + InMemory: CRUD, lifecycle
│   │   ├── certificate_test.go
│   │   ├── request.go         # RequestStore interface + InMemory: CRUD, expiry
│   │   ├── request_test.go
│   │   ├── document.go        # DocumentStore interface + InMemory: CRUD
│   │   └── document_test.go
│   ├── handler/
│   │   ├── certificate.go     # POST /certificates/request, GET /certificates
│   │   ├── certificate_test.go
│   │   ├── document.go        # POST /documents, POST /:id/sign, GET /:id, GET /:id/download
│   │   └── document_test.go
│   ├── hash/
│   │   ├── sha256.go          # ComputeHash, VerifyHash
│   │   └── sha256_test.go
│   ├── storage/
│   │   ├── file.go            # FileStorage: Save, Load, Delete with path sanitization
│   │   └── file_test.go
│   └── audit/
│       ├── emitter.go         # AuditEmitter interface + LogEmitter (slog-based for MVP)
│       └── emitter_test.go

infrastructure/db/migrations/
├── 007_create_signing_certificates.sql
├── 008_create_signing_requests.sql
└── 009_create_signed_documents.sql
```

---

### Task 1: Signing Simulator — CA Module

**Files:**
- Create: `services/signing-sim/go.mod`
- Create: `services/signing-sim/ca/ca.go`
- Create: `services/signing-sim/ca/ca_test.go`

- [ ] **Step 1: Initialize go module**

```bash
cd services/signing-sim
go mod init github.com/garudapass/gpass/services/signing-sim
```

Set `go 1.25.0` in go.mod.

- [ ] **Step 2: Write the failing tests for CA**

Create `services/signing-sim/ca/ca_test.go`:

```go
package ca

import (
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"
)

func TestNewCA(t *testing.T) {
	authority, err := NewCA()
	if err != nil {
		t.Fatalf("NewCA() error: %v", err)
	}
	if authority == nil {
		t.Fatal("NewCA() returned nil")
	}
	if authority.RootCert == nil {
		t.Fatal("RootCert is nil")
	}
	if authority.RootCert.IsCA != true {
		t.Error("RootCert.IsCA should be true")
	}
	if authority.RootCert.Subject.CommonName != "GarudaPass Dev Root CA" {
		t.Errorf("unexpected CN: %s", authority.RootCert.Subject.CommonName)
	}
}

func TestIssueCertificate(t *testing.T) {
	authority, err := NewCA()
	if err != nil {
		t.Fatalf("NewCA() error: %v", err)
	}

	cert, err := authority.IssueCertificate("John Doe", "user-123", 365)
	if err != nil {
		t.Fatalf("IssueCertificate error: %v", err)
	}

	if cert.SerialNumber == "" {
		t.Error("serial number should not be empty")
	}
	if cert.SubjectDN == "" {
		t.Error("subject DN should not be empty")
	}
	if cert.IssuerDN == "" {
		t.Error("issuer DN should not be empty")
	}
	if cert.CertificatePEM == "" {
		t.Error("certificate PEM should not be empty")
	}
	if cert.FingerprintSHA256 == "" {
		t.Error("fingerprint should not be empty")
	}

	// Verify it's valid PEM
	block, _ := pem.Decode([]byte(cert.CertificatePEM))
	if block == nil {
		t.Fatal("failed to decode PEM")
	}
	parsed, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}
	if parsed.Subject.CommonName != "John Doe" {
		t.Errorf("unexpected subject CN: %s", parsed.Subject.CommonName)
	}
	if parsed.NotBefore.After(time.Now()) {
		t.Error("NotBefore should be in the past")
	}
	if parsed.NotAfter.Before(time.Now().Add(364 * 24 * time.Hour)) {
		t.Error("NotAfter should be ~365 days from now")
	}
}

func TestIssueCertificate_UniqueSerialsAndFingerprints(t *testing.T) {
	authority, err := NewCA()
	if err != nil {
		t.Fatalf("NewCA() error: %v", err)
	}

	cert1, _ := authority.IssueCertificate("User A", "uid-1", 365)
	cert2, _ := authority.IssueCertificate("User B", "uid-2", 365)

	if cert1.SerialNumber == cert2.SerialNumber {
		t.Error("serial numbers should be unique")
	}
	if cert1.FingerprintSHA256 == cert2.FingerprintSHA256 {
		t.Error("fingerprints should be unique")
	}
}

func TestIssueCertificate_InvalidValidity(t *testing.T) {
	authority, err := NewCA()
	if err != nil {
		t.Fatalf("NewCA() error: %v", err)
	}

	_, err = authority.IssueCertificate("", "uid-1", 365)
	if err == nil {
		t.Error("expected error for empty common name")
	}

	_, err = authority.IssueCertificate("User", "uid-1", 0)
	if err == nil {
		t.Error("expected error for zero validity days")
	}

	_, err = authority.IssueCertificate("User", "uid-1", -1)
	if err == nil {
		t.Error("expected error for negative validity days")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd services/signing-sim && go test ./ca/... -v -count=1
```

Expected: FAIL — `ca` package doesn't exist yet.

- [ ] **Step 4: Implement CA module**

Create `services/signing-sim/ca/ca.go`:

```go
package ca

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// IssuedCertificate holds the result of a certificate issuance.
type IssuedCertificate struct {
	SerialNumber      string
	IssuerDN          string
	SubjectDN         string
	CertificatePEM    string
	FingerprintSHA256 string
	ValidFrom         time.Time
	ValidTo           time.Time
	PrivateKey        *ecdsa.PrivateKey // held in memory for signing
}

// CA is an in-memory certificate authority for dev/test.
type CA struct {
	RootCert *x509.Certificate
	rootKey  *ecdsa.PrivateKey
	rootDER  []byte
	mu       sync.Mutex
	issued   map[string]*IssuedCertificate // serial -> cert
}

// NewCA generates a self-signed root CA.
func NewCA() (*CA, error) {
	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate root key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "GarudaPass Dev Root CA",
			Organization: []string{"GarudaPass"},
			Country:      []string{"ID"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	rootDER, err := x509.CreateCertificate(rand.Reader, template, template, &rootKey.PublicKey, rootKey)
	if err != nil {
		return nil, fmt.Errorf("create root cert: %w", err)
	}

	rootCert, err := x509.ParseCertificate(rootDER)
	if err != nil {
		return nil, fmt.Errorf("parse root cert: %w", err)
	}

	return &CA{
		RootCert: rootCert,
		rootKey:  rootKey,
		rootDER:  rootDER,
		issued:   make(map[string]*IssuedCertificate),
	}, nil
}

// IssueCertificate issues an end-entity certificate signed by the root CA.
func (ca *CA) IssueCertificate(commonName, userID string, validityDays int) (*IssuedCertificate, error) {
	if commonName == "" {
		return nil, fmt.Errorf("common name is required")
	}
	if validityDays <= 0 {
		return nil, fmt.Errorf("validity days must be positive")
	}

	ca.mu.Lock()
	defer ca.mu.Unlock()

	eeKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"GarudaPass"},
			Country:      []string{"ID"},
		},
		NotBefore: now,
		NotAfter:  now.Add(time.Duration(validityDays) * 24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
		},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca.RootCert, &eeKey.PublicKey, ca.rootKey)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	fingerprint := sha256.Sum256(certDER)
	serialHex := hex.EncodeToString(serial.Bytes())

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("parse issued cert: %w", err)
	}

	issued := &IssuedCertificate{
		SerialNumber:      serialHex,
		IssuerDN:          ca.RootCert.Subject.String(),
		SubjectDN:         cert.Subject.String(),
		CertificatePEM:    string(certPEM),
		FingerprintSHA256: hex.EncodeToString(fingerprint[:]),
		ValidFrom:         cert.NotBefore,
		ValidTo:           cert.NotAfter,
		PrivateKey:        eeKey,
	}

	ca.issued[serialHex] = issued
	return issued, nil
}

// GetCertificate retrieves an issued certificate by serial number.
func (ca *CA) GetCertificate(serialNumber string) (*IssuedCertificate, bool) {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	cert, ok := ca.issued[serialNumber]
	return cert, ok
}

func randomSerial() (*big.Int, error) {
	max := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, max)
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}
	return serial, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd services/signing-sim && go test ./ca/... -v -count=1
```

Expected: All 4 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add services/signing-sim/go.mod services/signing-sim/ca/
git commit -m "feat(signing-sim): add in-memory CA module with ECDSA P-256 certificate issuance"
```

---

### Task 2: Signing Simulator — PAdES Mock Module

**Files:**
- Create: `services/signing-sim/pades/mock.go`
- Create: `services/signing-sim/pades/mock_test.go`

- [ ] **Step 1: Write the failing tests**

Create `services/signing-sim/pades/mock_test.go`:

```go
package pades

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

// Minimal valid PDF: just the header + EOF markers
var minimalPDF = []byte("%PDF-1.4\n1 0 obj<</Type/Catalog>>endobj\n%%EOF")

func TestSignPAdES(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	certPEM := "-----BEGIN CERTIFICATE-----\nMIIBtestcert\n-----END CERTIFICATE-----"

	docBase64 := base64.StdEncoding.EncodeToString(minimalPDF)

	result, err := SignPAdES(docBase64, certPEM, key)
	if err != nil {
		t.Fatalf("SignPAdES error: %v", err)
	}

	if result.SignedDocumentBase64 == "" {
		t.Error("signed document should not be empty")
	}
	if result.PAdESLevel != "B_LTA" {
		t.Errorf("expected PAdES level B_LTA, got %s", result.PAdESLevel)
	}
	if result.SignatureTimestamp.IsZero() {
		t.Error("signature timestamp should not be zero")
	}

	// Signed document should be different from original (signature appended)
	if result.SignedDocumentBase64 == docBase64 {
		t.Error("signed document should differ from original")
	}

	// Decode to verify it starts with %PDF
	decoded, err := base64.StdEncoding.DecodeString(result.SignedDocumentBase64)
	if err != nil {
		t.Fatalf("decode signed doc: %v", err)
	}
	if !strings.HasPrefix(string(decoded), "%PDF") {
		t.Error("signed document should start with %PDF")
	}
}

func TestSignPAdES_InvalidBase64(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	_, err := SignPAdES("not-valid-base64!!!", "cert", key)
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestSignPAdES_EmptyDocument(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	emptyDoc := base64.StdEncoding.EncodeToString([]byte{})
	_, err := SignPAdES(emptyDoc, "cert", key)
	if err == nil {
		t.Error("expected error for empty document")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd services/signing-sim && go test ./pades/... -v -count=1
```

Expected: FAIL

- [ ] **Step 3: Implement PAdES mock module**

Create `services/signing-sim/pades/mock.go`:

```go
package pades

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// SignResult holds the result of a mock PAdES signing operation.
type SignResult struct {
	SignedDocumentBase64 string
	SignatureTimestamp   time.Time
	PAdESLevel          string
}

// SignPAdES creates a mock PAdES-B-LTA signed PDF.
// It appends a simulated signature block to the document.
// This is NOT a real PAdES signature — it's for dev/test only.
func SignPAdES(documentBase64, certificatePEM string, privateKey *ecdsa.PrivateKey) (*SignResult, error) {
	docBytes, err := base64.StdEncoding.DecodeString(documentBase64)
	if err != nil {
		return nil, fmt.Errorf("decode document: %w", err)
	}
	if len(docBytes) == 0 {
		return nil, fmt.Errorf("document is empty")
	}

	// Hash the document
	docHash := sha256.Sum256(docBytes)

	// Sign the hash with ECDSA
	sig, err := ecdsa.SignASN1(rand.Reader, privateKey, docHash[:])
	if err != nil {
		return nil, fmt.Errorf("sign hash: %w", err)
	}

	// Build mock PAdES signature block
	sigTimestamp := time.Now().UTC()
	sigBlock := fmt.Sprintf(
		"\n%%PAdES-B-LTA-SIM\n%%%%SignatureHash:%s\n%%%%SignatureValue:%s\n%%%%Timestamp:%s\n%%%%Certificate:%s\n%%%%EOF",
		hex.EncodeToString(docHash[:]),
		hex.EncodeToString(sig),
		sigTimestamp.Format(time.RFC3339),
		"embedded",
	)

	signedDoc := append(docBytes, []byte(sigBlock)...)
	signedBase64 := base64.StdEncoding.EncodeToString(signedDoc)

	return &SignResult{
		SignedDocumentBase64: signedBase64,
		SignatureTimestamp:   sigTimestamp,
		PAdESLevel:          "B_LTA",
	}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd services/signing-sim && go test ./pades/... -v -count=1
```

Expected: All 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add services/signing-sim/pades/
git commit -m "feat(signing-sim): add mock PAdES-B-LTA signing module with ECDSA signature"
```

---

### Task 3: Signing Simulator — HTTP Handlers + Main

**Files:**
- Create: `services/signing-sim/handler/certificate.go`
- Create: `services/signing-sim/handler/certificate_test.go`
- Create: `services/signing-sim/handler/sign.go`
- Create: `services/signing-sim/handler/sign_test.go`
- Create: `services/signing-sim/main.go`
- Create: `services/signing-sim/Dockerfile`

- [ ] **Step 1: Write the failing tests for certificate handler**

Create `services/signing-sim/handler/certificate_test.go`:

```go
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/signing-sim/ca"
)

func setupCertHandler(t *testing.T) *CertificateHandler {
	t.Helper()
	authority, err := ca.NewCA()
	if err != nil {
		t.Fatalf("NewCA error: %v", err)
	}
	return NewCertificateHandler(authority)
}

func TestIssueCertificate_Success(t *testing.T) {
	h := setupCertHandler(t)

	body := `{"subject_cn":"John Doe","subject_uid":"user-123","validity_days":365}`
	req := httptest.NewRequest(http.MethodPost, "/certificates/issue", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Issue(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	for _, field := range []string{"serial_number", "certificate_pem", "issuer_dn", "subject_dn", "valid_from", "valid_to", "fingerprint_sha256"} {
		if _, ok := resp[field]; !ok {
			t.Errorf("missing field %q in response", field)
		}
	}
}

func TestIssueCertificate_MissingCN(t *testing.T) {
	h := setupCertHandler(t)

	body := `{"subject_uid":"user-123","validity_days":365}`
	req := httptest.NewRequest(http.MethodPost, "/certificates/issue", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Issue(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestIssueCertificate_InvalidJSON(t *testing.T) {
	h := setupCertHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/certificates/issue", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Issue(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Write the failing tests for sign handler**

Create `services/signing-sim/handler/sign_test.go`:

```go
package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/signing-sim/ca"
)

var testPDF = []byte("%PDF-1.4\n1 0 obj<</Type/Catalog>>endobj\n%%EOF")

func setupSignHandler(t *testing.T) (*SignHandler, *ca.IssuedCertificate) {
	t.Helper()
	authority, err := ca.NewCA()
	if err != nil {
		t.Fatalf("NewCA error: %v", err)
	}
	cert, err := authority.IssueCertificate("Test User", "user-1", 365)
	if err != nil {
		t.Fatalf("IssueCertificate error: %v", err)
	}
	return NewSignHandler(authority), cert
}

func TestSignPAdES_Success(t *testing.T) {
	h, cert := setupSignHandler(t)

	body, _ := json.Marshal(map[string]string{
		"document_base64":  base64.StdEncoding.EncodeToString(testPDF),
		"certificate_pem":  cert.CertificatePEM,
		"signature_level":  "PAdES_BASELINE_LTA",
	})
	req := httptest.NewRequest(http.MethodPost, "/sign/pades", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Sign(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, field := range []string{"signed_document_base64", "signature_timestamp", "pades_level"} {
		if _, ok := resp[field]; !ok {
			t.Errorf("missing field %q", field)
		}
	}
}

func TestSignPAdES_MissingDocument(t *testing.T) {
	h, cert := setupSignHandler(t)

	body, _ := json.Marshal(map[string]string{
		"certificate_pem": cert.CertificatePEM,
		"signature_level": "PAdES_BASELINE_LTA",
	})
	req := httptest.NewRequest(http.MethodPost, "/sign/pades", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Sign(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSignPAdES_UnknownCertificate(t *testing.T) {
	h, _ := setupSignHandler(t)

	body, _ := json.Marshal(map[string]string{
		"document_base64":  base64.StdEncoding.EncodeToString(testPDF),
		"certificate_pem":  "-----BEGIN CERTIFICATE-----\nunknown\n-----END CERTIFICATE-----",
		"signature_level":  "PAdES_BASELINE_LTA",
	})
	req := httptest.NewRequest(http.MethodPost, "/sign/pades", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Sign(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 3: Implement certificate handler**

Create `services/signing-sim/handler/certificate.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/garudapass/gpass/services/signing-sim/ca"
)

type issueRequest struct {
	SubjectCN    string `json:"subject_cn"`
	SubjectUID   string `json:"subject_uid"`
	ValidityDays int    `json:"validity_days"`
}

type issueResponse struct {
	SerialNumber      string `json:"serial_number"`
	CertificatePEM    string `json:"certificate_pem"`
	IssuerDN          string `json:"issuer_dn"`
	SubjectDN         string `json:"subject_dn"`
	ValidFrom         string `json:"valid_from"`
	ValidTo           string `json:"valid_to"`
	FingerprintSHA256 string `json:"fingerprint_sha256"`
}

// CertificateHandler handles certificate issuance requests.
type CertificateHandler struct {
	ca *ca.CA
}

// NewCertificateHandler creates a new CertificateHandler.
func NewCertificateHandler(authority *ca.CA) *CertificateHandler {
	return &CertificateHandler{ca: authority}
}

// Issue handles POST /certificates/issue.
func (h *CertificateHandler) Issue(w http.ResponseWriter, r *http.Request) {
	var req issueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.SubjectCN == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "subject_cn is required"})
		return
	}
	if req.ValidityDays <= 0 {
		req.ValidityDays = 365
	}

	cert, err := h.ca.IssueCertificate(req.SubjectCN, req.SubjectUID, req.ValidityDays)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, issueResponse{
		SerialNumber:      cert.SerialNumber,
		CertificatePEM:    cert.CertificatePEM,
		IssuerDN:          cert.IssuerDN,
		SubjectDN:         cert.SubjectDN,
		ValidFrom:         cert.ValidFrom.Format("2006-01-02T15:04:05Z07:00"),
		ValidTo:           cert.ValidTo.Format("2006-01-02T15:04:05Z07:00"),
		FingerprintSHA256: cert.FingerprintSHA256,
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 4: Implement sign handler**

Create `services/signing-sim/handler/sign.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/garudapass/gpass/services/signing-sim/ca"
	"github.com/garudapass/gpass/services/signing-sim/pades"
)

type signRequest struct {
	DocumentBase64 string `json:"document_base64"`
	CertificatePEM string `json:"certificate_pem"`
	SignatureLevel string `json:"signature_level"`
}

type signResponse struct {
	SignedDocumentBase64 string `json:"signed_document_base64"`
	SignatureTimestamp   string `json:"signature_timestamp"`
	PAdESLevel          string `json:"pades_level"`
}

// SignHandler handles document signing requests.
type SignHandler struct {
	ca *ca.CA
}

// NewSignHandler creates a new SignHandler.
func NewSignHandler(authority *ca.CA) *SignHandler {
	return &SignHandler{ca: authority}
}

// Sign handles POST /sign/pades.
func (h *SignHandler) Sign(w http.ResponseWriter, r *http.Request) {
	var req signRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.DocumentBase64 == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "document_base64 is required"})
		return
	}
	if req.CertificatePEM == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "certificate_pem is required"})
		return
	}

	// Look up the certificate's private key by matching PEM
	var issedCert *ca.IssuedCertificate
	// Iterate through issued certs to find by PEM match
	// (In a real CA, we'd look up by serial number)
	found := false
	// We need to find the cert by checking all issued ones
	// The CA stores issued certs by serial — we match by PEM content
	for _, serial := range h.ca.ListSerials() {
		cert, ok := h.ca.GetCertificate(serial)
		if ok && cert.CertificatePEM == req.CertificatePEM {
			issedCert = cert
			found = true
			break
		}
	}

	if !found {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "certificate not found or not issued by this CA"})
		return
	}

	result, err := pades.SignPAdES(req.DocumentBase64, req.CertificatePEM, issedCert.PrivateKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "signing failed: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, signResponse{
		SignedDocumentBase64: result.SignedDocumentBase64,
		SignatureTimestamp:   result.SignatureTimestamp.Format("2006-01-02T15:04:05Z07:00"),
		PAdESLevel:          result.PAdESLevel,
	})
}
```

- [ ] **Step 5: Add ListSerials method to CA**

Add to `services/signing-sim/ca/ca.go`:

```go
// ListSerials returns all issued certificate serial numbers.
func (ca *CA) ListSerials() []string {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	serials := make([]string, 0, len(ca.issued))
	for s := range ca.issued {
		serials = append(serials, s)
	}
	return serials
}
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
cd services/signing-sim && go test ./... -v -count=1
```

Expected: All tests PASS across ca/, pades/, handler/.

- [ ] **Step 7: Implement main.go**

Create `services/signing-sim/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/garudapass/gpass/services/signing-sim/ca"
	"github.com/garudapass/gpass/services/signing-sim/handler"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	port := os.Getenv("SIGNING_SIM_PORT")
	if port == "" {
		port = "4008"
	}

	authority, err := ca.NewCA()
	if err != nil {
		slog.Error("failed to initialize CA", "error", err)
		os.Exit(1)
	}
	slog.Info("CA initialized",
		"root_cn", authority.RootCert.Subject.CommonName,
		"root_serial", authority.RootCert.SerialNumber.Text(16),
	)

	certHandler := handler.NewCertificateHandler(authority)
	signHandler := handler.NewSignHandler(authority)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok","service":"signing-simulator"}`)
	})

	mux.HandleFunc("POST /certificates/issue", certHandler.Issue)
	mux.HandleFunc("POST /sign/pades", signHandler.Sign)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      60 * time.Second, // longer for signing ops
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("signing-sim listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutdown signal received", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	slog.Info("signing-sim shut down gracefully")
}
```

- [ ] **Step 8: Create Dockerfile**

Create `services/signing-sim/Dockerfile`:

```dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o signing-sim .

FROM alpine:3.20

RUN apk --no-cache add ca-certificates \
    && adduser -D -u 1000 appuser

WORKDIR /app

COPY --from=builder /build/signing-sim .

USER appuser

EXPOSE 4008

ENTRYPOINT ["./signing-sim"]
```

- [ ] **Step 9: Run all signing-sim tests**

```bash
cd services/signing-sim && go test ./... -v -count=1
```

Expected: All tests PASS.

- [ ] **Step 10: Commit**

```bash
git add services/signing-sim/
git commit -m "feat(signing-sim): add HTTP handlers, main entrypoint, and Dockerfile"
```

---

### Task 4: GarudaSign — Config + Hash + Storage

**Files:**
- Create: `services/garudasign/go.mod`
- Create: `services/garudasign/config/config.go`
- Create: `services/garudasign/config/config_test.go`
- Create: `services/garudasign/hash/sha256.go`
- Create: `services/garudasign/hash/sha256_test.go`
- Create: `services/garudasign/storage/file.go`
- Create: `services/garudasign/storage/file_test.go`

- [ ] **Step 1: Initialize go module**

```bash
cd services/garudasign
go mod init github.com/garudapass/gpass/services/garudasign
```

Set `go 1.25.0` in go.mod.

- [ ] **Step 2: Write failing config tests**

Create `services/garudasign/config/config_test.go`:

```go
package config

import (
	"os"
	"testing"
)

func clearEnv() {
	for _, key := range []string{
		"GARUDASIGN_PORT", "SIGNING_MODE", "SIGNING_SIM_URL",
		"IDENTITY_SERVICE_URL", "DOCUMENT_STORAGE_PATH",
		"DOCUMENT_MAX_SIZE_MB", "SIGNING_REQUEST_TTL",
		"CERT_VALIDITY_DAYS", "GARUDASIGN_DB_URL",
		"EJBCA_URL", "DSS_URL",
	} {
		os.Unsetenv(key)
	}
}

func setRequiredEnv() {
	os.Setenv("SIGNING_MODE", "simulator")
	os.Setenv("SIGNING_SIM_URL", "http://localhost:4008")
	os.Setenv("IDENTITY_SERVICE_URL", "http://localhost:4001")
	os.Setenv("DOCUMENT_STORAGE_PATH", "/tmp/test-signing")
	os.Setenv("GARUDASIGN_DB_URL", "postgres://user:pass@localhost/db")
}

func TestLoad_Success(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	defer clearEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Port != "4007" {
		t.Errorf("expected port 4007, got %s", cfg.Port)
	}
	if cfg.SigningMode != "simulator" {
		t.Errorf("expected mode simulator, got %s", cfg.SigningMode)
	}
	if cfg.MaxSizeMB != 10 {
		t.Errorf("expected max size 10, got %d", cfg.MaxSizeMB)
	}
	if cfg.CertValidityDays != 365 {
		t.Errorf("expected cert validity 365, got %d", cfg.CertValidityDays)
	}
}

func TestLoad_InvalidMode(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	os.Setenv("SIGNING_MODE", "invalid")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Error("expected error for invalid signing mode")
	}
}

func TestLoad_RealModeRequiresEJBCA(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	os.Setenv("SIGNING_MODE", "real")
	os.Unsetenv("EJBCA_URL")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Error("expected error when SIGNING_MODE=real but EJBCA_URL missing")
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	clearEnv()
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Error("expected error for missing required env vars")
	}
}

func TestLoad_InvalidURL(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	os.Setenv("SIGNING_SIM_URL", "not-a-url")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestLoad_InvalidMaxSize(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	os.Setenv("DOCUMENT_MAX_SIZE_MB", "-5")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Error("expected error for negative max size")
	}
}
```

- [ ] **Step 3: Implement config**

Create `services/garudasign/config/config.go`:

```go
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds GarudaSign service configuration.
type Config struct {
	Port               string
	SigningMode         string // "simulator" or "real"
	SigningSimURL       string
	EJBCAURL           string
	EJBCAClientCert    string
	EJBCAClientKey     string
	DSSURL             string
	IdentityServiceURL string
	DocumentStoragePath string
	MaxSizeMB          int
	RequestTTL         time.Duration
	CertValidityDays   int
	DatabaseURL        string
	KafkaBrokers       string
}

// IsSimulator returns true if running in simulator mode.
func (c *Config) IsSimulator() bool {
	return c.SigningMode == "simulator"
}

// Load reads config from environment variables.
func Load() (*Config, error) {
	mode := getEnv("SIGNING_MODE", "simulator")
	if mode != "simulator" && mode != "real" {
		return nil, fmt.Errorf("SIGNING_MODE must be 'simulator' or 'real', got %q", mode)
	}

	maxSizeStr := getEnv("DOCUMENT_MAX_SIZE_MB", "10")
	maxSize, err := strconv.Atoi(maxSizeStr)
	if err != nil || maxSize <= 0 {
		return nil, fmt.Errorf("DOCUMENT_MAX_SIZE_MB must be a positive integer, got %q", maxSizeStr)
	}

	ttlStr := getEnv("SIGNING_REQUEST_TTL", "30m")
	ttl, err := time.ParseDuration(ttlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SIGNING_REQUEST_TTL %q: %w", ttlStr, err)
	}

	validityStr := getEnv("CERT_VALIDITY_DAYS", "365")
	validity, err := strconv.Atoi(validityStr)
	if err != nil || validity <= 0 {
		return nil, fmt.Errorf("CERT_VALIDITY_DAYS must be a positive integer, got %q", validityStr)
	}

	cfg := &Config{
		Port:                getEnv("GARUDASIGN_PORT", "4007"),
		SigningMode:         mode,
		SigningSimURL:       os.Getenv("SIGNING_SIM_URL"),
		EJBCAURL:            os.Getenv("EJBCA_URL"),
		EJBCAClientCert:     os.Getenv("EJBCA_CLIENT_CERT"),
		EJBCAClientKey:      os.Getenv("EJBCA_CLIENT_KEY"),
		DSSURL:              os.Getenv("DSS_URL"),
		IdentityServiceURL:  os.Getenv("IDENTITY_SERVICE_URL"),
		DocumentStoragePath: os.Getenv("DOCUMENT_STORAGE_PATH"),
		MaxSizeMB:           maxSize,
		RequestTTL:          ttl,
		CertValidityDays:    validity,
		DatabaseURL:         os.Getenv("GARUDASIGN_DB_URL"),
		KafkaBrokers:        getEnv("KAFKA_BROKERS", "localhost:19092"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	required := []struct{ name, value string }{
		{"IDENTITY_SERVICE_URL", c.IdentityServiceURL},
		{"DOCUMENT_STORAGE_PATH", c.DocumentStoragePath},
		{"GARUDASIGN_DB_URL", c.DatabaseURL},
	}

	if c.SigningMode == "simulator" {
		required = append(required, struct{ name, value string }{"SIGNING_SIM_URL", c.SigningSimURL})
	} else {
		required = append(required,
			struct{ name, value string }{"EJBCA_URL", c.EJBCAURL},
			struct{ name, value string }{"DSS_URL", c.DSSURL},
		)
	}

	var missing []string
	for _, r := range required {
		if r.value == "" {
			missing = append(missing, r.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("required environment variables not set: %s", strings.Join(missing, ", "))
	}

	// Validate URLs
	urlChecks := []struct{ name, val string }{
		{"IDENTITY_SERVICE_URL", c.IdentityServiceURL},
	}
	if c.SigningMode == "simulator" {
		urlChecks = append(urlChecks, struct{ name, val string }{"SIGNING_SIM_URL", c.SigningSimURL})
	} else {
		urlChecks = append(urlChecks,
			struct{ name, val string }{"EJBCA_URL", c.EJBCAURL},
			struct{ name, val string }{"DSS_URL", c.DSSURL},
		)
	}

	for _, check := range urlChecks {
		if _, err := url.ParseRequestURI(check.val); err != nil {
			return fmt.Errorf("invalid URL for %s: %w", check.name, err)
		}
	}

	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 4: Write failing hash tests**

Create `services/garudasign/hash/sha256_test.go`:

```go
package hash

import (
	"bytes"
	"testing"
)

func TestComputeHash(t *testing.T) {
	data := []byte("hello world")
	h, err := ComputeHash(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ComputeHash error: %v", err)
	}
	// SHA-256 of "hello world"
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if h != expected {
		t.Errorf("expected %s, got %s", expected, h)
	}
}

func TestComputeHash_Empty(t *testing.T) {
	h, err := ComputeHash(bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatalf("ComputeHash error: %v", err)
	}
	// SHA-256 of empty
	expected := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if h != expected {
		t.Errorf("expected %s, got %s", expected, h)
	}
}

func TestVerifyHash(t *testing.T) {
	data := []byte("hello world")
	h := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if !VerifyHash(bytes.NewReader(data), h) {
		t.Error("VerifyHash should return true for matching hash")
	}
}

func TestVerifyHash_Mismatch(t *testing.T) {
	data := []byte("hello world")
	if VerifyHash(bytes.NewReader(data), "0000000000000000000000000000000000000000000000000000000000000000") {
		t.Error("VerifyHash should return false for mismatched hash")
	}
}
```

- [ ] **Step 5: Implement hash module**

Create `services/garudasign/hash/sha256.go`:

```go
package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// ComputeHash computes SHA-256 hash of the reader's contents.
func ComputeHash(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("compute hash: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyHash verifies the reader's contents match the expected SHA-256 hash.
func VerifyHash(r io.Reader, expectedHex string) bool {
	actual, err := ComputeHash(r)
	if err != nil {
		return false
	}
	return actual == expectedHex
}
```

- [ ] **Step 6: Write failing storage tests**

Create `services/garudasign/storage/file_test.go`:

```go
package storage

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileStorage_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStorage(dir)

	data := []byte("test PDF content")
	path, err := fs.Save("test-doc.pdf", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}

	if !strings.HasPrefix(path, dir) {
		t.Error("path should be within storage directory")
	}

	reader, err := fs.Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	defer reader.Close()

	loaded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}

	if !bytes.Equal(data, loaded) {
		t.Error("loaded data should match saved data")
	}
}

func TestFileStorage_Delete(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStorage(dir)

	data := []byte("to be deleted")
	path, _ := fs.Save("delete-me.pdf", bytes.NewReader(data))

	if err := fs.Delete(path); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	_, err := fs.Load(path)
	if err == nil {
		t.Error("expected error loading deleted file")
	}
}

func TestFileStorage_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStorage(dir)

	_, err := fs.Save("../../etc/passwd", bytes.NewReader([]byte("malicious")))
	if err == nil {
		t.Error("expected error for path traversal attempt")
	}
}

func TestFileStorage_LoadNonexistent(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStorage(dir)

	_, err := fs.Load(filepath.Join(dir, "nonexistent.pdf"))
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestFileStorage_LoadOutsideDir(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStorage(dir)

	// Create a file outside the storage dir
	outside := filepath.Join(os.TempDir(), "outside-storage.pdf")
	os.WriteFile(outside, []byte("outside"), 0644)
	defer os.Remove(outside)

	_, err := fs.Load(outside)
	if err == nil {
		t.Error("expected error for path outside storage directory")
	}
}
```

- [ ] **Step 7: Implement file storage**

Create `services/garudasign/storage/file.go`:

```go
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileStorage manages file persistence on local disk.
type FileStorage struct {
	baseDir string
}

// NewFileStorage creates a new FileStorage rooted at baseDir.
func NewFileStorage(baseDir string) *FileStorage {
	return &FileStorage{baseDir: baseDir}
}

// Save writes the reader contents to a file with a unique name derived from filename.
// Returns the absolute path of the saved file.
func (fs *FileStorage) Save(filename string, r io.Reader) (string, error) {
	clean := filepath.Base(filename)
	if clean == "." || clean == "/" || clean == ".." {
		return "", fmt.Errorf("invalid filename: %q", filename)
	}

	// Check for path traversal
	if strings.Contains(filename, "..") {
		return "", fmt.Errorf("path traversal not allowed: %q", filename)
	}

	// Generate unique prefix
	randBytes := make([]byte, 16)
	if _, err := rand.Read(randBytes); err != nil {
		return "", fmt.Errorf("generate random prefix: %w", err)
	}
	prefix := hex.EncodeToString(randBytes)
	safeName := prefix + "_" + clean

	if err := os.MkdirAll(fs.baseDir, 0750); err != nil {
		return "", fmt.Errorf("create storage directory: %w", err)
	}

	fullPath := filepath.Join(fs.baseDir, safeName)

	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0640)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		os.Remove(fullPath)
		return "", fmt.Errorf("write file: %w", err)
	}

	return fullPath, nil
}

// Load opens the file at path for reading. Caller must close the reader.
func (fs *FileStorage) Load(path string) (io.ReadCloser, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	absBase, err := filepath.Abs(fs.baseDir)
	if err != nil {
		return nil, fmt.Errorf("resolve base dir: %w", err)
	}

	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return nil, fmt.Errorf("access denied: path outside storage directory")
	}

	f, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	return f, nil
}

// Delete removes the file at path.
func (fs *FileStorage) Delete(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	absBase, err := filepath.Abs(fs.baseDir)
	if err != nil {
		return fmt.Errorf("resolve base dir: %w", err)
	}

	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) {
		return fmt.Errorf("access denied: path outside storage directory")
	}

	return os.Remove(absPath)
}
```

- [ ] **Step 8: Run all tests**

```bash
cd services/garudasign && go test ./... -v -count=1
```

Expected: All config, hash, storage tests PASS.

- [ ] **Step 9: Commit**

```bash
git add services/garudasign/go.mod services/garudasign/config/ services/garudasign/hash/ services/garudasign/storage/
git commit -m "feat(garudasign): add config, SHA-256 hash, and secure file storage modules"
```

---

### Task 5: GarudaSign — Signing Client + Types

**Files:**
- Create: `services/garudasign/signing/types.go`
- Create: `services/garudasign/signing/client.go`
- Create: `services/garudasign/signing/client_test.go`

- [ ] **Step 1: Create types**

Create `services/garudasign/signing/types.go`:

```go
package signing

import "time"

// CertificateIssueRequest is sent to the signing backend to request a certificate.
type CertificateIssueRequest struct {
	SubjectCN    string `json:"subject_cn"`
	SubjectUID   string `json:"subject_uid"`
	ValidityDays int    `json:"validity_days"`
}

// CertificateIssueResponse is returned by the signing backend.
type CertificateIssueResponse struct {
	SerialNumber      string `json:"serial_number"`
	CertificatePEM    string `json:"certificate_pem"`
	IssuerDN          string `json:"issuer_dn"`
	SubjectDN         string `json:"subject_dn"`
	ValidFrom         string `json:"valid_from"`
	ValidTo           string `json:"valid_to"`
	FingerprintSHA256 string `json:"fingerprint_sha256"`
}

// SignRequest is sent to the signing backend to sign a document.
type SignRequest struct {
	DocumentBase64 string `json:"document_base64"`
	CertificatePEM string `json:"certificate_pem"`
	SignatureLevel string `json:"signature_level"`
}

// SignResponse is returned by the signing backend.
type SignResponse struct {
	SignedDocumentBase64 string `json:"signed_document_base64"`
	SignatureTimestamp   string `json:"signature_timestamp"`
	PAdESLevel          string `json:"pades_level"`
}

// Certificate represents a stored signing certificate.
type Certificate struct {
	ID                string
	UserID            string
	SerialNumber      string
	IssuerDN          string
	SubjectDN         string
	Status            string // ACTIVE, EXPIRED, REVOKED
	ValidFrom         time.Time
	ValidTo           time.Time
	CertificatePEM    string
	FingerprintSHA256 string
	RevokedAt         *time.Time
	RevocationReason  string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// SigningRequest represents a document signing request.
type SigningRequest struct {
	ID            string
	UserID        string
	CertificateID string
	DocumentName  string
	DocumentSize  int64
	DocumentHash  string
	DocumentPath  string
	Status        string // PENDING, SIGNING, COMPLETED, FAILED, EXPIRED
	ErrorMessage  string
	ExpiresAt     time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// SignedDocument represents a completed signed document.
type SignedDocument struct {
	ID                 string
	RequestID          string
	CertificateID      string
	SignedHash         string
	SignedPath         string
	SignedSize         int64
	PAdESLevel         string
	SignatureTimestamp  time.Time
	CreatedAt          time.Time
}
```

- [ ] **Step 2: Write failing client tests**

Create `services/garudasign/signing/client_test.go`:

```go
package signing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_IssueCertificate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/certificates/issue" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var req CertificateIssueRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.SubjectCN != "Test User" {
			t.Errorf("unexpected CN: %s", req.SubjectCN)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CertificateIssueResponse{
			SerialNumber:      "abc123",
			CertificatePEM:    "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
			IssuerDN:          "CN=GarudaPass Dev Root CA",
			SubjectDN:         "CN=Test User",
			ValidFrom:         "2026-04-06T00:00:00Z",
			ValidTo:           "2027-04-06T00:00:00Z",
			FingerprintSHA256: "deadbeef",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second)
	resp, err := client.IssueCertificate(context.Background(), CertificateIssueRequest{
		SubjectCN:    "Test User",
		SubjectUID:   "uid-1",
		ValidityDays: 365,
	})
	if err != nil {
		t.Fatalf("IssueCertificate error: %v", err)
	}
	if resp.SerialNumber != "abc123" {
		t.Errorf("unexpected serial: %s", resp.SerialNumber)
	}
}

func TestClient_IssueCertificate_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"ca failure"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second)
	_, err := client.IssueCertificate(context.Background(), CertificateIssueRequest{})
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestClient_SignDocument_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sign/pades" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SignResponse{
			SignedDocumentBase64: "c2lnbmVk",
			SignatureTimestamp:   "2026-04-06T12:00:00Z",
			PAdESLevel:          "B_LTA",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second)
	resp, err := client.SignDocument(context.Background(), SignRequest{
		DocumentBase64: "dGVzdA==",
		CertificatePEM: "cert",
		SignatureLevel: "PAdES_BASELINE_LTA",
	})
	if err != nil {
		t.Fatalf("SignDocument error: %v", err)
	}
	if resp.PAdESLevel != "B_LTA" {
		t.Errorf("unexpected pades level: %s", resp.PAdESLevel)
	}
}

func TestClient_CircuitBreaker(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second)
	ctx := context.Background()

	// Trip the circuit breaker (5 failures)
	for i := 0; i < 5; i++ {
		client.IssueCertificate(ctx, CertificateIssueRequest{})
	}

	// Next call should be rejected by circuit breaker without hitting server
	beforeCount := callCount
	_, err := client.IssueCertificate(ctx, CertificateIssueRequest{})
	if err == nil {
		t.Error("expected circuit breaker error")
	}
	if callCount != beforeCount {
		t.Error("circuit breaker should have prevented the request")
	}
}
```

- [ ] **Step 3: Implement signing client with circuit breaker**

Create `services/garudasign/signing/client.go`:

```go
package signing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	stateClosed   = iota
	stateOpen
	stateHalfOpen
)

type circuitBreaker struct {
	mu           sync.Mutex
	state        int
	failureCount int
	threshold    int
	openUntil    time.Time
	cooldown     time.Duration
}

func newCircuitBreaker(threshold int, cooldown time.Duration) *circuitBreaker {
	return &circuitBreaker{
		state:     stateClosed,
		threshold: threshold,
		cooldown:  cooldown,
	}
}

func (cb *circuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case stateClosed:
		return true
	case stateOpen:
		if time.Now().After(cb.openUntil) {
			cb.state = stateHalfOpen
			return true
		}
		return false
	case stateHalfOpen:
		return true
	}
	return false
}

func (cb *circuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount = 0
	cb.state = stateClosed
}

func (cb *circuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount++
	if cb.failureCount >= cb.threshold {
		cb.state = stateOpen
		cb.openUntil = time.Now().Add(cb.cooldown)
	}
}

// Client is an HTTP client for the signing backend (signing-sim or EJBCA+DSS).
type Client struct {
	baseURL    string
	httpClient *http.Client
	cb         *circuitBreaker
}

// NewClient creates a signing backend client.
func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		cb: newCircuitBreaker(5, 30*time.Second),
	}
}

// IssueCertificate requests a certificate from the signing backend.
func (c *Client) IssueCertificate(ctx context.Context, req CertificateIssueRequest) (*CertificateIssueResponse, error) {
	var resp CertificateIssueResponse
	if err := c.doPost(ctx, "/certificates/issue", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SignDocument sends a document for PAdES signing.
func (c *Client) SignDocument(ctx context.Context, req SignRequest) (*SignResponse, error) {
	var resp SignResponse
	if err := c.doPost(ctx, "/sign/pades", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) doPost(ctx context.Context, path string, body, result interface{}) error {
	if !c.cb.allow() {
		return fmt.Errorf("circuit breaker open: signing service unavailable")
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.cb.recordFailure()
		return fmt.Errorf("signing request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.cb.recordFailure()
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		c.cb.recordFailure()
		return fmt.Errorf("signing backend returned status %d: %s", resp.StatusCode, string(respBody))
	}

	c.cb.recordSuccess()

	if err := json.Unmarshal(respBody, result); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd services/garudasign && go test ./signing/... -v -count=1
```

Expected: All 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add services/garudasign/signing/
git commit -m "feat(garudasign): add signing client with circuit breaker and shared types"
```

---

### Task 6: GarudaSign — Stores (Certificate, Request, Document)

**Files:**
- Create: `services/garudasign/store/certificate.go`
- Create: `services/garudasign/store/certificate_test.go`
- Create: `services/garudasign/store/request.go`
- Create: `services/garudasign/store/request_test.go`
- Create: `services/garudasign/store/document.go`
- Create: `services/garudasign/store/document_test.go`

- [ ] **Step 1: Write failing certificate store tests**

Create `services/garudasign/store/certificate_test.go`:

```go
package store

import (
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

func TestCertificateStore_CreateAndGet(t *testing.T) {
	s := NewInMemoryCertificateStore()
	cert := &signing.Certificate{
		UserID:            "user-1",
		SerialNumber:      "serial-1",
		IssuerDN:          "CN=Root",
		SubjectDN:         "CN=User",
		Status:            "ACTIVE",
		ValidFrom:         time.Now(),
		ValidTo:           time.Now().Add(365 * 24 * time.Hour),
		CertificatePEM:    "pem-data",
		FingerprintSHA256: "fingerprint-1",
	}

	created, err := s.Create(cert)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if created.ID == "" {
		t.Error("ID should be set")
	}

	got, err := s.GetByID(created.ID)
	if err != nil {
		t.Fatalf("GetByID error: %v", err)
	}
	if got.SerialNumber != "serial-1" {
		t.Errorf("unexpected serial: %s", got.SerialNumber)
	}
}

func TestCertificateStore_ListByUser(t *testing.T) {
	s := NewInMemoryCertificateStore()
	for i := 0; i < 3; i++ {
		s.Create(&signing.Certificate{
			UserID:       "user-1",
			SerialNumber: "serial-" + string(rune('a'+i)),
			Status:       "ACTIVE",
			ValidFrom:    time.Now(),
			ValidTo:      time.Now().Add(365 * 24 * time.Hour),
		})
	}
	s.Create(&signing.Certificate{
		UserID:       "user-2",
		SerialNumber: "serial-other",
		Status:       "ACTIVE",
		ValidFrom:    time.Now(),
		ValidTo:      time.Now().Add(365 * 24 * time.Hour),
	})

	certs, err := s.ListByUser("user-1", "")
	if err != nil {
		t.Fatalf("ListByUser error: %v", err)
	}
	if len(certs) != 3 {
		t.Errorf("expected 3 certs, got %d", len(certs))
	}
}

func TestCertificateStore_ListByUserWithStatusFilter(t *testing.T) {
	s := NewInMemoryCertificateStore()
	s.Create(&signing.Certificate{UserID: "u1", SerialNumber: "s1", Status: "ACTIVE", ValidFrom: time.Now(), ValidTo: time.Now().Add(time.Hour)})
	s.Create(&signing.Certificate{UserID: "u1", SerialNumber: "s2", Status: "REVOKED", ValidFrom: time.Now(), ValidTo: time.Now().Add(time.Hour)})

	certs, _ := s.ListByUser("u1", "ACTIVE")
	if len(certs) != 1 {
		t.Errorf("expected 1 active cert, got %d", len(certs))
	}
}

func TestCertificateStore_UpdateStatus(t *testing.T) {
	s := NewInMemoryCertificateStore()
	cert, _ := s.Create(&signing.Certificate{
		UserID: "user-1", SerialNumber: "s1", Status: "ACTIVE",
		ValidFrom: time.Now(), ValidTo: time.Now().Add(time.Hour),
	})

	now := time.Now()
	err := s.UpdateStatus(cert.ID, "REVOKED", &now, "key_compromise")
	if err != nil {
		t.Fatalf("UpdateStatus error: %v", err)
	}

	got, _ := s.GetByID(cert.ID)
	if got.Status != "REVOKED" {
		t.Errorf("expected REVOKED, got %s", got.Status)
	}
	if got.RevokedAt == nil {
		t.Error("RevokedAt should be set")
	}
	if got.RevocationReason != "key_compromise" {
		t.Errorf("unexpected reason: %s", got.RevocationReason)
	}
}

func TestCertificateStore_GetByID_NotFound(t *testing.T) {
	s := NewInMemoryCertificateStore()
	_, err := s.GetByID("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestCertificateStore_GetActiveByUser(t *testing.T) {
	s := NewInMemoryCertificateStore()
	s.Create(&signing.Certificate{UserID: "u1", SerialNumber: "s1", Status: "ACTIVE", ValidFrom: time.Now(), ValidTo: time.Now().Add(time.Hour)})
	s.Create(&signing.Certificate{UserID: "u1", SerialNumber: "s2", Status: "EXPIRED", ValidFrom: time.Now(), ValidTo: time.Now().Add(time.Hour)})

	cert, err := s.GetActiveByUser("u1")
	if err != nil {
		t.Fatalf("GetActiveByUser error: %v", err)
	}
	if cert.Status != "ACTIVE" {
		t.Errorf("expected ACTIVE, got %s", cert.Status)
	}
}
```

- [ ] **Step 2: Implement certificate store**

Create `services/garudasign/store/certificate.go`:

```go
package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

// CertificateStore manages signing certificates.
type CertificateStore interface {
	Create(cert *signing.Certificate) (*signing.Certificate, error)
	GetByID(id string) (*signing.Certificate, error)
	GetActiveByUser(userID string) (*signing.Certificate, error)
	ListByUser(userID, statusFilter string) ([]*signing.Certificate, error)
	UpdateStatus(id, status string, revokedAt *time.Time, reason string) error
}

// InMemoryCertificateStore is an in-memory implementation of CertificateStore.
type InMemoryCertificateStore struct {
	mu    sync.RWMutex
	certs map[string]*signing.Certificate
	seq   int
}

// NewInMemoryCertificateStore creates a new in-memory certificate store.
func NewInMemoryCertificateStore() *InMemoryCertificateStore {
	return &InMemoryCertificateStore{
		certs: make(map[string]*signing.Certificate),
	}
}

func (s *InMemoryCertificateStore) Create(cert *signing.Certificate) (*signing.Certificate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	cert.ID = fmt.Sprintf("cert-%d", s.seq)
	cert.CreatedAt = time.Now()
	cert.UpdatedAt = time.Now()

	// Make a copy
	stored := *cert
	s.certs[cert.ID] = &stored

	return cert, nil
}

func (s *InMemoryCertificateStore) GetByID(id string) (*signing.Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cert, ok := s.certs[id]
	if !ok {
		return nil, fmt.Errorf("certificate not found: %s", id)
	}
	result := *cert
	return &result, nil
}

func (s *InMemoryCertificateStore) GetActiveByUser(userID string) (*signing.Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, cert := range s.certs {
		if cert.UserID == userID && cert.Status == "ACTIVE" {
			result := *cert
			return &result, nil
		}
	}
	return nil, fmt.Errorf("no active certificate for user: %s", userID)
}

func (s *InMemoryCertificateStore) ListByUser(userID, statusFilter string) ([]*signing.Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*signing.Certificate
	for _, cert := range s.certs {
		if cert.UserID != userID {
			continue
		}
		if statusFilter != "" && cert.Status != statusFilter {
			continue
		}
		c := *cert
		results = append(results, &c)
	}
	return results, nil
}

func (s *InMemoryCertificateStore) UpdateStatus(id, status string, revokedAt *time.Time, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cert, ok := s.certs[id]
	if !ok {
		return fmt.Errorf("certificate not found: %s", id)
	}
	cert.Status = status
	cert.RevokedAt = revokedAt
	cert.RevocationReason = reason
	cert.UpdatedAt = time.Now()
	return nil
}
```

- [ ] **Step 3: Write failing request store tests**

Create `services/garudasign/store/request_test.go`:

```go
package store

import (
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

func TestRequestStore_CreateAndGet(t *testing.T) {
	s := NewInMemoryRequestStore()
	req := &signing.SigningRequest{
		UserID:       "user-1",
		DocumentName: "doc.pdf",
		DocumentSize: 1024,
		DocumentHash: "abc123",
		DocumentPath: "/data/signing/doc.pdf",
		Status:       "PENDING",
		ExpiresAt:    time.Now().Add(30 * time.Minute),
	}

	created, err := s.Create(req)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if created.ID == "" {
		t.Error("ID should be set")
	}

	got, err := s.GetByID(created.ID)
	if err != nil {
		t.Fatalf("GetByID error: %v", err)
	}
	if got.DocumentName != "doc.pdf" {
		t.Errorf("unexpected name: %s", got.DocumentName)
	}
}

func TestRequestStore_UpdateStatus(t *testing.T) {
	s := NewInMemoryRequestStore()
	req, _ := s.Create(&signing.SigningRequest{
		UserID: "u1", DocumentName: "d.pdf", Status: "PENDING",
		ExpiresAt: time.Now().Add(30 * time.Minute),
	})

	err := s.UpdateStatus(req.ID, "COMPLETED", "cert-1", "")
	if err != nil {
		t.Fatalf("UpdateStatus error: %v", err)
	}

	got, _ := s.GetByID(req.ID)
	if got.Status != "COMPLETED" {
		t.Errorf("expected COMPLETED, got %s", got.Status)
	}
	if got.CertificateID != "cert-1" {
		t.Errorf("expected cert-1, got %s", got.CertificateID)
	}
}

func TestRequestStore_ListByUser(t *testing.T) {
	s := NewInMemoryRequestStore()
	for i := 0; i < 3; i++ {
		s.Create(&signing.SigningRequest{
			UserID: "u1", DocumentName: "d.pdf", Status: "PENDING",
			ExpiresAt: time.Now().Add(30 * time.Minute),
		})
	}

	reqs, err := s.ListByUser("u1")
	if err != nil {
		t.Fatalf("ListByUser error: %v", err)
	}
	if len(reqs) != 3 {
		t.Errorf("expected 3, got %d", len(reqs))
	}
}

func TestRequestStore_GetByID_NotFound(t *testing.T) {
	s := NewInMemoryRequestStore()
	_, err := s.GetByID("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}
```

- [ ] **Step 4: Implement request store**

Create `services/garudasign/store/request.go`:

```go
package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

// RequestStore manages signing requests.
type RequestStore interface {
	Create(req *signing.SigningRequest) (*signing.SigningRequest, error)
	GetByID(id string) (*signing.SigningRequest, error)
	ListByUser(userID string) ([]*signing.SigningRequest, error)
	UpdateStatus(id, status, certificateID, errorMsg string) error
}

// InMemoryRequestStore is an in-memory implementation of RequestStore.
type InMemoryRequestStore struct {
	mu   sync.RWMutex
	reqs map[string]*signing.SigningRequest
	seq  int
}

// NewInMemoryRequestStore creates a new in-memory request store.
func NewInMemoryRequestStore() *InMemoryRequestStore {
	return &InMemoryRequestStore{
		reqs: make(map[string]*signing.SigningRequest),
	}
}

func (s *InMemoryRequestStore) Create(req *signing.SigningRequest) (*signing.SigningRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	req.ID = fmt.Sprintf("req-%d", s.seq)
	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()

	stored := *req
	s.reqs[req.ID] = &stored

	return req, nil
}

func (s *InMemoryRequestStore) GetByID(id string) (*signing.SigningRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	req, ok := s.reqs[id]
	if !ok {
		return nil, fmt.Errorf("signing request not found: %s", id)
	}
	result := *req
	return &result, nil
}

func (s *InMemoryRequestStore) ListByUser(userID string) ([]*signing.SigningRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*signing.SigningRequest
	for _, req := range s.reqs {
		if req.UserID == userID {
			r := *req
			results = append(results, &r)
		}
	}
	return results, nil
}

func (s *InMemoryRequestStore) UpdateStatus(id, status, certificateID, errorMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.reqs[id]
	if !ok {
		return fmt.Errorf("signing request not found: %s", id)
	}
	req.Status = status
	if certificateID != "" {
		req.CertificateID = certificateID
	}
	req.ErrorMessage = errorMsg
	req.UpdatedAt = time.Now()
	return nil
}
```

- [ ] **Step 5: Write failing document store tests**

Create `services/garudasign/store/document_test.go`:

```go
package store

import (
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

func TestDocumentStore_CreateAndGet(t *testing.T) {
	s := NewInMemoryDocumentStore()
	doc := &signing.SignedDocument{
		RequestID:         "req-1",
		CertificateID:     "cert-1",
		SignedHash:        "signed-hash",
		SignedPath:        "/data/signed/doc.pdf",
		SignedSize:        2048,
		PAdESLevel:        "B_LTA",
		SignatureTimestamp: time.Now(),
	}

	created, err := s.Create(doc)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if created.ID == "" {
		t.Error("ID should be set")
	}

	got, err := s.GetByRequestID("req-1")
	if err != nil {
		t.Fatalf("GetByRequestID error: %v", err)
	}
	if got.SignedHash != "signed-hash" {
		t.Errorf("unexpected hash: %s", got.SignedHash)
	}
}

func TestDocumentStore_GetByRequestID_NotFound(t *testing.T) {
	s := NewInMemoryDocumentStore()
	_, err := s.GetByRequestID("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestDocumentStore_DuplicateRequestID(t *testing.T) {
	s := NewInMemoryDocumentStore()
	s.Create(&signing.SignedDocument{RequestID: "req-1", SignatureTimestamp: time.Now()})
	_, err := s.Create(&signing.SignedDocument{RequestID: "req-1", SignatureTimestamp: time.Now()})
	if err == nil {
		t.Error("expected error for duplicate request ID")
	}
}
```

- [ ] **Step 6: Implement document store**

Create `services/garudasign/store/document.go`:

```go
package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

// DocumentStore manages signed documents.
type DocumentStore interface {
	Create(doc *signing.SignedDocument) (*signing.SignedDocument, error)
	GetByRequestID(requestID string) (*signing.SignedDocument, error)
}

// InMemoryDocumentStore is an in-memory implementation of DocumentStore.
type InMemoryDocumentStore struct {
	mu   sync.RWMutex
	docs map[string]*signing.SignedDocument // requestID -> doc
	seq  int
}

// NewInMemoryDocumentStore creates a new in-memory document store.
func NewInMemoryDocumentStore() *InMemoryDocumentStore {
	return &InMemoryDocumentStore{
		docs: make(map[string]*signing.SignedDocument),
	}
}

func (s *InMemoryDocumentStore) Create(doc *signing.SignedDocument) (*signing.SignedDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.docs[doc.RequestID]; exists {
		return nil, fmt.Errorf("signed document already exists for request: %s", doc.RequestID)
	}

	s.seq++
	doc.ID = fmt.Sprintf("sdoc-%d", s.seq)
	doc.CreatedAt = time.Now()

	stored := *doc
	s.docs[doc.RequestID] = &stored

	return doc, nil
}

func (s *InMemoryDocumentStore) GetByRequestID(requestID string) (*signing.SignedDocument, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	doc, ok := s.docs[requestID]
	if !ok {
		return nil, fmt.Errorf("signed document not found for request: %s", requestID)
	}
	result := *doc
	return &result, nil
}
```

- [ ] **Step 7: Run all store tests**

```bash
cd services/garudasign && go test ./store/... -v -count=1
```

Expected: All tests PASS.

- [ ] **Step 8: Commit**

```bash
git add services/garudasign/store/
git commit -m "feat(garudasign): add certificate, request, and document stores with lifecycle management"
```

---

### Task 7: GarudaSign — Audit Emitter

**Files:**
- Create: `services/garudasign/audit/emitter.go`
- Create: `services/garudasign/audit/emitter_test.go`

- [ ] **Step 1: Write failing tests**

Create `services/garudasign/audit/emitter_test.go`:

```go
package audit

import (
	"testing"
)

func TestLogEmitter_Emit(t *testing.T) {
	emitter := NewLogEmitter()

	err := emitter.Emit(Event{
		UserID: "user-1",
		Action: ActionCertRequested,
		Metadata: map[string]string{
			"serial_number": "abc123",
		},
	})
	if err != nil {
		t.Fatalf("Emit error: %v", err)
	}
}

func TestLogEmitter_EmitAllActions(t *testing.T) {
	emitter := NewLogEmitter()

	actions := []string{
		ActionCertRequested, ActionCertIssued, ActionCertRevoked,
		ActionDocUploaded, ActionDocSigned, ActionDocDownloaded,
		ActionSignFailed,
	}

	for _, action := range actions {
		err := emitter.Emit(Event{
			UserID: "user-1",
			Action: action,
		})
		if err != nil {
			t.Errorf("Emit(%s) error: %v", action, err)
		}
	}
}

func TestLogEmitter_EmitRequiresUserID(t *testing.T) {
	emitter := NewLogEmitter()

	err := emitter.Emit(Event{
		Action: ActionDocSigned,
	})
	if err == nil {
		t.Error("expected error for missing user ID")
	}
}

func TestLogEmitter_EmitRequiresAction(t *testing.T) {
	emitter := NewLogEmitter()

	err := emitter.Emit(Event{
		UserID: "user-1",
	})
	if err == nil {
		t.Error("expected error for missing action")
	}
}
```

- [ ] **Step 2: Implement audit emitter**

Create `services/garudasign/audit/emitter.go`:

```go
package audit

import (
	"fmt"
	"log/slog"
	"time"
)

// Audit actions
const (
	ActionCertRequested  = "CERT_REQUESTED"
	ActionCertIssued     = "CERT_ISSUED"
	ActionCertRevoked    = "CERT_REVOKED"
	ActionDocUploaded    = "DOC_UPLOADED"
	ActionDocSigned      = "DOC_SIGNED"
	ActionDocDownloaded  = "DOC_DOWNLOADED"
	ActionSignFailed     = "SIGN_FAILED"
)

// Event represents an audit event for signing actions.
type Event struct {
	UserID   string
	Action   string
	Metadata map[string]string
}

// Emitter emits audit events.
type Emitter interface {
	Emit(event Event) error
}

// LogEmitter writes audit events to slog (MVP; swap for Kafka producer later).
type LogEmitter struct{}

// NewLogEmitter creates a new log-based audit emitter.
func NewLogEmitter() *LogEmitter {
	return &LogEmitter{}
}

// Emit writes an audit event to structured log.
func (e *LogEmitter) Emit(event Event) error {
	if event.UserID == "" {
		return fmt.Errorf("audit event requires user_id")
	}
	if event.Action == "" {
		return fmt.Errorf("audit event requires action")
	}

	attrs := []any{
		"topic", "audit.signing",
		"user_id", event.UserID,
		"action", event.Action,
		"timestamp", time.Now().UTC().Format(time.RFC3339),
	}

	for k, v := range event.Metadata {
		attrs = append(attrs, k, v)
	}

	slog.Info("audit.signing", attrs...)
	return nil
}
```

- [ ] **Step 3: Run tests**

```bash
cd services/garudasign && go test ./audit/... -v -count=1
```

Expected: All 4 tests PASS.

- [ ] **Step 4: Commit**

```bash
git add services/garudasign/audit/
git commit -m "feat(garudasign): add audit emitter with structured logging for signing events"
```

---

### Task 8: GarudaSign — Certificate Handler

**Files:**
- Create: `services/garudasign/handler/certificate.go`
- Create: `services/garudasign/handler/certificate_test.go`

- [ ] **Step 1: Write failing tests**

Create `services/garudasign/handler/certificate_test.go`:

```go
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/audit"
	"github.com/garudapass/gpass/services/garudasign/signing"
	"github.com/garudapass/gpass/services/garudasign/store"
)

type mockSigningClient struct {
	issueFn func(ctx context.Context, req signing.CertificateIssueRequest) (*signing.CertificateIssueResponse, error)
}

func (m *mockSigningClient) IssueCertificate(ctx context.Context, req signing.CertificateIssueRequest) (*signing.CertificateIssueResponse, error) {
	return m.issueFn(ctx, req)
}

func (m *mockSigningClient) SignDocument(ctx context.Context, req signing.SignRequest) (*signing.SignResponse, error) {
	return nil, nil
}

func setupCertHandlerDeps() (*CertificateHandler, *store.InMemoryCertificateStore) {
	certStore := store.NewInMemoryCertificateStore()
	client := &mockSigningClient{
		issueFn: func(_ context.Context, req signing.CertificateIssueRequest) (*signing.CertificateIssueResponse, error) {
			return &signing.CertificateIssueResponse{
				SerialNumber:      "test-serial",
				CertificatePEM:    "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
				IssuerDN:          "CN=Root",
				SubjectDN:         "CN=" + req.SubjectCN,
				ValidFrom:         time.Now().Format(time.RFC3339),
				ValidTo:           time.Now().Add(365 * 24 * time.Hour).Format(time.RFC3339),
				FingerprintSHA256: "test-fingerprint",
			}, nil
		},
	}

	h := NewCertificateHandler(CertificateDeps{
		CertStore:    certStore,
		SignClient:   client,
		AuditEmitter: audit.NewLogEmitter(),
		ValidityDays: 365,
	})
	return h, certStore
}

func TestRequestCertificate_Success(t *testing.T) {
	h, _ := setupCertHandlerDeps()

	body := `{"common_name":"John Doe"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/certificates/request", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()

	h.RequestCertificate(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["serial_number"] != "test-serial" {
		t.Errorf("unexpected serial: %v", resp["serial_number"])
	}
}

func TestRequestCertificate_MissingUserID(t *testing.T) {
	h, _ := setupCertHandlerDeps()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/certificates/request", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.RequestCertificate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRequestCertificate_ActiveCertExists(t *testing.T) {
	h, certStore := setupCertHandlerDeps()

	certStore.Create(&signing.Certificate{
		UserID: "user-123", SerialNumber: "existing", Status: "ACTIVE",
		ValidFrom: time.Now(), ValidTo: time.Now().Add(time.Hour),
	})

	body := `{"common_name":"John Doe"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/certificates/request", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()

	h.RequestCertificate(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestListCertificates_Success(t *testing.T) {
	h, certStore := setupCertHandlerDeps()

	certStore.Create(&signing.Certificate{
		UserID: "user-1", SerialNumber: "s1", Status: "ACTIVE",
		ValidFrom: time.Now(), ValidTo: time.Now().Add(time.Hour),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sign/certificates?status=ACTIVE", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()

	h.ListCertificates(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	certs := resp["certificates"].([]interface{})
	if len(certs) != 1 {
		t.Errorf("expected 1 cert, got %d", len(certs))
	}
}
```

- [ ] **Step 2: Implement certificate handler**

Create `services/garudasign/handler/certificate.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/garudapass/gpass/services/garudasign/audit"
	"github.com/garudapass/gpass/services/garudasign/signing"
	"github.com/garudapass/gpass/services/garudasign/store"
)

// SigningClient abstracts the signing backend (simulator or real).
type SigningClient interface {
	IssueCertificate(ctx interface{ Deadline() (time.Time, bool); Done() <-chan struct{}; Err() error; Value(any) any }, req signing.CertificateIssueRequest) (*signing.CertificateIssueResponse, error)
	SignDocument(ctx interface{ Deadline() (time.Time, bool); Done() <-chan struct{}; Err() error; Value(any) any }, req signing.SignRequest) (*signing.SignResponse, error)
}

// CertificateDeps holds dependencies for the certificate handler.
type CertificateDeps struct {
	CertStore    store.CertificateStore
	SignClient   SigningClient
	AuditEmitter audit.Emitter
	ValidityDays int
}

// CertificateHandler handles certificate-related requests.
type CertificateHandler struct {
	deps CertificateDeps
}

// NewCertificateHandler creates a new CertificateHandler.
func NewCertificateHandler(deps CertificateDeps) *CertificateHandler {
	return &CertificateHandler{deps: deps}
}

type requestCertificateReq struct {
	CommonName string `json:"common_name"`
}

// RequestCertificate handles POST /api/v1/sign/certificates/request.
func (h *CertificateHandler) RequestCertificate(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-User-ID header required"})
		return
	}

	var req requestCertificateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Check for existing active certificate
	if _, err := h.deps.CertStore.GetActiveByUser(userID); err == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "active_cert_exists"})
		return
	}

	h.deps.AuditEmitter.Emit(audit.Event{
		UserID: userID,
		Action: audit.ActionCertRequested,
	})

	cn := req.CommonName
	if cn == "" {
		cn = "GarudaPass User"
	}

	// Issue certificate via signing backend
	issueResp, err := h.deps.SignClient.IssueCertificate(r.Context(), signing.CertificateIssueRequest{
		SubjectCN:    cn,
		SubjectUID:   userID,
		ValidityDays: h.deps.ValidityDays,
	})
	if err != nil {
		h.deps.AuditEmitter.Emit(audit.Event{
			UserID:   userID,
			Action:   audit.ActionSignFailed,
			Metadata: map[string]string{"error": err.Error()},
		})
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "ca_unavailable"})
		return
	}

	validFrom, _ := time.Parse(time.RFC3339, issueResp.ValidFrom)
	validTo, _ := time.Parse(time.RFC3339, issueResp.ValidTo)

	cert, err := h.deps.CertStore.Create(&signing.Certificate{
		UserID:            userID,
		SerialNumber:      issueResp.SerialNumber,
		IssuerDN:          issueResp.IssuerDN,
		SubjectDN:         issueResp.SubjectDN,
		Status:            "ACTIVE",
		ValidFrom:         validFrom,
		ValidTo:           validTo,
		CertificatePEM:    issueResp.CertificatePEM,
		FingerprintSHA256: issueResp.FingerprintSHA256,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "store_error"})
		return
	}

	h.deps.AuditEmitter.Emit(audit.Event{
		UserID: userID,
		Action: audit.ActionCertIssued,
		Metadata: map[string]string{
			"certificate_id": cert.ID,
			"serial_number":  cert.SerialNumber,
		},
	})

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"certificate_id":    cert.ID,
		"serial_number":     cert.SerialNumber,
		"subject_dn":        cert.SubjectDN,
		"valid_from":        cert.ValidFrom.Format(time.RFC3339),
		"valid_to":          cert.ValidTo.Format(time.RFC3339),
		"status":            cert.Status,
		"fingerprint_sha256": cert.FingerprintSHA256,
	})
}

// ListCertificates handles GET /api/v1/sign/certificates.
func (h *CertificateHandler) ListCertificates(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-User-ID header required"})
		return
	}

	statusFilter := r.URL.Query().Get("status")
	certs, err := h.deps.CertStore.ListByUser(userID, statusFilter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list_error"})
		return
	}

	type certResp struct {
		ID                string `json:"id"`
		SerialNumber      string `json:"serial_number"`
		SubjectDN         string `json:"subject_dn"`
		IssuerDN          string `json:"issuer_dn"`
		Status            string `json:"status"`
		ValidFrom         string `json:"valid_from"`
		ValidTo           string `json:"valid_to"`
		FingerprintSHA256 string `json:"fingerprint_sha256"`
	}

	results := make([]certResp, 0, len(certs))
	for _, c := range certs {
		results = append(results, certResp{
			ID:                c.ID,
			SerialNumber:      c.SerialNumber,
			SubjectDN:         c.SubjectDN,
			IssuerDN:          c.IssuerDN,
			Status:            c.Status,
			ValidFrom:         c.ValidFrom.Format(time.RFC3339),
			ValidTo:           c.ValidTo.Format(time.RFC3339),
			FingerprintSHA256: c.FingerprintSHA256,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"certificates": results})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 3: Run tests**

```bash
cd services/garudasign && go test ./handler/... -v -count=1
```

Expected: All certificate handler tests PASS.

- [ ] **Step 4: Commit**

```bash
git add services/garudasign/handler/certificate.go services/garudasign/handler/certificate_test.go
git commit -m "feat(garudasign): add certificate request and list handlers with audit"
```

---

### Task 9: GarudaSign — Document Handler (Upload, Sign, Status, Download)

**Files:**
- Create: `services/garudasign/handler/document.go`
- Create: `services/garudasign/handler/document_test.go`

- [ ] **Step 1: Write failing tests**

Create `services/garudasign/handler/document_test.go`:

```go
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/audit"
	"github.com/garudapass/gpass/services/garudasign/signing"
	"github.com/garudapass/gpass/services/garudasign/storage"
	"github.com/garudapass/gpass/services/garudasign/store"
)

type fullMockSigningClient struct {
	issueFn func(ctx context.Context, req signing.CertificateIssueRequest) (*signing.CertificateIssueResponse, error)
	signFn  func(ctx context.Context, req signing.SignRequest) (*signing.SignResponse, error)
}

func (m *fullMockSigningClient) IssueCertificate(ctx context.Context, req signing.CertificateIssueRequest) (*signing.CertificateIssueResponse, error) {
	if m.issueFn != nil {
		return m.issueFn(ctx, req)
	}
	return nil, nil
}

func (m *fullMockSigningClient) SignDocument(ctx context.Context, req signing.SignRequest) (*signing.SignResponse, error) {
	return m.signFn(ctx, req)
}

func setupDocHandlerDeps(t *testing.T) (*DocumentHandler, *store.InMemoryCertificateStore, *store.InMemoryRequestStore) {
	t.Helper()
	certStore := store.NewInMemoryCertificateStore()
	reqStore := store.NewInMemoryRequestStore()
	docStore := store.NewInMemoryDocumentStore()
	fileStorage := storage.NewFileStorage(t.TempDir())

	client := &fullMockSigningClient{
		signFn: func(_ context.Context, req signing.SignRequest) (*signing.SignResponse, error) {
			return &signing.SignResponse{
				SignedDocumentBase64: "c2lnbmVkcGRm",
				SignatureTimestamp:   time.Now().Format(time.RFC3339),
				PAdESLevel:          "B_LTA",
			}, nil
		},
	}

	h := NewDocumentHandler(DocumentDeps{
		CertStore:    certStore,
		RequestStore: reqStore,
		DocStore:     docStore,
		FileStorage:  fileStorage,
		SignClient:   client,
		AuditEmitter: audit.NewLogEmitter(),
		MaxSizeMB:    10,
		RequestTTL:   30 * time.Minute,
	})
	return h, certStore, reqStore
}

func TestUploadDocument_Success(t *testing.T) {
	h, _, _ := setupDocHandlerDeps(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.pdf")
	part.Write([]byte("%PDF-1.4 test content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/documents", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()

	h.Upload(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "PENDING" {
		t.Errorf("expected PENDING status, got %v", resp["status"])
	}
}

func TestUploadDocument_MissingUserID(t *testing.T) {
	h, _, _ := setupDocHandlerDeps(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.pdf")
	part.Write([]byte("%PDF-1.4"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/documents", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	h.Upload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUploadDocument_NotPDF(t *testing.T) {
	h, _, _ := setupDocHandlerDeps(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("not a PDF"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/documents", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()

	h.Upload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSignDocument_Success(t *testing.T) {
	h, certStore, reqStore := setupDocHandlerDeps(t)

	// Create a certificate
	cert, _ := certStore.Create(&signing.Certificate{
		UserID: "user-1", SerialNumber: "s1", Status: "ACTIVE",
		CertificatePEM: "pem", ValidFrom: time.Now(), ValidTo: time.Now().Add(time.Hour),
	})

	// Create a signing request
	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID: "user-1", DocumentName: "doc.pdf", DocumentHash: "hash",
		DocumentPath: "/tmp/test", Status: "PENDING", DocumentSize: 100,
		ExpiresAt: time.Now().Add(30 * time.Minute),
	})

	body, _ := json.Marshal(map[string]string{"certificate_id": cert.ID})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/documents/"+sigReq.ID+"/sign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", sigReq.ID)
	w := httptest.NewRecorder()

	h.Sign(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSignDocument_NotOwner(t *testing.T) {
	h, certStore, reqStore := setupDocHandlerDeps(t)

	cert, _ := certStore.Create(&signing.Certificate{
		UserID: "user-2", SerialNumber: "s1", Status: "ACTIVE",
		CertificatePEM: "pem", ValidFrom: time.Now(), ValidTo: time.Now().Add(time.Hour),
	})
	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID: "user-1", DocumentName: "doc.pdf", Status: "PENDING",
		ExpiresAt: time.Now().Add(30 * time.Minute),
	})

	body, _ := json.Marshal(map[string]string{"certificate_id": cert.ID})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/documents/"+sigReq.ID+"/sign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", sigReq.ID)
	w := httptest.NewRecorder()

	h.Sign(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestSignDocument_Expired(t *testing.T) {
	h, certStore, reqStore := setupDocHandlerDeps(t)

	cert, _ := certStore.Create(&signing.Certificate{
		UserID: "user-1", SerialNumber: "s1", Status: "ACTIVE",
		CertificatePEM: "pem", ValidFrom: time.Now(), ValidTo: time.Now().Add(time.Hour),
	})
	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID: "user-1", DocumentName: "doc.pdf", Status: "PENDING",
		ExpiresAt: time.Now().Add(-1 * time.Minute), // expired
	})

	body, _ := json.Marshal(map[string]string{"certificate_id": cert.ID})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/documents/"+sigReq.ID+"/sign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", sigReq.ID)
	w := httptest.NewRecorder()

	h.Sign(w, req)

	if w.Code != http.StatusGone {
		t.Errorf("expected 410, got %d", w.Code)
	}
}

func TestGetDocument_Success(t *testing.T) {
	h, _, reqStore := setupDocHandlerDeps(t)

	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID: "user-1", DocumentName: "doc.pdf", DocumentHash: "hash",
		Status: "PENDING", ExpiresAt: time.Now().Add(30 * time.Minute),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sign/documents/"+sigReq.ID, nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", sigReq.ID)
	w := httptest.NewRecorder()

	h.GetStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetDocument_NotOwner(t *testing.T) {
	h, _, reqStore := setupDocHandlerDeps(t)

	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID: "user-1", DocumentName: "doc.pdf",
		Status: "PENDING", ExpiresAt: time.Now().Add(30 * time.Minute),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sign/documents/"+sigReq.ID, nil)
	req.Header.Set("X-User-ID", "user-other")
	req.SetPathValue("id", sigReq.ID)
	w := httptest.NewRecorder()

	h.GetStatus(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Implement document handler**

Create `services/garudasign/handler/document.go`:

```go
package handler

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/garudapass/gpass/services/garudasign/audit"
	"github.com/garudapass/gpass/services/garudasign/hash"
	"github.com/garudapass/gpass/services/garudasign/signing"
	"github.com/garudapass/gpass/services/garudasign/storage"
	"github.com/garudapass/gpass/services/garudasign/store"
	"encoding/json"
)

// DocumentDeps holds dependencies for the document handler.
type DocumentDeps struct {
	CertStore    store.CertificateStore
	RequestStore store.RequestStore
	DocStore     store.DocumentStore
	FileStorage  *storage.FileStorage
	SignClient   SigningClient
	AuditEmitter audit.Emitter
	MaxSizeMB    int
	RequestTTL   time.Duration
}

// DocumentHandler handles document upload, signing, status, and download.
type DocumentHandler struct {
	deps DocumentDeps
}

// NewDocumentHandler creates a new DocumentHandler.
func NewDocumentHandler(deps DocumentDeps) *DocumentHandler {
	return &DocumentHandler{deps: deps}
}

// Upload handles POST /api/v1/sign/documents.
func (h *DocumentHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-User-ID header required"})
		return
	}

	maxBytes := int64(h.deps.MaxSizeMB) * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	file, header, err := r.FormFile("file")
	if err != nil {
		if err.Error() == "http: request body too large" {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "file_too_large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file upload required"})
		return
	}
	defer file.Close()

	// Validate PDF
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".pdf") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_file_type"})
		return
	}

	// Read content for hash + magic byte check
	content, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read_error"})
		return
	}

	if len(content) < 4 || string(content[:5]) != "%PDF-" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_file_type"})
		return
	}

	docHash, err := hash.ComputeHash(bytes.NewReader(content))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "hash_error"})
		return
	}

	path, err := h.deps.FileStorage.Save(header.Filename, bytes.NewReader(content))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "storage_error"})
		return
	}

	sigReq, err := h.deps.RequestStore.Create(&signing.SigningRequest{
		UserID:       userID,
		DocumentName: header.Filename,
		DocumentSize: int64(len(content)),
		DocumentHash: docHash,
		DocumentPath: path,
		Status:       "PENDING",
		ExpiresAt:    time.Now().Add(h.deps.RequestTTL),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "store_error"})
		return
	}

	h.deps.AuditEmitter.Emit(audit.Event{
		UserID: userID,
		Action: audit.ActionDocUploaded,
		Metadata: map[string]string{
			"request_id":    sigReq.ID,
			"document_name": header.Filename,
			"document_hash": docHash,
		},
	})

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"request_id":    sigReq.ID,
		"document_name": sigReq.DocumentName,
		"document_hash": sigReq.DocumentHash,
		"status":        sigReq.Status,
		"expires_at":    sigReq.ExpiresAt.Format(time.RFC3339),
	})
}

type signDocRequest struct {
	CertificateID string `json:"certificate_id"`
}

// Sign handles POST /api/v1/sign/documents/:id/sign.
func (h *DocumentHandler) Sign(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-User-ID header required"})
		return
	}

	requestID := r.PathValue("id")
	if requestID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request ID required"})
		return
	}

	var req signDocRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Get the signing request
	sigReq, err := h.deps.RequestStore.GetByID(requestID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "request_not_found"})
		return
	}

	// Verify ownership
	if sigReq.UserID != userID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not_owner"})
		return
	}

	// Check expiry
	if time.Now().After(sigReq.ExpiresAt) {
		h.deps.RequestStore.UpdateStatus(requestID, "EXPIRED", "", "")
		writeJSON(w, http.StatusGone, map[string]string{"error": "request_expired"})
		return
	}

	// Check already signed
	if sigReq.Status == "COMPLETED" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "already_signed"})
		return
	}

	// Verify certificate
	cert, err := h.deps.CertStore.GetByID(req.CertificateID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_certificate"})
		return
	}

	if cert.UserID != userID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not_owner"})
		return
	}

	if cert.Status != "ACTIVE" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "certificate_not_active"})
		return
	}

	// Update status to SIGNING
	h.deps.RequestStore.UpdateStatus(requestID, "SIGNING", req.CertificateID, "")

	// Read document from storage
	reader, err := h.deps.FileStorage.Load(sigReq.DocumentPath)
	if err != nil {
		h.deps.RequestStore.UpdateStatus(requestID, "FAILED", req.CertificateID, "storage_read_error")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "storage_error"})
		return
	}
	defer reader.Close()

	docBytes, err := io.ReadAll(reader)
	if err != nil {
		h.deps.RequestStore.UpdateStatus(requestID, "FAILED", req.CertificateID, "read_error")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "read_error"})
		return
	}

	docBase64 := base64.StdEncoding.EncodeToString(docBytes)

	// Sign via backend
	signResp, err := h.deps.SignClient.SignDocument(r.Context(), signing.SignRequest{
		DocumentBase64: docBase64,
		CertificatePEM: cert.CertificatePEM,
		SignatureLevel: "PAdES_BASELINE_LTA",
	})
	if err != nil {
		h.deps.RequestStore.UpdateStatus(requestID, "FAILED", req.CertificateID, err.Error())
		h.deps.AuditEmitter.Emit(audit.Event{
			UserID: userID,
			Action: audit.ActionSignFailed,
			Metadata: map[string]string{"request_id": requestID, "error": err.Error()},
		})
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "signing_engine_unavailable"})
		return
	}

	// Decode and store signed document
	signedBytes, err := base64.StdEncoding.DecodeString(signResp.SignedDocumentBase64)
	if err != nil {
		h.deps.RequestStore.UpdateStatus(requestID, "FAILED", req.CertificateID, "decode_error")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "decode_error"})
		return
	}

	signedHash, _ := hash.ComputeHash(bytes.NewReader(signedBytes))
	signedPath, err := h.deps.FileStorage.Save("signed_"+sigReq.DocumentName, bytes.NewReader(signedBytes))
	if err != nil {
		h.deps.RequestStore.UpdateStatus(requestID, "FAILED", req.CertificateID, "storage_write_error")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "storage_error"})
		return
	}

	sigTimestamp, _ := time.Parse(time.RFC3339, signResp.SignatureTimestamp)
	signedDoc, err := h.deps.DocStore.Create(&signing.SignedDocument{
		RequestID:         requestID,
		CertificateID:     req.CertificateID,
		SignedHash:        signedHash,
		SignedPath:        signedPath,
		SignedSize:        int64(len(signedBytes)),
		PAdESLevel:        signResp.PAdESLevel,
		SignatureTimestamp: sigTimestamp,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "store_error"})
		return
	}

	h.deps.RequestStore.UpdateStatus(requestID, "COMPLETED", req.CertificateID, "")

	h.deps.AuditEmitter.Emit(audit.Event{
		UserID: userID,
		Action: audit.ActionDocSigned,
		Metadata: map[string]string{
			"request_id":     requestID,
			"certificate_id": req.CertificateID,
			"signed_hash":    signedHash,
			"pades_level":    signResp.PAdESLevel,
		},
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"request_id": requestID,
		"status":     "COMPLETED",
		"signed_document": map[string]interface{}{
			"id":                  signedDoc.ID,
			"signed_hash":        signedDoc.SignedHash,
			"pades_level":        signedDoc.PAdESLevel,
			"signature_timestamp": signedDoc.SignatureTimestamp.Format(time.RFC3339),
		},
	})
}

// GetStatus handles GET /api/v1/sign/documents/:id.
func (h *DocumentHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-User-ID header required"})
		return
	}

	requestID := r.PathValue("id")
	sigReq, err := h.deps.RequestStore.GetByID(requestID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}

	if sigReq.UserID != userID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not_owner"})
		return
	}

	resp := map[string]interface{}{
		"id":            sigReq.ID,
		"document_name": sigReq.DocumentName,
		"document_hash": sigReq.DocumentHash,
		"status":        sigReq.Status,
		"created_at":    sigReq.CreatedAt.Format(time.RFC3339),
	}

	if sigReq.Status == "COMPLETED" {
		signedDoc, err := h.deps.DocStore.GetByRequestID(requestID)
		if err == nil {
			resp["signed_document"] = map[string]interface{}{
				"id":                  signedDoc.ID,
				"signed_hash":        signedDoc.SignedHash,
				"pades_level":        signedDoc.PAdESLevel,
				"signature_timestamp": signedDoc.SignatureTimestamp.Format(time.RFC3339),
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// Download handles GET /api/v1/sign/documents/:id/download.
func (h *DocumentHandler) Download(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-User-ID header required"})
		return
	}

	requestID := r.PathValue("id")
	sigReq, err := h.deps.RequestStore.GetByID(requestID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}

	if sigReq.UserID != userID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not_owner"})
		return
	}

	if sigReq.Status != "COMPLETED" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "not_yet_signed"})
		return
	}

	signedDoc, err := h.deps.DocStore.GetByRequestID(requestID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "signed_doc_not_found"})
		return
	}

	reader, err := h.deps.FileStorage.Load(signedDoc.SignedPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "storage_error"})
		return
	}
	defer reader.Close()

	h.deps.AuditEmitter.Emit(audit.Event{
		UserID: userID,
		Action: audit.ActionDocDownloaded,
		Metadata: map[string]string{
			"request_id":  requestID,
			"signed_hash": signedDoc.SignedHash,
		},
	})

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=\"signed_"+sigReq.DocumentName+"\"")
	io.Copy(w, reader)
}
```

- [ ] **Step 3: Fix the SigningClient interface to use context.Context**

Update `services/garudasign/handler/certificate.go` — replace the verbose `SigningClient` interface:

```go
import "context"

// SigningClient abstracts the signing backend (simulator or real).
type SigningClient interface {
	IssueCertificate(ctx context.Context, req signing.CertificateIssueRequest) (*signing.CertificateIssueResponse, error)
	SignDocument(ctx context.Context, req signing.SignRequest) (*signing.SignResponse, error)
}
```

- [ ] **Step 4: Run tests**

```bash
cd services/garudasign && go test ./handler/... -v -count=1
```

Expected: All handler tests PASS.

- [ ] **Step 5: Commit**

```bash
git add services/garudasign/handler/document.go services/garudasign/handler/document_test.go services/garudasign/handler/certificate.go
git commit -m "feat(garudasign): add document upload, signing, status, and download handlers"
```

---

### Task 10: GarudaSign — Main + Dockerfile + Migrations

**Files:**
- Create: `services/garudasign/main.go`
- Create: `services/garudasign/Dockerfile`
- Create: `infrastructure/db/migrations/007_create_signing_certificates.sql`
- Create: `infrastructure/db/migrations/008_create_signing_requests.sql`
- Create: `infrastructure/db/migrations/009_create_signed_documents.sql`

- [ ] **Step 1: Create main.go**

Create `services/garudasign/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/garudapass/gpass/services/garudasign/audit"
	"github.com/garudapass/gpass/services/garudasign/config"
	"github.com/garudapass/gpass/services/garudasign/handler"
	signingpkg "github.com/garudapass/gpass/services/garudasign/signing"
	"github.com/garudapass/gpass/services/garudasign/storage"
	"github.com/garudapass/gpass/services/garudasign/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("starting GarudaSign service",
		"port", cfg.Port,
		"signing_mode", cfg.SigningMode,
		"max_size_mb", cfg.MaxSizeMB,
	)

	// Stores (in-memory for MVP)
	certStore := store.NewInMemoryCertificateStore()
	reqStore := store.NewInMemoryRequestStore()
	docStore := store.NewInMemoryDocumentStore()

	// File storage
	fileStorage := storage.NewFileStorage(cfg.DocumentStoragePath)

	// Signing client
	var signingURL string
	if cfg.IsSimulator() {
		signingURL = cfg.SigningSimURL
	} else {
		signingURL = cfg.EJBCAURL
	}
	signClient := signingpkg.NewClient(signingURL, 30*time.Second)

	// Audit
	auditEmitter := audit.NewLogEmitter()

	// Handlers
	certHandler := handler.NewCertificateHandler(handler.CertificateDeps{
		CertStore:    certStore,
		SignClient:   signClient,
		AuditEmitter: auditEmitter,
		ValidityDays: cfg.CertValidityDays,
	})

	docHandler := handler.NewDocumentHandler(handler.DocumentDeps{
		CertStore:    certStore,
		RequestStore: reqStore,
		DocStore:     docStore,
		FileStorage:  fileStorage,
		SignClient:   signClient,
		AuditEmitter: auditEmitter,
		MaxSizeMB:    cfg.MaxSizeMB,
		RequestTTL:   cfg.RequestTTL,
	})

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok","service":"garudasign"}`)
	})

	// Certificate APIs
	mux.HandleFunc("POST /api/v1/sign/certificates/request", certHandler.RequestCertificate)
	mux.HandleFunc("GET /api/v1/sign/certificates", certHandler.ListCertificates)

	// Document APIs
	mux.HandleFunc("POST /api/v1/sign/documents", docHandler.Upload)
	mux.HandleFunc("POST /api/v1/sign/documents/{id}/sign", docHandler.Sign)
	mux.HandleFunc("GET /api/v1/sign/documents/{id}", docHandler.GetStatus)
	mux.HandleFunc("GET /api/v1/sign/documents/{id}/download", docHandler.Download)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("GarudaSign service listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutdown signal received", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	slog.Info("GarudaSign service shut down gracefully")
}
```

- [ ] **Step 2: Create Dockerfile**

Create `services/garudasign/Dockerfile`:

```dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o garudasign .

FROM alpine:3.20

RUN apk --no-cache add ca-certificates \
    && adduser -D -u 1000 appuser

WORKDIR /app

RUN mkdir -p /data/signing && chown appuser:appuser /data/signing

COPY --from=builder /build/garudasign .

USER appuser

EXPOSE 4007

ENTRYPOINT ["./garudasign"]
```

- [ ] **Step 3: Create database migrations**

Create `infrastructure/db/migrations/007_create_signing_certificates.sql`:

```sql
CREATE TABLE IF NOT EXISTS signing_certificates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    serial_number VARCHAR(64) UNIQUE NOT NULL,
    issuer_dn VARCHAR(500) NOT NULL,
    subject_dn VARCHAR(500) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    valid_from TIMESTAMPTZ NOT NULL,
    valid_to TIMESTAMPTZ NOT NULL,
    certificate_pem TEXT NOT NULL,
    fingerprint_sha256 VARCHAR(64) NOT NULL,
    revoked_at TIMESTAMPTZ,
    revocation_reason VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_signing_certs_user_id ON signing_certificates(user_id);
CREATE INDEX IF NOT EXISTS idx_signing_certs_status ON signing_certificates(status);
CREATE INDEX IF NOT EXISTS idx_signing_certs_serial ON signing_certificates(serial_number);
CREATE INDEX IF NOT EXISTS idx_signing_certs_fingerprint ON signing_certificates(fingerprint_sha256);
CREATE INDEX IF NOT EXISTS idx_signing_certs_valid_to ON signing_certificates(valid_to) WHERE status = 'ACTIVE';
```

Create `infrastructure/db/migrations/008_create_signing_requests.sql`:

```sql
CREATE TABLE IF NOT EXISTS signing_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    certificate_id UUID REFERENCES signing_certificates(id),
    document_name VARCHAR(255) NOT NULL,
    document_size BIGINT NOT NULL,
    document_hash VARCHAR(64) NOT NULL,
    document_path VARCHAR(500) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    error_message TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_signing_requests_user_id ON signing_requests(user_id);
CREATE INDEX IF NOT EXISTS idx_signing_requests_status ON signing_requests(status);
CREATE INDEX IF NOT EXISTS idx_signing_requests_expires_at ON signing_requests(expires_at) WHERE status = 'PENDING';
```

Create `infrastructure/db/migrations/009_create_signed_documents.sql`:

```sql
CREATE TABLE IF NOT EXISTS signed_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id UUID UNIQUE NOT NULL REFERENCES signing_requests(id),
    certificate_id UUID NOT NULL REFERENCES signing_certificates(id),
    signed_hash VARCHAR(64) NOT NULL,
    signed_path VARCHAR(500) NOT NULL,
    signed_size BIGINT NOT NULL,
    pades_level VARCHAR(20) NOT NULL,
    signature_timestamp TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_signed_docs_request_id ON signed_documents(request_id);
CREATE INDEX IF NOT EXISTS idx_signed_docs_certificate_id ON signed_documents(certificate_id);
```

- [ ] **Step 4: Build to verify compilation**

```bash
cd services/garudasign && go build -o /dev/null .
```

Expected: Successful build.

- [ ] **Step 5: Commit**

```bash
git add services/garudasign/main.go services/garudasign/Dockerfile infrastructure/db/migrations/007_create_signing_certificates.sql infrastructure/db/migrations/008_create_signing_requests.sql infrastructure/db/migrations/009_create_signed_documents.sql
git commit -m "feat(garudasign): add main entrypoint, Dockerfile, and signing database migrations"
```

---

### Task 11: Infrastructure Integration

**Files:**
- Modify: `docker-compose.yml`
- Modify: `.env.example`
- Modify: `go.work`
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Update go.work**

Add to `go.work`:

```
use (
    ...existing...
    ./services/garudasign
    ./services/signing-sim
)
```

- [ ] **Step 2: Update docker-compose.yml**

Add `signing-sim` and `garudasign` services (per spec §11), and `signing-data` volume.

- [ ] **Step 3: Update .env.example**

Add all Phase 4 env vars from spec §9.

- [ ] **Step 4: Update CI workflow**

Add `garudasign` and `signing-sim` to test matrix, security scan, and Docker build.

- [ ] **Step 5: Run all tests across all 9 services**

```bash
for svc in apps/bff services/identity services/garudainfo services/dukcapil-sim services/ahu-sim services/oss-sim services/garudacorp services/signing-sim services/garudasign; do
  echo "=== $svc ==="
  cd /opt/gpass/$svc && go test ./... -count=1
done
```

Expected: All tests PASS across all 9 services.

- [ ] **Step 6: Commit**

```bash
git add go.work docker-compose.yml .env.example .github/workflows/ci.yml
git commit -m "feat: integrate Phase 4 digital signing infrastructure"
```
