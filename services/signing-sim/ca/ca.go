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
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// IssuedCertificate holds the details of an issued certificate.
type IssuedCertificate struct {
	SerialNumber      string
	IssuerDN          string
	SubjectDN         string
	CertificatePEM    string
	FingerprintSHA256 string
	ValidFrom         time.Time
	ValidTo           time.Time
	PrivateKey        *ecdsa.PrivateKey
}

// CA is an in-memory Certificate Authority using ECDSA P-256.
type CA struct {
	rootCert *x509.Certificate
	rootKey  *ecdsa.PrivateKey
	rootPEM  string

	mu     sync.RWMutex
	certs  map[string]*IssuedCertificate // keyed by serial number (hex)
	serial *big.Int
}

// NewCA generates a self-signed root CA certificate.
func NewCA() (*CA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate CA key: %w", err)
	}

	serial := big.NewInt(1)

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

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create CA certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("parse CA certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	return &CA{
		rootCert: cert,
		rootKey:  key,
		rootPEM:  string(certPEM),
		certs:    make(map[string]*IssuedCertificate),
		serial:   big.NewInt(1), // next serial starts at 2
	}, nil
}

// RootCertificate returns the root CA certificate PEM.
func (ca *CA) RootCertificate() string {
	return ca.rootPEM
}

// RootCN returns the root CA common name.
func (ca *CA) RootCN() string {
	return ca.rootCert.Subject.CommonName
}

// RootSerial returns the root CA serial number in hex.
func (ca *CA) RootSerial() string {
	return ca.rootCert.SerialNumber.Text(16)
}

// IssueCertificate issues an end-entity certificate signed by the root CA.
func (ca *CA) IssueCertificate(commonName, userID string, validityDays int) (*IssuedCertificate, error) {
	if commonName == "" {
		return nil, errors.New("commonName is required")
	}
	if validityDays <= 0 {
		return nil, errors.New("validityDays must be greater than 0")
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	ca.mu.Lock()
	ca.serial.Add(ca.serial, big.NewInt(1))
	serialNum := new(big.Int).Set(ca.serial)
	ca.mu.Unlock()

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serialNum,
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

	if userID != "" {
		template.Subject.SerialNumber = userID
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca.rootCert, &key.PublicKey, ca.rootKey)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	fingerprint := sha256.Sum256(certDER)

	serialHex := serialNum.Text(16)

	issued := &IssuedCertificate{
		SerialNumber:      serialHex,
		IssuerDN:          ca.rootCert.Subject.String(),
		SubjectDN:         template.Subject.String(),
		CertificatePEM:    string(certPEM),
		FingerprintSHA256: hex.EncodeToString(fingerprint[:]),
		ValidFrom:         template.NotBefore,
		ValidTo:           template.NotAfter,
		PrivateKey:        key,
	}

	ca.mu.Lock()
	ca.certs[serialHex] = issued
	ca.mu.Unlock()

	return issued, nil
}

// GetCertificate retrieves an issued certificate by serial number.
func (ca *CA) GetCertificate(serialNumber string) (*IssuedCertificate, bool) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	cert, ok := ca.certs[serialNumber]
	return cert, ok
}

// ListSerials returns all issued certificate serial numbers.
func (ca *CA) ListSerials() []string {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	serials := make([]string, 0, len(ca.certs))
	for s := range ca.certs {
		serials = append(serials, s)
	}
	return serials
}
