package apidoc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry("identity", "1.0.0", "Identity service")
	if r.name != "identity" {
		t.Errorf("name = %q, want identity", r.name)
	}
	if r.version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", r.version)
	}
	if r.desc != "Identity service" {
		t.Errorf("desc = %q", r.desc)
	}
	if r.Count() != 0 {
		t.Errorf("Count = %d, want 0", r.Count())
	}
}

func TestSetBasePath(t *testing.T) {
	r := NewRegistry("svc", "1.0.0", "test")
	r.SetBasePath("/api/v1")

	doc := r.Doc()
	if doc.BasePath != "/api/v1" {
		t.Errorf("BasePath = %q, want /api/v1", doc.BasePath)
	}
}

func TestRegister(t *testing.T) {
	r := NewRegistry("svc", "1.0.0", "test")
	r.Register(Endpoint{
		Method:      "GET",
		Path:        "/users",
		Description: "List users",
		Tags:        []string{"users"},
		Auth:        "session",
	})

	if r.Count() != 1 {
		t.Fatalf("Count = %d, want 1", r.Count())
	}

	doc := r.Doc()
	if len(doc.Endpoints) != 1 {
		t.Fatalf("Endpoints len = %d, want 1", len(doc.Endpoints))
	}

	ep := doc.Endpoints[0]
	if ep.Method != "GET" {
		t.Errorf("Method = %q", ep.Method)
	}
	if ep.Path != "/users" {
		t.Errorf("Path = %q", ep.Path)
	}
	if ep.Description != "List users" {
		t.Errorf("Description = %q", ep.Description)
	}
	if ep.Auth != "session" {
		t.Errorf("Auth = %q", ep.Auth)
	}
	if len(ep.Tags) != 1 || ep.Tags[0] != "users" {
		t.Errorf("Tags = %v", ep.Tags)
	}
}

func TestRegisterRoute(t *testing.T) {
	r := NewRegistry("svc", "1.0.0", "test")
	r.RegisterRoute("POST", "/users", "Create user", "session", "users", "admin")

	doc := r.Doc()
	if len(doc.Endpoints) != 1 {
		t.Fatalf("len = %d", len(doc.Endpoints))
	}

	ep := doc.Endpoints[0]
	if ep.Method != "POST" {
		t.Errorf("Method = %q", ep.Method)
	}
	if ep.Auth != "session" {
		t.Errorf("Auth = %q", ep.Auth)
	}
	if len(ep.Tags) != 2 {
		t.Errorf("Tags = %v", ep.Tags)
	}
}

func TestDoc_SortedByPathThenMethod(t *testing.T) {
	r := NewRegistry("svc", "1.0.0", "test")
	r.RegisterRoute("DELETE", "/users/{id}", "Delete user", "session")
	r.RegisterRoute("GET", "/users/{id}", "Get user", "session")
	r.RegisterRoute("GET", "/health", "Health check", "none")
	r.RegisterRoute("POST", "/users", "Create user", "session")
	r.RegisterRoute("GET", "/users", "List users", "session")

	doc := r.Doc()

	expected := []struct {
		method string
		path   string
	}{
		{"GET", "/health"},
		{"GET", "/users"},
		{"POST", "/users"},
		{"DELETE", "/users/{id}"},
		{"GET", "/users/{id}"},
	}

	if len(doc.Endpoints) != len(expected) {
		t.Fatalf("len = %d, want %d", len(doc.Endpoints), len(expected))
	}

	for i, want := range expected {
		got := doc.Endpoints[i]
		if got.Method != want.method || got.Path != want.path {
			t.Errorf("[%d] = %s %s, want %s %s", i, got.Method, got.Path, want.method, want.path)
		}
	}
}

func TestDoc_ReturnsServiceInfo(t *testing.T) {
	r := NewRegistry("identity", "2.1.0", "Identity verification service")
	r.SetBasePath("/api/v2")

	doc := r.Doc()
	if doc.Name != "identity" {
		t.Errorf("Name = %q", doc.Name)
	}
	if doc.Version != "2.1.0" {
		t.Errorf("Version = %q", doc.Version)
	}
	if doc.Description != "Identity verification service" {
		t.Errorf("Description = %q", doc.Description)
	}
	if doc.BasePath != "/api/v2" {
		t.Errorf("BasePath = %q", doc.BasePath)
	}
}

func TestDoc_CopySlice(t *testing.T) {
	r := NewRegistry("svc", "1.0.0", "test")
	r.RegisterRoute("GET", "/a", "A", "none")

	doc1 := r.Doc()
	r.RegisterRoute("GET", "/b", "B", "none")
	doc2 := r.Doc()

	if len(doc1.Endpoints) != 1 {
		t.Error("doc1 should not be affected by later registrations")
	}
	if len(doc2.Endpoints) != 2 {
		t.Errorf("doc2 len = %d, want 2", len(doc2.Endpoints))
	}
}

func TestTags(t *testing.T) {
	r := NewRegistry("svc", "1.0.0", "test")
	r.RegisterRoute("GET", "/users", "List", "none", "users", "admin")
	r.RegisterRoute("GET", "/health", "Health", "none", "system")
	r.RegisterRoute("POST", "/users", "Create", "session", "users") // duplicate "users"

	tags := r.Tags()

	// Should be sorted and unique
	expected := []string{"admin", "system", "users"}
	if len(tags) != len(expected) {
		t.Fatalf("Tags = %v, want %v", tags, expected)
	}
	for i, want := range expected {
		if tags[i] != want {
			t.Errorf("Tags[%d] = %q, want %q", i, tags[i], want)
		}
	}
}

func TestTags_NoTags(t *testing.T) {
	r := NewRegistry("svc", "1.0.0", "test")
	r.RegisterRoute("GET", "/health", "Health", "none")

	tags := r.Tags()
	if len(tags) != 0 {
		t.Errorf("Tags = %v, want empty", tags)
	}
}

func TestTags_Empty(t *testing.T) {
	r := NewRegistry("svc", "1.0.0", "test")
	tags := r.Tags()
	if len(tags) != 0 {
		t.Errorf("Tags = %v, want empty", tags)
	}
}

func TestHandler(t *testing.T) {
	r := NewRegistry("identity", "1.0.0", "Identity service")
	r.SetBasePath("/api/v1")
	r.RegisterRoute("GET", "/health", "Health check", "none", "system")
	r.RegisterRoute("POST", "/verify", "Verify identity", "api_key", "verification")

	handler := r.Handler()
	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	cc := w.Header().Get("Cache-Control")
	if cc != "public, max-age=300" {
		t.Errorf("Cache-Control = %q", cc)
	}

	var doc ServiceDoc
	if err := json.NewDecoder(w.Body).Decode(&doc); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if doc.Name != "identity" {
		t.Errorf("Name = %q", doc.Name)
	}
	if doc.Version != "1.0.0" {
		t.Errorf("Version = %q", doc.Version)
	}
	if len(doc.Endpoints) != 2 {
		t.Fatalf("Endpoints len = %d, want 2", len(doc.Endpoints))
	}

	// Sorted: /health before /verify
	if doc.Endpoints[0].Path != "/health" {
		t.Errorf("first endpoint = %q, want /health", doc.Endpoints[0].Path)
	}
	if doc.Endpoints[1].Path != "/verify" {
		t.Errorf("second endpoint = %q, want /verify", doc.Endpoints[1].Path)
	}
}

func TestHandler_EmptyRegistry(t *testing.T) {
	r := NewRegistry("empty", "0.1.0", "empty service")
	handler := r.Handler()

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	var doc ServiceDoc
	if err := json.NewDecoder(w.Body).Decode(&doc); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if doc.Name != "empty" {
		t.Errorf("Name = %q", doc.Name)
	}
	if len(doc.Endpoints) != 0 {
		t.Errorf("Endpoints = %v, want empty", doc.Endpoints)
	}
}

func TestHandler_ValidJSON(t *testing.T) {
	r := NewRegistry("svc", "1.0.0", "test")
	r.RegisterRoute("GET", "/test", "Test endpoint", "none")

	handler := r.Handler()
	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	var raw json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestConcurrent_RegisterAndDoc(t *testing.T) {
	r := NewRegistry("svc", "1.0.0", "test")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			r.RegisterRoute("GET", "/path", "desc", "none", "tag")
		}(i)
		go func() {
			defer wg.Done()
			_ = r.Doc()
		}()
	}
	wg.Wait()

	if r.Count() != 100 {
		t.Errorf("Count = %d, want 100", r.Count())
	}
}

func TestConcurrent_Count(t *testing.T) {
	r := NewRegistry("svc", "1.0.0", "test")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			r.RegisterRoute("GET", "/a", "a", "none")
		}()
		go func() {
			defer wg.Done()
			_ = r.Count()
		}()
	}
	wg.Wait()
}

func TestConcurrent_Tags(t *testing.T) {
	r := NewRegistry("svc", "1.0.0", "test")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			r.RegisterRoute("GET", "/a", "a", "none", "tag1")
		}()
		go func() {
			defer wg.Done()
			_ = r.Tags()
		}()
	}
	wg.Wait()
}

func TestConcurrent_SetBasePath(t *testing.T) {
	r := NewRegistry("svc", "1.0.0", "test")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			r.SetBasePath("/api/v1")
		}()
		go func() {
			defer wg.Done()
			_ = r.Doc()
		}()
	}
	wg.Wait()
}

func TestEndpoint_JSON(t *testing.T) {
	ep := Endpoint{
		Method:      "POST",
		Path:        "/verify",
		Description: "Verify NIK",
		Tags:        []string{"identity"},
		Auth:        "api_key",
		RateLimit:   "100/min",
	}

	data, err := json.Marshal(ep)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Endpoint
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.RateLimit != "100/min" {
		t.Errorf("RateLimit = %q", decoded.RateLimit)
	}
}

func TestEndpoint_JSONOmitEmpty(t *testing.T) {
	ep := Endpoint{
		Method:      "GET",
		Path:        "/health",
		Description: "Health check",
	}

	data, err := json.Marshal(ep)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	str := string(data)
	if contains := "tags"; containsStr(str, contains) {
		t.Errorf("JSON should omit empty tags: %s", str)
	}
	if contains := "auth"; containsStr(str, contains) {
		t.Errorf("JSON should omit empty auth: %s", str)
	}
	if contains := "rate_limit"; containsStr(str, contains) {
		t.Errorf("JSON should omit empty rate_limit: %s", str)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && searchStr(s, sub)
}

func searchStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestCount_AfterMultipleRegistrations(t *testing.T) {
	r := NewRegistry("svc", "1.0.0", "test")

	for i := 0; i < 25; i++ {
		r.RegisterRoute("GET", "/path", "desc", "none")
	}

	if r.Count() != 25 {
		t.Errorf("Count = %d, want 25", r.Count())
	}
}

func TestRegisterRoute_NoTags(t *testing.T) {
	r := NewRegistry("svc", "1.0.0", "test")
	r.RegisterRoute("GET", "/health", "Health", "none")

	doc := r.Doc()
	if len(doc.Endpoints[0].Tags) != 0 {
		t.Errorf("Tags = %v, want empty", doc.Endpoints[0].Tags)
	}
}

func TestHandler_ServedAsHTTPHandler(t *testing.T) {
	r := NewRegistry("svc", "1.0.0", "test")
	r.RegisterRoute("GET", "/test", "Test", "none")

	// Handler() returns http.HandlerFunc which satisfies http.Handler
	var h http.Handler = r.Handler()
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}

	var doc ServiceDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if doc.Name != "svc" {
		t.Errorf("Name = %q", doc.Name)
	}
}
