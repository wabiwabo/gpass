package channel

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

// SMSSender sends SMS messages.
type SMSSender interface {
	Send(ctx context.Context, phoneNumber, message string) error
}

// SMSMessage represents a sent SMS for testing assertions.
type SMSMessage struct {
	PhoneNumber string
	Message     string
	SentAt      time.Time
}

// MockSMSSender is an in-memory SMS sender for testing.
type MockSMSSender struct {
	Sent       []SMSMessage
	FailOnSend bool // If true, Send returns an error.
	mu         sync.Mutex
}

// NewMockSMSSender creates a new MockSMSSender.
func NewMockSMSSender() *MockSMSSender {
	return &MockSMSSender{}
}

// Send records the SMS message. Validates that phone number starts with +62.
func (m *MockSMSSender) Send(_ context.Context, phoneNumber, message string) error {
	if m.FailOnSend {
		return errors.New("mock SMS send failure")
	}
	if phoneNumber == "" {
		return errors.New("phone number is required")
	}
	if !strings.HasPrefix(phoneNumber, "+62") {
		return errors.New("phone number must start with +62 (Indonesian format)")
	}
	if message == "" {
		return errors.New("message is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.Sent = append(m.Sent, SMSMessage{
		PhoneNumber: phoneNumber,
		Message:     message,
		SentAt:      time.Now().UTC(),
	})
	return nil
}
