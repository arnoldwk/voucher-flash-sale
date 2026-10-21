package http

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"voucher-flash-sale/pkg/apiresponse"
	"voucher-flash-sale/services/transaction-service/internal/domain"
	"voucher-flash-sale/services/transaction-service/internal/usecase"
)

type OrderHandler struct {
	usecase *usecase.ReserveOrderUsecase
	logger  *slog.Logger
}

func NewOrderHandler(u *usecase.ReserveOrderUsecase) *OrderHandler {
	return &OrderHandler{
		usecase: u,
		logger:  slog.Default(),
	}
}

func (h *OrderHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.HealthCheck)
	mux.HandleFunc("POST /orders", h.CreateOrder)
}

func (h *OrderHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	apiresponse.Success(w, http.StatusOK, "transaction-service healthy", nil)
}

// createOrderRequest matches the public contract:
// POST /v1/orders { "voucherProductId": "vp_fnb", "phone_number": "6283..." }
type createOrderRequest struct {
	VoucherProductID string `json:"voucherProductId"`
	PhoneNumber      string `json:"phone_number"`
}

func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	req, err := parseCreateOrderRequest(r)
	if err != nil {
		apiresponse.Error(w, http.StatusBadRequest, apiresponse.CodeInvalidRequest, "invalid request payload")
		return
	}

	res, err := h.usecase.ReserveOrder(r.Context(), domain.ReserveOrderInput{
		VoucherProductID: req.VoucherProductID,
		PhoneNumber:      req.PhoneNumber,
		IdempotencyKey:   r.Header.Get("Idempotency-Key"),
	})
	category := resCategory(err)
	switch {
	case err == nil:
		apiresponse.SuccessWithCode(w, http.StatusCreated, apiresponse.CodeCreated, "Order reserved successfully", res)
		return
	case errors.Is(err, domain.ErrOutOfStock):
		apiresponse.ErrorWithData(w, http.StatusConflict, apiresponse.CodeStockUnavailable, "Checkout failed", "Voucher stock is empty for this session.", map[string]any{"category": category})
		return
	case errors.Is(err, domain.ErrAlreadyEntitled):
		apiresponse.ErrorWithData(w, http.StatusConflict, apiresponse.CodeAlreadyEntitled, "Checkout failed", "Voucher stock is empty for this session.", map[string]any{"category": category})
		return
	case errors.Is(err, domain.ErrVoucherProductMissing):
		apiresponse.Error(w, http.StatusNotFound, apiresponse.CodeNotFound, "voucher product not found or not in an active session")
		return
	case errors.Is(err, domain.ErrInvalidPhone):
		apiresponse.Error(w, http.StatusBadRequest, apiresponse.CodeInvalidRequest, "invalid phone number")
		return
	case errors.Is(err, domain.ErrIdempotencyConflict):
		apiresponse.Error(w, http.StatusConflict, apiresponse.CodeIdempotencyConflict, "idempotency key reused with a different payload")
		return
	default:
		h.logger.Error("reserve order failed", "error", err)
		apiresponse.Error(w, http.StatusInternalServerError, apiresponse.CodeInternalError, "failed to reserve order")
		return
	}
}

func parseCreateOrderRequest(r *http.Request) (createOrderRequest, error) {
	var req createOrderRequest
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return req, err
	}
	if len(body) == 0 {
		return req, errors.New("empty body")
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return req, err
	}
	return req, nil
}

// resCategory attempts to read the category from a structured error so the
// handler can populate errorData. Returns "" when unavailable.
func resCategory(err error) string {
	var se *usecase.StockError
	if errors.As(err, &se) {
		return se.Category
	}
	var ae *usecase.AlreadyEntitledError
	if errors.As(err, &ae) {
		return ae.Category
	}
	return ""
}
