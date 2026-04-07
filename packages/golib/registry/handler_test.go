package registry

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// doJSON sends a request through the registry HTTP handler.
func doJSON(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var b []byte
	if body != nil {
		var err error
		b, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// TestHandler_RegisterFlow covers the full register → get → heartbeat
// → list HTTP path. The handler-level tests previously only covered
// register; the heartbeat/get/list branches were 50–80% uncovered.
func TestHandler_RegisterFlow(t *testing.T) {
	r := New()
	h := r.Handler()

	// Register
	w := doJSON(t, h, "POST", "/internal/registry/register", map[string]any{
		"name":     "garudaaudit",
		"host":     "10.0.0.1",
		"port":     4010,
		"version":  "v1.2.3",
		"metadata": map[string]string{"region": "id-jkt"},
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", w.Code, w.Body.String())
	}
	var reg struct{ ID string }
	if err := json.Unmarshal(w.Body.Bytes(), &reg); err != nil {
		t.Fatal(err)
	}
	if reg.ID == "" {
		t.Fatal("register did not return id")
	}

	// Get the service back
	w = doJSON(t, h, "GET", "/internal/registry/services/garudaaudit", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get status = %d", w.Code)
	}
	var got struct {
		Instances []ServiceInstance `json:"instances"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Instances) != 1 || got.Instances[0].Host != "10.0.0.1" {
		t.Errorf("get returned wrong instances: %+v", got)
	}
	// Metadata must round-trip — pins copyInstance map deep-copy.
	if got.Instances[0].Metadata["region"] != "id-jkt" {
		t.Errorf("metadata lost: %v", got.Instances[0].Metadata)
	}

	// Heartbeat success
	w = doJSON(t, h, "POST", "/internal/registry/heartbeat", map[string]string{
		"name":        "garudaaudit",
		"instance_id": reg.ID,
	})
	if w.Code != http.StatusOK {
		t.Errorf("heartbeat status = %d, body = %s", w.Code, w.Body.String())
	}
}

// TestHandler_RegisterValidation covers the missing-fields and bad-JSON
// rejection branches.
func TestHandler_RegisterValidation(t *testing.T) {
	h := New().Handler()

	// Bad JSON
	req := httptest.NewRequest("POST", "/internal/registry/register",
		strings.NewReader("not json"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("bad JSON: status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid JSON body") {
		t.Errorf("bad JSON body = %s", w.Body.String())
	}

	// Missing required fields
	w = doJSON(t, h, "POST", "/internal/registry/register", map[string]any{
		"name": "x",
		// host, port omitted
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("missing fields: status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "name, host, and port are required") {
		t.Errorf("missing fields body = %s", w.Body.String())
	}
}

// TestHandler_HeartbeatErrors covers the bad-JSON and not-found branches
// of the heartbeat handler.
func TestHandler_HeartbeatErrors(t *testing.T) {
	h := New().Handler()

	// Bad JSON
	req := httptest.NewRequest("POST", "/internal/registry/heartbeat",
		strings.NewReader("nope"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("bad JSON status = %d", w.Code)
	}

	// Unknown instance → 404
	w = doJSON(t, h, "POST", "/internal/registry/heartbeat", map[string]string{
		"name":        "ghost",
		"instance_id": "no-such-id",
	})
	if w.Code != http.StatusNotFound {
		t.Errorf("unknown service status = %d", w.Code)
	}
}

// TestHandler_GetService_EmptyReturnsArray pins that GET on a never-
// registered service returns an empty array, NOT null. Half of all
// JSON clients crash on a null value where they expect [].
func TestHandler_GetService_EmptyReturnsArray(t *testing.T) {
	h := New().Handler()
	w := doJSON(t, h, "GET", "/internal/registry/services/missing", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"instances":[]`) {
		t.Errorf("body = %s, want []", body)
	}
}

// TestRegistry_setStatusErrors covers the not-found branches of
// MarkDown / MarkDraining / setStatus.
func TestRegistry_setStatusErrors(t *testing.T) {
	r := New()
	if err := r.MarkDown("ghost", "id"); err != ErrServiceNotFound {
		t.Errorf("MarkDown ghost: err = %v", err)
	}
	id := r.Register(ServiceInstance{Name: "real", Host: "h", Port: 1})
	if err := r.MarkDraining("real", "wrong-id"); err != ErrInstanceNotFound {
		t.Errorf("MarkDraining wrong id: err = %v", err)
	}
	if err := r.MarkDown("real", id); err != nil {
		t.Errorf("MarkDown real: err = %v", err)
	}
}

// TestRegistry_HeartbeatNotFound covers the Heartbeat error branch
// (instance exists by name but not by id).
func TestRegistry_HeartbeatNotFound(t *testing.T) {
	r := New()
	if err := r.Heartbeat("ghost", "id"); err != ErrServiceNotFound {
		t.Errorf("ghost: err = %v", err)
	}
	r.Register(ServiceInstance{Name: "real", Host: "h", Port: 1})
	if err := r.Heartbeat("real", "wrong-id"); err != ErrInstanceNotFound {
		t.Errorf("wrong id: err = %v", err)
	}
}
