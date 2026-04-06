package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
