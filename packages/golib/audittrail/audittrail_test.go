package audittrail

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
)

func TestBuilder_FullEntry(t *testing.T) {
	entry := NewEntry(ActionCreate).
		Actor("user-123", "user").
		Resource("consent", "c-456").
		Description("User granted consent for KYC").
		IP("103.28.12.5").
		UA("Mozilla/5.0").
		Meta("client_id", "app-1").
		Meta("fields", []string{"name", "nik"}).
		Service("garudainfo").
		Success().
		Build()

	if entry.Action != ActionCreate {
		t.Errorf("action: got %q", entry.Action)
	}
	if entry.ActorID != "user-123" {
		t.Errorf("actor_id: got %q", entry.ActorID)
	}
	if entry.ResourceType != "consent" {
		t.Errorf("resource_type: got %q", entry.ResourceType)
	}
	if entry.ResourceID != "c-456" {
		t.Errorf("resource_id: got %q", entry.ResourceID)
	}
	if entry.IPAddress != "103.28.12.5" {
		t.Errorf("ip: got %q", entry.IPAddress)
	}
	if entry.Result != "success" {
		t.Errorf("result: got %q", entry.Result)
	}
	if entry.Service != "garudainfo" {
		t.Errorf("service: got %q", entry.Service)
	}
	if entry.Timestamp.IsZero() {
		t.Error("timestamp should be set")
	}
}

func TestBuilder_Failure(t *testing.T) {
	entry := NewEntry(ActionLogin).
		Actor("user-bad", "user").
		Failure().
		Build()

	if entry.Result != "failure" {
		t.Errorf("result: got %q", entry.Result)
	}
}

func TestBuilder_Denied(t *testing.T) {
	entry := NewEntry(ActionRead).
		Actor("user-unauth", "user").
		Resource("document", "doc-1").
		Denied().
		Build()

	if entry.Result != "denied" {
		t.Errorf("result: got %q", entry.Result)
	}
}

func TestBuilder_DefaultResult(t *testing.T) {
	entry := NewEntry(ActionRead).Build()
	if entry.Result != "success" {
		t.Errorf("default result: got %q", entry.Result)
	}
}

func TestBuilder_Metadata(t *testing.T) {
	entry := NewEntry(ActionUpdate).
		Meta("old_value", "A").
		Meta("new_value", "B").
		Build()

	if entry.Metadata["old_value"] != "A" {
		t.Errorf("old_value: got %v", entry.Metadata["old_value"])
	}
	if entry.Metadata["new_value"] != "B" {
		t.Errorf("new_value: got %v", entry.Metadata["new_value"])
	}
}

func TestEntry_JSON(t *testing.T) {
	entry := NewEntry(ActionDelete).
		Actor("admin", "user").
		Resource("user", "u-1").
		Build()

	jsonStr := entry.JSON()
	if jsonStr == "" {
		t.Fatal("JSON should not be empty")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["action"] != "DELETE" {
		t.Errorf("action: got %v", parsed["action"])
	}
}

func TestMemorySink_Record(t *testing.T) {
	sink := NewMemorySink()
	entry := NewEntry(ActionCreate).Actor("u1", "user").Build()

	err := sink.Record(context.Background(), entry)
	if err != nil {
		t.Fatal(err)
	}
	if sink.Count() != 1 {
		t.Errorf("count: got %d", sink.Count())
	}
}

func TestMemorySink_FindByAction(t *testing.T) {
	sink := NewMemorySink()
	sink.Record(context.Background(), NewEntry(ActionCreate).Build())
	sink.Record(context.Background(), NewEntry(ActionRead).Build())
	sink.Record(context.Background(), NewEntry(ActionCreate).Build())

	creates := sink.FindByAction(ActionCreate)
	if len(creates) != 2 {
		t.Errorf("creates: got %d", len(creates))
	}
}

func TestMemorySink_FindByActor(t *testing.T) {
	sink := NewMemorySink()
	sink.Record(context.Background(), NewEntry(ActionCreate).Actor("alice", "user").Build())
	sink.Record(context.Background(), NewEntry(ActionRead).Actor("bob", "user").Build())
	sink.Record(context.Background(), NewEntry(ActionUpdate).Actor("alice", "user").Build())

	aliceEntries := sink.FindByActor("alice")
	if len(aliceEntries) != 2 {
		t.Errorf("alice entries: got %d", len(aliceEntries))
	}
}

func TestMemorySink_Entries_ReturnsACopy(t *testing.T) {
	sink := NewMemorySink()
	sink.Record(context.Background(), NewEntry(ActionCreate).Build())

	entries := sink.Entries()
	entries = append(entries, Entry{}) // modify returned slice
	if sink.Count() != 1 {
		t.Error("modifying Entries() result should not affect sink")
	}
}

func TestMemorySink_ConcurrentAccess(t *testing.T) {
	sink := NewMemorySink()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			entry := NewEntry(ActionCreate).
				Actor("user-"+string(rune('A'+n%26)), "user").
				Build()
			sink.Record(context.Background(), entry)
			sink.Count()
			sink.Entries()
			sink.FindByAction(ActionCreate)
		}(i)
	}
	wg.Wait()

	if sink.Count() != 50 {
		t.Errorf("count after concurrent writes: got %d", sink.Count())
	}
}

func TestAllActions(t *testing.T) {
	actions := []Action{
		ActionCreate, ActionRead, ActionUpdate, ActionDelete,
		ActionLogin, ActionLogout, ActionGrant, ActionRevoke,
		ActionExport, ActionSign, ActionVerify, ActionConsent,
		ActionAccessDeny,
	}

	for _, action := range actions {
		entry := NewEntry(action).Build()
		if entry.Action != action {
			t.Errorf("action %q: got %q", action, entry.Action)
		}
	}
}
