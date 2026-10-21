package usecase

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"voucher-flash-sale/pkg/jwt"
	"voucher-flash-sale/services/auth-service/internal/client"
	"voucher-flash-sale/services/auth-service/internal/domain"

	"github.com/google/uuid"
)

type RequestOTPResult struct {
	ExpiresAt       time.Time `json:"expiresAt"`
	CooldownSeconds int       `json:"cooldownSeconds"`
	DebugOTP        string    `json:"-"` // For testing/logging only
}

type RateLimitError struct {
	RetryAfter int
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited, retry after %d seconds", e.RetryAfter)
}

type InvalidOTPError struct {
	AttemptsRemaining int
}

func (e *InvalidOTPError) Error() string {
	return fmt.Sprintf("invalid OTP, %d attempts remaining", e.AttemptsRemaining)
}

type VerifyOTPResult struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int    `json:"expiresIn"`
}

type AuthUsecase struct {
	repo           domain.AuthRepository
	rateLimiter    domain.RateLimiter
	customerClient *client.CustomerClient
	jwtSecret      string
	enableDemoOTP  bool
}

func NewAuthUsecase(
	repo domain.AuthRepository,
	limiter domain.RateLimiter,
	custClient *client.CustomerClient,
	jwtSecret string,
	enableDemoOTP bool,
) *AuthUsecase {
	return &AuthUsecase{
		repo:           repo,
		rateLimiter:    limiter,
		customerClient: custClient,
		jwtSecret:      jwtSecret,
		enableDemoOTP:  enableDemoOTP,
	}
}

func NormalizePhone(phone string) string {
	cleaned := strings.TrimSpace(phone)
	cleaned = strings.TrimPrefix(cleaned, "+")

	if strings.HasPrefix(cleaned, "08") {
		return "62" + cleaned[1:]
	} else if strings.HasPrefix(cleaned, "8") {
		return "62" + cleaned
	}
	return cleaned
}

func generateOTPCode() string {
	maxVal := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, maxVal)
	if err != nil {
		return "123456"
	}
	return fmt.Sprintf("%06d", n.Int64())
}

func (u *AuthUsecase) RequestOTP(ctx context.Context, rawPhone string) (*RequestOTPResult, error) {
	phone := NormalizePhone(rawPhone)
	if len(phone) < 8 {
		return nil, errors.New("invalid phone number")
	}

	// 1. Check Rate Limit (max 5x/hr)
	allowed, retryAfter, err := u.rateLimiter.CheckAndRecordOTPRequest(ctx, phone)
	if err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}
	if !allowed {
		return nil, &RateLimitError{RetryAfter: retryAfter}
	}

	// 2. Generate OTP challenge (1 minute TTL)
	now := time.Now().UTC()
	expiresAt := now.Add(1 * time.Minute)
	otpCode := generateOTPCode()

	challenge := &domain.OTPChallenge{
		ID:           uuid.New().String(),
		Phone:        phone,
		OTPCode:      otpCode,
		Status:       "ACTIVE",
		AttemptCount: 0,
		ExpiresAt:    expiresAt,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := u.repo.CreateOTPChallenge(ctx, challenge); err != nil {
		return nil, fmt.Errorf("failed to save OTP challenge: %w", err)
	}

	// 3. Dispatch simulated SMS
	slog.Info("📱 [SMS Gateway Simulated]", "phone", phone, "otp", otpCode, "expiresAt", expiresAt)

	return &RequestOTPResult{
		ExpiresAt:       expiresAt,
		CooldownSeconds: 60,
		DebugOTP:        otpCode,
	}, nil
}

func (u *AuthUsecase) VerifyOTP(ctx context.Context, rawPhone, otp string) (*VerifyOTPResult, error) {
	phone := NormalizePhone(rawPhone)
	if len(phone) < 8 {
		return nil, errors.New("invalid phone number")
	}
	if strings.TrimSpace(otp) == "" {
		return nil, errors.New("OTP cannot be empty")
	}

	// 1. Find latest active challenge
	challenge, err := u.repo.GetLatestActiveChallenge(ctx, phone)
	if err != nil {
		if errors.Is(err, domain.ErrChallengeEmpty) {
			return nil, domain.ErrChallengeEmpty
		}
		return nil, err
	}

	now := time.Now().UTC()

	// 2. Check Expiration
	if now.After(challenge.ExpiresAt) {
		_ = u.repo.UpdateChallengeStatus(ctx, challenge.ID, "EXPIRED")
		return nil, domain.ErrOTPExpired
	}

	// 3. Check Lock Status
	if challenge.Status == "LOCKED" || challenge.AttemptCount >= 3 {
		return nil, domain.ErrOTPLocked
	}

	// 4. Validate OTP Code
	isValid := (otp == challenge.OTPCode) || (u.enableDemoOTP && otp == "123456")

	if !isValid {
		newAttempts, err := u.repo.IncrementChallengeAttempt(ctx, challenge.ID)
		if err != nil {
			return nil, err
		}

		if newAttempts >= 3 {
			_ = u.repo.UpdateChallengeStatus(ctx, challenge.ID, "LOCKED")
			return nil, domain.ErrOTPLocked
		}

		return nil, &InvalidOTPError{AttemptsRemaining: 3 - newAttempts}
	}

	// 5. Mark Challenge Verified
	if err := u.repo.UpdateChallengeStatus(ctx, challenge.ID, "VERIFIED"); err != nil {
		return nil, err
	}

	// 6. Sync Customer Data
	customer, err := u.customerClient.GetCustomer(ctx, phone)
	if err != nil {
		return nil, fmt.Errorf("sync customer error: %w", err)
	}

	if err := u.customerClient.VerifyCustomer(ctx, customer.CustomerID, now); err != nil {
		return nil, fmt.Errorf("verify customer error: %w", err)
	}

	// 7. Generate JWT & Session
	tokenPair, err := jwt.GenerateTokens(customer.CustomerID, phone, true, u.jwtSecret, 1*time.Hour)
	if err != nil {
		return nil, err
	}

	session := &domain.AuthSession{
		ID:           uuid.New().String(),
		CustomerID:   customer.CustomerID,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    now.Add(30 * 24 * time.Hour),
		CreatedAt:    now,
	}

	if err := u.repo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save auth session: %w", err)
	}

	return &VerifyOTPResult{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	}, nil
}
