package featureflags

import (
	"hash/fnv"
	"sync"
)

// Flag represents a feature flag.
type Flag struct {
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
	Percentage  int    `json:"percentage"` // 0-100 for gradual rollout
}

// Store manages feature flags.
type Store struct {
	flags map[string]*Flag
	mu    sync.RWMutex
}

// New creates a feature flag store with initial flags.
func New(flags ...Flag) *Store {
	s := &Store{
		flags: make(map[string]*Flag, len(flags)),
	}
	for i := range flags {
		f := flags[i] // copy
		s.flags[f.Name] = &f
	}
	return s
}

// IsEnabled checks if a flag is enabled.
// Returns false for unknown flags.
func (s *Store) IsEnabled(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	f, ok := s.flags[name]
	if !ok {
		return false
	}
	return f.Enabled
}

// IsEnabledForUser checks if a flag is enabled for a specific user
// using deterministic hash-based percentage rollout.
// The flag must be enabled and the user must fall within the rollout percentage.
func (s *Store) IsEnabledForUser(name, userID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	f, ok := s.flags[name]
	if !ok {
		return false
	}
	if !f.Enabled {
		return false
	}
	if f.Percentage <= 0 {
		return false
	}
	if f.Percentage >= 100 {
		return true
	}

	// Deterministic hash: same (flag, user) always produces same result.
	h := fnv.New32a()
	h.Write([]byte(name))
	h.Write([]byte(":"))
	h.Write([]byte(userID))
	bucket := int(h.Sum32() % 100)
	return bucket < f.Percentage
}

// Set updates or creates a flag's enabled state.
func (s *Store) Set(name string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if f, ok := s.flags[name]; ok {
		f.Enabled = enabled
	} else {
		s.flags[name] = &Flag{Name: name, Enabled: enabled}
	}
}

// SetPercentage sets the rollout percentage (0-100) for a flag.
func (s *Store) SetPercentage(name string, pct int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}

	if f, ok := s.flags[name]; ok {
		f.Percentage = pct
	} else {
		s.flags[name] = &Flag{Name: name, Percentage: pct}
	}
}

// All returns all flags as a slice.
func (s *Store) All() []Flag {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Flag, 0, len(s.flags))
	for _, f := range s.flags {
		result = append(result, *f)
	}
	return result
}

// DefaultFlags returns the standard GarudaPass feature flags.
func DefaultFlags() []Flag {
	return []Flag{
		{Name: "signing.pades_lta", Enabled: true, Description: "PAdES-B-LTA signing"},
		{Name: "signing.multi_party", Enabled: false, Description: "Multi-party signing (Phase 9)"},
		{Name: "identity.biometric", Enabled: false, Description: "Biometric face verification"},
		{Name: "identity.passkey", Enabled: false, Description: "FIDO2 passkey enrollment"},
		{Name: "corp.ubo_auto", Enabled: true, Description: "Automatic UBO analysis on registration"},
		{Name: "portal.webhook_retry", Enabled: true, Description: "Webhook retry with backoff"},
		{Name: "audit.kafka_publish", Enabled: false, Description: "Publish audit events to Kafka"},
		{Name: "notify.sms_enabled", Enabled: false, Description: "SMS notification channel"},
	}
}
