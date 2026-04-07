package statuscode

import "testing"

// TestString_AllCodes pins every name in the public Code enum.
// These strings cross service boundaries (logged, returned in JSON
// envelopes, displayed in admin UIs) so a "harmless rename" would be a
// silent breaking change for downstream consumers.
func TestString_AllCodes(t *testing.T) {
	cases := map[Code]string{
		OK:                 "ok",
		InvalidInput:       "invalid_input",
		NotFound:           "not_found",
		AlreadyExists:      "already_exists",
		PermissionDenied:   "permission_denied",
		Unauthenticated:    "unauthenticated",
		ResourceExhausted:  "resource_exhausted",
		FailedPrecondition: "failed_precondition",
		Aborted:            "aborted",
		Internal:           "internal",
		Unavailable:        "unavailable",
		DataLoss:           "data_loss",
		Timeout:            "timeout",
		ConsentRequired:    "consent_required",
		ConsentExpired:     "consent_expired",
		NIKInvalid:         "nik_invalid",
		NPWPInvalid:        "npwp_invalid",
		NIBInvalid:         "nib_invalid",
		CertRevoked:        "cert_revoked",
		SignatureInvalid:   "signature_invalid",
	}
	for code, want := range cases {
		if got := code.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", int(code), got, want)
		}
	}

	if got := Code(99999).String(); got != "unknown" {
		t.Errorf("unmapped code = %q, want %q", got, "unknown")
	}
}

// TestHTTPStatus_AllCodes pins the HTTP-status mapping for every defined
// Code, including the unmapped-default branch (must be 500).
func TestHTTPStatus_AllCodes(t *testing.T) {
	cases := map[Code]int{
		OK:                 200,
		InvalidInput:       400,
		Unauthenticated:    401,
		PermissionDenied:   403,
		ConsentRequired:    403,
		ConsentExpired:     403,
		NotFound:           404,
		AlreadyExists:      409,
		Aborted:            409,
		FailedPrecondition: 422,
		NIKInvalid:         422,
		NPWPInvalid:        422,
		NIBInvalid:         422,
		CertRevoked:        422,
		SignatureInvalid:   422,
		ResourceExhausted:  429,
		Internal:           500,
		DataLoss:           500,
		Unavailable:        503,
		Timeout:            504,
	}
	for code, want := range cases {
		if got := code.HTTPStatus(); got != want {
			t.Errorf("Code(%d=%s).HTTPStatus() = %d, want %d",
				int(code), code.String(), got, want)
		}
	}

	if got := Code(99999).HTTPStatus(); got != 500 {
		t.Errorf("unmapped code HTTPStatus = %d, want 500 (default)", got)
	}
}

// TestCategoryHelpers covers IsClientError/IsServerError/IsDomainError/
// IsRetryable invariants on representative codes from each band.
func TestCategoryHelpers(t *testing.T) {
	if !OK.IsOK() || InvalidInput.IsOK() {
		t.Error("IsOK")
	}
	if !InvalidInput.IsClientError() || Internal.IsClientError() {
		t.Error("IsClientError")
	}
	if !Internal.IsServerError() || NIKInvalid.IsServerError() {
		t.Error("IsServerError")
	}
	if !NIKInvalid.IsDomainError() || Internal.IsDomainError() {
		t.Error("IsDomainError")
	}
	for _, c := range []Code{Internal, Unavailable, Timeout, ResourceExhausted} {
		if !c.IsRetryable() {
			t.Errorf("%s should be retryable", c)
		}
	}
	for _, c := range []Code{InvalidInput, NotFound, NIKInvalid} {
		if c.IsRetryable() {
			t.Errorf("%s should NOT be retryable", c)
		}
	}
}
