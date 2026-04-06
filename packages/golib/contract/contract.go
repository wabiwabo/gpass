package contract

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Contract defines an expected API behavior.
type Contract struct {
	Name            string
	Method          string
	Path            string
	Headers         map[string]string
	Body            string                 // JSON body to send
	ExpectedStatus  int
	ExpectedFields  []string               // JSON fields that must exist in response
	ExpectedValues  map[string]interface{} // exact value checks
	ForbiddenFields []string               // fields that must NOT exist
}

// Suite is a collection of contracts for a service.
type Suite struct {
	ServiceName string
	Contracts   []Contract
}

// NewSuite creates a contract suite for a service.
func NewSuite(serviceName string) *Suite {
	return &Suite{
		ServiceName: serviceName,
		Contracts:   []Contract{},
	}
}

// Add adds a contract to the suite.
func (s *Suite) Add(c Contract) *Suite {
	s.Contracts = append(s.Contracts, c)
	return s
}

// Verify runs all contracts against the given handler.
func (s *Suite) Verify(t *testing.T, handler http.Handler) {
	t.Helper()
	for _, c := range s.Contracts {
		t.Run(fmt.Sprintf("%s/%s", s.ServiceName, c.Name), func(t *testing.T) {
			VerifyContract(t, handler, c)
		})
	}
}

// VerifyContract runs a single contract against a handler.
func VerifyContract(t *testing.T, handler http.Handler, c Contract) {
	t.Helper()

	var body *strings.Reader
	if c.Body != "" {
		body = strings.NewReader(c.Body)
	} else {
		body = strings.NewReader("")
	}

	req := httptest.NewRequest(c.Method, c.Path, body)
	if c.Body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Check status code.
	if w.Code != c.ExpectedStatus {
		t.Errorf("contract %q: expected status %d, got %d (body: %s)",
			c.Name, c.ExpectedStatus, w.Code, w.Body.String())
		return
	}

	// Parse response body as JSON if we need to check fields.
	if len(c.ExpectedFields) == 0 && len(c.ExpectedValues) == 0 && len(c.ForbiddenFields) == 0 {
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Errorf("contract %q: failed to parse response JSON: %v", c.Name, err)
		return
	}

	// Check expected fields exist.
	for _, field := range c.ExpectedFields {
		if _, ok := result[field]; !ok {
			t.Errorf("contract %q: expected field %q missing from response", c.Name, field)
		}
	}

	// Check expected values.
	for field, expected := range c.ExpectedValues {
		actual, ok := result[field]
		if !ok {
			t.Errorf("contract %q: expected field %q missing from response", c.Name, field)
			continue
		}
		if fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", expected) {
			t.Errorf("contract %q: field %q expected %v, got %v", c.Name, field, expected, actual)
		}
	}

	// Check forbidden fields are absent.
	for _, field := range c.ForbiddenFields {
		if _, ok := result[field]; ok {
			t.Errorf("contract %q: forbidden field %q found in response", c.Name, field)
		}
	}
}

// GarudaPassContracts returns standard contracts for all GarudaPass services.
func GarudaPassContracts() map[string]*Suite {
	suites := make(map[string]*Suite)

	// Health endpoint — all services must respond with {"status":"ok"}.
	health := NewSuite("health")
	for _, svc := range []string{"bff", "identity", "garudainfo", "garudacorp", "garudasign", "garudaportal", "garudaaudit", "garudanotify"} {
		health.Add(Contract{
			Name:           fmt.Sprintf("%s-health", svc),
			Method:         http.MethodGet,
			Path:           "/health",
			ExpectedStatus: http.StatusOK,
			ExpectedValues: map[string]interface{}{
				"status": "ok",
			},
		})
	}
	suites["health"] = health

	// Identity service contracts.
	identity := NewSuite("identity")
	identity.Add(Contract{
		Name:           "register-missing-fields",
		Method:         http.MethodPost,
		Path:           "/api/v1/identity/register",
		Body:           `{}`,
		ExpectedStatus: http.StatusBadRequest,
		ExpectedFields: []string{"error"},
	})
	suites["identity"] = identity

	// Consent contracts.
	consent := NewSuite("consent")
	consent.Add(Contract{
		Name:           "grant-consent-created",
		Method:         http.MethodPost,
		Path:           "/api/v1/consent/grant",
		Body:           `{"user_id":"u1","service_id":"s1","scopes":["read"]}`,
		ExpectedStatus: http.StatusCreated,
		ExpectedFields: []string{"id"},
	})
	suites["consent"] = consent

	// Corporate service contracts.
	corporate := NewSuite("garudacorp")
	corporate.Add(Contract{
		Name:           "register-missing-sk-number",
		Method:         http.MethodPost,
		Path:           "/api/v1/corporate/register",
		Body:           `{"name":"Test Corp"}`,
		ExpectedStatus: http.StatusBadRequest,
		ExpectedFields: []string{"error"},
	})
	suites["garudacorp"] = corporate

	// Sign service contracts.
	sign := NewSuite("garudasign")
	sign.Add(Contract{
		Name:           "sign-certificate-missing-user-id",
		Method:         http.MethodPost,
		Path:           "/api/v1/sign/certificate",
		Body:           `{"document_id":"doc1"}`,
		ExpectedStatus: http.StatusBadRequest,
		ExpectedFields: []string{"error"},
	})
	suites["garudasign"] = sign

	// Portal service contracts.
	portal := NewSuite("garudaportal")
	portal.Add(Contract{
		Name:           "create-app-missing-user-id",
		Method:         http.MethodPost,
		Path:           "/api/v1/portal/apps",
		Body:           `{"name":"MyApp"}`,
		ExpectedStatus: http.StatusBadRequest,
		ExpectedFields: []string{"error"},
	})
	suites["garudaportal"] = portal

	// Audit service contracts.
	audit := NewSuite("garudaaudit")
	audit.Add(Contract{
		Name:           "ingest-missing-fields",
		Method:         http.MethodPost,
		Path:           "/api/v1/audit/events",
		Body:           `{"actor_id":"user-123"}`,
		ExpectedStatus: http.StatusBadRequest,
		ExpectedFields: []string{"error"},
	})
	suites["garudaaudit"] = audit

	return suites
}
