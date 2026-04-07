package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func vwReq(body string) *httptest.ResponseRecorder {
	h := NewVerifyWebhookHandler()
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.VerifySignature(rec, req)
	return rec
}

// TestVerifySignature_BadJSON pins the JSON decode error.
func TestVerifySignature_BadJSON(t *testing.T) {
	rec := vwReq("{not json")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d", rec.Code)
	}
}

// TestVerifySignature_MissingFields_Cov pins the required-fields guard.
func TestVerifySignature_MissingFields_Cov(t *testing.T) {
	rec := vwReq(`{"payload":"p"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d", rec.Code)
	}
}

// TestVerifySignature_BadSignatureFormat pins the parseWebhookSignature
// error → 200 with Valid:false response (not HTTP error).
func TestVerifySignature_BadSignatureFormat(t *testing.T) {
	rec := vwReq(`{"payload":"p","signature":"bogus","secret":"s"}`)
	if rec.Code != 200 {
		t.Errorf("code = %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"valid":false`)) {
		t.Errorf("body: %s", rec.Body)
	}
}

// TestVerifySignature_MissingTimestampPrefix pins that sub-error branch.
func TestVerifySignature_MissingTimestampPrefix(t *testing.T) {
	rec := vwReq(`{"payload":"p","signature":"bogus=1234,v1=abc","secret":"s"}`)
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"valid":false`)) {
		t.Errorf("body: %s", rec.Body)
	}
}

// TestVerifySignature_MissingV1Prefix pins the v1 prefix guard.
func TestVerifySignature_MissingV1Prefix(t *testing.T) {
	rec := vwReq(`{"payload":"p","signature":"t=1234,bogus=abc","secret":"s"}`)
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"valid":false`)) {
		t.Errorf("body: %s", rec.Body)
	}
}

// TestVerifySignature_BadTimestampInteger pins strconv.ParseInt failure.
func TestVerifySignature_BadTimestampInteger(t *testing.T) {
	rec := vwReq(`{"payload":"p","signature":"t=notanum,v1=abc","secret":"s"}`)
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"valid":false`)) {
		t.Errorf("body: %s", rec.Body)
	}
}

// TestVerifySignature_ExpiredTimestamp_Cov pins the tolerance window.
func TestVerifySignature_ExpiredTimestamp_Cov(t *testing.T) {
	oldTS := time.Now().Add(-1 * time.Hour).Unix()
	body := fmt.Sprintf(`{"payload":"p","signature":"t=%d,v1=abc","secret":"s"}`, oldTS)
	rec := vwReq(body)
	if !bytes.Contains(rec.Body.Bytes(), []byte("expired")) {
		t.Errorf("body: %s", rec.Body)
	}
}

// TestVerifySignature_SignatureMismatch pins hmac.Equal = false.
func TestVerifySignature_SignatureMismatch(t *testing.T) {
	ts := time.Now().Unix()
	body := fmt.Sprintf(`{"payload":"p","signature":"t=%d,v1=aaaa","secret":"s"}`, ts)
	rec := vwReq(body)
	if !bytes.Contains(rec.Body.Bytes(), []byte("does not match")) {
		t.Errorf("body: %s", rec.Body)
	}
}

// TestVerifySignature_Valid_Cov pins the full happy-path HMAC verification.
func TestVerifySignature_Valid_Cov(t *testing.T) {
	ts := time.Now().Unix()
	secret := "whsec_test"
	payload := `{"event":"test"}`
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.%s", ts, payload)))
	sig := hex.EncodeToString(mac.Sum(nil))

	body := fmt.Sprintf(`{"payload":%q,"signature":"t=%d,v1=%s","secret":%q}`, payload, ts, sig, secret)
	rec := vwReq(body)
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"valid":true`)) {
		t.Errorf("body: %s", rec.Body)
	}
}
