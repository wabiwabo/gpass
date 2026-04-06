package handler

import (
	"errors"
	"regexp"
	"strings"
)

var emailRegexp = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// validateEmail checks that the email address has a valid format.
func validateEmail(email string) error {
	if email == "" {
		return errors.New("email is required")
	}
	if !emailRegexp.MatchString(email) {
		return errors.New("invalid email format")
	}
	return nil
}

// validatePhone checks that the phone number starts with +62 and has sufficient digits.
func validatePhone(phone string) error {
	if phone == "" {
		return errors.New("phone number is required")
	}
	if !strings.HasPrefix(phone, "+62") {
		return errors.New("phone number must start with +62")
	}
	// +62 followed by 9-12 digits (Indonesian mobile numbers)
	digits := phone[3:]
	if len(digits) < 9 || len(digits) > 13 {
		return errors.New("phone number must have 9-13 digits after +62")
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return errors.New("phone number must contain only digits after +62")
		}
	}
	return nil
}

// validateOTPRequest validates all fields in an OTP notification request.
func validateOTPRequest(req otpRequest) error {
	if req.Channel == "" {
		return errors.New("channel is required")
	}
	if req.Recipient == "" {
		return errors.New("recipient is required")
	}
	if req.OTPCode == "" {
		return errors.New("otp_code is required")
	}

	switch req.Channel {
	case "email":
		if err := validateEmail(req.Recipient); err != nil {
			return err
		}
	case "sms":
		if err := validatePhone(req.Recipient); err != nil {
			return err
		}
	default:
		return errors.New("channel must be \"email\" or \"sms\"")
	}

	return nil
}

// validateAlertRequest validates all fields in an alert notification request.
func validateAlertRequest(req alertRequest) error {
	if req.Channel == "" {
		return errors.New("channel is required")
	}
	if req.Recipient == "" {
		return errors.New("recipient is required")
	}
	if req.Subject == "" {
		return errors.New("subject is required")
	}
	if req.Message == "" {
		return errors.New("message is required")
	}

	if req.Channel == "email" {
		if err := validateEmail(req.Recipient); err != nil {
			return err
		}
	}

	return nil
}
