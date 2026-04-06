package httperr

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPError_Error(t *testing.T) {
	tests := []struct {
		name   string
		err    *HTTPError
		expect string
	}{
		{
			name:   "without detail",
			err:    &HTTPError{Status: 400, Code: "bad_request", Message: "invalid input"},
			expect: "[400] bad_request: invalid input",
		},
		{
			name:   "with detail",
			err:    &HTTPError{Status: 404, Code: "not_found", Message: "resource not found", Detail: "user 123 does not exist"},
			expect: "[404] not_found: user 123 does not exist",
		},
		{
			name:   "500 internal",
			err:    &HTTPError{Status: 500, Code: "internal_error", Message: "something went wrong"},
			expect: "[500] internal_error: something went wrong",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expect {
				t.Errorf("Error() = %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestHTTPError_ImplementsError(t *testing.T) {
	var err error = &HTTPError{Status: 400, Code: "test", Message: "test"}
	if err == nil {
		t.Fatal("HTTPError should implement error interface")
	}
}

func TestHTTPError_Write(t *testing.T) {
	tests := []struct {
		name       string
		err        *HTTPError
		wantStatus int
		wantCode   string
		wantTitle  string
		wantDetail string
	}{
		{
			name:       "bad request",
			err:        BadRequest("invalid_field", "field X is invalid").WithDetail("must be alphanumeric"),
			wantStatus: 400,
			wantCode:   "invalid_field",
			wantTitle:  "field X is invalid",
			wantDetail: "must be alphanumeric",
		},
		{
			name:       "unauthorized",
			err:        Unauthorized("auth_required", "authentication required"),
			wantStatus: 401,
			wantCode:   "auth_required",
			wantTitle:  "authentication required",
			wantDetail: "",
		},
		{
			name:       "forbidden",
			err:        Forbidden("insufficient_scope", "missing admin role"),
			wantStatus: 403,
			wantCode:   "insufficient_scope",
			wantTitle:  "missing admin role",
			wantDetail: "",
		},
		{
			name:       "not found",
			err:        NotFound("resource_missing", "user not found"),
			wantStatus: 404,
			wantCode:   "resource_missing",
			wantTitle:  "user not found",
			wantDetail: "",
		},
		{
			name:       "conflict",
			err:        Conflict("already_exists", "email already registered"),
			wantStatus: 409,
			wantCode:   "already_exists",
			wantTitle:  "email already registered",
			wantDetail: "",
		},
		{
			name:       "too many requests",
			err:        TooManyRequests("rate limit exceeded"),
			wantStatus: 429,
			wantCode:   "rate_limited",
			wantTitle:  "rate limit exceeded",
			wantDetail: "",
		},
		{
			name:       "internal error",
			err:        Internal("unexpected failure"),
			wantStatus: 500,
			wantCode:   "internal_error",
			wantTitle:  "unexpected failure",
			wantDetail: "",
		},
		{
			name:       "service unavailable",
			err:        ServiceUnavailable("downstream timeout"),
			wantStatus: 503,
			wantCode:   "service_unavailable",
			wantTitle:  "downstream timeout",
			wantDetail: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tt.err.Write(w)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			ct := w.Header().Get("Content-Type")
			if ct != "application/problem+json" {
				t.Errorf("Content-Type = %q, want application/problem+json", ct)
			}

			cc := w.Header().Get("Cache-Control")
			if cc != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", cc)
			}

			var body map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode body: %v", err)
			}

			if body["type"] != "about:blank" {
				t.Errorf("type = %v, want about:blank", body["type"])
			}
			if body["code"] != tt.wantCode {
				t.Errorf("code = %v, want %s", body["code"], tt.wantCode)
			}
			if body["title"] != tt.wantTitle {
				t.Errorf("title = %v, want %s", body["title"], tt.wantTitle)
			}

			statusFloat, ok := body["status"].(float64)
			if !ok || int(statusFloat) != tt.wantStatus {
				t.Errorf("body status = %v, want %d", body["status"], tt.wantStatus)
			}

			if tt.wantDetail != "" {
				if body["detail"] != tt.wantDetail {
					t.Errorf("detail = %v, want %s", body["detail"], tt.wantDetail)
				}
			}
		})
	}
}

func TestNew(t *testing.T) {
	err := New(422, "unprocessable", "cannot process entity")
	if err.Status != 422 {
		t.Errorf("Status = %d, want 422", err.Status)
	}
	if err.Code != "unprocessable" {
		t.Errorf("Code = %q, want unprocessable", err.Code)
	}
	if err.Message != "cannot process entity" {
		t.Errorf("Message = %q, want 'cannot process entity'", err.Message)
	}
}

func TestWithDetail(t *testing.T) {
	err := BadRequest("bad", "bad request").WithDetail("field X must be > 0")
	if err.Detail != "field X must be > 0" {
		t.Errorf("Detail = %q, want 'field X must be > 0'", err.Detail)
	}
	if err.Status != 400 {
		t.Errorf("Status = %d, want 400", err.Status)
	}
}

func TestWithDetail_Chaining(t *testing.T) {
	err := NotFound("missing", "not found").WithDetail("user abc123")
	if err.Detail != "user abc123" {
		t.Errorf("Detail = %q", err.Detail)
	}
	if err.Code != "missing" {
		t.Errorf("Code = %q", err.Code)
	}
}

func TestHandle_NilError(t *testing.T) {
	w := httptest.NewRecorder()
	handled := Handle(w, nil)
	if handled {
		t.Error("Handle(nil) should return false")
	}
	if w.Code != 200 {
		t.Errorf("status = %d, want 200 (default)", w.Code)
	}
}

func TestHandle_HTTPError(t *testing.T) {
	w := httptest.NewRecorder()
	err := Forbidden("no_access", "access denied")
	handled := Handle(w, err)
	if !handled {
		t.Error("Handle should return true for HTTPError")
	}
	if w.Code != 403 {
		t.Errorf("status = %d, want 403", w.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["code"] != "no_access" {
		t.Errorf("code = %v, want no_access", body["code"])
	}
}

func TestHandle_GenericError(t *testing.T) {
	w := httptest.NewRecorder()
	handled := Handle(w, errors.New("something broke"))
	if !handled {
		t.Error("Handle should return true for generic error")
	}
	if w.Code != 500 {
		t.Errorf("status = %d, want 500", w.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["code"] != "internal_error" {
		t.Errorf("code = %v, want internal_error", body["code"])
	}
}

func TestHandle_GenericError_MessagePreserved(t *testing.T) {
	w := httptest.NewRecorder()
	Handle(w, fmt.Errorf("db connection failed"))

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)

	if !strings.Contains(body["title"].(string), "db connection failed") {
		t.Errorf("title = %v, expected to contain original message", body["title"])
	}
}

func TestMust_NilError(t *testing.T) {
	// Should not panic
	Must(nil)
}

func TestMust_WithError(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("Must should panic on non-nil error")
		}
	}()
	Must(errors.New("init failed"))
}

func TestMust_WithHTTPError(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("Must should panic on HTTPError")
		}
		httpErr, ok := r.(*HTTPError)
		if !ok {
			t.Error("recovered value should be *HTTPError")
		}
		if httpErr.Status != 500 {
			t.Errorf("Status = %d, want 500", httpErr.Status)
		}
	}()
	Must(Internal("startup failed"))
}

func TestConstructors_StatusCodes(t *testing.T) {
	tests := []struct {
		name   string
		err    *HTTPError
		status int
		code   string
	}{
		{"BadRequest", BadRequest("c", "m"), 400, "c"},
		{"Unauthorized", Unauthorized("c", "m"), 401, "c"},
		{"Forbidden", Forbidden("c", "m"), 403, "c"},
		{"NotFound", NotFound("c", "m"), 404, "c"},
		{"Conflict", Conflict("c", "m"), 409, "c"},
		{"TooManyRequests", TooManyRequests("m"), 429, "rate_limited"},
		{"Internal", Internal("m"), 500, "internal_error"},
		{"ServiceUnavailable", ServiceUnavailable("m"), 503, "service_unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Status != tt.status {
				t.Errorf("Status = %d, want %d", tt.err.Status, tt.status)
			}
			if tt.err.Code != tt.code {
				t.Errorf("Code = %q, want %q", tt.err.Code, tt.code)
			}
		})
	}
}

func TestWrite_ResponseBodyIsValidJSON(t *testing.T) {
	w := httptest.NewRecorder()
	Internal("test").Write(w)

	var raw json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
}

func TestWrite_SpecialCharactersInMessage(t *testing.T) {
	w := httptest.NewRecorder()
	BadRequest("test", `field "name" contains <script>alert(1)</script>`).Write(w)

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	title := body["title"].(string)
	if !strings.Contains(title, "<script>") {
		// JSON encoding should preserve the original string
		t.Errorf("title should contain original characters, got %q", title)
	}
}
