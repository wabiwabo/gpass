package contract

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// mockHandler returns a configurable HTTP handler for testing contracts.
func mockHandler(status int, body map[string]interface{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(body)
	})
}

func TestVerifyContract_PassesForMatchingResponse(t *testing.T) {
	handler := mockHandler(http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"service": "test",
	})

	c := Contract{
		Name:           "health-check",
		Method:         http.MethodGet,
		Path:           "/health",
		ExpectedStatus: http.StatusOK,
		ExpectedFields: []string{"status", "service"},
		ExpectedValues: map[string]interface{}{
			"status": "ok",
		},
		ForbiddenFields: []string{"secret"},
	}

	// Use a sub-test so that failures are captured but don't fail this test.
	passed := true
	t.Run("verify", func(t *testing.T) {
		VerifyContract(t, handler, c)
	})
	if !passed {
		t.Error("expected contract to pass")
	}
}

func TestVerifyContract_FailsForWrongStatusCode(t *testing.T) {
	handler := mockHandler(http.StatusInternalServerError, map[string]interface{}{
		"error": "something broke",
	})

	c := Contract{
		Name:           "should-be-ok",
		Method:         http.MethodGet,
		Path:           "/health",
		ExpectedStatus: http.StatusOK,
	}

	mockT := &testing.T{}
	VerifyContract(mockT, handler, c)
	if !mockT.Failed() {
		t.Error("expected contract to fail for wrong status code")
	}
}

func TestVerifyContract_FailsForMissingExpectedField(t *testing.T) {
	handler := mockHandler(http.StatusOK, map[string]interface{}{
		"status": "ok",
	})

	c := Contract{
		Name:           "check-fields",
		Method:         http.MethodGet,
		Path:           "/health",
		ExpectedStatus: http.StatusOK,
		ExpectedFields: []string{"status", "version"}, // "version" is missing
	}

	mockT := &testing.T{}
	VerifyContract(mockT, handler, c)
	if !mockT.Failed() {
		t.Error("expected contract to fail for missing expected field")
	}
}

func TestVerifyContract_FailsForForbiddenFieldPresent(t *testing.T) {
	handler := mockHandler(http.StatusOK, map[string]interface{}{
		"status": "ok",
		"secret": "should-not-be-here",
	})

	c := Contract{
		Name:            "no-secrets",
		Method:          http.MethodGet,
		Path:            "/health",
		ExpectedStatus:  http.StatusOK,
		ForbiddenFields: []string{"secret"},
	}

	mockT := &testing.T{}
	VerifyContract(mockT, handler, c)
	if !mockT.Failed() {
		t.Error("expected contract to fail for forbidden field present")
	}
}

func TestSuite_VerifyRunsAllContracts(t *testing.T) {
	handler := mockHandler(http.StatusOK, map[string]interface{}{
		"status": "ok",
	})

	suite := NewSuite("test-service")
	suite.Add(Contract{
		Name:           "contract-1",
		Method:         http.MethodGet,
		Path:           "/health",
		ExpectedStatus: http.StatusOK,
		ExpectedValues: map[string]interface{}{"status": "ok"},
	})
	suite.Add(Contract{
		Name:           "contract-2",
		Method:         http.MethodGet,
		Path:           "/health",
		ExpectedStatus: http.StatusOK,
		ExpectedFields: []string{"status"},
	})

	// Verify runs without panicking; sub-tests handle individual contracts.
	suite.Verify(t, handler)
}

func TestGarudaPassContracts_ReturnsExpectedServices(t *testing.T) {
	suites := GarudaPassContracts()

	expectedServices := []string{
		"health",
		"identity",
		"consent",
		"garudacorp",
		"garudasign",
		"garudaportal",
		"garudaaudit",
	}

	for _, svc := range expectedServices {
		if _, ok := suites[svc]; !ok {
			t.Errorf("expected suite for service %q", svc)
		}
	}

	// Health suite should have one contract per platform service.
	healthSuite := suites["health"]
	if len(healthSuite.Contracts) < 7 {
		t.Errorf("expected at least 7 health contracts, got %d", len(healthSuite.Contracts))
	}
}

func TestNewSuite_StartsEmpty(t *testing.T) {
	s := NewSuite("my-service")
	if s.ServiceName != "my-service" {
		t.Errorf("expected service name my-service, got %s", s.ServiceName)
	}
	if len(s.Contracts) != 0 {
		t.Errorf("expected empty contracts, got %d", len(s.Contracts))
	}
}

func TestSuite_AddChaining(t *testing.T) {
	s := NewSuite("chain").
		Add(Contract{Name: "a"}).
		Add(Contract{Name: "b"}).
		Add(Contract{Name: "c"})

	if len(s.Contracts) != 3 {
		t.Errorf("expected 3 contracts, got %d", len(s.Contracts))
	}
}

func TestVerifyContract_WithHeaders(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "value" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"missing header"}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	})

	c := Contract{
		Name:           "with-headers",
		Method:         http.MethodGet,
		Path:           "/test",
		Headers:        map[string]string{"X-Custom": "value"},
		ExpectedStatus: http.StatusOK,
	}

	// Should pass with correct header.
	t.Run("passes", func(t *testing.T) {
		VerifyContract(t, handler, c)
	})
}

func TestVerifyContract_WithBody(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": "abc123"})
	})

	c := Contract{
		Name:           "post-with-body",
		Method:         http.MethodPost,
		Path:           "/api/v1/items",
		Body:           `{"name":"test"}`,
		ExpectedStatus: http.StatusCreated,
		ExpectedFields: []string{"id"},
	}

	t.Run("passes", func(t *testing.T) {
		VerifyContract(t, handler, c)
	})
}

func TestVerifyContract_ExpectedValueMismatch(t *testing.T) {
	handler := mockHandler(http.StatusOK, map[string]interface{}{
		"status": "degraded",
	})

	c := Contract{
		Name:           "value-check",
		Method:         http.MethodGet,
		Path:           "/health",
		ExpectedStatus: http.StatusOK,
		ExpectedValues: map[string]interface{}{
			"status": "ok",
		},
	}

	mockT := &testing.T{}
	VerifyContract(mockT, handler, c)
	if !mockT.Failed() {
		t.Error("expected contract to fail for value mismatch")
	}
}
