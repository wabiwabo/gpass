package envelope_http

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestOK(t *testing.T) {
	w := httptest.NewRecorder()
	OK(w, map[string]string{"message": "hello"})

	if w.Code != 200 {
		t.Errorf("status = %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q", w.Header().Get("Content-Type"))
	}

	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)

	if !resp.Success {
		t.Error("Success should be true")
	}
	if resp.Data == nil {
		t.Error("Data should not be nil")
	}
	if resp.Error != nil {
		t.Error("Error should be nil")
	}
	if resp.Meta == nil || resp.Meta.Timestamp.IsZero() {
		t.Error("Meta.Timestamp should be set")
	}
}

func TestCreated(t *testing.T) {
	w := httptest.NewRecorder()
	Created(w, map[string]string{"id": "new-123"})

	if w.Code != 201 {
		t.Errorf("status = %d, want 201", w.Code)
	}

	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Error("Success should be true")
	}
}

func TestNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	NoContent(w)

	if w.Code != 204 {
		t.Errorf("status = %d, want 204", w.Code)
	}
	if w.Body.Len() > 0 {
		t.Error("body should be empty")
	}
}

func TestPaginated(t *testing.T) {
	w := httptest.NewRecorder()
	items := []string{"a", "b", "c"}
	Paginated(w, items, 2, 10, 25)

	if w.Code != 200 {
		t.Errorf("status = %d", w.Code)
	}

	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)

	if !resp.Success {
		t.Error("Success should be true")
	}
	if resp.Meta == nil || resp.Meta.Pagination == nil {
		t.Fatal("Pagination should be set")
	}

	p := resp.Meta.Pagination
	if p.Page != 2 {
		t.Errorf("Page = %d", p.Page)
	}
	if p.PerPage != 10 {
		t.Errorf("PerPage = %d", p.PerPage)
	}
	if p.Total != 25 {
		t.Errorf("Total = %d", p.Total)
	}
	if p.TotalPages != 3 {
		t.Errorf("TotalPages = %d, want 3", p.TotalPages)
	}
	if !p.HasNext {
		t.Error("HasNext should be true (page 2 of 3)")
	}
	if !p.HasPrev {
		t.Error("HasPrev should be true (page 2)")
	}
}

func TestPaginated_FirstPage(t *testing.T) {
	w := httptest.NewRecorder()
	Paginated(w, nil, 1, 10, 25)

	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)

	p := resp.Meta.Pagination
	if p.HasPrev {
		t.Error("HasPrev should be false on first page")
	}
	if !p.HasNext {
		t.Error("HasNext should be true")
	}
}

func TestPaginated_LastPage(t *testing.T) {
	w := httptest.NewRecorder()
	Paginated(w, nil, 3, 10, 25)

	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)

	p := resp.Meta.Pagination
	if p.HasNext {
		t.Error("HasNext should be false on last page")
	}
	if !p.HasPrev {
		t.Error("HasPrev should be true")
	}
}

func TestPaginated_ExactDivision(t *testing.T) {
	w := httptest.NewRecorder()
	Paginated(w, nil, 1, 10, 20)

	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Meta.Pagination.TotalPages != 2 {
		t.Errorf("TotalPages = %d, want 2", resp.Meta.Pagination.TotalPages)
	}
}

func TestError(t *testing.T) {
	w := httptest.NewRecorder()
	Error(w, 400, "validation_failed", "email is required")

	if w.Code != 400 {
		t.Errorf("status = %d", w.Code)
	}

	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Success {
		t.Error("Success should be false")
	}
	if resp.Error == nil {
		t.Fatal("Error should not be nil")
	}
	if resp.Error.Code != "validation_failed" {
		t.Errorf("Code = %q", resp.Error.Code)
	}
	if resp.Error.Message != "email is required" {
		t.Errorf("Message = %q", resp.Error.Message)
	}
}

func TestErrorWithDetail(t *testing.T) {
	w := httptest.NewRecorder()
	ErrorWithDetail(w, 422, "invalid_format", "NIK invalid", "province code 99 is not valid")

	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Error.Detail != "province code 99 is not valid" {
		t.Errorf("Detail = %q", resp.Error.Detail)
	}
}

func TestError_InternalServer(t *testing.T) {
	w := httptest.NewRecorder()
	Error(w, 500, "internal_error", "unexpected failure")

	if w.Code != 500 {
		t.Errorf("status = %d", w.Code)
	}
}

func TestWithRequestID(t *testing.T) {
	w := httptest.NewRecorder()
	resp := Response{
		Success: true,
		Data:    "test",
		Meta:    &Meta{},
	}
	WithRequestID(w, 200, resp, "req-abc-123")

	var got Response
	json.NewDecoder(w.Body).Decode(&got)

	if got.RequestID != "req-abc-123" {
		t.Errorf("RequestID = %q", got.RequestID)
	}
}

func TestOK_NilData(t *testing.T) {
	w := httptest.NewRecorder()
	OK(w, nil)

	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Error("should still be success with nil data")
	}
}

func TestPaginated_SinglePage(t *testing.T) {
	w := httptest.NewRecorder()
	Paginated(w, nil, 1, 10, 5)

	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)

	p := resp.Meta.Pagination
	if p.TotalPages != 1 {
		t.Errorf("TotalPages = %d, want 1", p.TotalPages)
	}
	if p.HasNext {
		t.Error("HasNext should be false")
	}
	if p.HasPrev {
		t.Error("HasPrev should be false")
	}
}
