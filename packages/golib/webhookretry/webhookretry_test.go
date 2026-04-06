package webhookretry

import (
	"testing"
	"time"
)

func TestPolicy_NextDelay(t *testing.T) {
	p := DefaultPolicy()

	d1 := p.NextDelay(1) // 1s
	d2 := p.NextDelay(2) // 2s
	d3 := p.NextDelay(3) // 4s

	if d1 != 1*time.Second {
		t.Errorf("attempt 1: got %v", d1)
	}
	if d2 != 2*time.Second {
		t.Errorf("attempt 2: got %v", d2)
	}
	if d3 != 4*time.Second {
		t.Errorf("attempt 3: got %v", d3)
	}
}

func TestPolicy_MaxDelay(t *testing.T) {
	p := Policy{
		MaxRetries:   20,
		InitialDelay: 1 * time.Second,
		MaxDelay:     1 * time.Minute,
		Multiplier:   2.0,
	}

	d := p.NextDelay(15) // Would be 2^14 = 16384s without cap.
	if d > 1*time.Minute {
		t.Errorf("should be capped at max: got %v", d)
	}
}

func TestPolicy_ShouldRetry(t *testing.T) {
	p := Policy{MaxRetries: 3}

	if !p.ShouldRetry(0) {
		t.Error("attempt 0 should retry")
	}
	if !p.ShouldRetry(2) {
		t.Error("attempt 2 should retry")
	}
	if p.ShouldRetry(3) {
		t.Error("attempt 3 should not retry (max)")
	}
}

func TestDelivery_RecordSuccess(t *testing.T) {
	d := &Delivery{Status: StatusPending}
	d.RecordSuccess()

	if d.Status != StatusDelivered {
		t.Errorf("status: got %q", d.Status)
	}
	if d.LastAttempt.IsZero() {
		t.Error("should set last attempt")
	}
}

func TestDelivery_RecordFailure_WithRetry(t *testing.T) {
	p := DefaultPolicy()
	d := &Delivery{Status: StatusPending}

	d.RecordFailure("connection refused", p)

	if d.Status != StatusRetrying {
		t.Errorf("status: got %q", d.Status)
	}
	if d.Attempt != 1 {
		t.Errorf("attempt: got %d", d.Attempt)
	}
	if d.LastError != "connection refused" {
		t.Errorf("error: got %q", d.LastError)
	}
	if d.NextRetry.IsZero() {
		t.Error("should set next retry")
	}
}

func TestDelivery_RecordFailure_MaxRetriesExhausted(t *testing.T) {
	p := Policy{MaxRetries: 1}
	d := &Delivery{Status: StatusPending, Attempt: 0}

	d.RecordFailure("timeout", p)
	if d.Status != StatusFailed {
		t.Errorf("should fail after max retries: got %q", d.Status)
	}
}

func TestDelivery_IsTerminal(t *testing.T) {
	d := &Delivery{Status: StatusDelivered}
	if !d.IsTerminal() {
		t.Error("delivered should be terminal")
	}

	d.Status = StatusFailed
	if !d.IsTerminal() {
		t.Error("failed should be terminal")
	}

	d.Status = StatusRetrying
	if d.IsTerminal() {
		t.Error("retrying should not be terminal")
	}

	d.Status = StatusPending
	if d.IsTerminal() {
		t.Error("pending should not be terminal")
	}
}

func TestPolicy_RetrySchedule(t *testing.T) {
	p := Policy{
		MaxRetries:   5,
		InitialDelay: 1 * time.Second,
		MaxDelay:     1 * time.Hour,
		Multiplier:   2.0,
	}

	schedule := p.RetrySchedule()
	if len(schedule) != 5 {
		t.Errorf("schedule length: got %d", len(schedule))
	}

	// Should be increasing.
	for i := 1; i < len(schedule); i++ {
		if schedule[i] < schedule[i-1] {
			t.Errorf("schedule should be increasing: %v < %v", schedule[i], schedule[i-1])
		}
	}
}

func TestPolicy_TotalRetryDuration(t *testing.T) {
	p := Policy{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     1 * time.Hour,
		Multiplier:   2.0,
	}

	total := p.TotalRetryDuration()
	expected := 1*time.Second + 2*time.Second + 4*time.Second
	if total != expected {
		t.Errorf("total: got %v, want %v", total, expected)
	}
}

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()
	if p.MaxRetries != 10 {
		t.Errorf("max retries: got %d", p.MaxRetries)
	}
	if p.InitialDelay != 1*time.Second {
		t.Errorf("initial delay: got %v", p.InitialDelay)
	}
	if p.MaxDelay != 1*time.Hour {
		t.Errorf("max delay: got %v", p.MaxDelay)
	}
}

func TestPolicy_NextDelay_ZeroAttempt(t *testing.T) {
	p := DefaultPolicy()
	d := p.NextDelay(0)
	if d != p.InitialDelay {
		t.Errorf("zero attempt: got %v", d)
	}
}
