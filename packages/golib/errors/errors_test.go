package errors

import (
	"net/http"
	"testing"
)

func TestBadRequest(t *testing.T) {
	err := BadRequest(CodeInvalidRequest, "name is required")
	if err.HTTPStatus != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", err.HTTPStatus)
	}
	if err.Code != CodeInvalidRequest {
		t.Errorf("expected %s, got %s", CodeInvalidRequest, err.Code)
	}
	if err.Error() != "invalid_request: name is required" {
		t.Errorf("unexpected error string: %s", err.Error())
	}
}

func TestNotFound(t *testing.T) {
	err := NotFound(CodeResourceNotFound, "user not found")
	if err.HTTPStatus != http.StatusNotFound {
		t.Errorf("expected 404, got %d", err.HTTPStatus)
	}
}

func TestConflict(t *testing.T) {
	err := Conflict(CodeAlreadyExists, "entity already registered")
	if err.HTTPStatus != http.StatusConflict {
		t.Errorf("expected 409, got %d", err.HTTPStatus)
	}
}

func TestForbidden(t *testing.T) {
	err := Forbidden(CodeNotOwner, "not the owner of this resource")
	if err.HTTPStatus != http.StatusForbidden {
		t.Errorf("expected 403, got %d", err.HTTPStatus)
	}
}

func TestTooManyRequests(t *testing.T) {
	err := TooManyRequests(CodeRateLimitExceeded, "rate limit exceeded")
	if err.HTTPStatus != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", err.HTTPStatus)
	}
}

func TestBadGateway(t *testing.T) {
	err := BadGateway(CodeUpstreamError, "signing service unavailable")
	if err.HTTPStatus != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", err.HTTPStatus)
	}
}

func TestWithDetails(t *testing.T) {
	details := map[string]string{"field": "nik", "reason": "must be 16 digits"}
	err := BadRequest(CodeInvalidNIK, "invalid NIK format").WithDetails(details)

	if err.Details == nil {
		t.Error("details should not be nil")
	}
	d := err.Details.(map[string]string)
	if d["field"] != "nik" {
		t.Errorf("expected field=nik, got %s", d["field"])
	}
}

func TestAllStatusCodes(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(string, string) *AppError
		expected int
	}{
		{"BadRequest", BadRequest, 400},
		{"Unauthorized", Unauthorized, 401},
		{"Forbidden", Forbidden, 403},
		{"NotFound", NotFound, 404},
		{"Conflict", Conflict, 409},
		{"Gone", Gone, 410},
		{"TooLarge", TooLarge, 413},
		{"TooManyRequests", TooManyRequests, 429},
		{"Internal", Internal, 500},
		{"BadGateway", BadGateway, 502},
		{"ServiceUnavailable", ServiceUnavailable, 503},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fn("code", "message")
			if err.HTTPStatus != tc.expected {
				t.Errorf("expected %d, got %d", tc.expected, err.HTTPStatus)
			}
		})
	}
}

func TestErrorImplementsError(t *testing.T) {
	var err error = BadRequest("test", "test message")
	if err.Error() != "test: test message" {
		t.Errorf("unexpected: %s", err.Error())
	}
}
