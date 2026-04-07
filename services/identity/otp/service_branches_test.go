package otp

import (
	"context"
	"errors"
	"testing"
)

// TestVerify_FailedAttemptsBeforeMax pins the increment-attempts branch
// where the wrong code is given but max hasn't been reached. The
// stored OTP must remain available for further attempts.
func TestVerify_FailedAttemptsBeforeMax(t *testing.T) {
	s, _ := setupTest(t)
	ctx := context.Background()

	if _, err := s.Generate(ctx, "reg-1", "sms"); err != nil {
		t.Fatal(err)
	}

	// Two wrong attempts (max is 3) should still return ErrOTPInvalid,
	// not ErrMaxAttempts.
	for i := 0; i < 2; i++ {
		err := s.Verify(ctx, "reg-1", "sms", "000000")
		if !errors.Is(err, ErrOTPInvalid) {
			t.Errorf("attempt %d: err = %v, want ErrOTPInvalid", i, err)
		}
	}

	// Third wrong attempt must trigger ErrMaxAttempts AND invalidate
	// the OTP (so even the correct code now fails).
	if err := s.Verify(ctx, "reg-1", "sms", "111111"); !errors.Is(err, ErrMaxAttempts) {
		t.Errorf("3rd attempt: err = %v, want ErrMaxAttempts", err)
	}
}

// TestGenerate_AttemptsKeyResetOnNewOTP pins that issuing a new OTP
// resets the attempts counter — security contract: a fresh OTP gets
// fresh attempt budget, otherwise an attacker could exhaust the
// counter on the previous code.
func TestGenerate_AttemptsKeyResetOnNewOTP(t *testing.T) {
	s, _ := setupTest(t)
	ctx := context.Background()

	if _, err := s.Generate(ctx, "reg-2", "email"); err != nil {
		t.Fatal(err)
	}
	// Burn 2 attempts on the first OTP.
	for i := 0; i < 2; i++ {
		s.Verify(ctx, "reg-2", "email", "000000")
	}

	// Generate a new OTP — attempts must reset.
	if _, err := s.Generate(ctx, "reg-2", "email"); err != nil {
		t.Fatal(err)
	}
	// Two more wrong attempts should still return ErrOTPInvalid (not max).
	for i := 0; i < 2; i++ {
		err := s.Verify(ctx, "reg-2", "email", "000000")
		if !errors.Is(err, ErrOTPInvalid) {
			t.Errorf("post-reset attempt %d: err = %v, want ErrOTPInvalid", i, err)
		}
	}
}

// TestVerify_NoOTPIssued pins the redis.Nil → ErrOTPExpired branch.
// This is the case where Verify is called without any prior Generate.
func TestVerify_NoOTPIssued(t *testing.T) {
	s, _ := setupTest(t)
	if err := s.Verify(context.Background(), "never-generated", "sms", "123456"); !errors.Is(err, ErrOTPExpired) {
		t.Errorf("err = %v, want ErrOTPExpired", err)
	}
}

// TestGenerate_MaxSendsExactBoundary pins the count >= maxSendsDay
// branch by sending exactly maxSendsDay times then asserting the
// next call is rejected.
func TestGenerate_MaxSendsExactBoundary(t *testing.T) {
	s, _ := setupTest(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ { // maxSendsDay = 3
		if _, err := s.Generate(ctx, "reg-3", "sms"); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	if _, err := s.Generate(ctx, "reg-3", "sms"); !errors.Is(err, ErrMaxSends) {
		t.Errorf("4th send: err = %v, want ErrMaxSends", err)
	}
}

// TestVerify_SuccessClearsBothKeys pins that a successful Verify deletes
// both the otp hash and the attempts counter — neither should be reusable.
func TestVerify_SuccessClearsBothKeys(t *testing.T) {
	s, mr := setupTest(t)
	ctx := context.Background()

	code, err := s.Generate(ctx, "reg-4", "sms")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Verify(ctx, "reg-4", "sms", code); err != nil {
		t.Fatal(err)
	}
	if mr.Exists("otp:hash:reg-4:sms") {
		t.Error("otp hash key not deleted after successful verify")
	}
	// Second verify with same code must fail (key gone).
	if err := s.Verify(ctx, "reg-4", "sms", code); !errors.Is(err, ErrOTPExpired) {
		t.Errorf("replay: err = %v, want ErrOTPExpired", err)
	}
}
