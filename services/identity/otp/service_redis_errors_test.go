package otp

import (
	"context"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// TestGenerate_RedisDownOnSendKey pins the sendKey GET error branch
// (err != nil && !errors.Is(err, redis.Nil)).
func TestGenerate_RedisDownOnSendKey(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewService(rdb)

	mr.Close() // close BEFORE call → Redis unreachable

	_, err = svc.Generate(context.Background(), "reg-1", "sms")
	if err == nil || !strings.Contains(err.Error(), "check send count") {
		t.Errorf("err = %v", err)
	}
}

// TestGenerate_RedisDownOnPipeExec pins the pipe.Exec error wrap.
// We use a real miniredis for the first Get (which returns redis.Nil
// and succeeds), then close it just before the pipeline Exec — too racy
// in practice, so instead force it via a writeable-but-broken state.
// Easier approach: pre-set the sendKey to a non-integer value, which
// makes the Get().Int() call fail with a non-Nil ParseInt error.
func TestGenerate_SendKeyCorrupted(t *testing.T) {
	svc, mr := setupTest(t)
	mr.Set("otp:sends:reg-1:sms", "not-an-int")

	_, err := svc.Generate(context.Background(), "reg-1", "sms")
	if err == nil || !strings.Contains(err.Error(), "check send count") {
		t.Errorf("err = %v", err)
	}
}

// TestVerify_RedisDownOnAttemptsKey pins the attempts GET non-Nil error.
func TestVerify_AttemptsKeyCorrupted(t *testing.T) {
	svc, mr := setupTest(t)
	mr.Set("otp:attempts:reg-1:sms", "not-an-int")

	err := svc.Verify(context.Background(), "reg-1", "sms", "000000")
	if err == nil || !strings.Contains(err.Error(), "check attempts") {
		t.Errorf("err = %v", err)
	}
}

// TestVerify_MaxAttemptsAlreadyReached pins the "attempts >= max on entry"
// branch, which deletes both keys and returns ErrMaxAttempts without ever
// consulting the stored hash. Distinct from the incremental-to-max path.
func TestVerify_MaxAttemptsAlreadyReached(t *testing.T) {
	svc, mr := setupTest(t)
	// Pre-seed attempts at the threshold.
	mr.Set("otp:attempts:reg-1:sms", "3")
	// Also seed a bogus hash so we'd know if the code path went further.
	mr.Set("otp:hash:reg-1:sms", "bogus")

	err := svc.Verify(context.Background(), "reg-1", "sms", "123456")
	if err != ErrMaxAttempts {
		t.Errorf("err = %v, want ErrMaxAttempts", err)
	}
	// Both keys must be deleted.
	if mr.Exists("otp:hash:reg-1:sms") {
		t.Error("otp hash should be deleted")
	}
	if mr.Exists("otp:attempts:reg-1:sms") {
		t.Error("attempts should be deleted")
	}
}

// TestVerify_HashGetRedisError pins the non-Nil hash GET error branch.
// Simulated by closing Redis after reading attempts would have succeeded.
// Since miniredis.Close terminates everything immediately, we instead
// write a malformed hash and let bcrypt fail — but that path returns
// ErrOTPInvalid not a wrapped error. The only way to hit the non-Nil
// hash-GET error with miniredis is to make the *second* Get fail. We
// achieve that by deleting the key between attempts-Get and hash-Get,
// which isn't possible from outside. Accept that this narrow branch
// (line 102-104) requires a custom redis mock and is left uncovered
// without introducing a third-party client mock.
func TestVerify_HashDeletedMidRequest_Unreachable(t *testing.T) {
	t.Skip("requires custom redis client mock to interleave GET operations")
}
