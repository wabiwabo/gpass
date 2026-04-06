package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func computeTestSignature(payload, secret string, timestamp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	msg := fmt.Sprintf("%d.%s", timestamp, payload)
	mac.Write([]byte(msg))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", timestamp, sig)
}

func TestVerifySignature_Valid(t *testing.T) {
	h := NewVerifyWebhookHandler()

	payload := `{"event":"app.created"}`
	secret := "whsec_testsecret123"
	ts := time.Now().Unix()
	signature := computeTestSignature(payload, secret, ts)

	body, _ := json.Marshal(verifyWebhookRequest{
		Payload:   payload,
		Signature: signature,
		Secret:    secret,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/webhooks/verify", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.VerifySignature(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp verifyWebhookResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if !resp.Valid {
		t.Errorf("expected valid=true, got false: %s", resp.Details)
	}
}

func TestVerifySignature_Invalid(t *testing.T) {
	h := NewVerifyWebhookHandler()

	payload := `{"event":"app.created"}`
	secret := "whsec_testsecret123"
	ts := time.Now().Unix()
	signature := fmt.Sprintf("t=%d,v1=deadbeef0000", ts)

	body, _ := json.Marshal(verifyWebhookRequest{
		Payload:   payload,
		Signature: signature,
		Secret:    secret,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/webhooks/verify", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.VerifySignature(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp verifyWebhookResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Valid {
		t.Error("expected valid=false for wrong signature")
	}
}

func TestVerifySignature_MissingFields(t *testing.T) {
	h := NewVerifyWebhookHandler()

	tests := []struct {
		name string
		body verifyWebhookRequest
	}{
		{"missing payload", verifyWebhookRequest{Signature: "t=1,v1=abc", Secret: "whsec_x"}},
		{"missing signature", verifyWebhookRequest{Payload: "test", Secret: "whsec_x"}},
		{"missing secret", verifyWebhookRequest{Payload: "test", Signature: "t=1,v1=abc"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/webhooks/verify", bytes.NewReader(body))
			w := httptest.NewRecorder()

			h.VerifySignature(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestVerifySignature_ExpiredTimestamp(t *testing.T) {
	h := NewVerifyWebhookHandler()

	payload := `{"event":"app.created"}`
	secret := "whsec_testsecret123"
	// 10 minutes ago -- beyond the 5 minute tolerance
	ts := time.Now().Add(-10 * time.Minute).Unix()
	signature := computeTestSignature(payload, secret, ts)

	body, _ := json.Marshal(verifyWebhookRequest{
		Payload:   payload,
		Signature: signature,
		Secret:    secret,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/webhooks/verify", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.VerifySignature(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp verifyWebhookResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Valid {
		t.Error("expected valid=false for expired timestamp")
	}
	if resp.Details != "Signature timestamp has expired" {
		t.Errorf("expected expired message, got: %s", resp.Details)
	}
}
