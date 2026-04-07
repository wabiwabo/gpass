package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

// TestDownload_HappyPath_FullLifecycle pins the previously-50% Download
// success path: a fully signed document must produce a 200 with
// Content-Type=application/pdf, a Content-Disposition attachment header
// carrying the original filename, and the signed bytes streamed via
// io.Copy. This is the only test that exercises the audit Emit call
// and the io.Copy(w, fileReader) tail of the handler.
func TestDownload_HappyPath_FullLifecycle(t *testing.T) {
	deps, reqStore, certStore := newDocumentDeps(t)

	// Wire a signing client that produces a deterministic signed payload.
	signedContent := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 SIGNED CONTENT"))
	deps.SignClient = &mockSigningClient{
		issueFn: defaultMockClient().issueFn,
		signFn: func(ctx context.Context, req signing.SignRequest) (*signing.SignResponse, error) {
			return &signing.SignResponse{
				SignedDocumentBase64: signedContent,
				SignatureTimestamp:   "2026-04-07T00:00:00Z",
				PAdESLevel:           "PAdES-B-LTA",
			}, nil
		},
	}
	handler := NewDocumentHandler(deps)

	// Persist a PDF + signing request + active certificate so Sign succeeds.
	pdfContent := createPDFContent()
	path, err := deps.FileStorage.Save("contract.pdf", bytes.NewReader(pdfContent))
	if err != nil {
		t.Fatal(err)
	}
	sigReq, err := reqStore.Create(&signing.SigningRequest{
		UserID:       "user-dl",
		DocumentName: "contract.pdf",
		DocumentSize: int64(len(pdfContent)),
		DocumentHash: "sha256",
		DocumentPath: path,
		Status:       "PENDING",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := certStore.Create(&signing.Certificate{
		UserID:         "user-dl",
		Status:         "ACTIVE",
		SerialNumber:   "SN-DL",
		CertificatePEM: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
	}); err != nil {
		t.Fatal(err)
	}

	// Sign so the request transitions to COMPLETED and the signed
	// document gets written to FileStorage.
	signReq := httptest.NewRequest(http.MethodPost, "/", nil)
	signReq.Header.Set("X-User-ID", "user-dl")
	signReq.SetPathValue("id", sigReq.ID)
	signRec := httptest.NewRecorder()
	handler.Sign(signRec, signReq)
	if signRec.Code != http.StatusOK {
		t.Fatalf("sign: %d %s", signRec.Code, signRec.Body)
	}

	// Now Download.
	dlReq := httptest.NewRequest(http.MethodGet, "/", nil)
	dlReq.Header.Set("X-User-ID", "user-dl")
	dlReq.SetPathValue("id", sigReq.ID)
	dlRec := httptest.NewRecorder()
	handler.Download(dlRec, dlReq)

	if dlRec.Code != http.StatusOK {
		t.Fatalf("Download: code = %d, body = %s", dlRec.Code, dlRec.Body)
	}
	if ct := dlRec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type = %q, want application/pdf", ct)
	}
	cd := dlRec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, `filename="signed_contract.pdf"`) {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if dlRec.Body.Len() == 0 {
		t.Error("response body empty — io.Copy did not stream the signed file")
	}
}
