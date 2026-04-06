package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/identity/dukcapil"
	"github.com/garudapass/gpass/services/identity/store"
)

// --- Helpers ---

func newEdgeRegisterHandler(dukcapilFn func(ctx context.Context, nik string) (*dukcapil.NIKVerifyResponse, error), otpMock *mockOTP) *RegisterHandler {
	if dukcapilFn == nil {
		dukcapilFn = func(ctx context.Context, nik string) (*dukcapil.NIKVerifyResponse, error) {
			return &dukcapil.NIKVerifyResponse{Valid: true, Alive: true, Name: "Test"}, nil
		}
	}
	if otpMock == nil {
		otpMock = &mockOTP{}
	}
	return NewRegisterHandler(RegisterDeps{
		Dukcapil: &mockDukcapil{verifyNIKFn: dukcapilFn},
		OTP:      otpMock,
		NIKKey:   []byte("01234567890123456789012345678901"),
	})
}

func decodeErrorResp(t *testing.T, w *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	return resp
}

// --- NIK Validation Edge Cases ---

func TestInitiate_NIKWithNonNumericCharacters(t *testing.T) {
	h := newEdgeRegisterHandler(nil, nil)

	cases := []struct {
		name string
		nik  string
	}{
		{"letters", "320123456789abcd"},
		{"special_chars", "3201234567890!@#"},
		{"spaces", "3201 34567890001"},
		{"unicode", "320123456789\u00e9\u00e9\u00e9\u00e9"},
		{"mixed_alpha", "32O1234567890001"}, // letter O instead of zero
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"nik":%q,"phone":"+62812345678","email":"test@example.com"}`, tc.nik)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/register/initiate", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.Initiate(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d for NIK %q", w.Code, http.StatusBadRequest, tc.nik)
			}
			resp := decodeErrorResp(t, w)
			if resp["error"] != "invalid_nik" {
				t.Errorf("error = %q, want %q", resp["error"], "invalid_nik")
			}
		})
	}
}

func TestInitiate_NIKBoundaryLengths(t *testing.T) {
	h := newEdgeRegisterHandler(nil, nil)

	cases := []struct {
		name string
		nik  string
	}{
		{"15_digits", "320123456789000"},
		{"17_digits", "32012345678900012"},
		{"empty", ""},
		{"one_digit", "3"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"nik":%q,"phone":"+62812345678","email":"test@example.com"}`, tc.nik)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/register/initiate", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.Initiate(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d for NIK %q (len=%d)", w.Code, http.StatusBadRequest, tc.nik, len(tc.nik))
			}
			resp := decodeErrorResp(t, w)
			if resp["error"] != "invalid_nik" {
				t.Errorf("error = %q, want %q", resp["error"], "invalid_nik")
			}
		})
	}
}

// --- OTP Edge Cases ---

func TestInitiate_OTPGenerationFailure(t *testing.T) {
	otpMock := &mockOTP{
		generateFn: func(ctx context.Context, registrationID, channel string) (string, error) {
			return "", fmt.Errorf("redis connection refused")
		},
	}
	h := newEdgeRegisterHandler(nil, otpMock)

	body := `{"nik":"3201234567890001","phone":"+62812345678","email":"test@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register/initiate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Initiate(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
	resp := decodeErrorResp(t, w)
	if resp["error"] != "internal_error" {
		t.Errorf("error = %q, want %q", resp["error"], "internal_error")
	}
}

func TestInitiate_SecondOTPChannelFails(t *testing.T) {
	callCount := 0
	otpMock := &mockOTP{
		generateFn: func(ctx context.Context, registrationID, channel string) (string, error) {
			callCount++
			if callCount == 2 {
				return "", fmt.Errorf("email service down")
			}
			return "123456", nil
		},
	}
	h := newEdgeRegisterHandler(nil, otpMock)

	body := `{"nik":"3201234567890001","phone":"+62812345678","email":"test@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register/initiate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Initiate(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// --- Registration with Invalid/Malformed Input ---

func TestInitiate_InvalidJSON(t *testing.T) {
	h := newEdgeRegisterHandler(nil, nil)

	cases := []struct {
		name string
		body string
	}{
		{"not_json", "not json at all"},
		{"truncated_json", `{"nik":"320123`},
		{"xml_body", `<nik>3201234567890001</nik>`},
		{"empty_body", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/register/initiate", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.Initiate(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d for body %q", w.Code, http.StatusBadRequest, tc.body)
			}
			resp := decodeErrorResp(t, w)
			if resp["error"] != "invalid_request" {
				t.Errorf("error = %q, want %q", resp["error"], "invalid_request")
			}
		})
	}
}

func TestInitiate_MissingRequiredFields(t *testing.T) {
	// NIK is the critical validated field. Missing NIK means empty string => validation fails.
	h := newEdgeRegisterHandler(nil, nil)

	cases := []struct {
		name string
		body string
	}{
		{"missing_nik", `{"phone":"+62812345678","email":"test@example.com"}`},
		{"empty_nik", `{"nik":"","phone":"+62812345678","email":"test@example.com"}`},
		{"null_nik", `{"nik":null,"phone":"+62812345678","email":"test@example.com"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/register/initiate", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.Initiate(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d for case %q", w.Code, http.StatusBadRequest, tc.name)
			}
			resp := decodeErrorResp(t, w)
			if resp["error"] != "invalid_nik" {
				t.Errorf("error = %q, want %q", resp["error"], "invalid_nik")
			}
		})
	}
}

func TestInitiate_DukcapilReturnsInvalidNIK(t *testing.T) {
	h := newEdgeRegisterHandler(
		func(ctx context.Context, nik string) (*dukcapil.NIKVerifyResponse, error) {
			return &dukcapil.NIKVerifyResponse{Valid: false, Alive: true}, nil
		},
		nil,
	)

	body := `{"nik":"3201234567890001","phone":"+62812345678","email":"test@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register/initiate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Initiate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	resp := decodeErrorResp(t, w)
	if resp["error"] != "invalid_nik" {
		t.Errorf("error = %q, want %q", resp["error"], "invalid_nik")
	}
}

// --- Request Body Size / Content-Type Edge Cases ---

func TestInitiate_OversizedBody(t *testing.T) {
	h := newEdgeRegisterHandler(nil, nil)

	// Create a body with a very large NIK field value (1MB of data).
	largeValue := strings.Repeat("1", 1_000_000)
	body := fmt.Sprintf(`{"nik":%q,"phone":"+62812345678","email":"test@example.com"}`, largeValue)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register/initiate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Initiate(w, req)

	// The handler should reject it at NIK validation since it's not 16 digits.
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d for oversized body", w.Code, http.StatusBadRequest)
	}
}

func TestInitiate_ResponseHeaders(t *testing.T) {
	h := newEdgeRegisterHandler(nil, nil)

	// Even error responses must have correct security headers.
	body := `{"nik":"invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register/initiate", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.Initiate(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json; charset=utf-8")
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", cc, "no-store")
	}
}

// --- Deletion Edge Cases ---

func TestRequestDeletion_MultipleDeletionsForSameUser(t *testing.T) {
	h, _, ae := newTestDeletionHandler()

	// First deletion request.
	body := `{"reason":"user_request"}`
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/identity/deletion", strings.NewReader(body))
	req1.Header.Set("X-User-ID", "user-001")
	w1 := httptest.NewRecorder()
	h.RequestDeletion(w1, req1)

	if w1.Code != http.StatusAccepted {
		t.Fatalf("first request: status = %d, want %d", w1.Code, http.StatusAccepted)
	}

	var resp1 deletionResponse
	json.NewDecoder(w1.Body).Decode(&resp1)

	// Second deletion request for the same user (idempotency check).
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/identity/deletion", strings.NewReader(body))
	req2.Header.Set("X-User-ID", "user-001")
	w2 := httptest.NewRecorder()
	h.RequestDeletion(w2, req2)

	if w2.Code != http.StatusAccepted {
		t.Fatalf("second request: status = %d, want %d", w2.Code, http.StatusAccepted)
	}

	var resp2 deletionResponse
	json.NewDecoder(w2.Body).Decode(&resp2)

	// Both should succeed and produce different deletion request IDs.
	if resp1.ID == resp2.ID {
		t.Error("two deletion requests for the same user should produce different IDs")
	}

	// Both should emit audit events.
	if len(ae.events) != 2 {
		t.Errorf("audit events = %d, want 2", len(ae.events))
	}
}

func TestRequestDeletion_EmptyBody(t *testing.T) {
	h, _, _ := newTestDeletionHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/deletion", strings.NewReader("{}"))
	req.Header.Set("X-User-ID", "user-001")
	w := httptest.NewRecorder()

	h.RequestDeletion(w, req)

	// Empty reason is not in ValidReasons, so should be rejected.
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	resp := decodeErrorResp(t, w)
	if resp["error"] != "invalid_reason" {
		t.Errorf("error = %q, want %q", resp["error"], "invalid_reason")
	}
}

func TestGetDeletionStatus_MissingPathID(t *testing.T) {
	h, _, _ := newTestDeletionHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/deletion/", nil)
	req.Header.Set("X-User-ID", "user-001")
	// Do NOT set PathValue("id") to simulate missing path parameter.
	w := httptest.NewRecorder()

	h.GetDeletionStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	resp := decodeErrorResp(t, w)
	if resp["error"] != "missing_id" {
		t.Errorf("error = %q, want %q", resp["error"], "missing_id")
	}
}

// --- Export Edge Cases ---

func TestExportData_NonExistentUser(t *testing.T) {
	h := NewExportHandler()

	// The current ExportHandler returns placeholder data for any valid user ID.
	// This test verifies it handles a non-existent user gracefully (returns data
	// with the supplied user ID rather than erroring).
	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/export", nil)
	req.Header.Set("X-User-ID", "nonexistent-user-999")
	w := httptest.NewRecorder()

	h.ExportData(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp exportResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.PersonalData.UserID != "nonexistent-user-999" {
		t.Errorf("UserID = %q, want %q", resp.PersonalData.UserID, "nonexistent-user-999")
	}
}

func TestExportData_ResponseSecurityHeaders(t *testing.T) {
	h := NewExportHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/export", nil)
	req.Header.Set("X-User-ID", "user-001")
	w := httptest.NewRecorder()

	h.ExportData(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json; charset=utf-8")
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", cc, "no-store")
	}
}

// --- Concurrent Request Edge Cases ---

func TestInitiate_ConcurrentRegistrations(t *testing.T) {
	h := newEdgeRegisterHandler(nil, nil)

	const goroutines = 10
	var wg sync.WaitGroup
	results := make([]int, goroutines)
	regIDs := make([]string, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			body := `{"nik":"3201234567890001","phone":"+62812345678","email":"test@example.com"}`
			req := httptest.NewRequest(http.MethodPost, "/api/v1/register/initiate", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.Initiate(w, req)

			results[idx] = w.Code
			if w.Code == http.StatusOK {
				var resp initiateResponse
				json.NewDecoder(w.Body).Decode(&resp)
				regIDs[idx] = resp.RegistrationID
			}
		}(i)
	}
	wg.Wait()

	// All concurrent requests should succeed.
	for i, code := range results {
		if code != http.StatusOK {
			t.Errorf("goroutine %d: status = %d, want %d", i, code, http.StatusOK)
		}
	}

	// All registration IDs must be unique.
	seen := make(map[string]bool)
	for i, id := range regIDs {
		if id == "" {
			continue
		}
		if seen[id] {
			t.Errorf("goroutine %d: duplicate registration_id %q", i, id)
		}
		seen[id] = true
	}
}

// safeAuditEmitter is a thread-safe mock audit emitter for concurrent tests.
type safeAuditEmitter struct {
	mu     sync.Mutex
	events []auditEvent
}

func (m *safeAuditEmitter) Emit(eventType, userID, resourceID string, metadata map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, auditEvent{
		eventType:  eventType,
		userID:     userID,
		resourceID: resourceID,
		metadata:   metadata,
	})
	return nil
}

func TestRequestDeletion_ConcurrentDeletions(t *testing.T) {
	s := store.NewInMemoryDeletionStore()
	ae := &safeAuditEmitter{}
	h := NewDeletionHandler(s, ae)

	const goroutines = 10
	var wg sync.WaitGroup
	results := make([]int, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			body := `{"reason":"user_request"}`
			req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/deletion", strings.NewReader(body))
			req.Header.Set("X-User-ID", "user-concurrent")
			w := httptest.NewRecorder()

			h.RequestDeletion(w, req)
			results[idx] = w.Code
		}(i)
	}
	wg.Wait()

	for i, code := range results {
		if code != http.StatusAccepted {
			t.Errorf("goroutine %d: status = %d, want %d", i, code, http.StatusAccepted)
		}
	}

	// Verify all were stored.
	requests, err := s.ListByUser("user-concurrent")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(requests) != goroutines {
		t.Errorf("stored requests = %d, want %d", len(requests), goroutines)
	}
}

// --- Session Management Edge Cases ---

func TestRevokeSession_AlreadyRevoked(t *testing.T) {
	mgr, sessionStore := newTestSessionManager()
	now := time.Now()

	sessionStore.Add(&Session{ID: "s1", UserID: "user-1", DeviceInfo: "Chrome", CreatedAt: now})

	// First revoke should succeed.
	req1 := httptest.NewRequest(http.MethodDelete, "/api/v1/identity/sessions/s1", nil)
	req1.Header.Set("X-User-ID", "user-1")
	w1 := httptest.NewRecorder()
	mgr.RevokeSession(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("first revoke: status = %d, want %d", w1.Code, http.StatusOK)
	}

	// Second revoke of same session should return not found.
	req2 := httptest.NewRequest(http.MethodDelete, "/api/v1/identity/sessions/s1", nil)
	req2.Header.Set("X-User-ID", "user-1")
	w2 := httptest.NewRecorder()
	mgr.RevokeSession(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Fatalf("second revoke: status = %d, want %d", w2.Code, http.StatusNotFound)
	}
}

func TestRevokeAllSessions_NoSessions(t *testing.T) {
	mgr, _ := newTestSessionManager()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/identity/sessions", nil)
	req.Header.Set("X-User-ID", "user-with-no-sessions")
	w := httptest.NewRecorder()

	mgr.RevokeAllSessions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if revoked, ok := resp["revoked"].(float64); !ok || int(revoked) != 0 {
		t.Errorf("revoked = %v, want 0", resp["revoked"])
	}
}

func TestListSessions_EmptyList(t *testing.T) {
	mgr, _ := newTestSessionManager()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/sessions", nil)
	req.Header.Set("X-User-ID", "user-with-no-sessions")
	w := httptest.NewRecorder()

	mgr.ListSessions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var sessions []Session
	if err := json.NewDecoder(w.Body).Decode(&sessions); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("sessions count = %d, want 0", len(sessions))
	}
}

// --- NIK Province Code Edge Cases ---

func TestInitiate_NIKInvalidProvinceCode(t *testing.T) {
	h := newEdgeRegisterHandler(nil, nil)

	cases := []struct {
		name string
		nik  string
	}{
		{"province_00", "0001234567890001"},
		{"province_10", "1001234567890001"},
		{"province_01", "0101234567890001"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"nik":%q,"phone":"+62812345678","email":"test@example.com"}`, tc.nik)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/register/initiate", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.Initiate(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d for province %q", w.Code, http.StatusBadRequest, tc.nik[:2])
			}
			resp := decodeErrorResp(t, w)
			if resp["error"] != "invalid_nik" {
				t.Errorf("error = %q, want %q", resp["error"], "invalid_nik")
			}
		})
	}
}

// --- Deletion Validation Combinations ---

func TestRequestDeletion_AllValidReasons(t *testing.T) {
	validReasons := []string{"user_request", "consent_revocation", "retention_expired"}

	for _, reason := range validReasons {
		t.Run(reason, func(t *testing.T) {
			h, _, _ := newTestDeletionHandler()

			body := fmt.Sprintf(`{"reason":%q}`, reason)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/deletion", strings.NewReader(body))
			req.Header.Set("X-User-ID", "user-001")
			w := httptest.NewRecorder()

			h.RequestDeletion(w, req)

			if w.Code != http.StatusAccepted {
				t.Errorf("status = %d, want %d for reason %q", w.Code, http.StatusAccepted, reason)
			}
		})
	}
}

// --- Oversized JSON fields for deletion ---

func TestRequestDeletion_OversizedReason(t *testing.T) {
	h, _, _ := newTestDeletionHandler()

	// Even a huge reason string is just treated as invalid (not in ValidReasons).
	largeReason := strings.Repeat("x", 100_000)
	body := fmt.Sprintf(`{"reason":%q}`, largeReason)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/identity/deletion", strings.NewReader(body))
	req.Header.Set("X-User-ID", "user-001")
	w := httptest.NewRecorder()

	h.RequestDeletion(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d for oversized reason", w.Code, http.StatusBadRequest)
	}
	resp := decodeErrorResp(t, w)
	if resp["error"] != "invalid_reason" {
		t.Errorf("error = %q, want %q", resp["error"], "invalid_reason")
	}
}

// --- Verify NIK with exactly 16 zeroes (valid format but province 00) ---

func TestInitiate_AllZeroNIK(t *testing.T) {
	h := newEdgeRegisterHandler(nil, nil)

	body := `{"nik":"0000000000000000","phone":"+62812345678","email":"test@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register/initiate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Initiate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d for all-zero NIK", w.Code, http.StatusBadRequest)
	}
}

// --- Large batch of export with different user IDs ---

func TestExportData_DifferentUserIDsReturnCorrectData(t *testing.T) {
	h := NewExportHandler()

	userIDs := []string{"user-aaa", "user-bbb", "user-ccc"}
	for _, uid := range userIDs {
		t.Run(uid, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/export", nil)
			req.Header.Set("X-User-ID", uid)
			w := httptest.NewRecorder()

			h.ExportData(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
			}

			var resp exportResponse
			json.NewDecoder(w.Body).Decode(&resp)
			if resp.PersonalData.UserID != uid {
				t.Errorf("UserID = %q, want %q", resp.PersonalData.UserID, uid)
			}
		})
	}
}

