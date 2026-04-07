//go:build integration

package store

import (
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
	_ "github.com/lib/pq"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM signing_certificates WHERE issuer_dn = 'CN=integration_test'")
		db.Close()
	})
	return db
}

func TestPostgresCertificateStore_Lifecycle(t *testing.T) {
	db := openTestDB(t)
	s := NewPostgresCertificateStore(db)

	now := time.Now().UTC()
	cert := &signing.Certificate{
		UserID:            "00000000-0000-0000-0000-000000000010",
		SerialNumber:      "INT-TEST-001",
		IssuerDN:          "CN=integration_test",
		SubjectDN:         "CN=user",
		Status:            "ACTIVE",
		ValidFrom:         now,
		ValidTo:           now.Add(365 * 24 * time.Hour),
		CertificatePEM:    "-----BEGIN CERTIFICATE-----\nFAKE\n-----END CERTIFICATE-----",
		FingerprintSHA256: "abcdef0123456789",
	}
	if _, err := s.Create(cert); err != nil {
		t.Fatalf("create: %v", err)
	}
	if cert.ID == "" {
		t.Fatal("expected ID")
	}

	got, err := s.GetByID(cert.ID)
	if err != nil {
		t.Fatalf("getbyid: %v", err)
	}
	if got.SerialNumber != "INT-TEST-001" || got.Status != "ACTIVE" {
		t.Errorf("mismatch: %+v", got)
	}

	active, err := s.GetActiveByUser(cert.UserID)
	if err != nil || active.ID != cert.ID {
		t.Errorf("getactive: err=%v id=%s", err, active)
	}

	revokedAt := time.Now().UTC()
	if err := s.UpdateStatus(cert.ID, "REVOKED", &revokedAt, "key_compromise"); err != nil {
		t.Fatalf("update: %v", err)
	}
	got2, _ := s.GetByID(cert.ID)
	if got2.Status != "REVOKED" || got2.RevocationReason != "key_compromise" {
		t.Errorf("after revoke: %+v", got2)
	}
}
