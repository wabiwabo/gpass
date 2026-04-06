package worker

import (
	"log/slog"
	"sync"
	"time"
)

// DeliveryStore interface for the worker (subset of full store).
type DeliveryStore interface {
	ListPending() ([]*Delivery, error)
	UpdateStatus(id, status string, code int, body string, nextRetry *time.Time) error
}

// WebhookStore interface for looking up subscriptions.
type WebhookStore interface {
	GetByID(id string) (*Subscription, error)
}

// Dispatcher interface for delivering webhooks.
type Dispatcher interface {
	Deliver(url string, payload []byte, secret string, eventType string) (int, string, error)
}

// Delivery represents a pending webhook delivery.
type Delivery struct {
	ID             string
	SubscriptionID string
	EventType      string
	Payload        []byte
	Attempts       int
	NextRetryAt    *time.Time
}

// Subscription represents a webhook subscription.
type Subscription struct {
	ID     string
	URL    string
	Secret string
	Status string
}

// retryDelays defines the exponential backoff schedule for webhook retries.
var retryDelays = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
	24 * time.Hour,
}

// nextRetryDelay returns the delay before the next retry attempt.
func nextRetryDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	if attempt >= len(retryDelays) {
		return retryDelays[len(retryDelays)-1]
	}
	return retryDelays[attempt]
}

// WebhookWorker processes pending webhook deliveries in the background.
type WebhookWorker struct {
	deliveryStore DeliveryStore
	webhookStore  WebhookStore
	dispatcher    Dispatcher
	interval      time.Duration
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// New creates a webhook worker that polls every interval.
func New(ds DeliveryStore, ws WebhookStore, d Dispatcher, interval time.Duration) *WebhookWorker {
	return &WebhookWorker{
		deliveryStore: ds,
		webhookStore:  ws,
		dispatcher:    d,
		interval:      interval,
		stopCh:        make(chan struct{}),
	}
}

// Start begins the worker loop.
func (w *WebhookWorker) Start() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-w.stopCh:
				return
			case <-ticker.C:
				processed, succeeded, failed := w.ProcessPending()
				if processed > 0 {
					slog.Info("webhook worker cycle complete",
						"processed", processed,
						"succeeded", succeeded,
						"failed", failed,
					)
				}
			}
		}
	}()
}

// Stop gracefully stops the worker.
func (w *WebhookWorker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

// ProcessPending finds and delivers all pending webhooks.
func (w *WebhookWorker) ProcessPending() (processed, succeeded, failed int) {
	pending, err := w.deliveryStore.ListPending()
	if err != nil {
		slog.Error("failed to list pending deliveries", "error", err)
		return 0, 0, 0
	}

	for _, delivery := range pending {
		processed++

		sub, err := w.webhookStore.GetByID(delivery.SubscriptionID)
		if err != nil {
			slog.Warn("subscription not found for delivery",
				"delivery_id", delivery.ID,
				"subscription_id", delivery.SubscriptionID,
				"error", err,
			)
			failed++
			continue
		}

		// Skip disabled subscriptions
		if sub.Status != "ACTIVE" {
			slog.Debug("skipping delivery for disabled subscription",
				"delivery_id", delivery.ID,
				"subscription_id", sub.ID,
				"status", sub.Status,
			)
			continue
		}

		statusCode, respBody, err := w.dispatcher.Deliver(sub.URL, delivery.Payload, sub.Secret, delivery.EventType)
		if err != nil {
			slog.Warn("webhook delivery failed",
				"delivery_id", delivery.ID,
				"error", err,
			)
			nextRetry := time.Now().UTC().Add(nextRetryDelay(delivery.Attempts))
			_ = w.deliveryStore.UpdateStatus(delivery.ID, "PENDING", 0, err.Error(), &nextRetry)
			failed++
			continue
		}

		if statusCode >= 200 && statusCode < 300 {
			_ = w.deliveryStore.UpdateStatus(delivery.ID, "DELIVERED", statusCode, respBody, nil)
			succeeded++
		} else {
			nextRetry := time.Now().UTC().Add(nextRetryDelay(delivery.Attempts))
			_ = w.deliveryStore.UpdateStatus(delivery.ID, "PENDING", statusCode, respBody, &nextRetry)
			failed++
		}
	}

	return processed, succeeded, failed
}
