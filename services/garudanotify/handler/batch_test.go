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

func TestSendBatch_Success(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	batch := BatchRequest{
		Items: []BatchItem{
			{Type: "otp", Channel: "email", Recipient: "user1@example.com", OTPCode: "111111"},
			{Type: "otp", Channel: "sms", Recipient: "+6281234567890", OTPCode: "222222"},
			{Type: "alert", Channel: "email", Recipient: "admin@example.com", Subject: "Test", Message: "Hello"},
		},
	}
	body, _ := json.Marshal(batch)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/batch", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.SendBatch(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp BatchResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Total != 3 {
		t.Errorf("expected total 3, got %d", resp.Total)
	}
	if resp.Succeeded != 3 {
		t.Errorf("expected 3 succeeded, got %d", resp.Succeeded)
	}
	if resp.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", resp.Failed)
	}
	if len(email.Sent) != 2 {
		t.Errorf("expected 2 emails sent, got %d", len(email.Sent))
	}
	if len(sms.Sent) != 1 {
		t.Errorf("expected 1 SMS sent, got %d", len(sms.Sent))
	}
}

func TestSendBatch_PartialFailure(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	batch := BatchRequest{
		Items: []BatchItem{
			{Type: "otp", Channel: "email", Recipient: "user@example.com", OTPCode: "111111"},
			{Type: "otp", Channel: "sms", Recipient: "invalid-phone", OTPCode: "222222"}, // will fail - no +62
			{Type: "alert", Channel: "email", Recipient: "admin@example.com", Subject: "Test", Message: "Hello"},
		},
	}
	body, _ := json.Marshal(batch)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/batch", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.SendBatch(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp BatchResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Succeeded != 2 {
		t.Errorf("expected 2 succeeded, got %d", resp.Succeeded)
	}
	if resp.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", resp.Failed)
	}
	if resp.Results[1].Status != "failed" {
		t.Errorf("expected item 1 to be failed, got %s", resp.Results[1].Status)
	}
}

func TestSendBatch_EmptyBatch(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	batch := BatchRequest{Items: []BatchItem{}}
	body, _ := json.Marshal(batch)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/batch", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.SendBatch(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "empty_batch" {
		t.Errorf("expected error %q, got %q", "empty_batch", resp["error"])
	}
}

func TestSendBatch_TooManyItems(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	items := make([]BatchItem, 101)
	for i := range items {
		items[i] = BatchItem{Type: "otp", Channel: "email", Recipient: "user@example.com", OTPCode: "123456"}
	}
	batch := BatchRequest{Items: items}
	body, _ := json.Marshal(batch)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/batch", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.SendBatch(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "batch_too_large" {
		t.Errorf("expected error %q, got %q", "batch_too_large", resp["error"])
	}
}

func TestSendBatch_InvalidJSON(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/batch", strings.NewReader("{invalid"))
	rec := httptest.NewRecorder()

	h.SendBatch(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestSendBatch_InvalidType(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	batch := BatchRequest{
		Items: []BatchItem{
			{Type: "unknown", Channel: "email", Recipient: "user@example.com"},
		},
	}
	body, _ := json.Marshal(batch)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/batch", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.SendBatch(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp BatchResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", resp.Failed)
	}
	if resp.Results[0].Error != "invalid_type" {
		t.Errorf("expected error %q, got %q", "invalid_type", resp.Results[0].Error)
	}
}

func TestSendBatch_EmailSendFailure(t *testing.T) {
	email := channel.NewMockEmailSender()
	email.FailOnSend = true
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	batch := BatchRequest{
		Items: []BatchItem{
			{Type: "otp", Channel: "email", Recipient: "user@example.com", OTPCode: "123456"},
		},
	}
	body, _ := json.Marshal(batch)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/batch", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.SendBatch(rec, req)

	var resp BatchResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", resp.Failed)
	}
}

func TestSendBatch_ExactlyMaxItems(t *testing.T) {
	email := channel.NewMockEmailSender()
	sms := channel.NewMockSMSSender()
	h := NewNotifyHandler(email, sms)

	items := make([]BatchItem, 100)
	for i := range items {
		items[i] = BatchItem{Type: "otp", Channel: "email", Recipient: "user@example.com", OTPCode: "123456"}
	}
	batch := BatchRequest{Items: items}
	body, _ := json.Marshal(batch)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notify/batch", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.SendBatch(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d for exactly 100 items, got %d", http.StatusOK, rec.Code)
	}

	var resp BatchResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Total != 100 {
		t.Errorf("expected total 100, got %d", resp.Total)
	}
	if resp.Succeeded != 100 {
		t.Errorf("expected 100 succeeded, got %d", resp.Succeeded)
	}
}
