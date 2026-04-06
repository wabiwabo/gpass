package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/garudapass/gpass/services/garudanotify/channel"
)

func TestSendOTP_Email(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	body := `{"channel":"email","recipient":"user@example.com","otp_code":"123456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/otp", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.SendOTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if len(email.Sent) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(email.Sent))
	}
	if email.Sent[0].To != "user@example.com" {
		t.Errorf("expected recipient %q, got %q", "user@example.com", email.Sent[0].To)
	}
}

func TestSendOTP_SMS(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	body := `{"channel":"sms","recipient":"+6281234567890","otp_code":"654321"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/otp", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.SendOTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if len(sms.Sent) != 1 {
		t.Fatalf("expected 1 SMS sent, got %d", len(sms.Sent))
	}
	if sms.Sent[0].PhoneNumber != "+6281234567890" {
		t.Errorf("expected phone %q, got %q", "+6281234567890", sms.Sent[0].PhoneNumber)
	}
}

func TestSendOTP_MissingChannel(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	body := `{"recipient":"user@example.com","otp_code":"123456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/otp", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.SendOTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "missing_channel" {
		t.Errorf("expected error %q, got %q", "missing_channel", resp["error"])
	}
}

func TestSendOTP_InvalidChannel(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	body := `{"channel":"pigeon","recipient":"user@example.com","otp_code":"123456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/otp", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.SendOTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "invalid_channel" {
		t.Errorf("expected error %q, got %q", "invalid_channel", resp["error"])
	}
}

func TestSendAlert_Email(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	body := `{"channel":"email","recipient":"admin@example.com","subject":"Security Alert","message":"Unusual login detected"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/alert", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.SendAlert(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if len(email.Sent) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(email.Sent))
	}
	if email.Sent[0].Subject != "Security Alert" {
		t.Errorf("expected subject %q, got %q", "Security Alert", email.Sent[0].Subject)
	}
	if email.Sent[0].Body != "Unusual login detected" {
		t.Errorf("expected body %q, got %q", "Unusual login detected", email.Sent[0].Body)
	}
}

func TestSendOTP_InvalidJSON(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/otp", strings.NewReader("{invalid"))
	rec := httptest.NewRecorder()

	h.SendOTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "invalid_request" {
		t.Errorf("expected error %q, got %q", "invalid_request", resp["error"])
	}
}

func TestSendOTP_EmptyBody(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/otp", strings.NewReader(""))
	rec := httptest.NewRecorder()

	h.SendOTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestSendOTP_MissingRecipient(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	body := `{"channel":"email","otp_code":"123456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/otp", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.SendOTP(rec, req)

	// Email sender returns error for empty recipient
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d: %s", http.StatusInternalServerError, rec.Code, rec.Body.String())
	}
}

func TestSendOTP_MissingOTPCode(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	body := `{"channel":"email","recipient":"user@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/otp", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.SendOTP(rec, req)

	// Still sends but with empty OTP code (handler doesn't validate OTP code)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestSendOTP_EmailSendFailure(t *testing.T) {
	email := channel.NewMockEmailSender()
	email.FailOnSend = true
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	body := `{"channel":"email","recipient":"user@example.com","otp_code":"123456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/otp", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.SendOTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "send_failed" {
		t.Errorf("expected error %q, got %q", "send_failed", resp["error"])
	}
}

func TestSendOTP_SMSSendFailure(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	sms.FailOnSend = true
	h := NewNotifyHandler(email, sms)

	body := `{"channel":"sms","recipient":"+6281234567890","otp_code":"123456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/otp", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.SendOTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "send_failed" {
		t.Errorf("expected error %q, got %q", "send_failed", resp["error"])
	}
}

func TestSendAlert_InvalidJSON(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/alert", strings.NewReader("not json"))
	rec := httptest.NewRecorder()

	h.SendAlert(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestSendAlert_MissingSubject(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	body := `{"channel":"email","recipient":"admin@example.com","message":"Hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/alert", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.SendAlert(rec, req)

	// MockEmailSender returns error for empty subject
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d: %s", http.StatusInternalServerError, rec.Code, rec.Body.String())
	}
}

func TestSendAlert_MissingMessage(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	body := `{"channel":"email","recipient":"admin@example.com","subject":"Test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/alert", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.SendAlert(rec, req)

	// Empty message is allowed by the mock sender
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestSendAlert_EmailSendFailure(t *testing.T) {
	email := channel.NewMockEmailSender()
	email.FailOnSend = true
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	body := `{"channel":"email","recipient":"admin@example.com","subject":"Alert","message":"Test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/alert", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.SendAlert(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestSendAlert_InvalidChannel(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	body := `{"channel":"sms","recipient":"+6281234567890","subject":"Alert","message":"Test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/alert", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.SendAlert(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestSendOTP_ContentTypeHeader(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	body := `{"channel":"email","recipient":"user@example.com","otp_code":"123456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/otp", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.SendOTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type %q, got %q", "application/json; charset=utf-8", ct)
	}
}

func TestSendOTP_ErrorCacheControlHeader(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	body := `{"channel":"pigeon","recipient":"user@example.com","otp_code":"123456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/otp", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.SendOTP(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if cc != "no-store" {
		t.Errorf("expected Cache-Control %q on error, got %q", "no-store", cc)
	}
}

func TestSendAlert_ContentTypeHeader(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	body := `{"channel":"email","recipient":"admin@example.com","subject":"Alert","message":"Hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/alert", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.SendAlert(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type %q, got %q", "application/json; charset=utf-8", ct)
	}
}
