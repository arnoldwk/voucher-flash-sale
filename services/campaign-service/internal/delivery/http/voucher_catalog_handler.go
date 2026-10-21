package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"voucher-flash-sale/pkg/apiresponse"
	"voucher-flash-sale/services/campaign-service/internal/domain"
	"voucher-flash-sale/services/campaign-service/internal/usecase"
)

// VoucherCatalogHandler serves the public voucher product catalog.
type VoucherCatalogHandler struct {
	usecase *usecase.ListVouchersUsecase
}

func NewVoucherCatalogHandler(u *usecase.ListVouchersUsecase) *VoucherCatalogHandler {
	return &VoucherCatalogHandler{usecase: u}
}

// RegisterRoutes mendaftarkan route API publik produk voucher.
// Karena main.go menggunakan StripPrefix("/product/v1", ...),
// path yang didaftarkan di sini adalah relatif terhadap prefix tersebut.
func (h *VoucherCatalogHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /vouchers", h.ListVouchers)
}

// listVouchersRequest adalah payload request. Nomor telepon dapat dikirim
// lewat body JSON maupun query parameter (?phone=) karena method-nya GET.
type listVouchersRequest struct {
	Phone string `json:"phone"`
}

// ListVouchous menampilkan katalog voucher yang tersedia beserta status stok
// dan status kelayakan customer.
//
// GET /product/v1/vouchers
// Authorization: Bearer {JWT_TOKEN}
// Body: {"phone": "08123456789"}
func (h *VoucherCatalogHandler) ListVouchers(w http.ResponseWriter, r *http.Request) {
	req, err := parseListVouchersRequest(r)
	if err != nil {
		apiresponse.Error(w, http.StatusBadRequest, apiresponse.CodeInvalidRequest, "invalid request payload")
		return
	}

	if req.Phone == "" {
		apiresponse.Error(w, http.StatusBadRequest, apiresponse.CodeInvalidRequest, "phone is required")
		return
	}

	products, err := h.usecase.Execute(r.Context(), usecase.ListVouchersInput{Phone: req.Phone})
	if err != nil {
		if errors.Is(err, domain.ErrInvalidPhone) {
			apiresponse.Error(w, http.StatusBadRequest, apiresponse.CodeInvalidRequest, "invalid phone number")
			return
		}
		apiresponse.Error(w, http.StatusInternalServerError, apiresponse.CodeInternalError, "failed to retrieve catalog")
		return
	}

	apiresponse.Success(w, http.StatusOK, "Catalog retrieved", products)
}

// parseListVouchersRequest membaca nomor telepon dari body JSON, dengan
// fallback ke query parameter. Body kosong diperbolehkan agar GET dengan
// query string tetap bisa dipakai.
func parseListVouchersRequest(r *http.Request) (listVouchersRequest, error) {
	var req listVouchersRequest

	if r.Body != nil {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return req, err
		}
		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return req, err
			}
		}
	}

	if req.Phone == "" {
		req.Phone = r.URL.Query().Get("phone")
	}

	return req, nil
}
