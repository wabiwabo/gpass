package channel

import (
	"context"
	"testing"
)

func TestMockEmailSender_RecordsMessages(t *testing.T) {
	sender := NewMockEmailSender()

	err := sender.Send(context.Background(), "user@example.com", "Your OTP", "Code: 123456")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if len(sender.Sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sender.Sent))
	}

	msg := sender.Sent[0]
	if msg.To != "user@example.com" {
		t.Errorf("expected to %q, got %q", "user@example.com", msg.To)
	}
	if msg.Subject != "Your OTP" {
		t.Errorf("expected subject %q, got %q", "Your OTP", msg.Subject)
	}
	if msg.Body != "Code: 123456" {
		t.Errorf("expected body %q, got %q", "Code: 123456", msg.Body)
	}
	if msg.SentAt.IsZero() {
		t.Error("expected non-zero SentAt")
	}
}

func TestMockEmailSender_EmptyTo(t *testing.T) {
	sender := NewMockEmailSender()

	err := sender.Send(context.Background(), "", "Subject", "Body")
	if err == nil {
		t.Fatal("expected error for empty to, got nil")
	}
}

func TestMockEmailSender_EmptySubject(t *testing.T) {
	sender := NewMockEmailSender()

	err := sender.Send(context.Background(), "user@example.com", "", "Body")
	if err == nil {
		t.Fatal("expected error for empty subject, got nil")
	}
}

func TestMockEmailSender_ImplementsInterface(t *testing.T) {
	var _ EmailSender = NewMockEmailSender()
}
