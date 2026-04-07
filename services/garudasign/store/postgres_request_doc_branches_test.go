package store

import (
	"database/sql"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

func gsValidRequest() *signing.SigningRequest {
	return &signing.SigningRequest{
		UserID: "u1", DocumentName: "doc.pdf", DocumentSize: 1024,
		DocumentHash: "abc123", DocumentPath: "/tmp/doc.pdf",
		Status: "PENDING", ExpiresAt: time.Now().Add(time.Hour),
	}
}

func gsValidSignedDoc() *signing.SignedDocument {
	return &signing.SignedDocument{
		RequestID: "r1", CertificateID: "c1",
		SignedHash: "abc", SignedPath: "/tmp/signed.pdf", SignedSize: 2048,
		PAdESLevel: "PAdES-B-LTA", SignatureTimestamp: time.Now(),
	}
}

func TestPostgresRequestStore_Create_ValidationFail(t *testing.T) {
	db, _ := sql.Open("gs-fake-ok", "")
	defer db.Close()
	s := NewPostgresRequestStore(db)
	if _, err := s.Create(&signing.SigningRequest{}); err == nil {
		t.Error("expected validation error")
	}
}

func TestPostgresRequestStore_Create_DriverError(t *testing.T) {
	db, _ := sql.Open("gs-fake-bad", "")
	defer db.Close()
	s := NewPostgresRequestStore(db)
	if _, err := s.Create(gsValidRequest()); err == nil {
		t.Error("expected driver error")
	}
}

func TestPostgresRequestStore_GetByID_NotFound(t *testing.T) {
	db, _ := sql.Open("gs-fake-ok", "")
	defer db.Close()
	s := NewPostgresRequestStore(db)
	if _, err := s.GetByID("missing"); err == nil {
		t.Error("expected not-found")
	}
}

func TestPostgresRequestStore_ListByUser_DriverError(t *testing.T) {
	db, _ := sql.Open("gs-fake-bad", "")
	defer db.Close()
	s := NewPostgresRequestStore(db)
	if _, err := s.ListByUser("u1"); err == nil {
		t.Error("expected error")
	}
}

func TestPostgresRequestStore_ListByUser_Empty(t *testing.T) {
	db, _ := sql.Open("gs-fake-ok", "")
	defer db.Close()
	s := NewPostgresRequestStore(db)
	list, err := s.ListByUser("u1")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(list) != 0 {
		t.Errorf("list = %d", len(list))
	}
}

func TestPostgresRequestStore_UpdateStatus_ValidationFail(t *testing.T) {
	db, _ := sql.Open("gs-fake-ok", "")
	defer db.Close()
	s := NewPostgresRequestStore(db)
	if err := s.UpdateStatus("id", "BOGUS", "", ""); err == nil {
		t.Error("expected validation error")
	}
}

func TestPostgresRequestStore_UpdateStatus_NotFound(t *testing.T) {
	db, _ := sql.Open("gs-fake-ok", "")
	defer db.Close()
	s := NewPostgresRequestStore(db)
	if err := s.UpdateStatus("id", "COMPLETED", "c1", ""); err == nil {
		t.Error("expected not-found")
	}
}

func TestPostgresRequestStore_UpdateStatus_DriverError(t *testing.T) {
	db, _ := sql.Open("gs-fake-bad", "")
	defer db.Close()
	s := NewPostgresRequestStore(db)
	if err := s.UpdateStatus("id", "COMPLETED", "c1", ""); err == nil {
		t.Error("expected driver error")
	}
}

func TestPostgresDocumentStore_Create_ValidationFail(t *testing.T) {
	db, _ := sql.Open("gs-fake-ok", "")
	defer db.Close()
	s := NewPostgresDocumentStore(db)
	if _, err := s.Create(&signing.SignedDocument{}); err == nil {
		t.Error("expected validation error")
	}
}

func TestPostgresDocumentStore_Create_DriverError(t *testing.T) {
	db, _ := sql.Open("gs-fake-bad", "")
	defer db.Close()
	s := NewPostgresDocumentStore(db)
	if _, err := s.Create(gsValidSignedDoc()); err == nil {
		t.Error("expected driver error")
	}
}

func TestPostgresDocumentStore_GetByRequestID_NotFound(t *testing.T) {
	db, _ := sql.Open("gs-fake-ok", "")
	defer db.Close()
	s := NewPostgresDocumentStore(db)
	if _, err := s.GetByRequestID("missing"); err == nil {
		t.Error("expected not-found")
	}
}

func TestPostgresDocumentStore_GetByRequestID_DriverError(t *testing.T) {
	db, _ := sql.Open("gs-fake-bad", "")
	defer db.Close()
	s := NewPostgresDocumentStore(db)
	if _, err := s.GetByRequestID("r1"); err == nil {
		t.Error("expected error")
	}
}

func TestNewPostgresRequestStore_NewPostgresDocumentStore(t *testing.T) {
	db, _ := sql.Open("gs-fake-ok", "")
	defer db.Close()
	if NewPostgresRequestStore(db) == nil {
		t.Error("nil request store")
	}
	if NewPostgresDocumentStore(db) == nil {
		t.Error("nil document store")
	}
}
