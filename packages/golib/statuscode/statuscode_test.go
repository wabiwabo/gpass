package statuscode

import "testing"

func TestString(t *testing.T) {
	tests := []struct {
		code Code
		want string
	}{
		{OK, "ok"},
		{InvalidInput, "invalid_input"},
		{NotFound, "not_found"},
		{ConsentRequired, "consent_required"},
		{NIKInvalid, "nik_invalid"},
		{Internal, "internal"},
		{Code(9999), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.code.String(); got != tt.want {
			t.Errorf("(%d).String() = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestIsOK(t *testing.T) {
	if !OK.IsOK() { t.Error("OK") }
	if InvalidInput.IsOK() { t.Error("InvalidInput") }
}

func TestIsClientError(t *testing.T) {
	if !InvalidInput.IsClientError() { t.Error("InvalidInput") }
	if !NotFound.IsClientError() { t.Error("NotFound") }
	if Internal.IsClientError() { t.Error("Internal") }
	if OK.IsClientError() { t.Error("OK") }
}

func TestIsServerError(t *testing.T) {
	if !Internal.IsServerError() { t.Error("Internal") }
	if !Unavailable.IsServerError() { t.Error("Unavailable") }
	if InvalidInput.IsServerError() { t.Error("InvalidInput") }
}

func TestIsDomainError(t *testing.T) {
	if !ConsentRequired.IsDomainError() { t.Error("ConsentRequired") }
	if !NIKInvalid.IsDomainError() { t.Error("NIKInvalid") }
	if InvalidInput.IsDomainError() { t.Error("InvalidInput") }
}

func TestIsRetryable(t *testing.T) {
	retryable := []Code{Internal, Unavailable, Timeout, ResourceExhausted}
	for _, c := range retryable {
		if !c.IsRetryable() { t.Errorf("%v should be retryable", c) }
	}
	notRetryable := []Code{InvalidInput, NotFound, PermissionDenied}
	for _, c := range notRetryable {
		if c.IsRetryable() { t.Errorf("%v should not be retryable", c) }
	}
}

func TestHTTPStatus(t *testing.T) {
	tests := []struct {
		code   Code
		status int
	}{
		{OK, 200},
		{InvalidInput, 400},
		{Unauthenticated, 401},
		{PermissionDenied, 403},
		{NotFound, 404},
		{AlreadyExists, 409},
		{ResourceExhausted, 429},
		{NIKInvalid, 422},
		{Internal, 500},
		{Unavailable, 503},
		{Timeout, 504},
		{ConsentRequired, 403},
	}
	for _, tt := range tests {
		if got := tt.code.HTTPStatus(); got != tt.status {
			t.Errorf("(%v).HTTPStatus() = %d, want %d", tt.code, got, tt.status)
		}
	}
}

func TestAllDomainCodes(t *testing.T) {
	domain := []Code{ConsentRequired, ConsentExpired, NIKInvalid, NPWPInvalid, NIBInvalid, CertRevoked, SignatureInvalid}
	for _, c := range domain {
		if !c.IsDomainError() { t.Errorf("%v should be domain", c) }
		if c.String() == "unknown" { t.Errorf("%v has no string", c) }
	}
}
