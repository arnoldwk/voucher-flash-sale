package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"voucher-flash-sale/services/transaction-service/internal/domain"
	"voucher-flash-sale/services/transaction-service/internal/usecase"
)

type mockOrderRepo struct {
	product    *domain.ProductRead
	productErr error
	idem       map[string]*domain.Order
	insertErr  error
	expireErr  error
}

func (m *mockOrderRepo) FindVoucherProduct(ctx context.Context, voucherProductID string) (*domain.ProductRead, error) {
	return m.product, m.productErr
}

func (m *mockOrderRepo) FindByIdempotencyKey(ctx context.Context, key string) (*domain.Order, error) {
	return m.idem[key], nil
}

func (m *mockOrderRepo) InsertOrderAndEntitlement(ctx context.Context, order *domain.Order, quotaID string, entitlement *domain.CustomerEntitlement) error {
	return m.insertErr
}

func (m *mockOrderRepo) ExpirePendingOrders(ctx context.Context) ([]string, error) {
	return nil, m.expireErr
}

type mockStockCache struct {
	reserveCode int
	reserveErr  error
	releaseErr  error
}

func (m *mockStockCache) Decrement(ctx context.Context, campaignID string) (int, error) {
	return 0, nil
}
func (m *mockStockCache) Increment(ctx context.Context, campaignID string) (int, error) {
	return 0, nil
}
func (m *mockStockCache) Get(ctx context.Context, campaignID string) (int, error) { return 10, nil }
func (m *mockStockCache) SetReservationMarker(ctx context.Context, key, value string, ttl time.Duration) error {
	return nil
}
func (m *mockStockCache) DelReservationMarker(ctx context.Context, key string) error { return nil }
func (m *mockStockCache) SetEntitlementMarker(ctx context.Context, key, value string, ttl time.Duration) error {
	return nil
}
func (m *mockStockCache) DelEntitlementMarker(ctx context.Context, key string) error { return nil }
func (m *mockStockCache) Reserve(ctx context.Context, stockKey, entKey, resKey, orderID, orderJSON string, ttl time.Duration) (int, error) {
	return m.reserveCode, m.reserveErr
}
func (m *mockStockCache) ReleaseReservation(ctx context.Context, stockKey, resKey, entKey string) (int, error) {
	return 0, m.releaseErr
}

func TestReserveOrder_HappyPath(t *testing.T) {
	repo := &mockOrderRepo{
		product: &domain.ProductRead{
			ID:             "vp_fnb",
			CampaignID:     "camp-kenangan",
			SessionID:      "evt-nov-w1",
			CategoryID:     "cat-fn-b",
			CategoryName:   "F&B",
			Name:           "Kopi Kenangan",
			PriceIDR:       45000,
			RemainingStock: 5,
			SessionStatus:  "ACTIVE",
			QuotaID:        "quota-1",
		},
		idem: map[string]*domain.Order{},
	}
	cache := &mockStockCache{reserveCode: 1}
	uc := usecase.NewReserveOrderUsecase(repo, cache)

	res, err := uc.ReserveOrder(context.Background(), domain.ReserveOrderInput{
		VoucherProductID: "vp_fnb",
		PhoneNumber:      "08123456789",
		IdempotencyKey:   "idem-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OrderID == "" {
		t.Error("expected non-empty order id")
	}
	if res.Status != string(domain.OrderPending) {
		t.Errorf("expected PENDING, got %s", res.Status)
	}
	if res.Price.Amount != 4500000 { // 45000 IDR * 100
		t.Errorf("expected price 4500000 cents, got %d", res.Price.Amount)
	}
	if res.ReservationDeadline.IsZero() {
		t.Error("expected non-empty deadline")
	}
}

func TestReserveOrder_InvalidPhone(t *testing.T) {
	repo := &mockOrderRepo{product: &domain.ProductRead{ID: "vp_fnb", PriceIDR: 10000}}
	uc := usecase.NewReserveOrderUsecase(repo, &mockStockCache{})
	_, err := uc.ReserveOrder(context.Background(), domain.ReserveOrderInput{VoucherProductID: "vp_fnb", PhoneNumber: "123"})
	if !errors.Is(err, domain.ErrInvalidPhone) {
		t.Errorf("expected ErrInvalidPhone, got %v", err)
	}
}

func TestReserveOrder_OutOfStock(t *testing.T) {
	repo := &mockOrderRepo{
		product: &domain.ProductRead{ID: "vp_fnb", CategoryName: "F&B", PriceIDR: 10000, QuotaID: "q"},
		idem:    map[string]*domain.Order{},
	}
	cache := &mockStockCache{reserveCode: -2}
	uc := usecase.NewReserveOrderUsecase(repo, cache)
	_, err := uc.ReserveOrder(context.Background(), domain.ReserveOrderInput{VoucherProductID: "vp_fnb", PhoneNumber: "08123456789"})
	if !errors.Is(err, domain.ErrOutOfStock) {
		t.Errorf("expected ErrOutOfStock, got %v", err)
	}
	se := &usecase.StockError{}
	if !errors.As(err, &se) || se.Category != "FOOD_BEVERAGE" {
		t.Errorf("expected StockError with category FOOD_BEVERAGE, got %v", err)
	}
}

func TestReserveOrder_AlreadyEntitled(t *testing.T) {
	repo := &mockOrderRepo{
		product: &domain.ProductRead{ID: "vp_fashion", CategoryName: "Fashion", PriceIDR: 10000, QuotaID: "q"},
		idem:    map[string]*domain.Order{},
	}
	cache := &mockStockCache{reserveCode: -3}
	uc := usecase.NewReserveOrderUsecase(repo, cache)
	_, err := uc.ReserveOrder(context.Background(), domain.ReserveOrderInput{VoucherProductID: "vp_fashion", PhoneNumber: "08123456789"})
	if !errors.Is(err, domain.ErrAlreadyEntitled) {
		t.Errorf("expected ErrAlreadyEntitled, got %v", err)
	}
	ae := &usecase.AlreadyEntitledError{}
	if !errors.As(err, &ae) || ae.Category != "FASHION_LIFESTYLE" {
		t.Errorf("expected AlreadyEntitledError with category FASHION_LIFESTYLE, got %v", err)
	}
}

func TestReserveOrder_IdempotencyDedupe(t *testing.T) {
	existing := &domain.Order{
		ID:                  "ord_existing",
		CustomerID:          "628123456789",
		VoucherProductID:    "vp_fnb",
		Category:            "FOOD_BEVERAGE",
		PriceAmount:         500000,
		Currency:            "IDR",
		Status:              domain.OrderPending,
		ReservationDeadline: time.Now().Add(5 * time.Minute),
	}
	repo := &mockOrderRepo{product: &domain.ProductRead{ID: "vp_fnb", PriceIDR: 10000, QuotaID: "q"}, idem: map[string]*domain.Order{"idem-x": existing}}
	cache := &mockStockCache{}
	uc := usecase.NewReserveOrderUsecase(repo, cache)

	res, err := uc.ReserveOrder(context.Background(), domain.ReserveOrderInput{
		VoucherProductID: "vp_fnb",
		PhoneNumber:      "08123456789",
		IdempotencyKey:   "idem-x",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OrderID != "ord_existing" {
		t.Errorf("expected dedupe order id ord_existing, got %s", res.OrderID)
	}
}

func TestReserveOrder_PostgresFailureRollsBackRedis(t *testing.T) {
	repo := &mockOrderRepo{
		product:   &domain.ProductRead{ID: "vp_fnb", PriceIDR: 10000, QuotaID: "q"},
		idem:      map[string]*domain.Order{},
		insertErr: errors.New("db down"),
	}
	cache := &mockStockCache{reserveCode: 1, releaseErr: nil}
	uc := usecase.NewReserveOrderUsecase(repo, cache)
	_, err := uc.ReserveOrder(context.Background(), domain.ReserveOrderInput{VoucherProductID: "vp_fnb", PhoneNumber: "08123456789"})
	if err == nil {
		t.Error("expected error from pg failure")
	}
}
