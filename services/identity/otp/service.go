package otp

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

const (
	otpTTL       = 5 * time.Minute
	maxSendsDay  = 3
	maxAttempts  = 3
	sendCountTTL = 24 * time.Hour
)

var (
	ErrOTPExpired   = errors.New("OTP has expired")
	ErrOTPInvalid   = errors.New("invalid OTP code")
	ErrMaxAttempts  = errors.New("maximum verification attempts exceeded")
	ErrMaxSends     = errors.New("maximum OTP sends per day exceeded")
)

// Service handles OTP generation and verification using Redis.
type Service struct {
	rdb *redis.Client
}

// NewService creates a new OTP service backed by Redis.
func NewService(rdb *redis.Client) *Service {
	return &Service{rdb: rdb}
}

// Generate creates a 6-digit OTP, stores a bcrypt hash in Redis with a 5-minute TTL,
// and enforces a maximum of 3 sends per day per registration+channel.
func (s *Service) Generate(ctx context.Context, registrationID, channel string) (string, error) {
	// Check send count
	sendKey := fmt.Sprintf("otp:sends:%s:%s", registrationID, channel)
	count, err := s.rdb.Get(ctx, sendKey).Int()
	if err != nil && !errors.Is(err, redis.Nil) {
		return "", fmt.Errorf("check send count: %w", err)
	}
	if count >= maxSendsDay {
		return "", ErrMaxSends
	}

	// Generate 6-digit code
	code, err := generateCode(6)
	if err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}

	// Hash the code
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash code: %w", err)
	}

	// Store hash in Redis
	otpKey := fmt.Sprintf("otp:hash:%s:%s", registrationID, channel)
	attemptsKey := fmt.Sprintf("otp:attempts:%s:%s", registrationID, channel)

	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, otpKey, string(hash), otpTTL)
	pipe.Del(ctx, attemptsKey) // Reset attempts on new OTP
	pipe.Incr(ctx, sendKey)
	pipe.Expire(ctx, sendKey, sendCountTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", fmt.Errorf("store OTP: %w", err)
	}

	return code, nil
}

// Verify checks the OTP code against the stored hash. After 3 failed attempts
// the OTP is invalidated.
func (s *Service) Verify(ctx context.Context, registrationID, channel, code string) error {
	otpKey := fmt.Sprintf("otp:hash:%s:%s", registrationID, channel)
	attemptsKey := fmt.Sprintf("otp:attempts:%s:%s", registrationID, channel)

	// Check attempts
	attempts, err := s.rdb.Get(ctx, attemptsKey).Int()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("check attempts: %w", err)
	}
	if attempts >= maxAttempts {
		// Invalidate the OTP
		s.rdb.Del(ctx, otpKey, attemptsKey)
		return ErrMaxAttempts
	}

	// Get stored hash
	hash, err := s.rdb.Get(ctx, otpKey).Result()
	if errors.Is(err, redis.Nil) {
		return ErrOTPExpired
	}
	if err != nil {
		return fmt.Errorf("get OTP hash: %w", err)
	}

	// Compare
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)); err != nil {
		// Increment attempts
		pipe := s.rdb.Pipeline()
		pipe.Incr(ctx, attemptsKey)
		pipe.Expire(ctx, attemptsKey, otpTTL)
		pipe.Exec(ctx)

		// Check if we just hit max attempts
		newAttempts := attempts + 1
		if newAttempts >= maxAttempts {
			s.rdb.Del(ctx, otpKey, attemptsKey)
			return ErrMaxAttempts
		}
		return ErrOTPInvalid
	}

	// Success — clean up
	s.rdb.Del(ctx, otpKey, attemptsKey)
	return nil
}

func generateCode(length int) (string, error) {
	code := ""
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		code += fmt.Sprintf("%d", n.Int64())
	}
	return code, nil
}
