package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/garudapass/gpass/services/garudanotify/channel"
)

func newTestNotifyHandler() (*NotifyHandler, *channel.MockEmailSender, *channel.MockSMSSender) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	return NewNotifyHandler(email, sms), email, sms
}

func TestSendOTPEmailSuccess(t *testing.T) {
	h, email, _ := newTestNotifyHandler()
	body := `{"channel":"email","recipient":"user@example.com","otp_code":"123456"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.SendOTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	msgs := email.Sent
	if len(msgs) != 1 {
		t.Fatalf("emails sent: got %d, want 1", len(msgs))
	}
	if msgs[0].To != "user@example.com" {
		t.Errorf("recipient: got %q", msgs[0].To)
	}
	if !strings.Contains(msgs[0].Body, "123456") {
		t.Error("email should contain OTP code")
	}
}

func TestSendOTPSMSSuccess(t *testing.T) {
	h, _, sms := newTestNotifyHandler()
	body := `{"channel":"sms","recipient":"+6281234567890","otp_code":"654321"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.SendOTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	msgs := sms.Sent
	if len(msgs) != 1 {
		t.Fatalf("SMS sent: got %d", len(msgs))
	}
	if !strings.Contains(msgs[0].Message, "654321") {
		t.Error("SMS should contain OTP code")
	}
}

func TestSendOTPInvalidChannel(t *testing.T) {
	h, _, _ := newTestNotifyHandler()
	body := `{"channel":"telegram","recipient":"user","otp_code":"123"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.SendOTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

func TestSendOTPMissingChannel(t *testing.T) {
	h, _, _ := newTestNotifyHandler()
	body := `{"recipient":"user@example.com","otp_code":"123"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.SendOTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

func TestSendOTPInvalidJSON(t *testing.T) {
	h, _, _ := newTestNotifyHandler()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad"))
	rr := httptest.NewRecorder()
	h.SendOTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

func TestSendOTPResponseJSON(t *testing.T) {
	h, _, _ := newTestNotifyHandler()
	body := `{"channel":"email","recipient":"a@b.com","otp_code":"111"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.SendOTP(rr, req)

	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type: %q", ct)
	}
	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["status"] != "sent" {
		t.Errorf("status: got %q", resp["status"])
	}
}

func TestSendAlertEmailSuccess(t *testing.T) {
	h, email, _ := newTestNotifyHandler()
	body := `{"channel":"email","recipient":"admin@example.com","subject":"Security Alert","message":"Suspicious login detected"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.SendAlert(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d", rr.Code)
	}
	msgs := email.Sent
	if len(msgs) != 1 {
		t.Fatalf("emails: got %d", len(msgs))
	}
	if msgs[0].Subject != "Security Alert" {
		t.Errorf("subject: got %q", msgs[0].Subject)
	}
}

func TestSendAlertInvalidChannel(t *testing.T) {
	h, _, _ := newTestNotifyHandler()
	body := `{"channel":"sms","recipient":"+62123","subject":"Alert","message":"Test"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.SendAlert(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

func TestSendAlertMissingChannel(t *testing.T) {
	h, _, _ := newTestNotifyHandler()
	body := `{"recipient":"a@b.com","subject":"Alert","message":"Test"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.SendAlert(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

func TestSendAlertInvalidJSON(t *testing.T) {
	h, _, _ := newTestNotifyHandler()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	rr := httptest.NewRecorder()
	h.SendAlert(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

func TestWriteErrorFormat(t *testing.T) {
	rr := httptest.NewRecorder()
	writeError(rr, http.StatusBadRequest, "test_error", "test message")

	if rr.Code != 400 {
		t.Errorf("status: %d", rr.Code)
	}
	if rr.Header().Get("Cache-Control") != "no-store" {
		t.Error("Cache-Control should be no-store")
	}
	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["error"] != "test_error" {
		t.Errorf("error: %q", resp["error"])
	}
	if resp["message"] != "test message" {
		t.Errorf("message: %q", resp["message"])
	}
}
