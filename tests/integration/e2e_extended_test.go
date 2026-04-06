package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// In-memory stores for extended E2E tests
// ---------------------------------------------------------------------------

type identity struct {
	ID         string `json:"id"`
	NIK        string `json:"nik"`
	FullName   string `json:"full_name"`
	Status     string `json:"status"`
	VerifiedAt string `json:"verified_at,omitempty"`
}

type consent struct {
	ID        string `json:"id"`
	PersonID  string `json:"person_id"`
	Scope     string `json:"scope"`
	GrantedAt string `json:"granted_at"`
	RevokedAt string `json:"revoked_at,omitempty"`
	Active    bool   `json:"active"`
}

type corporateEntity struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	AHURef   string   `json:"ahu_ref"`
	Status   string   `json:"status"`
	Roles    []string `json:"roles"`
}

type notification struct {
	ID        string `json:"id"`
	Channel   string `json:"channel"`
	Recipient string `json:"recipient"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Status    string `json:"status"`
	SentAt    string `json:"sent_at"`
}

// ---------------------------------------------------------------------------
// Identity + Dukcapil sim server
// ---------------------------------------------------------------------------

func newIdentityServer() *httptest.Server {
	var (
		mu         sync.Mutex
		identities = make(map[string]*identity)
	)

	mux := http.NewServeMux()

	// POST /identities — register a new identity
	mux.HandleFunc("POST /identities", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			NIK      string `json:"nik"`
			FullName string `json:"full_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NIK == "" || req.FullName == "" {
			http.Error(w, `{"error":"nik and full_name required"}`, http.StatusBadRequest)
			return
		}

		id := generateID("idn")
		ident := &identity{
			ID:       id,
			NIK:      req.NIK,
			FullName: req.FullName,
			Status:   "PENDING",
		}

		mu.Lock()
		identities[id] = ident
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ident)
	})

	// POST /identities/{id}/verify — simulate Dukcapil verification
	mux.HandleFunc("POST /identities/{id}/verify", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		mu.Lock()
		ident, ok := identities[id]
		if ok {
			// Simulate Dukcapil lookup: NIK starting with "32" passes
			if strings.HasPrefix(ident.NIK, "32") {
				ident.Status = "VERIFIED"
				ident.VerifiedAt = time.Now().UTC().Format(time.RFC3339)
			} else {
				ident.Status = "REJECTED"
			}
		}
		mu.Unlock()

		if !ok {
			http.Error(w, `{"error":"identity not found"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ident)
	})

	// GET /identities/{id} — get identity status
	mux.HandleFunc("GET /identities/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		mu.Lock()
		ident, ok := identities[id]
		mu.Unlock()
		if !ok {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ident)
	})

	return httptest.NewServer(mux)
}

// ---------------------------------------------------------------------------
// Consent management server
// ---------------------------------------------------------------------------

func newConsentServer() *httptest.Server {
	var (
		mu       sync.Mutex
		consents = make(map[string]*consent)
	)

	mux := http.NewServeMux()

	// POST /consents — grant consent
	mux.HandleFunc("POST /consents", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			PersonID string `json:"person_id"`
			Scope    string `json:"scope"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PersonID == "" || req.Scope == "" {
			http.Error(w, `{"error":"person_id and scope required"}`, http.StatusBadRequest)
			return
		}

		id := generateID("cns")
		c := &consent{
			ID:        id,
			PersonID:  req.PersonID,
			Scope:     req.Scope,
			GrantedAt: time.Now().UTC().Format(time.RFC3339),
			Active:    true,
		}

		mu.Lock()
		consents[id] = c
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(c)
	})

	// GET /consents?person_id=... — list active consents
	mux.HandleFunc("GET /consents", func(w http.ResponseWriter, r *http.Request) {
		personID := r.URL.Query().Get("person_id")
		mu.Lock()
		var results []consent
		for _, c := range consents {
			if personID != "" && c.PersonID != personID {
				continue
			}
			if c.Active {
				results = append(results, *c)
			}
		}
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"consents": results,
			"total":    len(results),
		})
	})

	// GET /person-data/{person_id} — get person data filtered by consent
	mux.HandleFunc("GET /person-data/{person_id}", func(w http.ResponseWriter, r *http.Request) {
		personID := r.PathValue("person_id")

		mu.Lock()
		scopes := make(map[string]bool)
		for _, c := range consents {
			if c.PersonID == personID && c.Active {
				scopes[c.Scope] = true
			}
		}
		mu.Unlock()

		data := map[string]interface{}{
			"person_id": personID,
		}
		if scopes["basic_profile"] {
			data["name"] = "Budi Santoso"
			data["nik"] = "3201****"
		}
		if scopes["contact_info"] {
			data["email"] = "budi@example.com"
			data["phone"] = "+62812****"
		}
		if scopes["address"] {
			data["address"] = "Jl. Merdeka No. 1, Jakarta"
		}
		data["granted_scopes"] = func() []string {
			var s []string
			for k := range scopes {
				s = append(s, k)
			}
			return s
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	})

	// DELETE /consents/{id} — revoke consent
	mux.HandleFunc("DELETE /consents/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		mu.Lock()
		c, ok := consents[id]
		if ok {
			c.Active = false
			c.RevokedAt = time.Now().UTC().Format(time.RFC3339)
		}
		mu.Unlock()

		if !ok {
			http.Error(w, `{"error":"consent not found"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "REVOKED"})
	})

	return httptest.NewServer(mux)
}

// ---------------------------------------------------------------------------
// Corporate entity server (AHU integration sim)
// ---------------------------------------------------------------------------

func newCorporateServer() *httptest.Server {
	var (
		mu       sync.Mutex
		entities = make(map[string]*corporateEntity)
	)

	mux := http.NewServeMux()

	// POST /entities — register corporate entity via AHU
	mux.HandleFunc("POST /entities", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name   string `json:"name"`
			AHURef string `json:"ahu_ref"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.AHURef == "" {
			http.Error(w, `{"error":"name and ahu_ref required"}`, http.StatusBadRequest)
			return
		}

		id := generateID("ent")
		entity := &corporateEntity{
			ID:     id,
			Name:   req.Name,
			AHURef: req.AHURef,
			Status: "REGISTERED",
			Roles:  []string{},
		}

		mu.Lock()
		entities[id] = entity
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(entity)
	})

	// POST /entities/{id}/roles — assign role
	mux.HandleFunc("POST /entities/{id}/roles", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req struct {
			Role string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Role == "" {
			http.Error(w, `{"error":"role required"}`, http.StatusBadRequest)
			return
		}

		mu.Lock()
		entity, ok := entities[id]
		if ok {
			entity.Roles = append(entity.Roles, req.Role)
		}
		mu.Unlock()

		if !ok {
			http.Error(w, `{"error":"entity not found"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entity)
	})

	// GET /entities/{id}/roles — list roles
	mux.HandleFunc("GET /entities/{id}/roles", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		mu.Lock()
		entity, ok := entities[id]
		mu.Unlock()
		if !ok {
			http.Error(w, `{"error":"entity not found"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"entity_id": id,
			"roles":     entity.Roles,
			"total":     len(entity.Roles),
		})
	})

	// GET /entities/{id} — get entity profile
	mux.HandleFunc("GET /entities/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		mu.Lock()
		entity, ok := entities[id]
		mu.Unlock()
		if !ok {
			http.Error(w, `{"error":"entity not found"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entity)
	})

	return httptest.NewServer(mux)
}

// ---------------------------------------------------------------------------
// Notification server
// ---------------------------------------------------------------------------

func newNotificationServer() *httptest.Server {
	var (
		mu            sync.Mutex
		notifications []notification
	)

	mux := http.NewServeMux()

	// POST /notifications/send — send notification
	mux.HandleFunc("POST /notifications/send", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Channel   string `json:"channel"`
			Recipient string `json:"recipient"`
			Subject   string `json:"subject"`
			Body      string `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Channel == "" || req.Recipient == "" {
			http.Error(w, `{"error":"channel and recipient required"}`, http.StatusBadRequest)
			return
		}

		notif := notification{
			ID:        generateID("ntf"),
			Channel:   req.Channel,
			Recipient: req.Recipient,
			Subject:   req.Subject,
			Body:      req.Body,
			Status:    "DELIVERED",
			SentAt:    time.Now().UTC().Format(time.RFC3339),
		}

		mu.Lock()
		notifications = append(notifications, notif)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(notif)
	})

	// GET /notifications?recipient=... — list notifications
	mux.HandleFunc("GET /notifications", func(w http.ResponseWriter, r *http.Request) {
		recipient := r.URL.Query().Get("recipient")
		channel := r.URL.Query().Get("channel")

		mu.Lock()
		var results []notification
		for _, n := range notifications {
			if recipient != "" && n.Recipient != recipient {
				continue
			}
			if channel != "" && n.Channel != channel {
				continue
			}
			results = append(results, n)
		}
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"notifications": results,
			"total":         len(results),
		})
	})

	return httptest.NewServer(mux)
}

// ---------------------------------------------------------------------------
// Health aggregation server
// ---------------------------------------------------------------------------

func newHealthServer(name string) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"service": name,
			"checks": map[string]string{
				"database": "ok",
				"cache":    "ok",
			},
		})
	})
	return httptest.NewServer(mux)
}

func newHealthAggregator(services map[string]string) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		client := &http.Client{Timeout: 2 * time.Second}
		results := make(map[string]interface{})
		allOK := true

		for name, url := range services {
			resp, err := client.Get(url + "/health")
			if err != nil {
				results[name] = map[string]string{"status": "unreachable", "error": err.Error()}
				allOK = false
				continue
			}
			var health map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&health)
			resp.Body.Close()
			results[name] = health
			if health["status"] != "ok" {
				allOK = false
			}
		}

		status := "ok"
		if !allOK {
			status = "degraded"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":       status,
			"services":     results,
			"total":        len(services),
			"checked_at":   time.Now().UTC().Format(time.RFC3339),
		})
	})
	return httptest.NewServer(mux)
}

// ---------------------------------------------------------------------------
// E2E Tests
// ---------------------------------------------------------------------------

// TestE2E_IdentityRegistrationFlow tests the full identity flow:
// Create user -> verify via Dukcapil sim -> check status.
func TestE2E_IdentityRegistrationFlow(t *testing.T) {
	srv := newIdentityServer()
	defer srv.Close()

	client := srv.Client()
	base := srv.URL

	// Step 1: Register identity with valid NIK (starts with "32" = West Java)
	body := `{"nik":"3201012345678901","full_name":"Budi Santoso"}`
	resp, err := client.Post(base+"/identities", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("register identity failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var ident identity
	json.NewDecoder(resp.Body).Decode(&ident)
	if ident.ID == "" {
		t.Fatal("identity ID should be assigned")
	}
	if ident.Status != "PENDING" {
		t.Fatalf("expected PENDING, got %s", ident.Status)
	}
	t.Logf("Identity registered: %s (NIK: %s)", ident.ID, ident.NIK)

	// Step 2: Verify via Dukcapil simulation
	resp, err = client.Post(base+"/identities/"+ident.ID+"/verify", "application/json", nil)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	defer resp.Body.Close()

	var verified identity
	json.NewDecoder(resp.Body).Decode(&verified)
	if verified.Status != "VERIFIED" {
		t.Fatalf("expected VERIFIED, got %s", verified.Status)
	}
	if verified.VerifiedAt == "" {
		t.Error("verified_at should be set")
	}
	t.Logf("Identity verified at: %s", verified.VerifiedAt)

	// Step 3: Check status
	resp, err = client.Get(base + "/identities/" + ident.ID)
	if err != nil {
		t.Fatalf("get identity failed: %v", err)
	}
	defer resp.Body.Close()

	var checked identity
	json.NewDecoder(resp.Body).Decode(&checked)
	if checked.Status != "VERIFIED" {
		t.Errorf("expected VERIFIED, got %s", checked.Status)
	}
	if checked.FullName != "Budi Santoso" {
		t.Errorf("expected full name Budi Santoso, got %s", checked.FullName)
	}
	t.Logf("Identity status confirmed: %s", checked.Status)

	// Step 4: Test rejected flow (NIK not starting with "32")
	body = `{"nik":"1101012345678901","full_name":"Andi Prasetyo"}`
	resp, err = client.Post(base+"/identities", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("register second identity failed: %v", err)
	}
	defer resp.Body.Close()

	var ident2 identity
	json.NewDecoder(resp.Body).Decode(&ident2)

	resp, err = client.Post(base+"/identities/"+ident2.ID+"/verify", "application/json", nil)
	if err != nil {
		t.Fatalf("verify second identity failed: %v", err)
	}
	defer resp.Body.Close()

	var rejected identity
	json.NewDecoder(resp.Body).Decode(&rejected)
	if rejected.Status != "REJECTED" {
		t.Errorf("expected REJECTED for non-32 NIK, got %s", rejected.Status)
	}
	t.Logf("Non-32 NIK correctly rejected: %s", rejected.Status)
}

// TestE2E_ConsentLifecycle tests consent management:
// Grant consent -> list active -> check person data filtered -> revoke -> verify revoked.
func TestE2E_ConsentLifecycle(t *testing.T) {
	srv := newConsentServer()
	defer srv.Close()

	client := srv.Client()
	base := srv.URL
	personID := "person-42"

	// Step 1: Grant basic_profile consent
	body := fmt.Sprintf(`{"person_id":"%s","scope":"basic_profile"}`, personID)
	resp, err := client.Post(base+"/consents", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("grant consent failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var c1 consent
	json.NewDecoder(resp.Body).Decode(&c1)
	if !c1.Active {
		t.Error("consent should be active")
	}
	t.Logf("Consent granted: %s (scope: %s)", c1.ID, c1.Scope)

	// Grant contact_info consent
	body = fmt.Sprintf(`{"person_id":"%s","scope":"contact_info"}`, personID)
	resp, err = client.Post(base+"/consents", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("grant second consent failed: %v", err)
	}
	defer resp.Body.Close()

	var c2 consent
	json.NewDecoder(resp.Body).Decode(&c2)
	t.Logf("Second consent granted: %s (scope: %s)", c2.ID, c2.Scope)

	// Step 2: List active consents
	resp, err = client.Get(base + "/consents?person_id=" + personID)
	if err != nil {
		t.Fatalf("list consents failed: %v", err)
	}
	defer resp.Body.Close()

	var listResult struct {
		Consents []consent `json:"consents"`
		Total    int       `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&listResult)
	if listResult.Total != 2 {
		t.Errorf("expected 2 active consents, got %d", listResult.Total)
	}
	t.Logf("Active consents: %d", listResult.Total)

	// Step 3: Check person data — should include basic_profile and contact_info
	resp, err = client.Get(base + "/person-data/" + personID)
	if err != nil {
		t.Fatalf("get person data failed: %v", err)
	}
	defer resp.Body.Close()

	var personData map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&personData)

	if personData["name"] == nil {
		t.Error("person data should include name (basic_profile consented)")
	}
	if personData["email"] == nil {
		t.Error("person data should include email (contact_info consented)")
	}
	if personData["address"] != nil {
		t.Error("person data should NOT include address (no address consent)")
	}
	t.Logf("Person data filtered by consent: name=%v, email=%v", personData["name"], personData["email"])

	// Step 4: Revoke contact_info consent
	req, _ := http.NewRequest(http.MethodDelete, base+"/consents/"+c2.ID, nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("revoke consent failed: %v", err)
	}
	defer resp.Body.Close()

	var revokeResult map[string]string
	json.NewDecoder(resp.Body).Decode(&revokeResult)
	if revokeResult["status"] != "REVOKED" {
		t.Errorf("expected REVOKED, got %s", revokeResult["status"])
	}
	t.Logf("Consent %s revoked", c2.ID)

	// Step 5: Verify revoked — list should show only 1
	resp, err = client.Get(base + "/consents?person_id=" + personID)
	if err != nil {
		t.Fatalf("list consents after revoke failed: %v", err)
	}
	defer resp.Body.Close()

	json.NewDecoder(resp.Body).Decode(&listResult)
	if listResult.Total != 1 {
		t.Errorf("expected 1 active consent after revoke, got %d", listResult.Total)
	}

	// Person data should no longer include contact_info
	resp, err = client.Get(base + "/person-data/" + personID)
	if err != nil {
		t.Fatalf("get person data after revoke failed: %v", err)
	}
	defer resp.Body.Close()

	var personDataAfterRevoke map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&personDataAfterRevoke)
	if personDataAfterRevoke["name"] == nil {
		t.Error("basic_profile data should still be present")
	}
	if personDataAfterRevoke["email"] != nil {
		t.Error("contact_info data should be removed after revoke")
	}
	t.Logf("Person data correctly filtered after revocation")
}

// TestE2E_CorporateRegistrationFlow tests corporate identity:
// Register entity via AHU -> assign role -> list roles -> get entity profile.
func TestE2E_CorporateRegistrationFlow(t *testing.T) {
	srv := newCorporateServer()
	defer srv.Close()

	client := srv.Client()
	base := srv.URL

	// Step 1: Register entity via AHU
	body := `{"name":"PT Garuda Digital","ahu_ref":"AHU-2024-001234"}`
	resp, err := client.Post(base+"/entities", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("register entity failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var entity corporateEntity
	json.NewDecoder(resp.Body).Decode(&entity)
	if entity.ID == "" {
		t.Fatal("entity ID should be assigned")
	}
	if entity.Status != "REGISTERED" {
		t.Fatalf("expected REGISTERED, got %s", entity.Status)
	}
	if entity.AHURef != "AHU-2024-001234" {
		t.Errorf("expected AHU ref AHU-2024-001234, got %s", entity.AHURef)
	}
	t.Logf("Entity registered: %s (AHU: %s)", entity.ID, entity.AHURef)

	// Step 2: Assign roles
	roles := []string{"DIRECTOR", "COMMISSIONER", "SHAREHOLDER"}
	for _, role := range roles {
		roleBody := fmt.Sprintf(`{"role":"%s"}`, role)
		resp, err = client.Post(base+"/entities/"+entity.ID+"/roles", "application/json", strings.NewReader(roleBody))
		if err != nil {
			t.Fatalf("assign role %s failed: %v", role, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for role assignment, got %d", resp.StatusCode)
		}
	}
	t.Logf("Assigned %d roles", len(roles))

	// Step 3: List roles
	resp, err = client.Get(base + "/entities/" + entity.ID + "/roles")
	if err != nil {
		t.Fatalf("list roles failed: %v", err)
	}
	defer resp.Body.Close()

	var rolesResult struct {
		EntityID string   `json:"entity_id"`
		Roles    []string `json:"roles"`
		Total    int      `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&rolesResult)

	if rolesResult.Total != 3 {
		t.Errorf("expected 3 roles, got %d", rolesResult.Total)
	}
	for _, expected := range roles {
		found := false
		for _, got := range rolesResult.Roles {
			if got == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("role %s not found in list", expected)
		}
	}
	t.Logf("Roles listed: %v", rolesResult.Roles)

	// Step 4: Get entity profile
	resp, err = client.Get(base + "/entities/" + entity.ID)
	if err != nil {
		t.Fatalf("get entity profile failed: %v", err)
	}
	defer resp.Body.Close()

	var profile corporateEntity
	json.NewDecoder(resp.Body).Decode(&profile)
	if profile.Name != "PT Garuda Digital" {
		t.Errorf("expected name PT Garuda Digital, got %s", profile.Name)
	}
	if len(profile.Roles) != 3 {
		t.Errorf("expected 3 roles in profile, got %d", len(profile.Roles))
	}
	t.Logf("Entity profile: %s with %d roles", profile.Name, len(profile.Roles))
}

// TestE2E_NotificationFlow tests notification delivery:
// Send OTP via email -> send alert -> verify recorded.
func TestE2E_NotificationFlow(t *testing.T) {
	srv := newNotificationServer()
	defer srv.Close()

	client := srv.Client()
	base := srv.URL
	recipient := "budi@example.com"

	// Step 1: Send OTP via email
	body := fmt.Sprintf(`{
		"channel": "email",
		"recipient": "%s",
		"subject": "Your OTP Code",
		"body": "Your verification code is 123456. Valid for 5 minutes."
	}`, recipient)
	resp, err := client.Post(base+"/notifications/send", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("send OTP failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var otp notification
	json.NewDecoder(resp.Body).Decode(&otp)
	if otp.Status != "DELIVERED" {
		t.Errorf("expected DELIVERED, got %s", otp.Status)
	}
	if otp.Channel != "email" {
		t.Errorf("expected email channel, got %s", otp.Channel)
	}
	t.Logf("OTP sent: %s to %s", otp.ID, otp.Recipient)

	// Step 2: Send alert notification
	body = fmt.Sprintf(`{
		"channel": "email",
		"recipient": "%s",
		"subject": "Security Alert",
		"body": "A new login was detected from Jakarta, Indonesia."
	}`, recipient)
	resp, err = client.Post(base+"/notifications/send", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("send alert failed: %v", err)
	}
	defer resp.Body.Close()

	var alert notification
	json.NewDecoder(resp.Body).Decode(&alert)
	if alert.Status != "DELIVERED" {
		t.Errorf("expected DELIVERED, got %s", alert.Status)
	}
	t.Logf("Alert sent: %s", alert.ID)

	// Step 3: Send SMS notification to different channel
	body = `{
		"channel": "sms",
		"recipient": "+6281234567890",
		"subject": "OTP",
		"body": "Code: 654321"
	}`
	resp, err = client.Post(base+"/notifications/send", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("send SMS failed: %v", err)
	}
	defer resp.Body.Close()

	var sms notification
	json.NewDecoder(resp.Body).Decode(&sms)
	if sms.Channel != "sms" {
		t.Errorf("expected sms channel, got %s", sms.Channel)
	}
	t.Logf("SMS sent: %s", sms.ID)

	// Step 4: Verify all notifications recorded for email recipient
	resp, err = client.Get(base + "/notifications?recipient=" + recipient)
	if err != nil {
		t.Fatalf("list notifications failed: %v", err)
	}
	defer resp.Body.Close()

	var listResult struct {
		Notifications []notification `json:"notifications"`
		Total         int            `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&listResult)

	if listResult.Total != 2 {
		t.Errorf("expected 2 notifications for %s, got %d", recipient, listResult.Total)
	}
	t.Logf("Notifications recorded for %s: %d", recipient, listResult.Total)

	// Verify by channel filter
	resp, err = client.Get(base + "/notifications?channel=sms")
	if err != nil {
		t.Fatalf("list SMS notifications failed: %v", err)
	}
	defer resp.Body.Close()

	json.NewDecoder(resp.Body).Decode(&listResult)
	if listResult.Total != 1 {
		t.Errorf("expected 1 SMS notification, got %d", listResult.Total)
	}
	t.Logf("SMS notifications: %d", listResult.Total)
}

// TestE2E_HealthCheckAllServices tests health aggregation:
// Start multiple services -> check aggregated health -> verify all report ok.
func TestE2E_HealthCheckAllServices(t *testing.T) {
	// Start individual service health endpoints
	identitySrv := newHealthServer("identity-service")
	defer identitySrv.Close()

	consentSrv := newHealthServer("consent-service")
	defer consentSrv.Close()

	notifSrv := newHealthServer("notification-service")
	defer notifSrv.Close()

	corporateSrv := newHealthServer("corporate-service")
	defer corporateSrv.Close()

	// Start aggregator
	services := map[string]string{
		"identity":     identitySrv.URL,
		"consent":      consentSrv.URL,
		"notification": notifSrv.URL,
		"corporate":    corporateSrv.URL,
	}
	aggregator := newHealthAggregator(services)
	defer aggregator.Close()

	client := aggregator.Client()

	// Step 1: Check aggregated health
	resp, err := client.Get(aggregator.URL + "/health")
	if err != nil {
		t.Fatalf("aggregated health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var health map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&health)

	// Step 2: Verify overall status is ok
	if health["status"] != "ok" {
		t.Errorf("expected overall status ok, got %v", health["status"])
	}

	total := int(health["total"].(float64))
	if total != 4 {
		t.Errorf("expected 4 services, got %d", total)
	}

	// Step 3: Verify each service reports ok
	svcResults := health["services"].(map[string]interface{})
	for name, result := range svcResults {
		svcHealth := result.(map[string]interface{})
		if svcHealth["status"] != "ok" {
			t.Errorf("service %s: expected ok, got %v", name, svcHealth["status"])
		}
	}

	if health["checked_at"] == nil {
		t.Error("checked_at should be present")
	}

	t.Logf("All %d services healthy, status=%s", total, health["status"])

	// Step 4: Test with a stopped service (degraded state)
	consentSrv.Close()

	resp, err = client.Get(aggregator.URL + "/health")
	if err != nil {
		t.Fatalf("degraded health check failed: %v", err)
	}
	defer resp.Body.Close()

	json.NewDecoder(resp.Body).Decode(&health)
	if health["status"] != "degraded" {
		t.Errorf("expected degraded status when service is down, got %v", health["status"])
	}
	t.Logf("Correctly detected degraded state: %s", health["status"])
}
