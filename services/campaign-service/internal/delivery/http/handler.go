package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"voucher-flash-sale/pkg/response"
	"voucher-flash-sale/services/campaign-service/internal/domain"
	"voucher-flash-sale/services/campaign-service/internal/usecase"
)

type CampaignHandler struct {
	usecase *usecase.CampaignUsecase
}

func NewCampaignHandler(u *usecase.CampaignUsecase) *CampaignHandler {
	return &CampaignHandler{usecase: u}
}

func (h *CampaignHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.HealthCheck)
	mux.HandleFunc("POST /api/v1/campaigns", h.CreateCampaign)
	mux.HandleFunc("GET /api/v1/campaigns/{id}", h.GetCampaign)
	mux.HandleFunc("GET /api/v1/campaigns", h.ListCampaigns)
}

func (h *CampaignHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response.Success(w, http.StatusOK, "campaign-service healthy", nil)
}

func (h *CampaignHandler) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	var input usecase.CreateCampaignInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	campaign, err := h.usecase.Create(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidCampaignDates), errors.Is(err, domain.ErrInvalidCampaignStock):
			response.Error(w, http.StatusBadRequest, err.Error())
		default:
			response.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	response.Success(w, http.StatusCreated, "campaign created successfully", campaign)
}

func (h *CampaignHandler) GetCampaign(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, "campaign id is required")
		return
	}

	campaign, err := h.usecase.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrCampaignNotFound) {
			response.Error(w, http.StatusNotFound, "campaign not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(w, http.StatusOK, "campaign fetched successfully", campaign)
}

func (h *CampaignHandler) ListCampaigns(w http.ResponseWriter, r *http.Request) {
	campaigns, err := h.usecase.List(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(w, http.StatusOK, "campaigns fetched successfully", campaigns)
}
