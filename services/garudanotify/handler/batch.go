package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

const maxBatchSize = 100

// BatchItem represents a single notification in a batch.
type BatchItem struct {
	Type      string `json:"type"`      // "otp" or "alert"
	Channel   string `json:"channel"`   // "email" or "sms"
	Recipient string `json:"recipient"` // email address or phone number
	OTPCode   string `json:"otp_code,omitempty"`
	Subject   string `json:"subject,omitempty"`
	Message   string `json:"message,omitempty"`
}

// BatchRequest is the request body for batch notifications.
type BatchRequest struct {
	Items []BatchItem `json:"items"`
}

// BatchItemResult represents the result of sending a single notification.
type BatchItemResult struct {
	Index   int    `json:"index"`
	Status  string `json:"status"` // "sent" or "failed"
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// BatchResponse is the response body for batch notifications.
type BatchResponse struct {
	Total     int               `json:"total"`
	Succeeded int               `json:"succeeded"`
	Failed    int               `json:"failed"`
	Results   []BatchItemResult `json:"results"`
}

// SendBatch handles POST /api/v1/notify/batch.
func (h *NotifyHandler) SendBatch(w http.ResponseWriter, r *http.Request) {
	var req BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "empty_batch", "Batch must contain at least one item")
		return
	}

	if len(req.Items) > maxBatchSize {
		writeError(w, http.StatusBadRequest, "batch_too_large", fmt.Sprintf("Batch exceeds maximum size of %d items", maxBatchSize))
		return
	}

	resp := BatchResponse{
		Total:   len(req.Items),
		Results: make([]BatchItemResult, len(req.Items)),
	}

	for i, item := range req.Items {
		result := BatchItemResult{Index: i}

		switch item.Type {
		case "otp":
			err := h.sendOTPItem(r, item)
			if err != nil {
				result.Status = "failed"
				result.Error = "send_failed"
				result.Message = err.Error()
				resp.Failed++
				slog.Error("batch OTP send failed", "index", i, "error", err)
			} else {
				result.Status = "sent"
				resp.Succeeded++
			}
		case "alert":
			err := h.sendAlertItem(r, item)
			if err != nil {
				result.Status = "failed"
				result.Error = "send_failed"
				result.Message = err.Error()
				resp.Failed++
				slog.Error("batch alert send failed", "index", i, "error", err)
			} else {
				result.Status = "sent"
				resp.Succeeded++
			}
		default:
			result.Status = "failed"
			result.Error = "invalid_type"
			result.Message = fmt.Sprintf("unsupported type %q, must be \"otp\" or \"alert\"", item.Type)
			resp.Failed++
		}

		resp.Results[i] = result
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *NotifyHandler) sendOTPItem(r *http.Request, item BatchItem) error {
	switch item.Channel {
	case "email":
		subject := "Your GarudaPass Verification Code"
		body := fmt.Sprintf("Your verification code is: %s\n\nThis code expires in 5 minutes. Do not share it with anyone.", item.OTPCode)
		return h.email.Send(r.Context(), item.Recipient, subject, body)
	case "sms":
		message := fmt.Sprintf("GarudaPass: Your verification code is %s. Valid for 5 minutes.", item.OTPCode)
		return h.sms.Send(r.Context(), item.Recipient, message)
	default:
		return fmt.Errorf("unsupported channel %q for OTP", item.Channel)
	}
}

func (h *NotifyHandler) sendAlertItem(r *http.Request, item BatchItem) error {
	switch item.Channel {
	case "email":
		return h.email.Send(r.Context(), item.Recipient, item.Subject, item.Message)
	default:
		return fmt.Errorf("unsupported channel %q for alerts", item.Channel)
	}
}
