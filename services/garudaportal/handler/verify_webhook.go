package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// VerifyWebhookHandler provides webhook signature verification testing.
type VerifyWebhookHandler struct{}

// NewVerifyWebhookHandler creates a new VerifyWebhookHandler.
func NewVerifyWebhookHandler() *VerifyWebhookHandler {
	return &VerifyWebhookHandler{}
}

type verifyWebhookRequest struct {
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
	Secret    string `json:"secret"`
}

type verifyWebhookResponse struct {
	Valid   bool   `json:"valid"`
	Details string `json:"details"`
}

// defaultTimestampTolerance is the maximum age of a webhook signature.
const defaultTimestampTolerance = 5 * time.Minute

// VerifySignature handles POST /api/v1/portal/webhooks/verify.
func (h *VerifyWebhookHandler) VerifySignature(w http.ResponseWriter, r *http.Request) {
	var req verifyWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if req.Payload == "" || req.Signature == "" || req.Secret == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "payload, signature, and secret are all required")
		return
	}

	// Parse signature: t={timestamp},v1={hmac}
	ts, sig, err := parseWebhookSignature(req.Signature)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(verifyWebhookResponse{
			Valid:   false,
			Details: fmt.Sprintf("Invalid signature format: %v", err),
		})
		return
	}

	// Check timestamp tolerance
	signedAt := time.Unix(ts, 0)
	if time.Since(signedAt) > defaultTimestampTolerance {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(verifyWebhookResponse{
			Valid:   false,
			Details: "Signature timestamp has expired",
		})
		return
	}

	// Compute expected HMAC
	mac := hmac.New(sha256.New, []byte(req.Secret))
	msg := fmt.Sprintf("%d.%s", ts, req.Payload)
	mac.Write([]byte(msg))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(verifyWebhookResponse{
			Valid:   false,
			Details: "Signature does not match the expected HMAC",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(verifyWebhookResponse{
		Valid:   true,
		Details: "Signature is valid",
	})
}

func parseWebhookSignature(signature string) (timestamp int64, sig string, err error) {
	parts := strings.Split(signature, ",")
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("expected format t=...,v1=...")
	}

	tsPart := parts[0]
	sigPart := parts[1]

	if !strings.HasPrefix(tsPart, "t=") {
		return 0, "", fmt.Errorf("missing timestamp prefix")
	}
	if !strings.HasPrefix(sigPart, "v1=") {
		return 0, "", fmt.Errorf("missing v1 signature prefix")
	}

	ts, err := strconv.ParseInt(tsPart[2:], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid timestamp: %w", err)
	}

	return ts, sigPart[3:], nil
}
