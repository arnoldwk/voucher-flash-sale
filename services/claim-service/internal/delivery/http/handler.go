package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"voucher-flash-sale/pkg/response"
	"voucher-flash-sale/services/claim-service/internal/domain"
	"voucher-flash-sale/services/claim-service/internal/usecase"
)

type ClaimHandler struct {
	usecase *usecase.ClaimUsecase
}

func NewClaimHandler(u *usecase.ClaimUsecase) *ClaimHandler {
	return &ClaimHandler{usecase: u}
}

func (h *ClaimHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.HealthCheck)
	mux.HandleFunc("POST /api/v1/campaigns/{id}/claim", h.ClaimVoucher)
	mux.HandleFunc("GET /api/v1/campaigns/{id}/stock", h.GetStock)
}

func (h *ClaimHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response.Success(w, http.StatusOK, "claim-service healthy", nil)
}

func (h *ClaimHandler) ClaimVoucher(w http.ResponseWriter, r *http.Request) {
	campaignID := r.PathValue("id")
	if campaignID == "" {
		response.Error(w, http.StatusBadRequest, "campaign id is required in URL path")
		return
	}

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body, user_id required")
		return
	}

	if req.UserID == "" {
		response.Error(w, http.StatusBadRequest, "user_id cannot be empty")
		return
	}

	result, err := h.usecase.Claim(r.Context(), domain.ClaimRequest{
		CampaignID: campaignID,
		UserID:     req.UserID,
	})

	if err != nil {
		switch {
		case errors.Is(err, domain.ErrAlreadyClaimed):
			response.Error(w, http.StatusConflict, err.Error())
		case errors.Is(err, domain.ErrOutOfStock):
			response.Error(w, http.StatusGone, err.Error())
		case errors.Is(err, domain.ErrCampaignNotActive):
			response.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, domain.ErrRateLimited):
			response.Error(w, http.StatusTooManyRequests, err.Error())
		default:
			response.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	response.Success(w, http.StatusOK, "voucher claimed successfully", result)
}

func (h *ClaimHandler) GetStock(w http.ResponseWriter, r *http.Request) {
	campaignID := r.PathValue("id")
	if campaignID == "" {
		response.Error(w, http.StatusBadRequest, "campaign id is required")
		return
	}

	stock, err := h.usecase.GetRemainingStock(r.Context(), campaignID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "campaign stock not found")
		return
	}

	response.Success(w, http.StatusOK, "remaining stock fetched", map[string]any{
		"campaign_id":     campaignID,
		"remaining_stock": stock,
	})
}
