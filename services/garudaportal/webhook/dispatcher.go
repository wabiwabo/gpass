package webhook

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

// retryDelays defines the exponential backoff schedule for webhook retries.
var retryDelays = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
	24 * time.Hour,
}

// Dispatcher handles webhook delivery with retry logic.
type Dispatcher struct {
	client        *http.Client
	deliveryStore store.DeliveryStore
}

// NewDispatcher creates a new webhook dispatcher.
func NewDispatcher(client *http.Client, deliveryStore store.DeliveryStore) *Dispatcher {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Dispatcher{
		client:        client,
		deliveryStore: deliveryStore,
	}
}

// Deliver sends a webhook event to the subscription URL and records the outcome.
func (d *Dispatcher) Deliver(sub *store.WebhookSubscription, eventType string, payload []byte) error {
	// Create delivery record
	delivery, err := d.deliveryStore.Create(&store.WebhookDelivery{
		SubscriptionID: sub.ID,
		EventType:      eventType,
		Payload:        string(payload),
		Status:         "PENDING",
	})
	if err != nil {
		return err
	}

	// Compute signature
	ts := time.Now().Unix()
	signature := Sign(payload, sub.Secret, ts)

	// Build request
	req, err := http.NewRequest(http.MethodPost, sub.URL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GarudaPass-Signature", signature)
	req.Header.Set("X-GarudaPass-Event", eventType)

	// Send
	resp, err := d.client.Do(req)
	if err != nil {
		slog.Warn("webhook delivery failed", "error", err, "subscription_id", sub.ID, "delivery_id", delivery.ID)
		nextRetry := time.Now().UTC().Add(NextRetryDelay(0))
		return d.deliveryStore.UpdateStatus(delivery.ID, "PENDING", 0, err.Error(), &nextRetry)
	}
	defer resp.Body.Close()

	// Read response body (truncated to 1KB)
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	respBody := string(bodyBytes)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return d.deliveryStore.UpdateStatus(delivery.ID, "DELIVERED", resp.StatusCode, respBody, nil)
	}

	// Failed - schedule retry
	slog.Warn("webhook delivery got non-2xx response",
		"status", resp.StatusCode,
		"subscription_id", sub.ID,
		"delivery_id", delivery.ID,
	)
	nextRetry := time.Now().UTC().Add(NextRetryDelay(0))
	return d.deliveryStore.UpdateStatus(delivery.ID, "PENDING", resp.StatusCode, respBody, &nextRetry)
}

// NextRetryDelay returns the delay before the next retry attempt.
func NextRetryDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	if attempt >= len(retryDelays) {
		return retryDelays[len(retryDelays)-1]
	}
	return retryDelays[attempt]
}
