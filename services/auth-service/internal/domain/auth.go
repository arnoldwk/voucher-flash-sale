package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrRateLimited    = errors.New("rate limited")
	ErrOTPExpired     = errors.New("OTP expired")
	ErrOTPLocked      = errors.New("max retry reached")
	ErrInvalidOTP     = errors.New("invalid OTP")
	ErrChallengeEmpty = errors.New("no active OTP challenge found")
)

type OTPChallenge struct {
	ID           string    `json:"id"`
	Phone        string    `json:"phone"`
	OTPCode      string    `json:"-"`
	Status       string    `json:"status"` // ACTIVE, VERIFIED, EXPIRED, LOCKED
	AttemptCount int       `json:"attemptCount"`
	ExpiresAt    time.Time `json:"expiresAt"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type AuthSession struct {
	ID           string    `json:"id"`
	CustomerID   string    `json:"customerId"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
	CreatedAt    time.Time `json:"createdAt"`
}

type AuthRepository interface {
	CreateOTPChallenge(ctx context.Context, challenge *OTPChallenge) error
	GetLatestActiveChallenge(ctx context.Context, phone string) (*OTPChallenge, error)
	UpdateChallengeStatus(ctx context.Context, id, status string) error
	IncrementChallengeAttempt(ctx context.Context, id string) (newAttemptCount int, err error)
	CreateSession(ctx context.Context, session *AuthSession) error
}

type RateLimiter interface {
	CheckAndRecordOTPRequest(ctx context.Context, phone string) (allowed bool, retryAfter int, err error)
}
