package mtls

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestNewHTTPClient_ErrorBubblesUp covers the only branch of NewHTTPClient
// that wasn't reached by existing tests: NewTLSConfig fails → NewHTTPClient
// returns the wrapped error rather than a half-built http.Client.
func TestNewHTTPClient_ErrorBubblesUp(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		CertFile: filepath.Join(dir, "absent.crt"),
		KeyFile:  filepath.Join(dir, "absent.key"),
	}
	c, err := NewHTTPClient(cfg, time.Second)
	if err == nil {
		t.Fatal("expected error from NewHTTPClient")
	}
	if c != nil {
		t.Errorf("client should be nil on error, got %+v", c)
	}
	if !strings.Contains(err.Error(), "mtls:") {
		t.Errorf("err = %q, want mtls-prefixed wrap", err.Error())
	}
}

// TestNewTLSConfig_CAFileMissing covers the os.ReadFile error branch when
// the CAFile path is set but the file does not exist.
func TestNewTLSConfig_CAFileMissing(t *testing.T) {
	ca, _, _, clientCert, clientKey, err := GenerateTestCerts("test.local")
	if err != nil {
		t.Fatal(err)
	}
	_ = ca

	dir := t.TempDir()
	certPath := writeTempFile(t, dir, "client.crt", clientCert)
	keyPath := writeTempFile(t, dir, "client.key", clientKey)

	cfg := Config{
		CertFile: certPath,
		KeyFile:  keyPath,
		CAFile:   filepath.Join(dir, "no-such-ca.pem"),
	}
	_, err = NewTLSConfig(cfg)
	if err == nil {
		t.Fatal("expected error for missing CA file")
	}
	if !strings.Contains(err.Error(), "read CA file") {
		t.Errorf("err = %q, want substring 'read CA file'", err.Error())
	}
}

// TestNewTLSConfig_CAFileNotPEM covers the AppendCertsFromPEM-returns-false
// branch — the file exists but contains no parseable PEM blocks.
func TestNewTLSConfig_CAFileNotPEM(t *testing.T) {
	_, _, _, clientCert, clientKey, err := GenerateTestCerts("test.local")
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	certPath := writeTempFile(t, dir, "client.crt", clientCert)
	keyPath := writeTempFile(t, dir, "client.key", clientKey)
	garbagePath := writeTempFile(t, dir, "garbage.pem", []byte("this is not a certificate, just bytes"))

	cfg := Config{
		CertFile: certPath,
		KeyFile:  keyPath,
		CAFile:   garbagePath,
	}
	_, err = NewTLSConfig(cfg)
	if err == nil {
		t.Fatal("expected error for non-PEM CA file")
	}
	if !strings.Contains(err.Error(), "failed to parse CA certificate") {
		t.Errorf("err = %q, want substring 'failed to parse CA certificate'", err.Error())
	}
}

// TestNewTLSConfig_NoCAStillWorks covers the branch where CAFile is empty
// — the resulting tls.Config has no RootCAs set (uses system roots) but
// the function returns nil error.
func TestNewTLSConfig_NoCAStillWorks(t *testing.T) {
	_, _, _, clientCert, clientKey, err := GenerateTestCerts("test.local")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certPath := writeTempFile(t, dir, "client.crt", clientCert)
	keyPath := writeTempFile(t, dir, "client.key", clientKey)

	tlsCfg, err := NewTLSConfig(Config{CertFile: certPath, KeyFile: keyPath})
	if err != nil {
		t.Fatalf("NewTLSConfig: %v", err)
	}
	if tlsCfg.RootCAs != nil {
		t.Error("RootCAs should be nil when CAFile is unset")
	}
	if tlsCfg.MinVersion != 0x0303 { // TLS 1.2
		t.Errorf("MinVersion = 0x%x, want TLS 1.2", tlsCfg.MinVersion)
	}
}
