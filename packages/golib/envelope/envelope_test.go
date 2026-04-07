package envelope

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNew_Builder(t *testing.T) {
	b, err := New("evt-1", "user.created", "identity").
		Topic("users").
		SchemaVersion("1.0").
		Correlation("corr-1", "cause-1").
		TraceID("trace-abc").
		MaxRetries(3).
		Header("tenant", "acme").
		Data(map[string]string{"name": "Budi"})
	if err != nil {
		t.Fatal(err)
	}

	env := b.Build()

	if env.ID != "evt-1" {
		t.Errorf("id: got %q", env.ID)
	}
	if env.Type != "user.created" {
		t.Errorf("type: got %q", env.Type)
	}
	if env.Source != "identity" {
		t.Errorf("source: got %q", env.Source)
	}
	if env.Topic != "users" {
		t.Errorf("topic: got %q", env.Topic)
	}
	if env.SchemaVersion != "1.0" {
		t.Errorf("schema version: got %q", env.SchemaVersion)
	}
	if env.CorrelationID != "corr-1" {
		t.Errorf("correlation: got %q", env.CorrelationID)
	}
	if env.CausationID != "cause-1" {
		t.Errorf("causation: got %q", env.CausationID)
	}
	if env.TraceID != "trace-abc" {
		t.Errorf("trace: got %q", env.TraceID)
	}
	if env.MaxRetries != 3 {
		t.Errorf("max retries: got %d", env.MaxRetries)
	}
	if env.Headers["tenant"] != "acme" {
		t.Errorf("header: got %q", env.Headers["tenant"])
	}
	if env.EnvVersion != Version {
		t.Errorf("env version: got %q", env.EnvVersion)
	}
	if env.ContentType != "application/json" {
		t.Errorf("content type: got %q", env.ContentType)
	}
	if env.DataHash == "" {
		t.Error("data hash should be computed")
	}
	if env.Timestamp.IsZero() {
		t.Error("timestamp should be set")
	}
}

func TestBuilder_DataRaw(t *testing.T) {
	raw := json.RawMessage(`{"key":"value"}`)
	env := New("evt-2", "test", "svc").DataRaw(raw).Build()

	if string(env.Data) != `{"key":"value"}` {
		t.Errorf("data: got %s", env.Data)
	}
	if env.DataHash == "" {
		t.Error("hash should be computed for raw data")
	}
}

func TestBuilder_ExpiresIn(t *testing.T) {
	env := New("evt-3", "test", "svc").ExpiresIn(1 * time.Hour).Build()

	if env.ExpiresAt.IsZero() {
		t.Error("expires_at should be set")
	}
	if time.Until(env.ExpiresAt) < 59*time.Minute {
		t.Error("should expire in ~1 hour")
	}
}

func TestBuilder_ExpiresAt(t *testing.T) {
	target := time.Now().Add(24 * time.Hour)
	env := New("evt-4", "test", "svc").ExpiresAt(target).Build()

	if !env.ExpiresAt.Equal(target) {
		t.Errorf("expires_at: got %v, want %v", env.ExpiresAt, target)
	}
}

func TestEnvelope_Validate_Valid(t *testing.T) {
	b, _ := New("evt-5", "test", "svc").Data(map[string]string{"a": "b"})
	env := b.Build()

	if err := env.Validate(); err != nil {
		t.Errorf("valid envelope should pass: %v", err)
	}
}

func TestEnvelope_Validate_MissingID(t *testing.T) {
	env := Envelope{Type: "test", Source: "svc", Timestamp: time.Now(), Data: json.RawMessage(`{}`)}
	if err := env.Validate(); err == nil {
		t.Error("should fail without id")
	}
}

func TestEnvelope_Validate_MissingType(t *testing.T) {
	env := Envelope{ID: "1", Source: "svc", Timestamp: time.Now(), Data: json.RawMessage(`{}`)}
	if err := env.Validate(); err == nil {
		t.Error("should fail without type")
	}
}

func TestEnvelope_Validate_MissingSource(t *testing.T) {
	env := Envelope{ID: "1", Type: "test", Timestamp: time.Now(), Data: json.RawMessage(`{}`)}
	if err := env.Validate(); err == nil {
		t.Error("should fail without source")
	}
}

func TestEnvelope_Validate_MissingTimestamp(t *testing.T) {
	env := Envelope{ID: "1", Type: "test", Source: "svc", Data: json.RawMessage(`{}`)}
	if err := env.Validate(); err == nil {
		t.Error("should fail without timestamp")
	}
}

func TestEnvelope_Validate_MissingData(t *testing.T) {
	env := Envelope{ID: "1", Type: "test", Source: "svc", Timestamp: time.Now()}
	if err := env.Validate(); err == nil {
		t.Error("should fail without data")
	}
}

func TestEnvelope_Validate_HashMismatch(t *testing.T) {
	b, _ := New("evt-6", "test", "svc").Data(map[string]string{"a": "b"})
	env := b.Build()
	env.DataHash = "tampered-hash"

	if err := env.Validate(); err == nil {
		t.Error("should fail on hash mismatch")
	}
}

func TestEnvelope_IsExpired(t *testing.T) {
	env := Envelope{ExpiresAt: time.Now().Add(-1 * time.Hour)}
	if !env.IsExpired() {
		t.Error("past expiry should be expired")
	}

	env.ExpiresAt = time.Now().Add(1 * time.Hour)
	if env.IsExpired() {
		t.Error("future expiry should not be expired")
	}

	env.ExpiresAt = time.Time{}
	if env.IsExpired() {
		t.Error("zero expiry means no expiration")
	}
}

func TestEnvelope_CanRetry(t *testing.T) {
	env := Envelope{MaxRetries: 3, Attempt: 2}
	if !env.CanRetry() {
		t.Error("attempt 2/3 should allow retry")
	}

	env.Attempt = 3
	if env.CanRetry() {
		t.Error("attempt 3/3 should not allow retry")
	}

	env.MaxRetries = 0
	if !env.CanRetry() {
		t.Error("no limit should always allow retry")
	}
}

func TestEnvelope_RecordError(t *testing.T) {
	env := Envelope{}
	var dst any
	env.RecordError(json.Unmarshal([]byte("invalid"), &dst)) // Some error.

	if env.Attempt != 1 {
		t.Errorf("attempt: got %d", env.Attempt)
	}
	if env.LastError == "" {
		t.Error("last error should be set")
	}
	if env.FirstError.IsZero() {
		t.Error("first error should be set")
	}

	firstErr := env.FirstError
	env.RecordError(json.Unmarshal([]byte("bad"), &dst))
	if env.Attempt != 2 {
		t.Errorf("second attempt: got %d", env.Attempt)
	}
	if !env.FirstError.Equal(firstErr) {
		t.Error("first error should not change")
	}
}

func TestEnvelope_Decode(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	b, _ := New("evt-7", "test", "svc").Data(payload{Name: "Budi", Age: 30})
	env := b.Build()

	var got payload
	if err := env.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "Budi" || got.Age != 30 {
		t.Errorf("decode: got %+v", got)
	}
}

func TestEnvelope_MarshalUnmarshal(t *testing.T) {
	b, _ := New("evt-8", "user.created", "identity").
		Topic("users").
		Correlation("corr-1", "").
		Header("env", "test").
		Data(map[string]string{"name": "Siti"})
	env := b.Build()

	data, err := env.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.ID != "evt-8" {
		t.Errorf("id: got %q", parsed.ID)
	}
	if parsed.Type != "user.created" {
		t.Errorf("type: got %q", parsed.Type)
	}
	if parsed.Headers["env"] != "test" {
		t.Errorf("header: got %q", parsed.Headers["env"])
	}
}

func TestUnmarshal_Invalid(t *testing.T) {
	_, err := Unmarshal([]byte("not json"))
	if err == nil {
		t.Error("should fail on invalid JSON")
	}
}

func TestRouter_Route(t *testing.T) {
	r := NewRouter()
	var handled string
	r.On("user.created", func(env *Envelope) error {
		handled = env.ID
		return nil
	})

	b, _ := New("evt-9", "user.created", "svc").Data(map[string]string{"a": "b"})
	env := b.Build()

	if err := r.Route(&env); err != nil {
		t.Fatal(err)
	}
	if handled != "evt-9" {
		t.Errorf("handled: got %q", handled)
	}
}

func TestRouter_Fallback(t *testing.T) {
	r := NewRouter()
	var fallbackCalled bool
	r.Fallback(func(env *Envelope) error {
		fallbackCalled = true
		return nil
	})

	b, _ := New("evt-10", "unknown.type", "svc").Data(map[string]string{"a": "b"})
	env := b.Build()

	if err := r.Route(&env); err != nil {
		t.Fatal(err)
	}
	if !fallbackCalled {
		t.Error("fallback should be called for unregistered type")
	}
}

func TestRouter_NoHandler(t *testing.T) {
	r := NewRouter()

	b, _ := New("evt-11", "unknown.type", "svc").Data(map[string]string{"a": "b"})
	env := b.Build()

	if err := r.Route(&env); err == nil {
		t.Error("should fail with no handler and no fallback")
	}
}

func TestRouter_InvalidEnvelope(t *testing.T) {
	r := NewRouter()
	env := &Envelope{} // Missing required fields.

	if err := r.Route(env); err == nil {
		t.Error("should fail on invalid envelope")
	}
}

func TestRouter_ExpiredEnvelope(t *testing.T) {
	r := NewRouter()
	r.On("test", func(env *Envelope) error { return nil })

	b, _ := New("evt-12", "test", "svc").Data(map[string]string{"a": "b"})
	env := b.Build()
	env.ExpiresAt = time.Now().Add(-1 * time.Hour)

	if err := r.Route(&env); err == nil {
		t.Error("should fail on expired envelope")
	}
}

func TestRouter_Types(t *testing.T) {
	r := NewRouter()
	r.On("user.created", func(env *Envelope) error { return nil })
	r.On("user.deleted", func(env *Envelope) error { return nil })

	types := r.Types()
	if len(types) != 2 {
		t.Errorf("types: got %d", len(types))
	}
}

func TestBuilder_DataError(t *testing.T) {
	// json.Marshal can't fail on map[string]string, but test the error path.
	_, err := New("evt-13", "test", "svc").Data(make(chan int))
	if err == nil {
		t.Error("should fail on unmarshalable data")
	}
}
