package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/identity/dukcapil"
)

// --- Mocks ---

type mockDukcapil struct {
	verifyNIKFn func(ctx context.Context, nik string) (*dukcapil.NIKVerifyResponse, error)
}

func (m *mockDukcapil) VerifyNIK(ctx context.Context, nik string) (*dukcapil.NIKVerifyResponse, error) {
	return m.verifyNIKFn(ctx, nik)
}

func (m *mockDukcapil) VerifyBiometric(ctx context.Context, nik, selfieB64 string) (*dukcapil.BiometricResponse, error) {
	return nil, nil
}

func (m *mockDukcapil) VerifyDemographic(ctx context.Context, req *dukcapil.DemographicRequest) (*dukcapil.DemographicResponse, error) {
	return nil, nil
}

type mockOTP struct {
	generateFn func(ctx context.Context, registrationID, channel string) (string, error)
}

func (m *mockOTP) Generate(ctx context.Context, registrationID, channel string) (string, error) {
	if m.generateFn != nil {
		return m.generateFn(ctx, registrationID, channel)
	}
	return "123456", nil
}

func (m *mockOTP) Verify(ctx context.Context, registrationID, channel, code string) error {
	return nil
}

// --- Tests ---

func TestInitiate_Success(t *testing.T) {
	h := NewRegisterHandler(RegisterDeps{
		Dukcapil: &mockDukcapil{
			verifyNIKFn: func(ctx context.Context, nik string) (*dukcapil.NIKVerifyResponse, error) {
				return &dukcapil.NIKVerifyResponse{Valid: true, Alive: true, Name: "Budi"}, nil
			},
		},
		OTP:    &mockOTP{},
		NIKKey: []byte("01234567890123456789012345678901"),
	})

	body := `{"nik":"3201234567890001","phone":"+62812345678","email":"budi@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register/initiate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Initiate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp initiateResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.RegistrationID == "" {
		t.Error("registration_id should not be empty")
	}
	if resp.OTPExpiresAt.IsZero() {
		t.Error("otp_expires_at should not be zero")
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestInitiate_InvalidNIK(t *testing.T) {
	h := NewRegisterHandler(RegisterDeps{
		Dukcapil: &mockDukcapil{
			verifyNIKFn: func(ctx context.Context, nik string) (*dukcapil.NIKVerifyResponse, error) {
				return &dukcapil.NIKVerifyResponse{Valid: true, Alive: true}, nil
			},
		},
		OTP:    &mockOTP{},
		NIKKey: []byte("01234567890123456789012345678901"),
	})

	body := `{"nik":"123","phone":"+62812345678","email":"budi@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register/initiate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.Initiate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestInitiate_Deceased(t *testing.T) {
	h := NewRegisterHandler(RegisterDeps{
		Dukcapil: &mockDukcapil{
			verifyNIKFn: func(ctx context.Context, nik string) (*dukcapil.NIKVerifyResponse, error) {
				return &dukcapil.NIKVerifyResponse{Valid: true, Alive: false}, nil
			},
		},
		OTP:    &mockOTP{},
		NIKKey: []byte("01234567890123456789012345678901"),
	})

	body := `{"nik":"3201234567890001","phone":"+62812345678","email":"budi@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register/initiate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.Initiate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "deceased" {
		t.Errorf("error = %q, want %q", errResp["error"], "deceased")
	}
}

func TestInitiate_DukcapilUnavailable(t *testing.T) {
	h := NewRegisterHandler(RegisterDeps{
		Dukcapil: &mockDukcapil{
			verifyNIKFn: func(ctx context.Context, nik string) (*dukcapil.NIKVerifyResponse, error) {
				return nil, fmt.Errorf("connection refused")
			},
		},
		OTP:    &mockOTP{},
		NIKKey: []byte("01234567890123456789012345678901"),
	})

	body := `{"nik":"3201234567890001","phone":"+62812345678","email":"budi@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register/initiate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.Initiate(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadGateway)
	}
}
