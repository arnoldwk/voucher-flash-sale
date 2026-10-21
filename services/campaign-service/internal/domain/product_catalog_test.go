package domain_test

import (
	"testing"
	"time"

	"voucher-flash-sale/services/campaign-service/internal/domain"
)

func TestCategoryAPICode(t *testing.T) {
	tests := []struct {
		dbName  string
		apiCode string
	}{
		{"F&B", "FOOD_BEVERAGE"},
		{"Fashion", "FASHION_LIFESTYLE"},
		{"Elektronik", "GADGET_ELECTRONICS"},
		{"Beauty", "BEAUTY_HEALTH"},
		{"Retail", "GENERAL_RETAIL"},
		{"Leisure", "ENTERTAINMENT_LEISURE"},
		{"Unknown", "GENERAL_RETAIL"},
		{"", "GENERAL_RETAIL"},
	}

	for _, tt := range tests {
		t.Run(tt.dbName, func(t *testing.T) {
			got := domain.CategoryAPICode(tt.dbName)
			if got != tt.apiCode {
				t.Fatalf("CategoryAPICode(%q) = %q, want %q", tt.dbName, got, tt.apiCode)
			}
		})
	}
}

func TestMoneyToCents(t *testing.T) {
	tests := []struct {
		idr   float64
		cents int64
	}{
		{5000, 500000},
		{100000, 10000000},
		{0, 0},
		{50000, 5000000},
		{500.50, 50050},
		{999.99, 99999},
		{50000.00, 5000000},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := domain.MoneyToCents(tt.idr)
			if got != tt.cents {
				t.Fatalf("MoneyToCents(%.2f) = %d, want %d", tt.idr, got, tt.cents)
			}
		})
	}
}

func TestNewIDRMoney(t *testing.T) {
	got := domain.NewIDRMoney(5000)
	want := domain.Money{Amount: 500000, Currency: "IDR"}
	if got != want {
		t.Fatalf("NewIDRMoney(5000) = %+v, want %+v", got, want)
	}
}

func TestCatalogItem_ToVoucherProduct(t *testing.T) {
	now := time.Now()
	item := &domain.CatalogItem{
		VoucherProductID:  "vp_fnb",
		CampaignID:        "cmp-kenangan-live",
		EventID:           "evt-nov-w1",
		CategoryID:        "cat-fn-b",
		CategoryName:      "F&B",
		Name:              "Kopi Kenangan",
		DiscountValueIDR:  45000,
		PriceIDR:          5000,
		MinTransactionIDR: 100000,
		RemainingStock:    23,
		SessionStart:      now.Add(-time.Hour),
		SessionEnd:        now.Add(4 * time.Hour),
		SessionStatus:     "ACTIVE",
	}

	t.Run("available stock + not claimed", func(t *testing.T) {
		got := item.ToVoucherProduct(false)
		if got.VoucherProductID != "vp_fnb" {
			t.Errorf("VoucherProductID: got %s", got.VoucherProductID)
		}
		if got.Category != "FOOD_BEVERAGE" {
			t.Errorf("Category: got %s, want FOOD_BEVERAGE", got.Category)
		}
		if got.DiscountValue.Amount != 4500000 {
			t.Errorf("DiscountValue.Amount: got %d, want 4500000", got.DiscountValue.Amount)
		}
		if got.DiscountValue.Currency != "IDR" {
			t.Errorf("DiscountValue.Currency: got %s, want IDR", got.DiscountValue.Currency)
		}
		if got.Price.Amount != 500000 {
			t.Errorf("Price.Amount: got %d, want 500000", got.Price.Amount)
		}
		if got.MinTransaction.Amount != 10000000 {
			t.Errorf("MinTransaction.Amount: got %d, want 10000000", got.MinTransaction.Amount)
		}
		if got.StockStatus != domain.StockAvailable {
			t.Errorf("StockStatus: got %s, want AVAILABLE", got.StockStatus)
		}
		if got.CustomerState != domain.CustomerEligible {
			t.Errorf("CustomerState: got %s, want ELIGIBLE", got.CustomerState)
		}
	})

	t.Run("sold out stock", func(t *testing.T) {
		soldOut := *item
		soldOut.RemainingStock = 0
		got := soldOut.ToVoucherProduct(false)
		if got.StockStatus != domain.StockSoldOut {
			t.Errorf("StockStatus: got %s, want SOLD_OUT", got.StockStatus)
		}
	})

	t.Run("already claimed", func(t *testing.T) {
		got := item.ToVoucherProduct(true)
		if got.CustomerState != domain.CustomerClaimed {
			t.Errorf("CustomerState: got %s, want CLAIMED", got.CustomerState)
		}
	})
}
