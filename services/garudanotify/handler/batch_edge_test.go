package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBatchSuccess(t *testing.T) {
	h, _, _ := newTestNotifyHandler()
	body := `{"items":[
		{"type":"otp","channel":"email","recipient":"a@b.com","otp_code":"111"},
		{"type":"otp","channel":"sms","recipient":"+6281234567890","otp_code":"222"},
		{"type":"alert","channel":"email","recipient":"admin@b.com","subject":"Alert","message":"Test"}
	]}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.SendBatch(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d", rr.Code)
	}
	var resp BatchResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Total != 3 {
		t.Errorf("total: got %d", resp.Total)
	}
	if resp.Succeeded != 3 {
		t.Errorf("succeeded: got %d", resp.Succeeded)
	}
	if resp.Failed != 0 {
		t.Errorf("failed: got %d", resp.Failed)
	}
}

func TestBatchEmptyItems(t *testing.T) {
	h, _, _ := newTestNotifyHandler()
	body := `{"items":[]}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.SendBatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

func TestBatchTooLarge(t *testing.T) {
	h, _, _ := newTestNotifyHandler()
	items := make([]BatchItem, 101)
	for i := range items {
		items[i] = BatchItem{Type: "otp", Channel: "email", Recipient: "a@b.com", OTPCode: "123"}
	}
	data, _ := json.Marshal(BatchRequest{Items: items})
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(data)))
	rr := httptest.NewRecorder()
	h.SendBatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

func TestBatchExactMaxSize(t *testing.T) {
	h, _, _ := newTestNotifyHandler()
	items := make([]BatchItem, 100)
	for i := range items {
		items[i] = BatchItem{Type: "otp", Channel: "email", Recipient: "a@b.com", OTPCode: "123"}
	}
	data, _ := json.Marshal(BatchRequest{Items: items})
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(data)))
	rr := httptest.NewRecorder()
	h.SendBatch(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rr.Code)
	}
}

func TestBatchInvalidType(t *testing.T) {
	h, _, _ := newTestNotifyHandler()
	body := `{"items":[{"type":"push","channel":"email","recipient":"a@b.com"}]}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.SendBatch(rr, req)

	var resp BatchResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Failed != 1 {
		t.Errorf("failed: got %d, want 1", resp.Failed)
	}
	if resp.Results[0].Error != "invalid_type" {
		t.Errorf("error: got %q", resp.Results[0].Error)
	}
}

func TestBatchMixedResults(t *testing.T) {
	h, _, _ := newTestNotifyHandler()
	body := `{"items":[
		{"type":"otp","channel":"email","recipient":"a@b.com","otp_code":"111"},
		{"type":"invalid","channel":"email","recipient":"a@b.com"},
		{"type":"alert","channel":"email","recipient":"admin@b.com","subject":"Alert","message":"Test"}
	]}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.SendBatch(rr, req)

	var resp BatchResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Total != 3 {
		t.Errorf("total: got %d", resp.Total)
	}
	if resp.Succeeded != 2 {
		t.Errorf("succeeded: got %d", resp.Succeeded)
	}
	if resp.Failed != 1 {
		t.Errorf("failed: got %d", resp.Failed)
	}
	if resp.Results[0].Status != "sent" {
		t.Errorf("item 0: got %q", resp.Results[0].Status)
	}
	if resp.Results[1].Status != "failed" {
		t.Errorf("item 1: got %q", resp.Results[1].Status)
	}
	if resp.Results[2].Status != "sent" {
		t.Errorf("item 2: got %q", resp.Results[2].Status)
	}
}

func TestBatchInvalidJSON(t *testing.T) {
	h, _, _ := newTestNotifyHandler()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad"))
	rr := httptest.NewRecorder()
	h.SendBatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rr.Code)
	}
}

func TestBatchResultIndices(t *testing.T) {
	h, _, _ := newTestNotifyHandler()
	body := `{"items":[
		{"type":"otp","channel":"email","recipient":"a@b.com","otp_code":"1"},
		{"type":"otp","channel":"email","recipient":"b@b.com","otp_code":"2"},
		{"type":"otp","channel":"email","recipient":"c@b.com","otp_code":"3"}
	]}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.SendBatch(rr, req)

	var resp BatchResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	for i, r := range resp.Results {
		if r.Index != i {
			t.Errorf("result %d: index=%d", i, r.Index)
		}
	}
}

func TestBatchAlertInvalidChannel(t *testing.T) {
	h, _, _ := newTestNotifyHandler()
	body := `{"items":[{"type":"alert","channel":"sms","recipient":"+62123","subject":"Alert","message":"Test"}]}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.SendBatch(rr, req)

	var resp BatchResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Failed != 1 {
		t.Errorf("failed: got %d, want 1", resp.Failed)
	}
}

func TestBatchOTPInvalidChannel(t *testing.T) {
	h, _, _ := newTestNotifyHandler()
	body := `{"items":[{"type":"otp","channel":"webhook","recipient":"url","otp_code":"123"}]}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.SendBatch(rr, req)

	var resp BatchResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Failed != 1 {
		t.Errorf("failed: got %d", resp.Failed)
	}
}

func TestBatchResponseHeaders(t *testing.T) {
	h, _, _ := newTestNotifyHandler()
	body := `{"items":[{"type":"otp","channel":"email","recipient":"a@b.com","otp_code":"1"}]}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.SendBatch(rr, req)

	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type: %q", ct)
	}
}
