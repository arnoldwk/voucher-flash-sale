package usecase_test

import (
	"context"
	"testing"
	"time"

	"voucher-flash-sale/services/campaign-service/internal/domain"
	"voucher-flash-sale/services/campaign-service/internal/usecase"
)

type mockCampaignRepo struct {
	campaigns map[string]*domain.Campaign
}

func newMockCampaignRepo() *mockCampaignRepo {
	return &mockCampaignRepo{
		campaigns: make(map[string]*domain.Campaign),
	}
}

func (m *mockCampaignRepo) Create(ctx context.Context, c *domain.Campaign) error {
	m.campaigns[c.ID] = c
	return nil
}

func (m *mockCampaignRepo) GetByID(ctx context.Context, id string) (*domain.Campaign, error) {
	c, ok := m.campaigns[id]
	if !ok {
		return nil, domain.ErrCampaignNotFound
	}
	return c, nil
}

func (m *mockCampaignRepo) List(ctx context.Context) ([]*domain.Campaign, error) {
	var list []*domain.Campaign
	for _, c := range m.campaigns {
		list = append(list, c)
	}
	return list, nil
}

func (m *mockCampaignRepo) UpdateRemainingStock(ctx context.Context, id string, remainingStock int) error {
	if c, ok := m.campaigns[id]; ok {
		c.RemainingStock = remainingStock
	}
	return nil
}

type mockStockCache struct {
	stock map[string]int
}

func newMockStockCache() *mockStockCache {
	return &mockStockCache{stock: make(map[string]int)}
}

func (m *mockStockCache) Warmup(ctx context.Context, campaignID string, stock int, ttl time.Duration) error {
	m.stock[campaignID] = stock
	return nil
}

func (m *mockStockCache) Stock(ctx context.Context, campaignID string) (int, error) {
	if s, ok := m.stock[campaignID]; ok {
		return s, nil
	}
	return -1, nil
}

func (m *mockStockCache) PublishMeta(ctx context.Context, campaignID string, meta domain.CampaignMeta) error {
	return nil
}

func (m *mockStockCache) Stocks(ctx context.Context, campaignIDs []string) (map[string]int, error) {
	result := make(map[string]int)
	for _, id := range campaignIDs {
		if s, ok := m.stock[id]; ok {
			result[id] = s
		} else {
			result[id] = -1
		}
	}
	return result, nil
}

func TestCampaignUsecase_Create(t *testing.T) {
	repo := newMockCampaignRepo()
	cache := newMockStockCache()
	uc := usecase.NewCampaignUsecase(repo, cache)
	ctx := context.Background()

	t.Run("invalid stock <= 0", func(t *testing.T) {
		_, err := uc.Create(ctx, usecase.CreateCampaignInput{
			Title:      "Flash Sale",
			TotalStock: 0,
			StartTime:  time.Now(),
			EndTime:    time.Now().Add(1 * time.Hour),
		})
		if err != domain.ErrInvalidCampaignStock {
			t.Fatalf("expected %v, got %v", domain.ErrInvalidCampaignStock, err)
		}
	})

	t.Run("invalid end time before start time", func(t *testing.T) {
		_, err := uc.Create(ctx, usecase.CreateCampaignInput{
			Title:      "Flash Sale",
			TotalStock: 100,
			StartTime:  time.Now().Add(2 * time.Hour),
			EndTime:    time.Now().Add(1 * time.Hour),
		})
		if err != domain.ErrInvalidCampaignDates {
			t.Fatalf("expected %v, got %v", domain.ErrInvalidCampaignDates, err)
		}
	})

	t.Run("successful campaign creation", func(t *testing.T) {
		camp, err := uc.Create(ctx, usecase.CreateCampaignInput{
			Title:          "Super 11.11 Flash Sale",
			Description:    "Huge discounts",
			TotalStock:     500,
			DiscountAmount: 75.0,
			StartTime:      time.Now(),
			EndTime:        time.Now().Add(2 * time.Hour),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if camp.ID == "" {
			t.Fatalf("expected non-empty ID")
		}
		if camp.RemainingStock != 500 {
			t.Fatalf("expected remaining stock 500, got %d", camp.RemainingStock)
		}
		if cache.stock[camp.ID] != 500 {
			t.Fatalf("expected cache stock 500, got %d", cache.stock[camp.ID])
		}
	})
}
