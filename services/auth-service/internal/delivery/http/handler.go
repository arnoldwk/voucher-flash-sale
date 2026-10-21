package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"voucher-flash-sale/pkg/contract"
	"voucher-flash-sale/services/auth-service/internal/domain"
	"voucher-flash-sale/services/auth-service/internal/usecase"
)

type AuthHandler struct {
	usecase *usecase.AuthUsecase
}

func NewAuthHandler(u *usecase.AuthUsecase) *AuthHandler {
	return &AuthHandler{usecase: u}
}

func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.HealthCheck)

	// login.md endpoints
	mux.HandleFunc("POST /auth/v1/otp/request", h.RequestOTP)
	mux.HandleFunc("POST /auth/v1/otp/verify", h.VerifyOTP)

	// sequence diagram alias endpoints
	mux.HandleFunc("POST /v1/auth/otp/request", h.RequestOTP)
	mux.HandleFunc("POST /v1/auth/otp/verify", h.VerifyOTP)
}

func (h *AuthHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	contract.WriteSuccess(w, http.StatusOK, "SUCCESS", "auth-service healthy", nil)
}

func (h *AuthHandler) RequestOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		contract.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request payload", "phone is required", nil, nil)
		return
	}

	result, err := h.usecase.RequestOTP(r.Context(), req.Phone)
	if err != nil {
		var rateLimitErr *usecase.RateLimitError
		if errors.As(err, &rateLimitErr) {
			contract.WriteError(
				w,
				http.StatusTooManyRequests,
				"RATE_LIMITED",
				"Too many requests",
				"You have reached the maximum OTP request limit for this hour.",
				map[string]any{"retryAfter": rateLimitErr.RetryAfter},
				nil,
			)
			return
		}

		contract.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "Failed to send OTP", err.Error(), nil, nil)
		return
	}

	// Add debug header in dev/test mode for easy testing
	if result.DebugOTP != "" {
		w.Header().Set("X-Debug-OTP", result.DebugOTP)
	}

	data := map[string]any{
		"expiresAt":       result.ExpiresAt.Format(time.RFC3339),
		"cooldownSeconds": result.CooldownSeconds,
	}

	contract.WriteSuccess(w, http.StatusOK, "SUCCESS", "OTP sent successfully", data)
}

func (h *AuthHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
		OTP   string `json:"otp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		contract.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request payload", "phone and otp are required", nil, map[string]any{})
		return
	}

	result, err := h.usecase.VerifyOTP(r.Context(), req.Phone, req.OTP)
	if err != nil {
		var invalidErr *usecase.InvalidOTPError
		switch {
		case errors.As(err, &invalidErr):
			contract.WriteError(
				w,
				http.StatusBadRequest,
				"INVALID_OTP",
				"Verification failed",
				fmt.Sprintf("The OTP you entered is incorrect. %d attempts remaining.", invalidErr.AttemptsRemaining),
				map[string]any{"attemptsRemaining": invalidErr.AttemptsRemaining},
				map[string]any{},
			)
		case errors.Is(err, domain.ErrOTPLocked):
			contract.WriteError(
				w,
				http.StatusBadRequest,
				"LOCKED",
				"Verification failed",
				"Max retry reached. Please request a new OTP.",
				map[string]any{"attemptsRemaining": 0},
				map[string]any{},
			)
		case errors.Is(err, domain.ErrOTPExpired):
			contract.WriteError(
				w,
				http.StatusBadRequest,
				"OTP_EXPIRED",
				"Verification failed",
				"The OTP has expired. Please request a new OTP.",
				nil,
				map[string]any{},
			)
		case errors.Is(err, domain.ErrChallengeEmpty):
			contract.WriteError(
				w,
				http.StatusBadRequest,
				"INVALID_OTP",
				"Verification failed",
				"No active OTP challenge found. Please request a new OTP.",
				map[string]any{"attemptsRemaining": 0},
				map[string]any{},
			)
		default:
			contract.WriteError(
				w,
				http.StatusInternalServerError,
				"INTERNAL_ERROR",
				"Login failed",
				err.Error(),
				nil,
				map[string]any{},
			)
		}
		return
	}

	data := map[string]any{
		"accessToken":  result.AccessToken,
		"refreshToken": result.RefreshToken,
		"expiresIn":    result.ExpiresIn,
	}

	contract.WriteSuccess(w, http.StatusOK, "SUCCESS", "Login successful", data)
}
