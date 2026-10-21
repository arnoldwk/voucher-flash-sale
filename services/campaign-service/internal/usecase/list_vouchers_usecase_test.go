package usecase_test

import (
	"context"
	"testing"
	"time"

	"voucher-flash-sale/services/campaign-service/internal/domain"
	"voucher-flash-sale/services/campaign-service/internal/usecase"
)

type mockCatalogRepo struct {
	items   []*domain.CatalogItem
	claimed map[string]map[string]bool
}

func (m *mockCatalogRepo) ListVoucherProducts(ctx context.Context, now time.Time) ([]*domain.CatalogItem, error) {
	return m.items, nil
}

func (m *mockCatalogRepo) ClaimedCategoriesByEvent(ctx context.Context, customerID string) (map[string]map[string]bool, error) {
	return m.claimed, nil
}

type mockCatalogCache struct{}

func (s *mockCatalogCache) Warmup(ctx context.Context, campaignID string, stock int, ttl time.Duration) error {
	return nil
}
func (s *mockCatalogCache) Stock(ctx context.Context, campaignID string) (int, error) { return -1, nil }
func (s *mockCatalogCache) PublishMeta(ctx context.Context, campaignID string, meta domain.CampaignMeta) error {
	return nil
}
func (s *mockCatalogCache) Stocks(ctx context.Context, campaignIDs []string) (map[string]int, error) {
	return nil, nil
}

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"mobile 08-prefix", "08123456789", "628123456789", false},
		{"mobile with dashes", "0812-345-6789", "628123456789", false},
		{"mobile with spaces", "0812 3456 789", "628123456789", false},
		{"with +62 prefix", "+628123456789", "628123456789", false},
		{"with parentheses", "(0812) 3456 789", "628123456789", false},
		{"already normalized", "628123456789", "628123456789", false},
		{"empty", "", "", true},
		{"whitespace only", "   ", "", true},
		{"too short", "123", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// normalizePhone is unexported; test via Execute with a no-op repo
			repo := &mockCatalogRepo{items: []*domain.CatalogItem{}}
			uc := usecase.NewListVouchersUsecase(repo, &mockCatalogCache{})
			_, err := uc.Execute(context.Background(), usecase.ListVouchersInput{Phone: tt.input})
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for input %q: %v", tt.input, err)
				}
			}
		})
	}
}

func TestListVouchersExecute(t *testing.T) {
	now := time.Now().UTC()

	items := []*domain.CatalogItem{
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
			SessionStart:      now.Add(-1 * time.Hour),
			SessionEnd:        now.Add(2 * time.Hour),
			SessionStatus:     "ACTIVE",
		},
		{
			VoucherProductID:  "vp-2",
			CampaignID:        "camp-2",
			EventID:           "event-1",
			CategoryID:        "cat-2",
			CategoryName:      "Fashion",
			Name:              "Diskon 50K",
			DiscountValueIDR:  50000,
			PriceIDR:          10000,
			MinTransactionIDR: 100000,
			RemainingStock:    0,
			SessionStart:      now.Add(-1 * time.Hour),
			SessionEnd:        now.Add(2 * time.Hour),
			SessionStatus:     "ACTIVE",
		},
	}

	t.Run("eligible customer — not claimed anything", func(t *testing.T) {
		repo := &mockCatalogRepo{items: items, claimed: map[string]map[string]bool{}}
		uc := usecase.NewListVouchersUsecase(repo, &mockCatalogCache{})
		result, err := uc.Execute(context.Background(), usecase.ListVouchersInput{Phone: "08123456789"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 products, got %d", len(result))
		}

		// vp-1: stock available, customer eligible
		if result[0].VoucherProductID != "vp-1" {
			t.Errorf("expected vp-1, got %s", result[0].VoucherProductID)
		}
		if result[0].StockStatus != domain.StockAvailable {
			t.Errorf("expected AVAILABLE stock, got %s", result[0].StockStatus)
		}
		if result[0].CustomerState != domain.CustomerEligible {
			t.Errorf("expected ELIGIBLE, got %s", result[0].CustomerState)
		}
		if result[0].Category != domain.CategoryFoodBeverage {
			t.Errorf("expected FOOD_BEVERAGE, got %s", result[0].Category)
		}
		if result[0].DiscountValue.Amount != 2500000 { // 25000 IDR * 100
			t.Errorf("expected discount 2500000 cents, got %d", result[0].DiscountValue.Amount)
		}

		// vp-2: stock sold out, customer eligible
		if result[1].StockStatus != domain.StockSoldOut {
			t.Errorf("expected SOLD_OUT stock, got %s", result[1].StockStatus)
		}
		if result[1].CustomerState != domain.CustomerEligible {
			t.Errorf("expected ELIGIBLE, got %s", result[1].CustomerState)
		}
	})

	t.Run("already claimed — category marked CLAIMED", func(t *testing.T) {
		repo := &mockCatalogRepo{
			items: items,
			claimed: map[string]map[string]bool{
				"event-1": {"cat-1": true}, // vp-1's category claimed
			},
		}
		uc := usecase.NewListVouchersUsecase(repo, &mockCatalogCache{})
		result, err := uc.Execute(context.Background(), usecase.ListVouchersInput{Phone: "08123456789"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result[0].CustomerState != domain.CustomerClaimed {
			t.Errorf("expected CLAIMED for vp-1, got %s", result[0].CustomerState)
		}
		if result[1].CustomerState != domain.CustomerEligible {
			t.Errorf("expected ELIGIBLE for vp-2 (different category), got %s", result[1].CustomerState)
		}
	})

	t.Run("claimed in different event — still eligible", func(t *testing.T) {
		repo := &mockCatalogRepo{
			items: items,
			claimed: map[string]map[string]bool{
				"other-event": {"cat-1": true}, // cat-1 claimed in different event
			},
		}
		uc := usecase.NewListVouchersUsecase(repo, &mockCatalogCache{})
		result, err := uc.Execute(context.Background(), usecase.ListVouchersInput{Phone: "08123456789"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// cat-1 is claimed in other-event, but vp-1 is in event-1
		// so event-1 has no claim for cat-1 → ELIGIBLE
		if result[0].CustomerState != domain.CustomerEligible {
			t.Errorf("expected ELIGIBLE (claimed in different event), got %s", result[0].CustomerState)
		}
	})

	t.Run("empty catalog", func(t *testing.T) {
		repo := &mockCatalogRepo{items: []*domain.CatalogItem{}, claimed: map[string]map[string]bool{}}
		uc := usecase.NewListVouchersUsecase(repo, &mockCatalogCache{})
		result, err := uc.Execute(context.Background(), usecase.ListVouchersInput{Phone: "08123456789"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected 0 products, got %d", len(result))
		}
	})

	t.Run("already normalized phone", func(t *testing.T) {
		repo := &mockCatalogRepo{items: items, claimed: map[string]map[string]bool{}}
		uc := usecase.NewListVouchersUsecase(repo, &mockCatalogCache{})
		_, err := uc.Execute(context.Background(), usecase.ListVouchersInput{Phone: "628123456789"})
		if err != nil {
			t.Fatalf("unexpected error for already-normalized phone: %v", err)
		}
	})

	t.Run("unknown category falls back to GENERAL_RETAIL", func(t *testing.T) {
		unknownCatItems := []*domain.CatalogItem{
			{
				VoucherProductID:  "vp-3",
				CampaignID:        "camp-3",
				EventID:           "event-1",
				CategoryID:        "cat-3",
				CategoryName:      "UnknownCategory",
				Name:              "Voucher",
				DiscountValueIDR:  10000,
				PriceIDR:          1000,
				MinTransactionIDR: 0,
				RemainingStock:    5,
				SessionStart:      now.Add(-1 * time.Hour),
				SessionEnd:        now.Add(2 * time.Hour),
				SessionStatus:     "ACTIVE",
			},
		}
		repo := &mockCatalogRepo{items: unknownCatItems, claimed: map[string]map[string]bool{}}
		uc := usecase.NewListVouchersUsecase(repo, &mockCatalogCache{})
		result, err := uc.Execute(context.Background(), usecase.ListVouchersInput{Phone: "08123456789"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result[0].Category != domain.CategoryGeneralRetail {
			t.Errorf("expected GENERAL_RETAIL fallback, got %s", result[0].Category)
		}
	})
}
