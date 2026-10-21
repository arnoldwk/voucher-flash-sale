package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"voucher-flash-sale/pkg/contract"
	"voucher-flash-sale/services/customer-service/internal/domain"
	"voucher-flash-sale/services/customer-service/internal/usecase"
)

type CustomerHandler struct {
	usecase *usecase.CustomerUsecase
	apiKey  string
}

func NewCustomerHandler(u *usecase.CustomerUsecase, apiKey string) *CustomerHandler {
	return &CustomerHandler{
		usecase: u,
		apiKey:  apiKey,
	}
}

func (h *CustomerHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.HealthCheck)

	// Contract endpoints from login.md
	mux.HandleFunc("GET /customer/private/customers", RequireAPIKey(h.apiKey, h.GetCustomer))
	mux.HandleFunc("POST /customer/private/customers", RequireAPIKey(h.apiKey, h.GetCustomer))
	mux.HandleFunc("POST /customer/v1/internal/{customerId}/verify", RequireAPIKey(h.apiKey, h.VerifyCustomer))

	// Sequence diagram alias endpoints
	mux.HandleFunc("GET /v1/internal/customers", RequireAPIKey(h.apiKey, h.GetCustomer))
	mux.HandleFunc("PATCH /v1/internal/customers/{customerId}/verify", RequireAPIKey(h.apiKey, h.VerifyCustomer))
}

func (h *CustomerHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	contract.WriteSuccess(w, http.StatusOK, "SUCCESS", "customer-service healthy", nil)
}

func (h *CustomerHandler) GetCustomer(w http.ResponseWriter, r *http.Request) {
	phone := r.URL.Query().Get("phone")
	if phone == "" && r.Method == http.MethodPost {
		var req struct {
			Phone string `json:"phone"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		phone = req.Phone
	}

	if phone == "" {
		contract.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "Validation failed", "phone parameter is required", nil, nil)
		return
	}

	customer, err := h.usecase.GetOrCreateCustomer(r.Context(), phone)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidPhone) {
			contract.WriteError(w, http.StatusBadRequest, "INVALID_PHONE", "Validation failed", "invalid phone format", nil, nil)
			return
		}
		contract.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Server error", err.Error(), nil, nil)
		return
	}

	data := map[string]any{
		"customerId": customer.ID,
		"phone":      customer.Phone,
		"isVerified": customer.IsVerified,
	}

	contract.WriteSuccess(w, http.StatusOK, "SUCCESS", "Customer retrieved", data)
}

func (h *CustomerHandler) VerifyCustomer(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("customerId")
	if customerID == "" {
		contract.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "Validation failed", "customerId is required", nil, nil)
		return
	}

	var req struct {
		VerifiedAt string `json:"verifiedAt"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	verifiedAt := time.Now().UTC()
	if req.VerifiedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, req.VerifiedAt); err == nil {
			verifiedAt = parsed
		}
	}

	customer, err := h.usecase.VerifyCustomer(r.Context(), customerID, verifiedAt)
	if err != nil {
		if errors.Is(err, domain.ErrCustomerNotFound) {
			contract.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Customer not found", err.Error(), nil, nil)
			return
		}
		contract.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Server error", err.Error(), nil, nil)
		return
	}

	data := map[string]any{
		"customerId":      customer.ID,
		"phoneVerifiedAt": customer.PhoneVerifiedAt.Format(time.RFC3339),
	}

	contract.WriteSuccess(w, http.StatusOK, "SUCCESS", "Customer verified successfully", data)
}
