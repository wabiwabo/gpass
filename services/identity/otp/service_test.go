package otp

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTest(t *testing.T) (*Service, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	return NewService(rdb), mr
}

func TestGenerateAndVerify_Success(t *testing.T) {
	svc, _ := setupTest(t)
	ctx := context.Background()

	code, err := svc.Generate(ctx, "reg-123", "phone")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(code) != 6 {
		t.Errorf("code length = %d, want 6", len(code))
	}

	if err := svc.Verify(ctx, "reg-123", "phone", code); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestVerify_WrongCode(t *testing.T) {
	svc, _ := setupTest(t)
	ctx := context.Background()

	_, err := svc.Generate(ctx, "reg-123", "phone")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	err = svc.Verify(ctx, "reg-123", "phone", "000000")
	if err == nil {
		t.Fatal("expected error for wrong code")
	}
	if err != ErrOTPInvalid {
		t.Errorf("got %v, want ErrOTPInvalid", err)
	}
}

func TestVerify_Expired(t *testing.T) {
	svc, mr := setupTest(t)
	ctx := context.Background()

	_, err := svc.Generate(ctx, "reg-123", "phone")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Fast forward past TTL
	mr.FastForward(otpTTL + 1)

	err = svc.Verify(ctx, "reg-123", "phone", "123456")
	if err != ErrOTPExpired {
		t.Errorf("got %v, want ErrOTPExpired", err)
	}
}

func TestVerify_MaxAttempts(t *testing.T) {
	svc, _ := setupTest(t)
	ctx := context.Background()

	_, err := svc.Generate(ctx, "reg-123", "phone")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// 3 wrong attempts
	for i := 0; i < 3; i++ {
		svc.Verify(ctx, "reg-123", "phone", "000000")
	}

	// 4th attempt should get max attempts error (OTP already invalidated)
	err = svc.Verify(ctx, "reg-123", "phone", "000000")
	if err != ErrOTPExpired {
		t.Errorf("got %v, want ErrOTPExpired (OTP should be invalidated)", err)
	}
}

func TestGenerate_MaxSends(t *testing.T) {
	svc, _ := setupTest(t)
	ctx := context.Background()

	for i := 0; i < maxSendsDay; i++ {
		_, err := svc.Generate(ctx, "reg-123", "phone")
		if err != nil {
			t.Fatalf("Generate %d: %v", i+1, err)
		}
	}

	_, err := svc.Generate(ctx, "reg-123", "phone")
	if err != ErrMaxSends {
		t.Errorf("got %v, want ErrMaxSends", err)
	}
}
