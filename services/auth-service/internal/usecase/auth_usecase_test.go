package usecase_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"voucher-flash-sale/pkg/jwt"
	"voucher-flash-sale/services/auth-service/internal/client"
	"voucher-flash-sale/services/auth-service/internal/domain"
	"voucher-flash-sale/services/auth-service/internal/usecase"
)

type mockAuthRepo struct {
	challenges map[string]*domain.OTPChallenge // by phone
	sessions   map[string]*domain.AuthSession
}

func newMockAuthRepo() *mockAuthRepo {
	return &mockAuthRepo{
		challenges: make(map[string]*domain.OTPChallenge),
		sessions:   make(map[string]*domain.AuthSession),
	}
}

func (m *mockAuthRepo) CreateOTPChallenge(ctx context.Context, c *domain.OTPChallenge) error {
	m.challenges[c.Phone] = c
	return nil
}

func (m *mockAuthRepo) GetLatestActiveChallenge(ctx context.Context, phone string) (*domain.OTPChallenge, error) {
	c, ok := m.challenges[phone]
	if !ok || c.Status != "ACTIVE" {
		return nil, domain.ErrChallengeEmpty
	}
	return c, nil
}

func (m *mockAuthRepo) UpdateChallengeStatus(ctx context.Context, id, status string) error {
	for _, c := range m.challenges {
		if c.ID == id {
			c.Status = status
			return nil
		}
	}
	return nil
}

func (m *mockAuthRepo) IncrementChallengeAttempt(ctx context.Context, id string) (int, error) {
	for _, c := range m.challenges {
		if c.ID == id {
			c.AttemptCount++
			return c.AttemptCount, nil
		}
	}
	return 0, domain.ErrChallengeEmpty
}

func (m *mockAuthRepo) CreateSession(ctx context.Context, s *domain.AuthSession) error {
	m.sessions[s.RefreshToken] = s
	return nil
}

type mockRateLimiter struct {
	allowed    bool
	retryAfter int
}

func (m *mockRateLimiter) CheckAndRecordOTPRequest(ctx context.Context, phone string) (bool, int, error) {
	return m.allowed, m.retryAfter, nil
}

func TestAuthUsecase(t *testing.T) {
	ctx := context.Background()
	jwtSecret := "test-secret-key-for-unit-testing-32bytes"

	// Mock customer-service HTTP server
	mockCustomerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/customer/private/customers" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"status": true,
				"code": "SUCCESS",
				"message": "Customer retrieved",
				"data": {
					"customerId": "c_01TEST12345",
					"phone": "628123456789",
					"isVerified": false
				}
			}`))
			return
		}
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"status": true,
				"code": "SUCCESS",
				"message": "Customer verified successfully",
				"data": {
					"customerId": "c_01TEST12345",
					"phoneVerifiedAt": "2026-10-21T12:00:30Z"
				}
			}`))
			return
		}
	}))
	defer mockCustomerServer.Close()

	custClient := client.NewCustomerClient(mockCustomerServer.URL, "test-secret")

	t.Run("rate limit rejection", func(t *testing.T) {
		repo := newMockAuthRepo()
		limiter := &mockRateLimiter{allowed: false, retryAfter: 3600}
		uc := usecase.NewAuthUsecase(repo, limiter, custClient, jwtSecret, true)

		_, err := uc.RequestOTP(ctx, "08123456789")
		var rErr *usecase.RateLimitError
		if !errors.As(err, &rErr) {
			t.Fatalf("expected rate limit error, got %v", err)
		}
	})

	t.Run("request and verify OTP success with demo OTP", func(t *testing.T) {
		repo := newMockAuthRepo()
		limiter := &mockRateLimiter{allowed: true}
		uc := usecase.NewAuthUsecase(repo, limiter, custClient, jwtSecret, true)

		res, err := uc.RequestOTP(ctx, "08123456789")
		if err != nil {
			t.Fatalf("unexpected request error: %v", err)
		}
		if res.CooldownSeconds != 60 {
			t.Fatalf("expected cooldown 60, got %d", res.CooldownSeconds)
		}

		// Verify with demo OTP "123456"
		verifyRes, err := uc.VerifyOTP(ctx, "08123456789", "123456")
		if err != nil {
			t.Fatalf("unexpected verify error: %v", err)
		}
		if verifyRes.AccessToken == "" || verifyRes.RefreshToken == "" {
			t.Fatal("expected non-empty tokens")
		}

		claims, err := jwt.ValidateToken(verifyRes.AccessToken, jwtSecret)
		if err != nil {
			t.Fatalf("failed to validate token: %v", err)
		}
		if claims.CustomerID != "c_01TEST12345" {
			t.Fatalf("expected customerId c_01TEST12345, got %s", claims.CustomerID)
		}
	})

	t.Run("retry counter on invalid OTP and lock", func(t *testing.T) {
		repo := newMockAuthRepo()
		limiter := &mockRateLimiter{allowed: true}
		uc := usecase.NewAuthUsecase(repo, limiter, custClient, jwtSecret, false) // demo OTP disabled

		_, err := uc.RequestOTP(ctx, "08123456789")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Attempt 1: invalid
		_, err = uc.VerifyOTP(ctx, "08123456789", "000000")
		var invErr *usecase.InvalidOTPError
		if err == nil {
			t.Fatal("expected invalid OTP error")
		}

		// Attempt 2: invalid
		_, err = uc.VerifyOTP(ctx, "08123456789", "000000")
		if err == nil {
			t.Fatal("expected invalid OTP error")
		}

		// Attempt 3: invalid -> should lock
		_, err = uc.VerifyOTP(ctx, "08123456789", "000000")
		if err != domain.ErrOTPLocked {
			t.Fatalf("expected ErrOTPLocked on 3rd attempt, got %v", err)
		}
		_ = invErr
	})

	t.Run("expired OTP", func(t *testing.T) {
		repo := newMockAuthRepo()
		limiter := &mockRateLimiter{allowed: true}
		uc := usecase.NewAuthUsecase(repo, limiter, custClient, jwtSecret, true)

		res, _ := uc.RequestOTP(ctx, "08123456789")
		// Manually expire challenge in repo
		repo.challenges["628123456789"].ExpiresAt = time.Now().UTC().Add(-10 * time.Second)

		_, err := uc.VerifyOTP(ctx, "08123456789", res.DebugOTP)
		if err != domain.ErrOTPExpired {
			t.Fatalf("expected ErrOTPExpired, got %v", err)
		}
	})
}
