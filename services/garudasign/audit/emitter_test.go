package audit

import "testing"

func TestEmit_Success(t *testing.T) {
	emitter := NewLogEmitter()

	err := emitter.Emit(Event{
		UserID: "user-1",
		Action: ActionCertIssued,
		Metadata: map[string]string{
			"serial_number": "SN001",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEmit_AllActions(t *testing.T) {
	emitter := NewLogEmitter()

	actions := []string{
		ActionCertRequested,
		ActionCertIssued,
		ActionCertRevoked,
		ActionDocUploaded,
		ActionDocSigned,
		ActionDocDownloaded,
		ActionSignFailed,
	}

	for _, action := range actions {
		err := emitter.Emit(Event{
			UserID: "user-1",
			Action: action,
		})
		if err != nil {
			t.Errorf("unexpected error for action %s: %v", action, err)
		}
	}
}

func TestEmit_MissingUserID(t *testing.T) {
	emitter := NewLogEmitter()

	err := emitter.Emit(Event{
		Action: ActionCertIssued,
	})
	if err == nil {
		t.Error("expected error for missing user ID")
	}
}

func TestEmit_MissingAction(t *testing.T) {
	emitter := NewLogEmitter()

	err := emitter.Emit(Event{
		UserID: "user-1",
	})
	if err == nil {
		t.Error("expected error for missing action")
	}
}
