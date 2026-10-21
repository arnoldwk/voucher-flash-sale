package http

import (
	"errors"
	"net/http"

	"voucher-flash-sale/pkg/response"
	"voucher-flash-sale/services/voucher-service/internal/domain"
	"voucher-flash-sale/services/voucher-service/internal/usecase"
)

type VoucherHandler struct {
	usecase *usecase.VoucherUsecase
}

func NewVoucherHandler(u *usecase.VoucherUsecase) *VoucherHandler {
	return &VoucherHandler{usecase: u}
}

func (h *VoucherHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.HealthCheck)
	mux.HandleFunc("GET /api/v1/users/{user_id}/vouchers", h.ListUserVouchers)
	mux.HandleFunc("POST /api/v1/vouchers/{code}/redeem", h.RedeemVoucher)
}

func (h *VoucherHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response.Success(w, http.StatusOK, "voucher-service healthy", nil)
}

func (h *VoucherHandler) ListUserVouchers(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	if userID == "" {
		response.Error(w, http.StatusBadRequest, "user_id is required")
		return
	}

	vouchers, err := h.usecase.ListUserVouchers(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(w, http.StatusOK, "user vouchers fetched", vouchers)
}

func (h *VoucherHandler) RedeemVoucher(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		response.Error(w, http.StatusBadRequest, "voucher code is required")
		return
	}

	voucher, err := h.usecase.RedeemVoucher(r.Context(), code)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrVoucherNotFound):
			response.Error(w, http.StatusNotFound, "voucher not found")
		case errors.Is(err, domain.ErrVoucherAlreadyRedeemed):
			response.Error(w, http.StatusConflict, "voucher has already been redeemed")
		default:
			response.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	response.Success(w, http.StatusOK, "voucher redeemed successfully", voucher)
}
