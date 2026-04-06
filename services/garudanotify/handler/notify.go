package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/garudapass/gpass/services/garudanotify/channel"
)

// NotifyHandler handles notification requests from internal services.
type NotifyHandler struct {
	email channel.EmailSender
	sms   channel.SMSSender
}

// NewNotifyHandler creates a new NotifyHandler.
func NewNotifyHandler(email channel.EmailSender, sms channel.SMSSender) *NotifyHandler {
	return &NotifyHandler{
		email: email,
		sms:   sms,
	}
}

type otpRequest struct {
	Channel   string `json:"channel"`
	Recipient string `json:"recipient"`
	OTPCode   string `json:"otp_code"`
}

type alertRequest struct {
	Channel   string `json:"channel"`
	Recipient string `json:"recipient"`
	Subject   string `json:"subject"`
	Message   string `json:"message"`
}

// SendOTP handles POST /api/v1/notify/otp.
func (h *NotifyHandler) SendOTP(w http.ResponseWriter, r *http.Request) {
	var req otpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if req.Channel == "" {
		writeError(w, http.StatusBadRequest, "missing_channel", "channel is required")
		return
	}

	switch req.Channel {
	case "email":
		subject := "Your GarudaPass Verification Code"
		body := fmt.Sprintf("Your verification code is: %s\n\nThis code expires in 5 minutes. Do not share it with anyone.", req.OTPCode)
		if err := h.email.Send(r.Context(), req.Recipient, subject, body); err != nil {
			slog.Error("failed to send OTP email", "error", err, "recipient", req.Recipient)
			writeError(w, http.StatusInternalServerError, "send_failed", "Failed to send OTP")
			return
		}
	case "sms":
		message := fmt.Sprintf("GarudaPass: Your verification code is %s. Valid for 5 minutes.", req.OTPCode)
		if err := h.sms.Send(r.Context(), req.Recipient, message); err != nil {
			slog.Error("failed to send OTP SMS", "error", err, "recipient", req.Recipient)
			writeError(w, http.StatusInternalServerError, "send_failed", "Failed to send OTP")
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "invalid_channel", fmt.Sprintf("unsupported channel %q, must be \"email\" or \"sms\"", req.Channel))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
}

// SendAlert handles POST /api/v1/notify/alert.
func (h *NotifyHandler) SendAlert(w http.ResponseWriter, r *http.Request) {
	var req alertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if req.Channel == "" {
		writeError(w, http.StatusBadRequest, "missing_channel", "channel is required")
		return
	}

	switch req.Channel {
	case "email":
		if err := h.email.Send(r.Context(), req.Recipient, req.Subject, req.Message); err != nil {
			slog.Error("failed to send alert email", "error", err, "recipient", req.Recipient)
			writeError(w, http.StatusInternalServerError, "send_failed", "Failed to send alert")
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "invalid_channel", fmt.Sprintf("unsupported channel %q for alerts, must be \"email\"", req.Channel))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	})
}
