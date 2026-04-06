package integration

import (
	"bytes"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// In-memory stores
// ---------------------------------------------------------------------------

type certificate struct {
	ID         string    `json:"id"`
	CommonName string    `json:"common_name"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	PEM        string    `json:"pem,omitempty"`
}

type document struct {
	ID            string `json:"id"`
	OwnerID       string `json:"owner_id"`
	FileName      string `json:"file_name"`
	ContentBase64 string `json:"content_base64,omitempty"`
	Status        string `json:"status"`
}

type signedResult struct {
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	SignedDoc struct {
		ID                 string `json:"id"`
		SignedHash         string `json:"signed_hash"`
		PAdESLevel         string `json:"pades_level"`
		SignatureTimestamp  string `json:"signature_timestamp"`
	} `json:"signed_document"`
}

type portalApp struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	UserID      string `json:"user_id"`
}

type apiKey struct {
	ID       string `json:"id"`
	AppID    string `json:"app_id"`
	Key      string `json:"key"`
	Status   string `json:"status"`
	Env      string `json:"env"`
}

type auditEvent struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	Actor     string    `json:"actor"`
	Resource  string    `json:"resource"`
	Timestamp time.Time `json:"timestamp"`
	Details   string    `json:"details"`
}

// ---------------------------------------------------------------------------
// Signing Sim server — lightweight certificate authority + document signer
// ---------------------------------------------------------------------------

func newSigningSimServer() *httptest.Server {
	var (
		mu    sync.Mutex
		certs = make(map[string]*certificate)
		docs  = make(map[string]*document)
	)

	mux := http.NewServeMux()

	// POST /certificates/request — issue a certificate
	mux.HandleFunc("POST /certificates/request", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CommonName string `json:"common_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CommonName == "" {
			http.Error(w, `{"error":"common_name required"}`, http.StatusBadRequest)
			return
		}

		id := generateID("cert")
		cert := &certificate{
			ID:         id,
			CommonName: req.CommonName,
			Status:     "ACTIVE",
			CreatedAt:  time.Now(),
			PEM:        fakePEM(req.CommonName),
		}

		mu.Lock()
		certs[id] = cert
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(cert)
	})

	// GET /certificates/{id}
	mux.HandleFunc("GET /certificates/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		mu.Lock()
		cert, ok := certs[id]
		mu.Unlock()
		if !ok {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cert)
	})

	// POST /documents/upload — upload a document
	mux.HandleFunc("POST /documents/upload", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			OwnerID  string `json:"owner_id"`
			FileName string `json:"file_name"`
			Content  string `json:"content_base64"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FileName == "" {
			http.Error(w, `{"error":"file_name required"}`, http.StatusBadRequest)
			return
		}

		id := generateID("doc")
		doc := &document{
			ID:            id,
			OwnerID:       req.OwnerID,
			FileName:      req.FileName,
			ContentBase64: req.Content,
			Status:        "UPLOADED",
		}

		mu.Lock()
		docs[id] = doc
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(doc)
	})

	// POST /documents/{id}/sign — sign a document
	mux.HandleFunc("POST /documents/{id}/sign", func(w http.ResponseWriter, r *http.Request) {
		docID := r.PathValue("id")
		var req struct {
			CertificateID string `json:"certificate_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CertificateID == "" {
			http.Error(w, `{"error":"certificate_id required"}`, http.StatusBadRequest)
			return
		}

		mu.Lock()
		doc, docOK := docs[docID]
		cert, certOK := certs[req.CertificateID]
		if docOK {
			doc.Status = "SIGNED"
		}
		mu.Unlock()

		if !docOK {
			http.Error(w, `{"error":"document not found"}`, http.StatusNotFound)
			return
		}
		if !certOK {
			http.Error(w, `{"error":"certificate not found"}`, http.StatusNotFound)
			return
		}

		hash := fakeHash(doc.FileName + cert.CommonName)
		result := signedResult{
			RequestID: generateID("req"),
			Status:    "COMPLETED",
		}
		result.SignedDoc.ID = generateID("sdoc")
		result.SignedDoc.SignedHash = hash
		result.SignedDoc.PAdESLevel = "B_LTA"
		result.SignedDoc.SignatureTimestamp = time.Now().UTC().Format(time.RFC3339)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// GET /documents/{id}/download — download signed document
	mux.HandleFunc("GET /documents/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		docID := r.PathValue("id")
		mu.Lock()
		doc, ok := docs[docID]
		mu.Unlock()
		if !ok {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		if doc.Status != "SIGNED" {
			http.Error(w, `{"error":"document not signed"}`, http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", doc.FileName))
		w.Write([]byte("signed-content-" + doc.ID))
	})

	return httptest.NewServer(mux)
}

// ---------------------------------------------------------------------------
// Portal server — app + API key management
// ---------------------------------------------------------------------------

func newPortalServer() *httptest.Server {
	var (
		mu   sync.Mutex
		apps = make(map[string]*portalApp)
		keys = make(map[string]*apiKey)
	)

	mux := http.NewServeMux()

	// POST /apps — create app
	mux.HandleFunc("POST /apps", func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			http.Error(w, `{"error":"X-User-ID required"}`, http.StatusBadRequest)
			return
		}
		var req struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			http.Error(w, `{"error":"name required"}`, http.StatusBadRequest)
			return
		}

		app := &portalApp{
			ID:          generateID("app"),
			Name:        req.Name,
			Description: req.Description,
			UserID:      userID,
		}

		mu.Lock()
		apps[app.ID] = app
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(app)
	})

	// POST /apps/{id}/keys — create API key
	mux.HandleFunc("POST /apps/{id}/keys", func(w http.ResponseWriter, r *http.Request) {
		appID := r.PathValue("id")
		mu.Lock()
		_, ok := apps[appID]
		mu.Unlock()
		if !ok {
			http.Error(w, `{"error":"app not found"}`, http.StatusNotFound)
			return
		}

		plaintext := "gp_test_" + randomHex(16)
		key := &apiKey{
			ID:     generateID("key"),
			AppID:  appID,
			Key:    plaintext,
			Status: "ACTIVE",
			Env:    "sandbox",
		}

		mu.Lock()
		keys[key.ID] = key
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(key)
	})

	// POST /keys/validate — validate API key
	mux.HandleFunc("POST /keys/validate", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Key string `json:"key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
			return
		}

		mu.Lock()
		var found *apiKey
		for _, k := range keys {
			if k.Key == req.Key {
				found = k
				break
			}
		}
		mu.Unlock()

		if found == nil || found.Status != "ACTIVE" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"valid": false})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":  true,
			"app_id": found.AppID,
			"env":    found.Env,
		})
	})

	// POST /keys/{id}/rotate — rotate API key
	mux.HandleFunc("POST /keys/{id}/rotate", func(w http.ResponseWriter, r *http.Request) {
		keyID := r.PathValue("id")
		mu.Lock()
		existing, ok := keys[keyID]
		if ok {
			existing.Key = "gp_test_" + randomHex(16)
		}
		mu.Unlock()

		if !ok {
			http.Error(w, `{"error":"key not found"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(existing)
	})

	// DELETE /keys/{id} — revoke API key
	mux.HandleFunc("DELETE /keys/{id}", func(w http.ResponseWriter, r *http.Request) {
		keyID := r.PathValue("id")
		mu.Lock()
		existing, ok := keys[keyID]
		if ok {
			existing.Status = "REVOKED"
		}
		mu.Unlock()

		if !ok {
			http.Error(w, `{"error":"key not found"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "REVOKED"})
	})

	return httptest.NewServer(mux)
}

// ---------------------------------------------------------------------------
// Audit server — event ingestion, query, compliance report
// ---------------------------------------------------------------------------

func newAuditServer() *httptest.Server {
	var (
		mu     sync.Mutex
		events []auditEvent
	)

	mux := http.NewServeMux()

	// POST /events — ingest audit event
	mux.HandleFunc("POST /events", func(w http.ResponseWriter, r *http.Request) {
		var evt auditEvent
		if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
			http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
			return
		}
		evt.ID = generateID("evt")
		if evt.Timestamp.IsZero() {
			evt.Timestamp = time.Now()
		}

		mu.Lock()
		events = append(events, evt)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(evt)
	})

	// GET /events?actor=...&action=... — query events
	mux.HandleFunc("GET /events", func(w http.ResponseWriter, r *http.Request) {
		actor := r.URL.Query().Get("actor")
		action := r.URL.Query().Get("action")

		mu.Lock()
		var results []auditEvent
		for _, evt := range events {
			if actor != "" && evt.Actor != actor {
				continue
			}
			if action != "" && evt.Action != action {
				continue
			}
			results = append(results, evt)
		}
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"events": results,
			"total":  len(results),
		})
	})

	// GET /reports/compliance — generate compliance report
	mux.HandleFunc("GET /reports/compliance", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		total := len(events)
		actionCounts := make(map[string]int)
		actorCounts := make(map[string]int)
		for _, evt := range events {
			actionCounts[evt.Action]++
			actorCounts[evt.Actor]++
		}
		mu.Unlock()

		report := map[string]interface{}{
			"total_events":    total,
			"actions_summary": actionCounts,
			"actors_summary":  actorCounts,
			"generated_at":    time.Now().UTC().Format(time.RFC3339),
			"compliant":       total > 0,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(report)
	})

	return httptest.NewServer(mux)
}

// ---------------------------------------------------------------------------
// E2E Tests
// ---------------------------------------------------------------------------

// TestE2E_SigningFlow starts signing-sim as an embedded server,
// then exercises: request certificate -> upload document -> sign -> download.
func TestE2E_SigningFlow(t *testing.T) {
	srv := newSigningSimServer()
	defer srv.Close()

	client := srv.Client()
	base := srv.URL

	// Step 1: Request certificate
	certBody := `{"common_name":"John Doe"}`
	resp, err := client.Post(base+"/certificates/request", "application/json", strings.NewReader(certBody))
	if err != nil {
		t.Fatalf("certificate request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var cert certificate
	if err := json.NewDecoder(resp.Body).Decode(&cert); err != nil {
		t.Fatalf("decode certificate: %v", err)
	}
	if cert.ID == "" || cert.Status != "ACTIVE" {
		t.Fatalf("unexpected certificate: %+v", cert)
	}
	t.Logf("Certificate created: %s", cert.ID)

	// Step 2: Upload document
	docBody := `{"owner_id":"user-123","file_name":"contract.pdf","content_base64":"dGVzdA=="}`
	resp, err = client.Post(base+"/documents/upload", "application/json", strings.NewReader(docBody))
	if err != nil {
		t.Fatalf("document upload failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var doc document
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		t.Fatalf("decode document: %v", err)
	}
	if doc.Status != "UPLOADED" {
		t.Fatalf("expected UPLOADED status, got %s", doc.Status)
	}
	t.Logf("Document uploaded: %s", doc.ID)

	// Step 3: Sign document
	signBody := fmt.Sprintf(`{"certificate_id":"%s"}`, cert.ID)
	resp, err = client.Post(base+"/documents/"+doc.ID+"/sign", "application/json", strings.NewReader(signBody))
	if err != nil {
		t.Fatalf("sign request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result signedResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode signed result: %v", err)
	}
	if result.Status != "COMPLETED" {
		t.Fatalf("expected COMPLETED, got %s", result.Status)
	}
	if result.SignedDoc.PAdESLevel != "B_LTA" {
		t.Fatalf("expected PAdES level B_LTA, got %s", result.SignedDoc.PAdESLevel)
	}
	if result.SignedDoc.SignedHash == "" {
		t.Fatal("signed hash should not be empty")
	}
	t.Logf("Document signed: %s (hash: %s)", result.SignedDoc.ID, result.SignedDoc.SignedHash)

	// Step 4: Download signed document
	resp, err = client.Get(base + "/documents/" + doc.ID + "/download")
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), doc.ID) {
		t.Error("downloaded content should reference the document ID")
	}
	if resp.Header.Get("Content-Type") != "application/pdf" {
		t.Errorf("expected application/pdf content type, got %s", resp.Header.Get("Content-Type"))
	}
	t.Logf("Document downloaded: %d bytes", len(body))
}

// TestE2E_PortalKeyLifecycle tests: create app -> create key -> validate -> rotate -> revoke.
func TestE2E_PortalKeyLifecycle(t *testing.T) {
	srv := newPortalServer()
	defer srv.Close()

	client := srv.Client()
	base := srv.URL

	// Step 1: Create app
	appBody := `{"name":"Test App","description":"E2E integration test app"}`
	req, _ := http.NewRequest(http.MethodPost, base+"/apps", strings.NewReader(appBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-42")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("create app failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
	}

	var app portalApp
	json.NewDecoder(resp.Body).Decode(&app)
	if app.ID == "" || app.Name != "Test App" {
		t.Fatalf("unexpected app: %+v", app)
	}
	t.Logf("App created: %s", app.ID)

	// Step 2: Create API key
	resp, err = client.Post(base+"/apps/"+app.ID+"/keys", "application/json", nil)
	if err != nil {
		t.Fatalf("create key failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var key apiKey
	json.NewDecoder(resp.Body).Decode(&key)
	if !strings.HasPrefix(key.Key, "gp_test_") {
		t.Fatalf("key should start with gp_test_, got %s", key.Key)
	}
	if key.Status != "ACTIVE" {
		t.Fatalf("expected ACTIVE, got %s", key.Status)
	}
	t.Logf("Key created: %s (key: %s)", key.ID, key.Key)

	// Step 3: Validate key
	valBody := fmt.Sprintf(`{"key":"%s"}`, key.Key)
	resp, err = client.Post(base+"/keys/validate", "application/json", strings.NewReader(valBody))
	if err != nil {
		t.Fatalf("validate key failed: %v", err)
	}
	defer resp.Body.Close()

	var valResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&valResult)
	if valResult["valid"] != true {
		t.Fatalf("key should be valid: %+v", valResult)
	}
	if valResult["app_id"] != app.ID {
		t.Errorf("expected app_id %s, got %v", app.ID, valResult["app_id"])
	}
	t.Logf("Key validated successfully")

	// Step 4: Rotate key
	resp, err = client.Post(base+"/keys/"+key.ID+"/rotate", "application/json", nil)
	if err != nil {
		t.Fatalf("rotate key failed: %v", err)
	}
	defer resp.Body.Close()

	var rotated apiKey
	json.NewDecoder(resp.Body).Decode(&rotated)
	if rotated.Key == key.Key {
		t.Error("rotated key should be different from original")
	}
	if !strings.HasPrefix(rotated.Key, "gp_test_") {
		t.Errorf("rotated key should start with gp_test_, got %s", rotated.Key)
	}
	t.Logf("Key rotated: old=%s new=%s", key.Key[:16]+"...", rotated.Key[:16]+"...")

	// Verify old key no longer works
	valBody = fmt.Sprintf(`{"key":"%s"}`, key.Key)
	resp, err = client.Post(base+"/keys/validate", "application/json", strings.NewReader(valBody))
	if err != nil {
		t.Fatalf("validate old key failed: %v", err)
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&valResult)
	if valResult["valid"] == true {
		t.Error("old key should no longer be valid after rotation")
	}

	// Step 5: Revoke key
	req, _ = http.NewRequest(http.MethodDelete, base+"/keys/"+key.ID, nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("revoke key failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify revoked key fails validation
	valBody = fmt.Sprintf(`{"key":"%s"}`, rotated.Key)
	resp, err = client.Post(base+"/keys/validate", "application/json", strings.NewReader(valBody))
	if err != nil {
		t.Fatalf("validate revoked key failed: %v", err)
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&valResult)
	if valResult["valid"] == true {
		t.Error("revoked key should not be valid")
	}
	t.Logf("Key revoked and validation correctly fails")
}

// TestE2E_AuditTrail tests: ingest events -> query -> generate compliance report.
func TestE2E_AuditTrail(t *testing.T) {
	srv := newAuditServer()
	defer srv.Close()

	client := srv.Client()
	base := srv.URL

	// Step 1: Ingest several audit events
	testEvents := []auditEvent{
		{Action: "SIGN_DOCUMENT", Actor: "user-1", Resource: "doc-1", Details: "Signed contract"},
		{Action: "SIGN_DOCUMENT", Actor: "user-2", Resource: "doc-2", Details: "Signed NDA"},
		{Action: "CREATE_APP", Actor: "user-1", Resource: "app-1", Details: "Created portal app"},
		{Action: "REVOKE_KEY", Actor: "admin-1", Resource: "key-1", Details: "Revoked compromised key"},
		{Action: "SIGN_DOCUMENT", Actor: "user-1", Resource: "doc-3", Details: "Signed agreement"},
	}

	for i, evt := range testEvents {
		body, _ := json.Marshal(evt)
		resp, err := client.Post(base+"/events", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("ingest event %d failed: %v", i, err)
		}
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("event %d: expected 201, got %d", i, resp.StatusCode)
		}

		var created auditEvent
		json.NewDecoder(resp.Body).Decode(&created)
		resp.Body.Close()
		if created.ID == "" {
			t.Fatalf("event %d: ID should be assigned", i)
		}
	}
	t.Logf("Ingested %d audit events", len(testEvents))

	// Step 2: Query events by actor
	resp, err := client.Get(base + "/events?actor=user-1")
	if err != nil {
		t.Fatalf("query events failed: %v", err)
	}
	defer resp.Body.Close()

	var queryResult struct {
		Events []auditEvent `json:"events"`
		Total  int          `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&queryResult)

	if queryResult.Total != 3 {
		t.Errorf("expected 3 events for user-1, got %d", queryResult.Total)
	}
	for _, evt := range queryResult.Events {
		if evt.Actor != "user-1" {
			t.Errorf("unexpected actor %s in user-1 query", evt.Actor)
		}
	}
	t.Logf("Query by actor returned %d events", queryResult.Total)

	// Query by action
	resp, err = client.Get(base + "/events?action=SIGN_DOCUMENT")
	if err != nil {
		t.Fatalf("query by action failed: %v", err)
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&queryResult)

	if queryResult.Total != 3 {
		t.Errorf("expected 3 SIGN_DOCUMENT events, got %d", queryResult.Total)
	}
	t.Logf("Query by action returned %d events", queryResult.Total)

	// Step 3: Generate compliance report
	resp, err = client.Get(base + "/reports/compliance")
	if err != nil {
		t.Fatalf("compliance report failed: %v", err)
	}
	defer resp.Body.Close()

	var report map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&report)

	totalEvents := int(report["total_events"].(float64))
	if totalEvents != 5 {
		t.Errorf("expected 5 total events in report, got %d", totalEvents)
	}

	if report["compliant"] != true {
		t.Error("report should be compliant when events exist")
	}

	actionsSummary := report["actions_summary"].(map[string]interface{})
	if int(actionsSummary["SIGN_DOCUMENT"].(float64)) != 3 {
		t.Errorf("expected 3 SIGN_DOCUMENT in summary, got %v", actionsSummary["SIGN_DOCUMENT"])
	}
	if int(actionsSummary["CREATE_APP"].(float64)) != 1 {
		t.Errorf("expected 1 CREATE_APP in summary, got %v", actionsSummary["CREATE_APP"])
	}

	actorsSummary := report["actors_summary"].(map[string]interface{})
	if int(actorsSummary["user-1"].(float64)) != 3 {
		t.Errorf("expected 3 events for user-1, got %v", actorsSummary["user-1"])
	}

	if report["generated_at"] == nil || report["generated_at"].(string) == "" {
		t.Error("report should have generated_at timestamp")
	}
	t.Logf("Compliance report: %d events, compliant=%v", totalEvents, report["compliant"])
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var idCounter struct {
	mu    sync.Mutex
	count int
}

func generateID(prefix string) string {
	idCounter.mu.Lock()
	idCounter.count++
	n := idCounter.count
	idCounter.mu.Unlock()
	return fmt.Sprintf("%s-%d", prefix, n)
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func fakePEM(cn string) string {
	// Generate a minimal self-signed certificate for testing.
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
	}
	_ = tmpl // In a real impl we'd sign; here we return a placeholder.
	return fmt.Sprintf("-----BEGIN CERTIFICATE-----\nFAKE-CERT-%s\n-----END CERTIFICATE-----", cn)
}

func fakeHash(data string) string {
	b := make([]byte, 32)
	// Deterministic-ish hash for testing.
	for i, c := range []byte(data) {
		b[i%32] ^= c
	}
	return hex.EncodeToString(b)
}
