package httpstatus

import "testing"

// TestDescription_AllMappedCodes pins the machine-readable strings for
// every explicitly-mapped status. These strings are part of the public
// contract — clients (and OpenAPI generators) parse them as enum values
// so even a typo correction is a breaking change.
func TestDescription_AllMappedCodes(t *testing.T) {
	cases := map[int]string{
		200: "ok",
		201: "created",
		204: "no_content",
		301: "moved_permanently",
		302: "found",
		304: "not_modified",
		400: "bad_request",
		401: "unauthorized",
		403: "forbidden",
		404: "not_found",
		405: "method_not_allowed",
		408: "request_timeout",
		409: "conflict",
		422: "unprocessable_entity",
		429: "too_many_requests",
		500: "internal_server_error",
		502: "bad_gateway",
		503: "service_unavailable",
		504: "gateway_timeout",
	}
	for code, want := range cases {
		if got := Description(code); got != want {
			t.Errorf("Description(%d) = %q, want %q", code, got, want)
		}
	}
}

// TestDescription_UnmappedFallsBackToCategory covers the default branch:
// codes without an explicit mapping return the bucket name from Category.
func TestDescription_UnmappedFallsBackToCategory(t *testing.T) {
	cases := map[int]string{
		418: "client_error", // I'm a teapot
		451: "client_error", // Unavailable for legal reasons
		507: "server_error", // Insufficient storage
		100: "informational",
		206: "success",
		308: "redirection",
	}
	for code, want := range cases {
		if got := Description(code); got != want {
			t.Errorf("Description(%d) = %q, want %q (Category fallback)", code, got, want)
		}
	}
}
