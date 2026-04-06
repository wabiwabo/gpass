package handler

import (
	"testing"
)

func TestValidateEmail_Valid(t *testing.T) {
	valid := []string{
		"user@example.com",
		"first.last@domain.co.id",
		"user+tag@example.org",
		"name@sub.domain.com",
		"a@b.cd",
	}
	for _, email := range valid {
		if err := validateEmail(email); err != nil {
			t.Errorf("validateEmail(%q) unexpected error: %v", email, err)
		}
	}
}

func TestValidateEmail_Invalid(t *testing.T) {
	invalid := []string{
		"",
		"notanemail",
		"@example.com",
		"user@",
		"user@.com",
		"user@com",
		"user @example.com",
		"user@exam ple.com",
	}
	for _, email := range invalid {
		if err := validateEmail(email); err == nil {
			t.Errorf("validateEmail(%q) expected error, got nil", email)
		}
	}
}

func TestValidatePhone_Valid(t *testing.T) {
	valid := []string{
		"+6281234567890",
		"+62812345678",
		"+628123456789012",
	}
	for _, phone := range valid {
		if err := validatePhone(phone); err != nil {
			t.Errorf("validatePhone(%q) unexpected error: %v", phone, err)
		}
	}
}

func TestValidatePhone_Invalid(t *testing.T) {
	invalid := []string{
		"",
		"081234567890",
		"+1234567890",
		"+628",
		"+62abc",
		"+6281234567890123456",
	}
	for _, phone := range invalid {
		if err := validatePhone(phone); err == nil {
			t.Errorf("validatePhone(%q) expected error, got nil", phone)
		}
	}
}

func TestValidateOTPRequest_Valid_Email(t *testing.T) {
	req := otpRequest{
		Channel:   "email",
		Recipient: "user@example.com",
		OTPCode:   "123456",
	}
	if err := validateOTPRequest(req); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateOTPRequest_Valid_SMS(t *testing.T) {
	req := otpRequest{
		Channel:   "sms",
		Recipient: "+6281234567890",
		OTPCode:   "123456",
	}
	if err := validateOTPRequest(req); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateOTPRequest_MissingChannel(t *testing.T) {
	req := otpRequest{
		Recipient: "user@example.com",
		OTPCode:   "123456",
	}
	if err := validateOTPRequest(req); err == nil {
		t.Error("expected error for missing channel")
	}
}

func TestValidateOTPRequest_MissingRecipient(t *testing.T) {
	req := otpRequest{
		Channel: "email",
		OTPCode: "123456",
	}
	if err := validateOTPRequest(req); err == nil {
		t.Error("expected error for missing recipient")
	}
}

func TestValidateOTPRequest_MissingOTPCode(t *testing.T) {
	req := otpRequest{
		Channel:   "email",
		Recipient: "user@example.com",
	}
	if err := validateOTPRequest(req); err == nil {
		t.Error("expected error for missing otp_code")
	}
}

func TestValidateOTPRequest_InvalidEmail(t *testing.T) {
	req := otpRequest{
		Channel:   "email",
		Recipient: "notanemail",
		OTPCode:   "123456",
	}
	if err := validateOTPRequest(req); err == nil {
		t.Error("expected error for invalid email")
	}
}

func TestValidateOTPRequest_InvalidPhone(t *testing.T) {
	req := otpRequest{
		Channel:   "sms",
		Recipient: "081234567890",
		OTPCode:   "123456",
	}
	if err := validateOTPRequest(req); err == nil {
		t.Error("expected error for invalid phone")
	}
}

func TestValidateOTPRequest_InvalidChannel(t *testing.T) {
	req := otpRequest{
		Channel:   "pigeon",
		Recipient: "user@example.com",
		OTPCode:   "123456",
	}
	if err := validateOTPRequest(req); err == nil {
		t.Error("expected error for invalid channel")
	}
}

func TestValidateAlertRequest_Valid(t *testing.T) {
	req := alertRequest{
		Channel:   "email",
		Recipient: "admin@example.com",
		Subject:   "Alert",
		Message:   "Something happened",
	}
	if err := validateAlertRequest(req); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateAlertRequest_MissingChannel(t *testing.T) {
	req := alertRequest{
		Recipient: "admin@example.com",
		Subject:   "Alert",
		Message:   "Something happened",
	}
	if err := validateAlertRequest(req); err == nil {
		t.Error("expected error for missing channel")
	}
}

func TestValidateAlertRequest_MissingRecipient(t *testing.T) {
	req := alertRequest{
		Channel: "email",
		Subject: "Alert",
		Message: "Something happened",
	}
	if err := validateAlertRequest(req); err == nil {
		t.Error("expected error for missing recipient")
	}
}

func TestValidateAlertRequest_MissingSubject(t *testing.T) {
	req := alertRequest{
		Channel:   "email",
		Recipient: "admin@example.com",
		Message:   "Something happened",
	}
	if err := validateAlertRequest(req); err == nil {
		t.Error("expected error for missing subject")
	}
}

func TestValidateAlertRequest_MissingMessage(t *testing.T) {
	req := alertRequest{
		Channel:   "email",
		Recipient: "admin@example.com",
		Subject:   "Alert",
	}
	if err := validateAlertRequest(req); err == nil {
		t.Error("expected error for missing message")
	}
}

func TestValidateAlertRequest_InvalidEmail(t *testing.T) {
	req := alertRequest{
		Channel:   "email",
		Recipient: "notanemail",
		Subject:   "Alert",
		Message:   "Something happened",
	}
	if err := validateAlertRequest(req); err == nil {
		t.Error("expected error for invalid email")
	}
}
