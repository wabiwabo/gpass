package httpstatus

import "testing"

func TestIsSuccess(t *testing.T) {
	for _, code := range []int{200, 201, 204, 299} {
		if !IsSuccess(code) {
			t.Errorf("%d should be success", code)
		}
	}
	for _, code := range []int{100, 301, 400, 500} {
		if IsSuccess(code) {
			t.Errorf("%d should not be success", code)
		}
	}
}

func TestIsRedirect(t *testing.T) {
	if !IsRedirect(301) { t.Error("301") }
	if !IsRedirect(302) { t.Error("302") }
	if IsRedirect(200) { t.Error("200") }
}

func TestIsClientError(t *testing.T) {
	if !IsClientError(400) { t.Error("400") }
	if !IsClientError(404) { t.Error("404") }
	if !IsClientError(429) { t.Error("429") }
	if IsClientError(500) { t.Error("500") }
}

func TestIsServerError(t *testing.T) {
	if !IsServerError(500) { t.Error("500") }
	if !IsServerError(503) { t.Error("503") }
	if IsServerError(400) { t.Error("400") }
}

func TestIsError(t *testing.T) {
	if !IsError(400) { t.Error("400") }
	if !IsError(500) { t.Error("500") }
	if IsError(200) { t.Error("200") }
	if IsError(301) { t.Error("301") }
}

func TestIsRetryable(t *testing.T) {
	retryable := []int{408, 429, 500, 502, 503, 504}
	for _, code := range retryable {
		if !IsRetryable(code) {
			t.Errorf("%d should be retryable", code)
		}
	}
	notRetryable := []int{400, 401, 403, 404, 501}
	for _, code := range notRetryable {
		if IsRetryable(code) {
			t.Errorf("%d should not be retryable", code)
		}
	}
}

func TestCategory(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{100, "informational"},
		{200, "success"},
		{301, "redirection"},
		{404, "client_error"},
		{500, "server_error"},
		{0, "unknown"},
		{999, "unknown"},
	}
	for _, tt := range tests {
		if got := Category(tt.code); got != tt.want {
			t.Errorf("Category(%d) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestDescription(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{200, "ok"},
		{201, "created"},
		{400, "bad_request"},
		{401, "unauthorized"},
		{403, "forbidden"},
		{404, "not_found"},
		{429, "too_many_requests"},
		{500, "internal_server_error"},
		{503, "service_unavailable"},
	}
	for _, tt := range tests {
		if got := Description(tt.code); got != tt.want {
			t.Errorf("Description(%d) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestDescription_Unknown(t *testing.T) {
	// Unknown code should return category
	d := Description(299)
	if d != "success" {
		t.Errorf("Description(299) = %q, want 'success'", d)
	}
}
