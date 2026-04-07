package store

import (
	"strings"
	"testing"
)

func TestValidateDeletionRequest_Valid(t *testing.T) {
	r := &DeletionRequest{UserID: "user-1", Reason: "user_request"}
	if err := ValidateDeletionRequest(r); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateDeletionRequest_Nil(t *testing.T) {
	if err := ValidateDeletionRequest(nil); err == nil {
		t.Error("expected error for nil")
	}
}

func TestValidateDeletionRequest_Required(t *testing.T) {
	r := &DeletionRequest{Reason: "user_request"}
	if err := ValidateDeletionRequest(r); err == nil {
		t.Error("expected user_id required")
	}
}

func TestValidateDeletionRequest_BadReason(t *testing.T) {
	r := &DeletionRequest{UserID: "u1", Reason: "bogus"}
	if err := ValidateDeletionRequest(r); err != ErrInvalidReason {
		t.Errorf("got %v, want ErrInvalidReason", err)
	}
}

func TestValidateDeletionRequest_LengthBound(t *testing.T) {
	r := &DeletionRequest{UserID: strings.Repeat("u", MaxUserIDLen+1), Reason: "user_request"}
	if err := ValidateDeletionRequest(r); err == nil {
		t.Error("expected length error")
	}
}

func TestValidateDeletionRequest_NullByte(t *testing.T) {
	r := &DeletionRequest{UserID: "evil\x00user", Reason: "user_request"}
	if err := ValidateDeletionRequest(r); err == nil {
		t.Error("expected null byte rejection")
	}
}

func TestValidateStatusUpdate_Valid(t *testing.T) {
	if err := ValidateStatusUpdate("COMPLETED", []string{"personal_info", "biometric"}); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateStatusUpdate_BadStatus(t *testing.T) {
	if err := ValidateStatusUpdate("MAYBE", nil); err == nil {
		t.Error("expected enum violation")
	}
}

func TestValidateStatusUpdate_BadCategory(t *testing.T) {
	if err := ValidateStatusUpdate("COMPLETED", []string{"hacker_data"}); err == nil {
		t.Error("expected category allow-list violation")
	}
}

func TestValidateStatusUpdate_TooMany(t *testing.T) {
	cats := make([]string, MaxDataCategories+1)
	for i := range cats {
		cats[i] = "personal_info"
	}
	if err := ValidateStatusUpdate("COMPLETED", cats); err == nil {
		t.Error("expected too-many error")
	}
}

func TestValidateStatusUpdate_EmptyCategory(t *testing.T) {
	if err := ValidateStatusUpdate("COMPLETED", []string{""}); err == nil {
		t.Error("expected empty category error")
	}
}
