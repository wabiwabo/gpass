package mtls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTempFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGenerateTestCerts(t *testing.T) {
	ca, serverCert, serverKey, clientCert, clientKey, err := GenerateTestCerts("test.local")
	if err != nil {
		t.Fatalf("GenerateTestCerts: %v", err)
	}

	for _, b := range []struct {
		name string
		data []byte
	}{
		{"ca", ca},
		{"serverCert", serverCert},
		{"serverKey", serverKey},
		{"clientCert", clientCert},
		{"clientKey", clientKey},
	} {
		if len(b.data) == 0 {
			t.Errorf("%s is empty", b.name)
		}
	}

	// Verify server cert is parseable
	_, err = tls.X509KeyPair(serverCert, serverKey)
	if err != nil {
		t.Errorf("server cert/key pair invalid: %v", err)
	}

	// Verify client cert is parseable
	_, err = tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		t.Errorf("client cert/key pair invalid: %v", err)
	}

	// Verify CA can be added to pool
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		t.Error("CA cert could not be added to pool")
	}
}

func TestNewTLSConfigValid(t *testing.T) {
	ca, _, _, clientCert, clientKey, err := GenerateTestCerts("test.local")
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	certPath := writeTempFile(t, dir, "client.crt", clientCert)
	keyPath := writeTempFile(t, dir, "client.key", clientKey)
	caPath := writeTempFile(t, dir, "ca.crt", ca)

	cfg := Config{
		CertFile:   certPath,
		KeyFile:    keyPath,
		CAFile:     caPath,
		ServerName: "test.local",
	}

	tlsCfg, err := NewTLSConfig(cfg)
	if err != nil {
		t.Fatalf("NewTLSConfig: %v", err)
	}

	if len(tlsCfg.Certificates) != 1 {
		t.Error("expected 1 certificate")
	}
	if tlsCfg.RootCAs == nil {
		t.Error("expected RootCAs to be set")
	}
	if tlsCfg.ServerName != "test.local" {
		t.Errorf("expected ServerName test.local, got %s", tlsCfg.ServerName)
	}
}

func TestNewTLSConfigMissingCert(t *testing.T) {
	dir := t.TempDir()
	keyPath := writeTempFile(t, dir, "client.key", []byte("dummy"))

	cfg := Config{
		CertFile: filepath.Join(dir, "nonexistent.crt"),
		KeyFile:  keyPath,
	}

	_, err := NewTLSConfig(cfg)
	if err == nil {
		t.Fatal("expected error for missing cert file")
	}
}

func TestNewTLSConfigMissingKey(t *testing.T) {
	dir := t.TempDir()
	certPath := writeTempFile(t, dir, "client.crt", []byte("dummy"))

	cfg := Config{
		CertFile: certPath,
		KeyFile:  filepath.Join(dir, "nonexistent.key"),
	}

	_, err := NewTLSConfig(cfg)
	if err == nil {
		t.Fatal("expected error for missing key file")
	}
}

func TestNewHTTPClientCreates(t *testing.T) {
	ca, _, _, clientCert, clientKey, err := GenerateTestCerts("test.local")
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	certPath := writeTempFile(t, dir, "client.crt", clientCert)
	keyPath := writeTempFile(t, dir, "client.key", clientKey)
	caPath := writeTempFile(t, dir, "ca.crt", ca)

	client, err := NewHTTPClient(Config{
		CertFile:   certPath,
		KeyFile:    keyPath,
		CAFile:     caPath,
		ServerName: "test.local",
	}, 30*time.Second)

	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", client.Timeout)
	}
}

func TestMTLSRoundTrip(t *testing.T) {
	ca, serverCert, serverKey, clientCert, clientKey, err := GenerateTestCerts("localhost")
	if err != nil {
		t.Fatal(err)
	}

	// Set up server with mTLS
	serverTLSCert, err := tls.X509KeyPair(serverCert, serverKey)
	if err != nil {
		t.Fatal(err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(ca) {
		t.Fatal("failed to add CA to pool")
	}

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "hello mtls")
	}))
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverTLSCert},
		ClientCAs:    caPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}
	server.StartTLS()
	defer server.Close()

	// Set up client with mTLS
	dir := t.TempDir()
	certPath := writeTempFile(t, dir, "client.crt", clientCert)
	keyPath := writeTempFile(t, dir, "client.key", clientKey)
	caPath := writeTempFile(t, dir, "ca.crt", ca)

	client, err := NewHTTPClient(Config{
		CertFile:   certPath,
		KeyFile:    keyPath,
		CAFile:     caPath,
		ServerName: "localhost",
	}, 10*time.Second)
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello mtls" {
		t.Errorf("expected 'hello mtls', got %q", string(body))
	}
}
