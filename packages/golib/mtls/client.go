package mtls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"time"
)

// Config holds mTLS configuration.
type Config struct {
	CertFile           string // client certificate PEM file path
	KeyFile            string // client private key PEM file path
	CAFile             string // CA certificate PEM file path (for server verification)
	ServerName         string // expected server name for verification
	InsecureSkipVerify bool   // ONLY for testing
}

// NewHTTPClient creates an HTTP client configured with mTLS.
func NewHTTPClient(cfg Config, timeout time.Duration) (*http.Client, error) {
	tlsCfg, err := NewTLSConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
		},
	}, nil
}

// NewTLSConfig creates a TLS config from the mTLS config.
func NewTLSConfig(cfg Config) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("mtls: load client cert/key: %w", err)
	}

	tlsCfg := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		ServerName:         cfg.ServerName,
		MinVersion:         tls.VersionTLS12,
	}

	if cfg.CAFile != "" {
		caPEM, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("mtls: read CA file: %w", err)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("mtls: failed to parse CA certificate")
		}
		tlsCfg.RootCAs = caPool
	}

	return tlsCfg, nil
}

// GenerateTestCerts generates self-signed CA, server, and client certs for testing.
// Returns (caCert, serverCert, serverKey, clientCert, clientKey []byte, err error).
func GenerateTestCerts(cn string) (ca, serverCert, serverKey, clientCert, clientKey []byte, err error) {
	// Generate CA key and cert
	caPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("generate CA key: %w", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   cn + " CA",
			Organization: []string{"GarudaPass Test"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caPriv.PublicKey, caPriv)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("create CA cert: %w", err)
	}

	ca = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("parse CA cert: %w", err)
	}

	// Generate server cert
	serverCert, serverKey, err = generateLeafCert(cn, caCert, caPriv, false)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("generate server cert: %w", err)
	}

	// Generate client cert
	clientCert, clientKey, err = generateLeafCert(cn+" client", caCert, caPriv, true)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("generate client cert: %w", err)
	}

	return ca, serverCert, serverKey, clientCert, clientKey, nil
}

func generateLeafCert(cn string, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, isClient bool) (certPEM, keyPEM []byte, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: []string{"GarudaPass Test"},
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}

	if isClient {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		template.DNSNames = []string{cn, "localhost"}
	}

	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &priv.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	privDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})

	return certPEM, keyPEM, nil
}
