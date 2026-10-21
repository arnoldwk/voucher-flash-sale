package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"voucher-flash-sale/pkg/apiresponse"
	pkgjwt "voucher-flash-sale/pkg/jwt"
	pkgmiddleware "voucher-flash-sale/pkg/middleware"
	deliveryhttp "voucher-flash-sale/services/transaction-service/internal/delivery/http"
	"voucher-flash-sale/services/transaction-service/internal/domain"
	"voucher-flash-sale/services/transaction-service/internal/usecase"
)

type stubOrderRepo struct {
	product   *domain.ProductRead
	idem      map[string]*domain.Order
	insertErr error
}

func (s *stubOrderRepo) FindVoucherProduct(ctx context.Context, voucherProductID string) (*domain.ProductRead, error) {
	return s.product, nil
}

func (s *stubOrderRepo) FindByIdempotencyKey(ctx context.Context, key string) (*domain.Order, error) {
	return s.idem[key], nil
}

func (s *stubOrderRepo) InsertOrderAndEntitlement(ctx context.Context, order *domain.Order, quotaID string, entitlement *domain.CustomerEntitlement) error {
	return s.insertErr
}

func (s *stubOrderRepo) ExpirePendingOrders(ctx context.Context) ([]string, error) {
	return nil, nil
}

type stubStockCache struct {
	reserveCode int
}

func (s *stubStockCache) Decrement(ctx context.Context, campaignID string) (int, error) {
	return 0, nil
}
func (s *stubStockCache) Increment(ctx context.Context, campaignID string) (int, error) {
	return 0, nil
}
func (s *stubStockCache) Get(ctx context.Context, campaignID string) (int, error) { return 10, nil }
func (s *stubStockCache) SetReservationMarker(ctx context.Context, key, value string, ttl time.Duration) error {
	return nil
}
func (s *stubStockCache) DelReservationMarker(ctx context.Context, key string) error { return nil }
func (s *stubStockCache) SetEntitlementMarker(ctx context.Context, key, value string, ttl time.Duration) error {
	return nil
}
func (s *stubStockCache) DelEntitlementMarker(ctx context.Context, key string) error { return nil }
func (s *stubStockCache) Reserve(ctx context.Context, stockKey, entKey, resKey, orderID, orderJSON string, ttl time.Duration) (int, error) {
	return s.reserveCode, nil
}
func (s *stubStockCache) ReleaseReservation(ctx context.Context, stockKey, resKey, entKey string) (int, error) {
	return 0, nil
}

func newHandler(product *domain.ProductRead, reserveCode int) *deliveryhttp.OrderHandler {
	repo := &stubOrderRepo{product: product, idem: map[string]*domain.Order{}}
	uc := usecase.NewReserveOrderUsecase(repo, &stubStockCache{reserveCode: reserveCode})
	return deliveryhttp.NewOrderHandler(uc)
}

func newServer(handler *deliveryhttp.OrderHandler) *http.ServeMux {
	inner := http.NewServeMux()
	handler.RegisterRoutes(inner)
	mux := http.NewServeMux()
	mux.Handle("/v1/", http.StripPrefix("/v1", pkgmiddleware.AuthBearer(inner)))
	return mux
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) apiresponse.Envelope {
	t.Helper()
	var env apiresponse.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode envelope: %v (body=%s)", err, rec.Body.String())
	}
	return env
}

func makeToken() string {
	token, err := pkgjwt.GenerateTokens("cust-demo", "08123456789", true, os.Getenv("JWT_SECRET"), time.Hour)
	if err != nil {
		return ""
	}
	return token.AccessToken
}

func TestCreateOrder_Success(t *testing.T) {
	os.Setenv("JWT_SECRET", "s3cr3t")
	defer os.Unsetenv("JWT_SECRET")

	handler := newHandler(&domain.ProductRead{
		ID:             "vp_fnb",
		CampaignID:     "camp-1",
		SessionID:      "evt-nov-w1",
		CategoryName:   "F&B",
		PriceIDR:       10000,
		RemainingStock: 5,
		SessionStatus:  "ACTIVE",
		QuotaID:        "q-1",
	}, 1)
	srv := newServer(handler)

	body := `{"voucherProductId":"vp_fnb","phone_number":"08123456789"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/orders", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+makeToken())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if !env.Status || env.Code != apiresponse.CodeCreated || env.ErrorMessage != nil || env.ErrorData != nil {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %T", env.Data)
	}
	if data["status"] != "PENDING" {
		t.Errorf("expected status PENDING, got %v", data["status"])
	}
	price, _ := data["price"].(map[string]any)
	if price["amount"] != float64(1000000) || price["currency"] != "IDR" {
		t.Errorf("expected price 1000000 IDR, got %v", price)
	}
	if _, ok := data["orderId"].(string); !ok || data["orderId"] == "" {
		t.Errorf("expected non-empty orderId, got %v", data["orderId"])
	}
	if _, ok := data["reservationDeadline"].(string); !ok {
		t.Errorf("expected reservationDeadline string, got %v", data["reservationDeadline"])
	}
}

func TestCreateOrder_StockUnavailable(t *testing.T) {
	os.Setenv("JWT_SECRET", "s3cr3t")
	defer os.Unsetenv("JWT_SECRET")

	handler := newHandler(&domain.ProductRead{
		ID:           "vp_fnb",
		CampaignID:   "camp-1",
		SessionID:    "evt-nov-w1",
		CategoryName: "F&B",
		PriceIDR:     10000,
		QuotaID:      "q-1",
	}, -2)
	srv := newServer(handler)

	body := `{"voucherProductId":"vp_fnb","phone_number":"08123456789"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/orders", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+makeToken())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Code != apiresponse.CodeStockUnavailable {
		t.Errorf("expected STOCK_UNAVAILABLE, got %s", env.Code)
	}
	if env.ErrorMessage == nil || *env.ErrorMessage != "Voucher stock is empty for this session." {
		t.Errorf("unexpected errorMessage: %q", *env.ErrorMessage)
	}
	data, ok := env.ErrorData.(map[string]any)
	if !ok || data["category"] != "FOOD_BEVERAGE" {
		t.Errorf("expected errorData.category FOOD_BEVERAGE, got %v", env.ErrorData)
	}
}

func TestCreateOrder_RequiresAuth(t *testing.T) {
	handler := newHandler(&domain.ProductRead{ID: "vp_fnb", PriceIDR: 10000}, 1)
	srv := newServer(handler)

	req := httptest.NewRequest(http.MethodPost, "/v1/orders", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
