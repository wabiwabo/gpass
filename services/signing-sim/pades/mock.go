package pades

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

// SignResult holds the result of a PAdES signing operation.
type SignResult struct {
	SignedDocumentBase64 string `json:"signed_document_base64"`
	SignatureTimestamp   string `json:"signature_timestamp"`
	PAdESLevel          string `json:"pades_level"`
}

// SignPAdES performs a mock PAdES-B-LTA signing of a base64-encoded document.
func SignPAdES(documentBase64, certificatePEM string, privateKey *ecdsa.PrivateKey) (*SignResult, error) {
	if documentBase64 == "" {
		return nil, errors.New("document_base64 is required")
	}
	if certificatePEM == "" {
		return nil, errors.New("certificate_pem is required")
	}
	if privateKey == nil {
		return nil, errors.New("private key is required")
	}

	docBytes, err := base64.StdEncoding.DecodeString(documentBase64)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}

	if len(docBytes) == 0 {
		return nil, errors.New("document is empty")
	}

	// Compute SHA-256 hash of the document
	hash := sha256.Sum256(docBytes)

	// Sign with ECDSA
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, hash[:])
	if err != nil {
		return nil, fmt.Errorf("ECDSA sign: %w", err)
	}

	// Append mock PAdES signature block
	sigBlock := fmt.Sprintf("\n%%PAdES-B-LTA-SIG%%\n%%SIG:%x\n%%END-SIG%%\n", signature)
	signedDoc := append(docBytes, []byte(sigBlock)...)

	return &SignResult{
		SignedDocumentBase64: base64.StdEncoding.EncodeToString(signedDoc),
		SignatureTimestamp:   time.Now().UTC().Format(time.RFC3339),
		PAdESLevel:          "B_LTA",
	}, nil
}
