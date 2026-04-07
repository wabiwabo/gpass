package store

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Validation limits enforced on every audit event.
// These bound storage cost and prevent injection / DoS via oversized events.
const (
	MaxEventTypeLen     = 64
	MaxActorIDLen       = 128
	MaxActorTypeLen     = 32
	MaxResourceIDLen    = 128
	MaxResourceTypeLen  = 64
	MaxActionLen        = 64
	MaxIPAddressLen     = 45 // IPv6 max
	MaxUserAgentLen     = 512
	MaxServiceNameLen   = 64
	MaxRequestIDLen     = 128
	MaxStatusLen        = 32
	MaxMetadataKeys     = 50
	MaxMetadataKeyLen   = 64
	MaxMetadataValueLen = 1024
)

// allowedActorTypes is the closed set of valid actor types.
var allowedActorTypes = map[string]bool{
	"":        true, // empty defaults to USER in Append
	"USER":            true,
	"SERVICE":         true,
	"SERVICE_ACCOUNT": true,
	"SYSTEM":          true,
	"ADMIN":           true,
}

// allowedStatuses is the closed set of valid statuses.
var allowedStatuses = map[string]bool{
	"":        true, // empty defaults to SUCCESS
	"SUCCESS": true,
	"FAILURE": true,
	"DENIED":  true,
	"ERROR":   true,
}

// ValidateEvent enforces required fields, length limits, and enum constraints.
// It is called by both InMemory and Postgres Append implementations so the
// validation contract is identical regardless of backing store.
func ValidateEvent(e *AuditEvent) error {
	if e == nil {
		return fmt.Errorf("event is nil")
	}

	if err := requireBounded("event_type", e.EventType, MaxEventTypeLen); err != nil {
		return err
	}
	if err := requireBounded("actor_id", e.ActorID, MaxActorIDLen); err != nil {
		return err
	}
	if err := requireBounded("action", e.Action, MaxActionLen); err != nil {
		return err
	}

	if err := bounded("actor_type", e.ActorType, MaxActorTypeLen); err != nil {
		return err
	}
	if !allowedActorTypes[e.ActorType] {
		return fmt.Errorf("actor_type %q not in allowed set", e.ActorType)
	}

	if err := bounded("resource_id", e.ResourceID, MaxResourceIDLen); err != nil {
		return err
	}
	if err := bounded("resource_type", e.ResourceType, MaxResourceTypeLen); err != nil {
		return err
	}
	if err := bounded("ip_address", e.IPAddress, MaxIPAddressLen); err != nil {
		return err
	}
	if err := bounded("user_agent", e.UserAgent, MaxUserAgentLen); err != nil {
		return err
	}
	if err := bounded("service_name", e.ServiceName, MaxServiceNameLen); err != nil {
		return err
	}
	if err := bounded("request_id", e.RequestID, MaxRequestIDLen); err != nil {
		return err
	}
	if err := bounded("status", e.Status, MaxStatusLen); err != nil {
		return err
	}
	if !allowedStatuses[e.Status] {
		return fmt.Errorf("status %q not in allowed set", e.Status)
	}

	if len(e.Metadata) > MaxMetadataKeys {
		return fmt.Errorf("metadata has %d keys, max %d", len(e.Metadata), MaxMetadataKeys)
	}
	for k, v := range e.Metadata {
		if k == "" {
			return fmt.Errorf("metadata key is empty")
		}
		if utf8.RuneCountInString(k) > MaxMetadataKeyLen {
			return fmt.Errorf("metadata key %q exceeds %d chars", k, MaxMetadataKeyLen)
		}
		if utf8.RuneCountInString(v) > MaxMetadataValueLen {
			return fmt.Errorf("metadata[%q] value exceeds %d chars", k, MaxMetadataValueLen)
		}
		if strings.ContainsAny(k, "\x00\n\r") {
			return fmt.Errorf("metadata key %q contains control characters", k)
		}
	}
	return nil
}

func requireBounded(name, v string, max int) error {
	if v == "" {
		return fmt.Errorf("%s is required", name)
	}
	return bounded(name, v, max)
}

func bounded(name, v string, max int) error {
	if utf8.RuneCountInString(v) > max {
		return fmt.Errorf("%s exceeds %d chars", name, max)
	}
	if strings.ContainsAny(v, "\x00") {
		return fmt.Errorf("%s contains null bytes", name)
	}
	return nil
}
