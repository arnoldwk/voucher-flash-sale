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
	deliveryhttp "voucher-flash-sale/services/campaign-service/internal/delivery/http"
	"voucher-flash-sale/services/campaign-service/internal/domain"
	"voucher-flash-sale/services/campaign-service/internal/usecase"
)

type stubCatalogRepo struct {
	items   []*domain.CatalogItem
	claimed map[string]map[string]bool
}

func (s *stubCatalogRepo) ListVoucherProducts(ctx context.Context, now time.Time) ([]*domain.CatalogItem, error) {
	return s.items, nil
}

func (s *stubCatalogRepo) ClaimedCategoriesByEvent(ctx context.Context, customerID string) (map[string]map[string]bool, error) {
	return s.claimed, nil
}

type stubStockCache struct{}

func (s *stubStockCache) Warmup(ctx context.Context, campaignID string, stock int, ttl time.Duration) error {
	return nil
}
func (s *stubStockCache) Stock(ctx context.Context, campaignID string) (int, error) { return -1, nil }
func (s *stubStockCache) PublishMeta(ctx context.Context, campaignID string, meta domain.CampaignMeta) error {
	return nil
}
func (s *stubStockCache) Stocks(ctx context.Context, campaignIDs []string) (map[string]int, error) {
	return nil, nil
}

// newTestServer menyusun router yang sama dengan main.go:
// /product/v1/ dilindungi JWT middleware.
func newTestServer(t *testing.T, repo domain.ProductCatalogRepository) *http.ServeMux {
	t.Helper()

	uc := usecase.NewListVouchersUsecase(repo, &stubStockCache{})
	handler := deliveryhttp.NewVoucherCatalogHandler(uc)

	inner := http.NewServeMux()
	handler.RegisterRoutes(inner)

	mux := http.NewServeMux()
	mux.Handle("/product/v1/", http.StripPrefix("/product/v1", pkgmiddleware.AuthBearer(inner)))
	return mux
}

func sampleItems() []*domain.CatalogItem {
	now := time.Now().UTC()
	return []*domain.CatalogItem{
		{
			VoucherProductID:  "vp-1",
			CampaignID:        "camp-1",
			EventID:           "event-1",
			CategoryID:        "cat-1",
			CategoryName:      "F&B",
			Name:              "Diskon 25K",
			DiscountValueIDR:  25000,
			PriceIDR:          5000,
			MinTransactionIDR: 50000,
			RemainingStock:    10,
			SessionStart:      now.Add(-time.Hour),
			SessionEnd:        now.Add(2 * time.Hour),
			SessionStatus:     "ACTIVE",
		},
	}
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) apiresponse.Envelope {
	t.Helper()
	var env apiresponse.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode envelope: %v (body=%s)", err, rec.Body.String())
	}
	return env
}

func TestListVouchers_RequiresAuth(t *testing.T) {
	srv := newTestServer(t, &stubCatalogRepo{items: sampleItems()})

	req := httptest.NewRequest(http.MethodGet, "/product/v1/vouchers", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Status {
		t.Error("expected status=false")
	}
	if env.Code != apiresponse.CodeUnauthorized {
		t.Errorf("expected code %s, got %s", apiresponse.CodeUnauthorized, env.Code)
	}
}

func TestListVouchers_Success(t *testing.T) {
	const secret = "test-secret"
	os.Setenv("JWT_SECRET", secret)
	defer os.Unsetenv("JWT_SECRET")

	tokenPair, err := pkgjwt.GenerateTokens("cust-1", "08123456789", true, secret, time.Hour)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	srv := newTestServer(t, &stubCatalogRepo{items: sampleItems(), claimed: map[string]map[string]bool{}})

	req := httptest.NewRequest(http.MethodPost, "/product/v1/vouchers", strings.NewReader(`{"phone":"08123456789"}`))
	req.Method = http.MethodGet
	req.Header.Set("Authorization", "Bearer "+tokenPair.AccessToken)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rec.Code, rec.Body.String())
	}

	env := decodeEnvelope(t, rec)
	if !env.Status {
		t.Error("expected status=true")
	}
	if env.Code != apiresponse.CodeSuccess {
		t.Errorf("expected code %s, got %s", apiresponse.CodeSuccess, env.Code)
	}
	if env.ErrorMessage != nil {
		t.Errorf("expected errorMessage=null, got %q", *env.ErrorMessage)
	}
	if env.ErrorData != nil {
		t.Errorf("expected errorData=null, got %v", env.ErrorData)
	}

	raw, ok := env.Data.([]any)
	if !ok {
		t.Fatalf("expected data to be an array, got %T", env.Data)
	}
	if len(raw) != 1 {
		t.Fatalf("expected 1 product, got %d", len(raw))
	}
	product, ok := raw[0].(map[string]any)
	if !ok {
		t.Fatalf("expected product object, got %T", raw[0])
	}
	if product["category"] != "FOOD_BEVERAGE" {
		t.Errorf("expected category FOOD_BEVERAGE, got %v", product["category"])
	}
	if product["stockStatus"] != "AVAILABLE" {
		t.Errorf("expected stockStatus AVAILABLE, got %v", product["stockStatus"])
	}
	if product["customerState"] != "ELIGIBLE" {
		t.Errorf("expected customerState ELIGIBLE, got %v", product["customerState"])
	}
}

func TestListVouchers_InvalidPhone(t *testing.T) {
	const secret = "test-secret"
	os.Setenv("JWT_SECRET", secret)
	defer os.Unsetenv("JWT_SECRET")

	tokenPair, err := pkgjwt.GenerateTokens("cust-1", "08123456789", true, secret, time.Hour)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	srv := newTestServer(t, &stubCatalogRepo{items: sampleItems()})

	req := httptest.NewRequest(http.MethodGet, "/product/v1/vouchers?phone=123", nil)
	req.Header.Set("Authorization", "Bearer "+tokenPair.AccessToken)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%s)", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Status {
		t.Error("expected status=false")
	}
	if env.Code != apiresponse.CodeInvalidRequest {
		t.Errorf("expected code %s, got %s", apiresponse.CodeInvalidRequest, env.Code)
	}
}

func TestListVouchers_MissingPhone(t *testing.T) {
	const secret = "test-secret"
	os.Setenv("JWT_SECRET", secret)
	defer os.Unsetenv("JWT_SECRET")

	tokenPair, err := pkgjwt.GenerateTokens("cust-1", "08123456789", true, secret, time.Hour)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	srv := newTestServer(t, &stubCatalogRepo{items: sampleItems()})

	req := httptest.NewRequest(http.MethodGet, "/product/v1/vouchers", nil)
	req.Header.Set("Authorization", "Bearer "+tokenPair.AccessToken)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%s)", rec.Code, rec.Body.String())
	}
}
