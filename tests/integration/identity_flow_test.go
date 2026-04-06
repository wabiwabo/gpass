package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// In-memory stores for identity flow tests
// ---------------------------------------------------------------------------

type otpRecord struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Code      string `json:"code"`
	Channel   string `json:"channel"`
	ExpiresAt string `json:"expires_at"`
	Verified  bool   `json:"verified"`
}

type verifiedUser struct {
	ID         string `json:"id"`
	NIK        string `json:"nik"`
	FullName   string `json:"full_name"`
	Status     string `json:"status"`
	OTPVerified bool  `json:"otp_verified"`
	VerifiedAt string `json:"verified_at,omitempty"`
}

type consentRecord struct {
	ID        string   `json:"id"`
	PersonID  string   `json:"person_id"`
	Scope     string   `json:"scope"`
	Fields    []string `json:"fields"`
	GrantedAt string   `json:"granted_at"`
	RevokedAt string   `json:"revoked_at,omitempty"`
	Active    bool     `json:"active"`
}

type uboShareholder struct {
	Name       string  `json:"name"`
	NIK        string  `json:"nik"`
	Percentage float64 `json:"percentage"`
	IsUBO      bool    `json:"is_ubo"`
}

type corpEntity struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	AHURef       string           `json:"ahu_ref"`
	OSSRef       string           `json:"oss_ref,omitempty"`
	NIBNumber    string           `json:"nib_number,omitempty"`
	Status       string           `json:"status"`
	Shareholders []uboShareholder `json:"shareholders"`
}

type batchResult struct {
	BatchID    string `json:"batch_id"`
	Total      int    `json:"total"`
	Delivered  int    `json:"delivered"`
	Failed     int    `json:"failed"`
	Status     string `json:"status"`
}

type notifRecord struct {
	ID        string `json:"id"`
	Channel   string `json:"channel"`
	Recipient string `json:"recipient"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Status    string `json:"status"`
	BatchID   string `json:"batch_id,omitempty"`
	SentAt    string `json:"sent_at"`
}

// ---------------------------------------------------------------------------
// Identity + Dukcapil + OTP server
// ---------------------------------------------------------------------------

func newIdentityOTPServer() *httptest.Server {
	var (
		mu    sync.Mutex
		users = make(map[string]*verifiedUser)
		otps  = make(map[string]*otpRecord)
	)

	// Simulated Dukcapil population data
	dukcapilDB := map[string]string{
		"3201012345670001": "Budi Santoso",
		"3201012345670002": "Siti Rahayu",
		"3201012345670003": "Ahmad Hidayat",
	}

	mux := http.NewServeMux()

	// POST /users/register — register with NIK
	mux.HandleFunc("POST /users/register", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			NIK      string `json:"nik"`
			FullName string `json:"full_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NIK == "" || req.FullName == "" {
			http.Error(w, `{"error":"nik and full_name required"}`, http.StatusBadRequest)
			return
		}

		id := generateID("usr")
		u := &verifiedUser{
			ID:       id,
			NIK:      req.NIK,
			FullName: req.FullName,
			Status:   "PENDING",
		}

		mu.Lock()
		users[id] = u
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(u)
	})

	// POST /users/{id}/dukcapil-verify — verify against Dukcapil sim
	mux.HandleFunc("POST /users/{id}/dukcapil-verify", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		mu.Lock()
		u, ok := users[id]
		mu.Unlock()
		if !ok {
			http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
			return
		}

		// Check against Dukcapil simulated data
		registeredName, found := dukcapilDB[u.NIK]
		if !found {
			mu.Lock()
			u.Status = "NIK_NOT_FOUND"
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "NIK_NOT_FOUND",
				"user_id": id,
				"message": "NIK not found in population database",
			})
			return
		}

		if registeredName != u.FullName {
			mu.Lock()
			u.Status = "NAME_MISMATCH"
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "NAME_MISMATCH",
				"user_id": id,
				"message": "Name does not match population record",
			})
			return
		}

		mu.Lock()
		u.Status = "DUKCAPIL_VERIFIED"
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "DUKCAPIL_VERIFIED",
			"user_id": id,
		})
	})

	// POST /users/{id}/otp/send — send OTP
	mux.HandleFunc("POST /users/{id}/otp/send", func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("id")
		var req struct {
			Channel string `json:"channel"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Channel == "" {
			http.Error(w, `{"error":"channel required (email or sms)"}`, http.StatusBadRequest)
			return
		}

		mu.Lock()
		u, ok := users[userID]
		mu.Unlock()
		if !ok {
			http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
			return
		}
		if u.Status != "DUKCAPIL_VERIFIED" {
			http.Error(w, `{"error":"user must be Dukcapil-verified first"}`, http.StatusConflict)
			return
		}

		otpID := generateID("otp")
		otp := &otpRecord{
			ID:        otpID,
			UserID:    userID,
			Code:      "123456", // Fixed code for testing
			Channel:   req.Channel,
			ExpiresAt: time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339),
			Verified:  false,
		}

		mu.Lock()
		otps[otpID] = otp
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"otp_id":     otpID,
			"channel":    req.Channel,
			"expires_at": otp.ExpiresAt,
		})
	})

	// POST /users/{id}/otp/verify — verify OTP code
	mux.HandleFunc("POST /users/{id}/otp/verify", func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("id")
		var req struct {
			OTPID string `json:"otp_id"`
			Code  string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OTPID == "" || req.Code == "" {
			http.Error(w, `{"error":"otp_id and code required"}`, http.StatusBadRequest)
			return
		}

		mu.Lock()
		otp, otpOK := otps[req.OTPID]
		u, userOK := users[userID]

		if !otpOK || !userOK {
			mu.Unlock()
			http.Error(w, `{"error":"invalid otp or user"}`, http.StatusNotFound)
			return
		}

		if otp.UserID != userID {
			mu.Unlock()
			http.Error(w, `{"error":"otp does not belong to this user"}`, http.StatusForbidden)
			return
		}

		if otp.Code != req.Code {
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"verified": false,
				"message":  "invalid OTP code",
			})
			return
		}

		otp.Verified = true
		u.OTPVerified = true
		u.Status = "VERIFIED"
		u.VerifiedAt = time.Now().UTC().Format(time.RFC3339)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"verified":    true,
			"user_id":     userID,
			"verified_at": u.VerifiedAt,
		})
	})

	// GET /users/{id} — get user status
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		mu.Lock()
		u, ok := users[id]
		mu.Unlock()
		if !ok {
			http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(u)
	})

	return httptest.NewServer(mux)
}

// ---------------------------------------------------------------------------
// Consent management server with field-level control
// ---------------------------------------------------------------------------

func newFieldConsentServer() *httptest.Server {
	var (
		mu       sync.Mutex
		consents = make(map[string]*consentRecord)
	)

	// Simulated person data store
	personStore := map[string]map[string]interface{}{
		"person-100": {
			"full_name":    "Budi Santoso",
			"nik":          "3201****",
			"birth_date":   "1990-01-15",
			"email":        "budi@example.com",
			"phone":        "+62812345678",
			"address":      "Jl. Merdeka No. 1, Jakarta",
			"blood_type":   "O",
			"religion":     "Islam",
			"marital":      "Married",
			"occupation":   "Engineer",
		},
	}

	// Define which fields belong to which scope
	scopeFields := map[string][]string{
		"basic_profile": {"full_name", "nik", "birth_date"},
		"contact_info":  {"email", "phone"},
		"address":       {"address"},
		"demographics":  {"blood_type", "religion", "marital", "occupation"},
	}

	mux := http.NewServeMux()

	// POST /consents — grant consent for specific fields
	mux.HandleFunc("POST /consents", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			PersonID string `json:"person_id"`
			Scope    string `json:"scope"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PersonID == "" || req.Scope == "" {
			http.Error(w, `{"error":"person_id and scope required"}`, http.StatusBadRequest)
			return
		}

		fields, validScope := scopeFields[req.Scope]
		if !validScope {
			http.Error(w, `{"error":"invalid scope"}`, http.StatusBadRequest)
			return
		}

		id := generateID("fcn")
		c := &consentRecord{
			ID:        id,
			PersonID:  req.PersonID,
			Scope:     req.Scope,
			Fields:    fields,
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

	// GET /consents — list consents for a person
	mux.HandleFunc("GET /consents", func(w http.ResponseWriter, r *http.Request) {
		personID := r.URL.Query().Get("person_id")
		mu.Lock()
		var results []consentRecord
		for _, c := range consents {
			if personID != "" && c.PersonID != personID {
				continue
			}
			results = append(results, *c)
		}
		mu.Unlock()

		active := 0
		for _, c := range results {
			if c.Active {
				active++
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"consents":     results,
			"total":        len(results),
			"total_active": active,
		})
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
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "REVOKED",
			"consent_id": id,
			"revoked_at": c.RevokedAt,
		})
	})

	// GET /person-data/{person_id} — get person data filtered by active consents
	mux.HandleFunc("GET /person-data/{person_id}", func(w http.ResponseWriter, r *http.Request) {
		personID := r.PathValue("person_id")

		allData, exists := personStore[personID]
		if !exists {
			http.Error(w, `{"error":"person not found"}`, http.StatusNotFound)
			return
		}

		mu.Lock()
		allowedFields := make(map[string]bool)
		var activeScopes []string
		for _, c := range consents {
			if c.PersonID == personID && c.Active {
				activeScopes = append(activeScopes, c.Scope)
				for _, f := range c.Fields {
					allowedFields[f] = true
				}
			}
		}
		mu.Unlock()

		data := map[string]interface{}{
			"person_id":      personID,
			"granted_scopes": activeScopes,
		}
		for field, value := range allData {
			if allowedFields[field] {
				data[field] = value
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	})

	return httptest.NewServer(mux)
}

// ---------------------------------------------------------------------------
// Corporate + AHU + OSS + UBO server
// ---------------------------------------------------------------------------

func newCorporateUBOServer() *httptest.Server {
	var (
		mu       sync.Mutex
		entities = make(map[string]*corpEntity)
	)

	// Simulated AHU database
	ahuDB := map[string]string{
		"AHU-2024-001234": "PT Garuda Digital Nusantara",
		"AHU-2024-005678": "PT Merdeka Teknologi",
	}

	// Simulated OSS/BKPM NIB database
	ossDB := map[string]map[string]string{
		"AHU-2024-001234": {
			"nib_number":    "1234567890123",
			"business_type": "Technology",
			"oss_status":    "ACTIVE",
		},
		"AHU-2024-005678": {
			"nib_number":    "9876543210987",
			"business_type": "Consulting",
			"oss_status":    "ACTIVE",
		},
	}

	mux := http.NewServeMux()

	// POST /entities/register — register with AHU reference
	mux.HandleFunc("POST /entities/register", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name   string `json:"name"`
			AHURef string `json:"ahu_ref"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.AHURef == "" {
			http.Error(w, `{"error":"name and ahu_ref required"}`, http.StatusBadRequest)
			return
		}

		id := generateID("corp")
		entity := &corpEntity{
			ID:           id,
			Name:         req.Name,
			AHURef:       req.AHURef,
			Status:       "PENDING",
			Shareholders: []uboShareholder{},
		}

		mu.Lock()
		entities[id] = entity
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(entity)
	})

	// POST /entities/{id}/verify-ahu — verify against AHU sim
	mux.HandleFunc("POST /entities/{id}/verify-ahu", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		mu.Lock()
		entity, ok := entities[id]
		mu.Unlock()
		if !ok {
			http.Error(w, `{"error":"entity not found"}`, http.StatusNotFound)
			return
		}

		ahuName, found := ahuDB[entity.AHURef]
		if !found {
			mu.Lock()
			entity.Status = "AHU_NOT_FOUND"
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "AHU_NOT_FOUND",
				"message": "AHU reference not found in legal entity database",
			})
			return
		}

		if ahuName != entity.Name {
			mu.Lock()
			entity.Status = "NAME_MISMATCH"
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "NAME_MISMATCH",
				"message": "Entity name does not match AHU record",
			})
			return
		}

		mu.Lock()
		entity.Status = "AHU_VERIFIED"
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "AHU_VERIFIED",
			"entity_id": id,
			"ahu_name":  ahuName,
		})
	})

	// POST /entities/{id}/cross-reference-oss — cross-reference with OSS/BKPM
	mux.HandleFunc("POST /entities/{id}/cross-reference-oss", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		mu.Lock()
		entity, ok := entities[id]
		mu.Unlock()
		if !ok {
			http.Error(w, `{"error":"entity not found"}`, http.StatusNotFound)
			return
		}

		ossData, found := ossDB[entity.AHURef]
		if !found {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "OSS_NOT_FOUND",
				"message": "No OSS/NIB record found for this AHU reference",
			})
			return
		}

		mu.Lock()
		entity.OSSRef = ossData["oss_status"]
		entity.NIBNumber = ossData["nib_number"]
		if entity.Status == "AHU_VERIFIED" {
			entity.Status = "FULLY_VERIFIED"
		}
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":        entity.Status,
			"entity_id":     id,
			"nib_number":    ossData["nib_number"],
			"business_type": ossData["business_type"],
			"oss_status":    ossData["oss_status"],
		})
	})

	// POST /entities/{id}/shareholders — add shareholder for UBO analysis
	mux.HandleFunc("POST /entities/{id}/shareholders", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var req struct {
			Name       string  `json:"name"`
			NIK        string  `json:"nik"`
			Percentage float64 `json:"percentage"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.Percentage <= 0 {
			http.Error(w, `{"error":"name and percentage required"}`, http.StatusBadRequest)
			return
		}

		mu.Lock()
		entity, ok := entities[id]
		if ok {
			sh := uboShareholder{
				Name:       req.Name,
				NIK:        req.NIK,
				Percentage: req.Percentage,
				IsUBO:      req.Percentage >= 25.0, // PP 13/2018: 25% threshold
			}
			entity.Shareholders = append(entity.Shareholders, sh)
		}
		mu.Unlock()

		if !ok {
			http.Error(w, `{"error":"entity not found"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(entity)
	})

	// GET /entities/{id}/ubo-analysis — analyze beneficial ownership
	mux.HandleFunc("GET /entities/{id}/ubo-analysis", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		mu.Lock()
		entity, ok := entities[id]
		mu.Unlock()
		if !ok {
			http.Error(w, `{"error":"entity not found"}`, http.StatusNotFound)
			return
		}

		var ubos []uboShareholder
		var totalPct float64
		for _, sh := range entity.Shareholders {
			totalPct += sh.Percentage
			if sh.IsUBO {
				ubos = append(ubos, sh)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"entity_id":          id,
			"entity_name":        entity.Name,
			"total_shareholders": len(entity.Shareholders),
			"total_percentage":   totalPct,
			"ubo_count":          len(ubos),
			"ubos":               ubos,
			"ubo_threshold":      25.0,
			"compliant":          len(ubos) > 0,
		})
	})

	// GET /entities/{id} — get entity details
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
// Notification server with batch support
// ---------------------------------------------------------------------------

func newBatchNotificationServer() *httptest.Server {
	var (
		mu     sync.Mutex
		notifs []notifRecord
	)

	mux := http.NewServeMux()

	// POST /notifications/send — send single notification
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

		n := notifRecord{
			ID:        generateID("nrf"),
			Channel:   req.Channel,
			Recipient: req.Recipient,
			Subject:   req.Subject,
			Body:      req.Body,
			Status:    "DELIVERED",
			SentAt:    time.Now().UTC().Format(time.RFC3339),
		}

		mu.Lock()
		notifs = append(notifs, n)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(n)
	})

	// POST /notifications/batch — send batch notifications
	mux.HandleFunc("POST /notifications/batch", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Notifications []struct {
				Channel   string `json:"channel"`
				Recipient string `json:"recipient"`
				Subject   string `json:"subject"`
				Body      string `json:"body"`
			} `json:"notifications"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Notifications) == 0 {
			http.Error(w, `{"error":"notifications array required"}`, http.StatusBadRequest)
			return
		}

		batchID := generateID("batch")
		delivered := 0
		failed := 0

		mu.Lock()
		for _, item := range req.Notifications {
			status := "DELIVERED"
			if item.Recipient == "" || item.Channel == "" {
				status = "FAILED"
				failed++
			} else {
				delivered++
			}
			n := notifRecord{
				ID:        generateID("nrf"),
				Channel:   item.Channel,
				Recipient: item.Recipient,
				Subject:   item.Subject,
				Body:      item.Body,
				Status:    status,
				BatchID:   batchID,
				SentAt:    time.Now().UTC().Format(time.RFC3339),
			}
			notifs = append(notifs, n)
		}
		mu.Unlock()

		result := batchResult{
			BatchID:   batchID,
			Total:     len(req.Notifications),
			Delivered: delivered,
			Failed:    failed,
			Status:    "COMPLETED",
		}
		if failed > 0 && delivered == 0 {
			result.Status = "FAILED"
		} else if failed > 0 {
			result.Status = "PARTIAL"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(result)
	})

	// GET /notifications — list notifications with filters
	mux.HandleFunc("GET /notifications", func(w http.ResponseWriter, r *http.Request) {
		recipient := r.URL.Query().Get("recipient")
		channel := r.URL.Query().Get("channel")
		batchID := r.URL.Query().Get("batch_id")

		mu.Lock()
		var results []notifRecord
		for _, n := range notifs {
			if recipient != "" && n.Recipient != recipient {
				continue
			}
			if channel != "" && n.Channel != channel {
				continue
			}
			if batchID != "" && n.BatchID != batchID {
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

	// GET /notifications/stats — delivery statistics
	mux.HandleFunc("GET /notifications/stats", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		channelCounts := make(map[string]int)
		statusCounts := make(map[string]int)
		for _, n := range notifs {
			channelCounts[n.Channel]++
			statusCounts[n.Status]++
		}
		total := len(notifs)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total":           total,
			"by_channel":      channelCounts,
			"by_status":       statusCounts,
			"generated_at":    time.Now().UTC().Format(time.RFC3339),
		})
	})

	return httptest.NewServer(mux)
}

// ---------------------------------------------------------------------------
// E2E Tests
// ---------------------------------------------------------------------------

// TestE2E_IdentityVerificationFlow simulates the full identity verification
// pipeline: register -> Dukcapil verify -> OTP send -> OTP verify -> confirmed.
func TestE2E_IdentityVerificationFlow(t *testing.T) {
	srv := newIdentityOTPServer()
	defer srv.Close()

	client := srv.Client()
	base := srv.URL

	t.Run("successful_full_verification", func(t *testing.T) {
		// Step 1: Register user with valid NIK
		body := `{"nik":"3201012345670001","full_name":"Budi Santoso"}`
		resp, err := client.Post(base+"/users/register", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("register failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}

		var user verifiedUser
		json.NewDecoder(resp.Body).Decode(&user)
		if user.ID == "" {
			t.Fatal("user ID should be assigned")
		}
		if user.Status != "PENDING" {
			t.Fatalf("expected PENDING, got %s", user.Status)
		}
		t.Logf("User registered: %s (status: %s)", user.ID, user.Status)

		// Step 2: Verify against Dukcapil
		resp, err = client.Post(base+"/users/"+user.ID+"/dukcapil-verify", "application/json", nil)
		if err != nil {
			t.Fatalf("dukcapil verify failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var dukcapilResult map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&dukcapilResult)
		if dukcapilResult["status"] != "DUKCAPIL_VERIFIED" {
			t.Fatalf("expected DUKCAPIL_VERIFIED, got %v", dukcapilResult["status"])
		}
		t.Logf("Dukcapil verification passed")

		// Step 3: Send OTP via email
		otpBody := `{"channel":"email"}`
		resp, err = client.Post(base+"/users/"+user.ID+"/otp/send", "application/json", strings.NewReader(otpBody))
		if err != nil {
			t.Fatalf("send OTP failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}

		var otpResult map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&otpResult)
		otpID := otpResult["otp_id"].(string)
		if otpID == "" {
			t.Fatal("otp_id should be assigned")
		}
		if otpResult["channel"] != "email" {
			t.Errorf("expected email channel, got %v", otpResult["channel"])
		}
		if otpResult["expires_at"] == nil {
			t.Error("expires_at should be set")
		}
		t.Logf("OTP sent: %s (channel: %s)", otpID, otpResult["channel"])

		// Step 4: Verify OTP with correct code
		verifyBody := fmt.Sprintf(`{"otp_id":"%s","code":"123456"}`, otpID)
		resp, err = client.Post(base+"/users/"+user.ID+"/otp/verify", "application/json", strings.NewReader(verifyBody))
		if err != nil {
			t.Fatalf("verify OTP failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var verifyResult map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&verifyResult)
		if verifyResult["verified"] != true {
			t.Fatalf("OTP should be verified: %+v", verifyResult)
		}
		if verifyResult["verified_at"] == nil || verifyResult["verified_at"].(string) == "" {
			t.Error("verified_at should be set")
		}
		t.Logf("OTP verified at: %s", verifyResult["verified_at"])

		// Step 5: Confirm user is fully verified
		resp, err = client.Get(base + "/users/" + user.ID)
		if err != nil {
			t.Fatalf("get user failed: %v", err)
		}
		defer resp.Body.Close()

		var finalUser verifiedUser
		json.NewDecoder(resp.Body).Decode(&finalUser)
		if finalUser.Status != "VERIFIED" {
			t.Errorf("expected VERIFIED, got %s", finalUser.Status)
		}
		if !finalUser.OTPVerified {
			t.Error("otp_verified should be true")
		}
		if finalUser.VerifiedAt == "" {
			t.Error("verified_at should be set")
		}
		t.Logf("User fully verified: %s at %s", finalUser.ID, finalUser.VerifiedAt)
	})

	t.Run("dukcapil_rejection_cases", func(t *testing.T) {
		tests := []struct {
			name         string
			nik          string
			fullName     string
			expectStatus string
			expectCode   int
		}{
			{
				name:         "NIK not found in population DB",
				nik:          "9999999999999999",
				fullName:     "Unknown Person",
				expectStatus: "NIK_NOT_FOUND",
				expectCode:   http.StatusUnprocessableEntity,
			},
			{
				name:         "name mismatch with Dukcapil record",
				nik:          "3201012345670001",
				fullName:     "Wrong Name Here",
				expectStatus: "NAME_MISMATCH",
				expectCode:   http.StatusUnprocessableEntity,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				body := fmt.Sprintf(`{"nik":"%s","full_name":"%s"}`, tc.nik, tc.fullName)
				resp, err := client.Post(base+"/users/register", "application/json", strings.NewReader(body))
				if err != nil {
					t.Fatalf("register failed: %v", err)
				}
				defer resp.Body.Close()

				var user verifiedUser
				json.NewDecoder(resp.Body).Decode(&user)

				resp, err = client.Post(base+"/users/"+user.ID+"/dukcapil-verify", "application/json", nil)
				if err != nil {
					t.Fatalf("verify failed: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != tc.expectCode {
					t.Fatalf("expected %d, got %d", tc.expectCode, resp.StatusCode)
				}

				var result map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&result)
				if result["status"] != tc.expectStatus {
					t.Errorf("expected status %s, got %v", tc.expectStatus, result["status"])
				}
				t.Logf("Correctly rejected: %s -> %s", tc.name, result["status"])
			})
		}
	})

	t.Run("invalid_otp_code", func(t *testing.T) {
		// Register and verify via Dukcapil
		body := `{"nik":"3201012345670002","full_name":"Siti Rahayu"}`
		resp, err := client.Post(base+"/users/register", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("register failed: %v", err)
		}
		defer resp.Body.Close()

		var user verifiedUser
		json.NewDecoder(resp.Body).Decode(&user)

		resp, err = client.Post(base+"/users/"+user.ID+"/dukcapil-verify", "application/json", nil)
		if err != nil {
			t.Fatalf("dukcapil verify failed: %v", err)
		}
		resp.Body.Close()

		// Send OTP
		resp, err = client.Post(base+"/users/"+user.ID+"/otp/send", "application/json", strings.NewReader(`{"channel":"sms"}`))
		if err != nil {
			t.Fatalf("send OTP failed: %v", err)
		}
		defer resp.Body.Close()

		var otpResult map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&otpResult)
		otpID := otpResult["otp_id"].(string)

		// Verify with wrong code
		verifyBody := fmt.Sprintf(`{"otp_id":"%s","code":"999999"}`, otpID)
		resp, err = client.Post(base+"/users/"+user.ID+"/otp/verify", "application/json", strings.NewReader(verifyBody))
		if err != nil {
			t.Fatalf("verify OTP failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		if result["verified"] != false {
			t.Error("OTP should not be verified with wrong code")
		}
		t.Logf("Invalid OTP correctly rejected")

		// Confirm user is NOT fully verified
		resp, err = client.Get(base + "/users/" + user.ID)
		if err != nil {
			t.Fatalf("get user failed: %v", err)
		}
		defer resp.Body.Close()

		var finalUser verifiedUser
		json.NewDecoder(resp.Body).Decode(&finalUser)
		if finalUser.Status == "VERIFIED" {
			t.Error("user should NOT be VERIFIED after failed OTP")
		}
		t.Logf("User remains unverified: %s", finalUser.Status)
	})

	t.Run("otp_requires_dukcapil_first", func(t *testing.T) {
		// Register but do NOT verify via Dukcapil
		body := `{"nik":"3201012345670003","full_name":"Ahmad Hidayat"}`
		resp, err := client.Post(base+"/users/register", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("register failed: %v", err)
		}
		defer resp.Body.Close()

		var user verifiedUser
		json.NewDecoder(resp.Body).Decode(&user)

		// Try to send OTP without Dukcapil verification
		resp, err = client.Post(base+"/users/"+user.ID+"/otp/send", "application/json", strings.NewReader(`{"channel":"email"}`))
		if err != nil {
			t.Fatalf("send OTP failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("expected 409 (must verify Dukcapil first), got %d", resp.StatusCode)
		}
		t.Logf("OTP correctly blocked until Dukcapil verification")
	})
}

// TestE2E_ConsentManagementFlow tests field-level consent management per
// UU PDP No. 27/2022: grant -> list -> verify data filtering -> revoke -> verify removal.
func TestE2E_ConsentManagementFlow(t *testing.T) {
	srv := newFieldConsentServer()
	defer srv.Close()

	client := srv.Client()
	base := srv.URL
	personID := "person-100"

	t.Run("grant_and_verify_field_filtering", func(t *testing.T) {
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

		var c1 consentRecord
		json.NewDecoder(resp.Body).Decode(&c1)
		if !c1.Active {
			t.Error("consent should be active")
		}
		if len(c1.Fields) != 3 {
			t.Errorf("basic_profile should grant 3 fields, got %d", len(c1.Fields))
		}
		t.Logf("Consent granted: %s (scope: %s, fields: %v)", c1.ID, c1.Scope, c1.Fields)

		// Step 2: Verify only basic_profile fields are accessible
		resp, err = client.Get(base + "/person-data/" + personID)
		if err != nil {
			t.Fatalf("get person data failed: %v", err)
		}
		defer resp.Body.Close()

		var data map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&data)

		// Should have basic_profile fields
		if data["full_name"] == nil {
			t.Error("full_name should be present (basic_profile consented)")
		}
		if data["nik"] == nil {
			t.Error("nik should be present (basic_profile consented)")
		}
		if data["birth_date"] == nil {
			t.Error("birth_date should be present (basic_profile consented)")
		}

		// Should NOT have other fields
		if data["email"] != nil {
			t.Error("email should NOT be present (no contact_info consent)")
		}
		if data["address"] != nil {
			t.Error("address should NOT be present (no address consent)")
		}
		if data["blood_type"] != nil {
			t.Error("blood_type should NOT be present (no demographics consent)")
		}
		t.Logf("Person data correctly filtered to basic_profile fields")
	})

	t.Run("multiple_scopes_expand_access", func(t *testing.T) {
		// Grant contact_info consent
		body := fmt.Sprintf(`{"person_id":"%s","scope":"contact_info"}`, personID)
		resp, err := client.Post(base+"/consents", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("grant contact_info failed: %v", err)
		}
		defer resp.Body.Close()

		var c2 consentRecord
		json.NewDecoder(resp.Body).Decode(&c2)

		// Grant address consent
		body = fmt.Sprintf(`{"person_id":"%s","scope":"address"}`, personID)
		resp, err = client.Post(base+"/consents", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("grant address failed: %v", err)
		}
		defer resp.Body.Close()

		var c3 consentRecord
		json.NewDecoder(resp.Body).Decode(&c3)

		// Verify expanded data access
		resp, err = client.Get(base + "/person-data/" + personID)
		if err != nil {
			t.Fatalf("get person data failed: %v", err)
		}
		defer resp.Body.Close()

		var data map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&data)

		if data["full_name"] == nil {
			t.Error("full_name should be present")
		}
		if data["email"] == nil {
			t.Error("email should be present (contact_info consented)")
		}
		if data["phone"] == nil {
			t.Error("phone should be present (contact_info consented)")
		}
		if data["address"] == nil {
			t.Error("address should be present (address consented)")
		}
		// Demographics still not consented
		if data["blood_type"] != nil {
			t.Error("blood_type should NOT be present (no demographics consent)")
		}
		t.Logf("Data access expanded with additional consents")

		// List all consents
		resp, err = client.Get(base + "/consents?person_id=" + personID)
		if err != nil {
			t.Fatalf("list consents failed: %v", err)
		}
		defer resp.Body.Close()

		var listResult struct {
			Consents    []consentRecord `json:"consents"`
			Total       int             `json:"total"`
			TotalActive int             `json:"total_active"`
		}
		json.NewDecoder(resp.Body).Decode(&listResult)
		if listResult.TotalActive != 3 {
			t.Errorf("expected 3 active consents, got %d", listResult.TotalActive)
		}
		t.Logf("Active consents: %d", listResult.TotalActive)

		// Revoke contact_info
		req, _ := http.NewRequest(http.MethodDelete, base+"/consents/"+c2.ID, nil)
		resp, err = client.Do(req)
		if err != nil {
			t.Fatalf("revoke consent failed: %v", err)
		}
		defer resp.Body.Close()

		var revokeResult map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&revokeResult)
		if revokeResult["status"] != "REVOKED" {
			t.Errorf("expected REVOKED, got %v", revokeResult["status"])
		}
		if revokeResult["revoked_at"] == nil {
			t.Error("revoked_at should be set")
		}
		t.Logf("Consent %s revoked at %s", c2.ID, revokeResult["revoked_at"])

		// Verify contact_info fields removed, others remain
		resp, err = client.Get(base + "/person-data/" + personID)
		if err != nil {
			t.Fatalf("get person data after revoke failed: %v", err)
		}
		defer resp.Body.Close()

		var dataAfter map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&dataAfter)
		if dataAfter["full_name"] == nil {
			t.Error("full_name should still be present (basic_profile active)")
		}
		if dataAfter["address"] == nil {
			t.Error("address should still be present (address active)")
		}
		if dataAfter["email"] != nil {
			t.Error("email should be removed (contact_info revoked)")
		}
		if dataAfter["phone"] != nil {
			t.Error("phone should be removed (contact_info revoked)")
		}
		t.Logf("Contact info correctly removed after revocation")
	})

	t.Run("invalid_scope_rejected", func(t *testing.T) {
		body := fmt.Sprintf(`{"person_id":"%s","scope":"invalid_scope"}`, personID)
		resp, err := client.Post(base+"/consents", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("grant consent failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid scope, got %d", resp.StatusCode)
		}
		t.Logf("Invalid scope correctly rejected")
	})

	t.Run("nonexistent_person_returns_404", func(t *testing.T) {
		resp, err := client.Get(base + "/person-data/person-nonexistent")
		if err != nil {
			t.Fatalf("get person data failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404 for nonexistent person, got %d", resp.StatusCode)
		}
		t.Logf("Nonexistent person correctly returns 404")
	})
}

// TestE2E_CorporateVerificationFlow tests the full corporate identity pipeline:
// register -> AHU verify -> OSS cross-reference -> add shareholders -> UBO analysis.
func TestE2E_CorporateVerificationFlow(t *testing.T) {
	srv := newCorporateUBOServer()
	defer srv.Close()

	client := srv.Client()
	base := srv.URL

	t.Run("full_corporate_verification_with_ubo", func(t *testing.T) {
		// Step 1: Register corporate entity
		body := `{"name":"PT Garuda Digital Nusantara","ahu_ref":"AHU-2024-001234"}`
		resp, err := client.Post(base+"/entities/register", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("register failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}

		var entity corpEntity
		json.NewDecoder(resp.Body).Decode(&entity)
		if entity.ID == "" {
			t.Fatal("entity ID should be assigned")
		}
		if entity.Status != "PENDING" {
			t.Fatalf("expected PENDING, got %s", entity.Status)
		}
		t.Logf("Entity registered: %s (AHU: %s)", entity.ID, entity.AHURef)

		// Step 2: Verify against AHU
		resp, err = client.Post(base+"/entities/"+entity.ID+"/verify-ahu", "application/json", nil)
		if err != nil {
			t.Fatalf("AHU verify failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var ahuResult map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&ahuResult)
		if ahuResult["status"] != "AHU_VERIFIED" {
			t.Fatalf("expected AHU_VERIFIED, got %v", ahuResult["status"])
		}
		t.Logf("AHU verification passed: %s", ahuResult["ahu_name"])

		// Step 3: Cross-reference with OSS/BKPM
		resp, err = client.Post(base+"/entities/"+entity.ID+"/cross-reference-oss", "application/json", nil)
		if err != nil {
			t.Fatalf("OSS cross-reference failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var ossResult map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&ossResult)
		if ossResult["status"] != "FULLY_VERIFIED" {
			t.Fatalf("expected FULLY_VERIFIED, got %v", ossResult["status"])
		}
		if ossResult["nib_number"] == nil || ossResult["nib_number"].(string) == "" {
			t.Error("NIB number should be assigned")
		}
		if ossResult["business_type"] != "Technology" {
			t.Errorf("expected Technology, got %v", ossResult["business_type"])
		}
		t.Logf("OSS cross-reference: NIB=%s, type=%s", ossResult["nib_number"], ossResult["business_type"])

		// Step 4: Add shareholders for UBO analysis (PP 13/2018: 25% threshold)
		shareholders := []struct {
			Name       string  `json:"name"`
			NIK        string  `json:"nik"`
			Percentage float64 `json:"percentage"`
		}{
			{"Budi Santoso", "3201012345670001", 40.0},      // UBO (>=25%)
			{"Siti Rahayu", "3201012345670002", 30.0},       // UBO (>=25%)
			{"Ahmad Hidayat", "3201012345670003", 20.0},     // Not UBO (<25%)
			{"Dewi Lestari", "3201012345670004", 10.0},      // Not UBO (<25%)
		}

		for _, sh := range shareholders {
			shBody, _ := json.Marshal(sh)
			resp, err = client.Post(base+"/entities/"+entity.ID+"/shareholders", "application/json", strings.NewReader(string(shBody)))
			if err != nil {
				t.Fatalf("add shareholder %s failed: %v", sh.Name, err)
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("expected 201 for shareholder, got %d", resp.StatusCode)
			}
		}
		t.Logf("Added %d shareholders", len(shareholders))

		// Step 5: UBO analysis
		resp, err = client.Get(base + "/entities/" + entity.ID + "/ubo-analysis")
		if err != nil {
			t.Fatalf("UBO analysis failed: %v", err)
		}
		defer resp.Body.Close()

		var uboResult map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&uboResult)

		totalShareholders := int(uboResult["total_shareholders"].(float64))
		if totalShareholders != 4 {
			t.Errorf("expected 4 shareholders, got %d", totalShareholders)
		}

		uboCount := int(uboResult["ubo_count"].(float64))
		if uboCount != 2 {
			t.Errorf("expected 2 UBOs (>=25%%), got %d", uboCount)
		}

		totalPct := uboResult["total_percentage"].(float64)
		if totalPct != 100.0 {
			t.Errorf("expected 100%% total, got %.1f%%", totalPct)
		}

		if uboResult["ubo_threshold"].(float64) != 25.0 {
			t.Error("UBO threshold should be 25%")
		}

		if uboResult["compliant"] != true {
			t.Error("should be compliant when UBOs are identified")
		}

		ubos := uboResult["ubos"].([]interface{})
		for _, u := range ubos {
			ubo := u.(map[string]interface{})
			pct := ubo["percentage"].(float64)
			if pct < 25.0 {
				t.Errorf("UBO %s has percentage %.1f%% which is below threshold", ubo["name"], pct)
			}
		}
		t.Logf("UBO analysis: %d UBOs identified out of %d shareholders, compliant=%v",
			uboCount, totalShareholders, uboResult["compliant"])

		// Step 6: Verify final entity state
		resp, err = client.Get(base + "/entities/" + entity.ID)
		if err != nil {
			t.Fatalf("get entity failed: %v", err)
		}
		defer resp.Body.Close()

		var finalEntity corpEntity
		json.NewDecoder(resp.Body).Decode(&finalEntity)
		if finalEntity.Status != "FULLY_VERIFIED" {
			t.Errorf("expected FULLY_VERIFIED, got %s", finalEntity.Status)
		}
		if finalEntity.NIBNumber == "" {
			t.Error("NIB number should be set after OSS cross-reference")
		}
		if len(finalEntity.Shareholders) != 4 {
			t.Errorf("expected 4 shareholders, got %d", len(finalEntity.Shareholders))
		}
		t.Logf("Final entity: %s (status: %s, NIB: %s)", finalEntity.Name, finalEntity.Status, finalEntity.NIBNumber)
	})

	t.Run("ahu_verification_failures", func(t *testing.T) {
		tests := []struct {
			name       string
			entityName string
			ahuRef     string
			wantStatus string
		}{
			{
				name:       "AHU reference not found",
				entityName: "PT Unknown Corp",
				ahuRef:     "AHU-9999-999999",
				wantStatus: "AHU_NOT_FOUND",
			},
			{
				name:       "name mismatch with AHU record",
				entityName: "PT Wrong Name",
				ahuRef:     "AHU-2024-001234",
				wantStatus: "NAME_MISMATCH",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				body := fmt.Sprintf(`{"name":"%s","ahu_ref":"%s"}`, tc.entityName, tc.ahuRef)
				resp, err := client.Post(base+"/entities/register", "application/json", strings.NewReader(body))
				if err != nil {
					t.Fatalf("register failed: %v", err)
				}
				defer resp.Body.Close()

				var entity corpEntity
				json.NewDecoder(resp.Body).Decode(&entity)

				resp, err = client.Post(base+"/entities/"+entity.ID+"/verify-ahu", "application/json", nil)
				if err != nil {
					t.Fatalf("AHU verify failed: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusUnprocessableEntity {
					t.Fatalf("expected 422, got %d", resp.StatusCode)
				}

				var result map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&result)
				if result["status"] != tc.wantStatus {
					t.Errorf("expected %s, got %v", tc.wantStatus, result["status"])
				}
				t.Logf("AHU verification correctly failed: %s", result["status"])
			})
		}
	})

	t.Run("no_ubo_when_all_below_threshold", func(t *testing.T) {
		body := `{"name":"PT Merdeka Teknologi","ahu_ref":"AHU-2024-005678"}`
		resp, err := client.Post(base+"/entities/register", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("register failed: %v", err)
		}
		defer resp.Body.Close()

		var entity corpEntity
		json.NewDecoder(resp.Body).Decode(&entity)

		// Add shareholders all below 25%
		smallHolders := []struct {
			Name       string  `json:"name"`
			NIK        string  `json:"nik"`
			Percentage float64 `json:"percentage"`
		}{
			{"Person A", "1100000000000001", 20.0},
			{"Person B", "1100000000000002", 20.0},
			{"Person C", "1100000000000003", 20.0},
			{"Person D", "1100000000000004", 20.0},
			{"Person E", "1100000000000005", 20.0},
		}

		for _, sh := range smallHolders {
			shBody, _ := json.Marshal(sh)
			resp, err = client.Post(base+"/entities/"+entity.ID+"/shareholders", "application/json", strings.NewReader(string(shBody)))
			if err != nil {
				t.Fatalf("add shareholder failed: %v", err)
			}
			resp.Body.Close()
		}

		// UBO analysis should find no UBOs
		resp, err = client.Get(base + "/entities/" + entity.ID + "/ubo-analysis")
		if err != nil {
			t.Fatalf("UBO analysis failed: %v", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		uboCount := int(result["ubo_count"].(float64))
		if uboCount != 0 {
			t.Errorf("expected 0 UBOs, got %d", uboCount)
		}
		if result["compliant"] != false {
			t.Error("should be non-compliant when no UBOs identified")
		}
		t.Logf("Correctly identified 0 UBOs when all below threshold, compliant=%v", result["compliant"])
	})
}

// TestE2E_NotificationDeliveryFlow tests the full notification pipeline:
// send OTP email -> send OTP SMS -> batch notifications -> verify delivery stats.
func TestE2E_NotificationDeliveryFlow(t *testing.T) {
	srv := newBatchNotificationServer()
	defer srv.Close()

	client := srv.Client()
	base := srv.URL

	t.Run("email_otp_delivery", func(t *testing.T) {
		body := `{
			"channel": "email",
			"recipient": "budi@example.com",
			"subject": "Kode OTP Anda",
			"body": "Kode verifikasi Anda adalah 123456. Berlaku 5 menit."
		}`
		resp, err := client.Post(base+"/notifications/send", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("send email OTP failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}

		var n notifRecord
		json.NewDecoder(resp.Body).Decode(&n)
		if n.Status != "DELIVERED" {
			t.Errorf("expected DELIVERED, got %s", n.Status)
		}
		if n.Channel != "email" {
			t.Errorf("expected email, got %s", n.Channel)
		}
		if n.SentAt == "" {
			t.Error("sent_at should be set")
		}
		t.Logf("Email OTP delivered: %s to %s", n.ID, n.Recipient)
	})

	t.Run("sms_otp_delivery", func(t *testing.T) {
		body := `{
			"channel": "sms",
			"recipient": "+6281234567890",
			"subject": "OTP",
			"body": "Kode: 654321"
		}`
		resp, err := client.Post(base+"/notifications/send", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("send SMS OTP failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}

		var n notifRecord
		json.NewDecoder(resp.Body).Decode(&n)
		if n.Channel != "sms" {
			t.Errorf("expected sms, got %s", n.Channel)
		}
		if n.Recipient != "+6281234567890" {
			t.Errorf("expected +6281234567890, got %s", n.Recipient)
		}
		t.Logf("SMS OTP delivered: %s", n.ID)
	})

	t.Run("batch_notification_all_delivered", func(t *testing.T) {
		body := `{
			"notifications": [
				{"channel":"email","recipient":"user1@example.com","subject":"Welcome","body":"Welcome to GarudaPass"},
				{"channel":"email","recipient":"user2@example.com","subject":"Welcome","body":"Welcome to GarudaPass"},
				{"channel":"sms","recipient":"+6281111111111","subject":"Verify","body":"Code: 111111"},
				{"channel":"email","recipient":"user3@example.com","subject":"Alert","body":"New login detected"}
			]
		}`
		resp, err := client.Post(base+"/notifications/batch", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("batch send failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}

		var result batchResult
		json.NewDecoder(resp.Body).Decode(&result)
		if result.BatchID == "" {
			t.Fatal("batch_id should be assigned")
		}
		if result.Total != 4 {
			t.Errorf("expected 4 total, got %d", result.Total)
		}
		if result.Delivered != 4 {
			t.Errorf("expected 4 delivered, got %d", result.Delivered)
		}
		if result.Failed != 0 {
			t.Errorf("expected 0 failed, got %d", result.Failed)
		}
		if result.Status != "COMPLETED" {
			t.Errorf("expected COMPLETED, got %s", result.Status)
		}
		t.Logf("Batch %s: %d/%d delivered", result.BatchID, result.Delivered, result.Total)

		// Verify batch notifications retrievable
		resp, err = client.Get(base + "/notifications?batch_id=" + result.BatchID)
		if err != nil {
			t.Fatalf("list batch notifications failed: %v", err)
		}
		defer resp.Body.Close()

		var listResult struct {
			Notifications []notifRecord `json:"notifications"`
			Total         int           `json:"total"`
		}
		json.NewDecoder(resp.Body).Decode(&listResult)
		if listResult.Total != 4 {
			t.Errorf("expected 4 notifications in batch, got %d", listResult.Total)
		}
		for _, n := range listResult.Notifications {
			if n.BatchID != result.BatchID {
				t.Errorf("notification %s has wrong batch_id: %s", n.ID, n.BatchID)
			}
		}
		t.Logf("Batch notifications verified: %d records", listResult.Total)
	})

	t.Run("batch_with_partial_failures", func(t *testing.T) {
		body := `{
			"notifications": [
				{"channel":"email","recipient":"valid@example.com","subject":"Test","body":"OK"},
				{"channel":"","recipient":"missing-channel@example.com","subject":"Test","body":"No channel"},
				{"channel":"sms","recipient":"","subject":"Test","body":"No recipient"}
			]
		}`
		resp, err := client.Post(base+"/notifications/batch", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("batch send failed: %v", err)
		}
		defer resp.Body.Close()

		var result batchResult
		json.NewDecoder(resp.Body).Decode(&result)
		if result.Delivered != 1 {
			t.Errorf("expected 1 delivered, got %d", result.Delivered)
		}
		if result.Failed != 2 {
			t.Errorf("expected 2 failed, got %d", result.Failed)
		}
		if result.Status != "PARTIAL" {
			t.Errorf("expected PARTIAL status, got %s", result.Status)
		}
		t.Logf("Partial batch: %d delivered, %d failed, status=%s",
			result.Delivered, result.Failed, result.Status)
	})

	t.Run("delivery_statistics", func(t *testing.T) {
		resp, err := client.Get(base + "/notifications/stats")
		if err != nil {
			t.Fatalf("get stats failed: %v", err)
		}
		defer resp.Body.Close()

		var stats map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&stats)

		total := int(stats["total"].(float64))
		if total < 8 { // 2 single + 4 batch + 2 partial (at minimum)
			t.Errorf("expected at least 8 total notifications, got %d", total)
		}

		byChannel := stats["by_channel"].(map[string]interface{})
		if byChannel["email"] == nil {
			t.Error("should have email channel in stats")
		}
		if byChannel["sms"] == nil {
			t.Error("should have sms channel in stats")
		}

		byStatus := stats["by_status"].(map[string]interface{})
		if byStatus["DELIVERED"] == nil {
			t.Error("should have DELIVERED status in stats")
		}

		if stats["generated_at"] == nil {
			t.Error("generated_at should be set")
		}
		t.Logf("Stats: total=%d, by_channel=%v, by_status=%v", total, byChannel, byStatus)
	})

	t.Run("filter_notifications_by_channel", func(t *testing.T) {
		resp, err := client.Get(base + "/notifications?channel=sms")
		if err != nil {
			t.Fatalf("filter by channel failed: %v", err)
		}
		defer resp.Body.Close()

		var result struct {
			Notifications []notifRecord `json:"notifications"`
			Total         int           `json:"total"`
		}
		json.NewDecoder(resp.Body).Decode(&result)

		for _, n := range result.Notifications {
			if n.Channel != "sms" {
				t.Errorf("expected sms channel, got %s for notification %s", n.Channel, n.ID)
			}
		}
		if result.Total < 2 {
			t.Errorf("expected at least 2 SMS notifications, got %d", result.Total)
		}
		t.Logf("SMS filter returned %d notifications", result.Total)
	})
}
