package channel

import (
	"context"
	"errors"
	"sync"
	"time"
)

// EmailSender sends email messages.
type EmailSender interface {
	Send(ctx context.Context, to, subject, body string) error
}

// EmailMessage represents a sent email for testing assertions.
type EmailMessage struct {
	To      string
	Subject string
	Body    string
	SentAt  time.Time
}

// MockEmailSender is an in-memory email sender for testing.
type MockEmailSender struct {
	Sent       []EmailMessage
	FailOnSend bool // If true, Send returns an error.
	mu         sync.Mutex
}

// NewMockEmailSender creates a new MockEmailSender.
func NewMockEmailSender() *MockEmailSender {
	return &MockEmailSender{}
}

// Send records the email message. Returns an error if to or subject is empty.
func (m *MockEmailSender) Send(_ context.Context, to, subject, body string) error {
	if m.FailOnSend {
		return errors.New("mock email send failure")
	}
	if to == "" {
		return errors.New("recipient (to) is required")
	}
	if subject == "" {
		return errors.New("subject is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.Sent = append(m.Sent, EmailMessage{
		To:      to,
		Subject: subject,
		Body:    body,
		SentAt:  time.Now().UTC(),
	})
	return nil
}
