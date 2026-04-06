package channel

import (
	"context"
	"testing"
)

func TestMockSMSSender_RecordsMessages(t *testing.T) {
	sender := NewMockSMSSender()

	err := sender.Send(context.Background(), "+6281234567890", "Your OTP is 123456")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if len(sender.Sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sender.Sent))
	}

	msg := sender.Sent[0]
	if msg.PhoneNumber != "+6281234567890" {
		t.Errorf("expected phone %q, got %q", "+6281234567890", msg.PhoneNumber)
	}
	if msg.Message != "Your OTP is 123456" {
		t.Errorf("expected message %q, got %q", "Your OTP is 123456", msg.Message)
	}
	if msg.SentAt.IsZero() {
		t.Error("expected non-zero SentAt")
	}
}

func TestMockSMSSender_InvalidPhoneFormat(t *testing.T) {
	sender := NewMockSMSSender()

	err := sender.Send(context.Background(), "081234567890", "Hello")
	if err == nil {
		t.Fatal("expected error for phone not starting with +62, got nil")
	}
}

func TestMockSMSSender_EmptyPhone(t *testing.T) {
	sender := NewMockSMSSender()

	err := sender.Send(context.Background(), "", "Hello")
	if err == nil {
		t.Fatal("expected error for empty phone, got nil")
	}
}

func TestMockSMSSender_EmptyMessage(t *testing.T) {
	sender := NewMockSMSSender()

	err := sender.Send(context.Background(), "+6281234567890", "")
	if err == nil {
		t.Fatal("expected error for empty message, got nil")
	}
}

func TestMockSMSSender_ImplementsInterface(t *testing.T) {
	var _ SMSSender = NewMockSMSSender()
}
