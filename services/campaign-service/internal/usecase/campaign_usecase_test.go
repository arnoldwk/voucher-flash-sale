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
	redis     map[string]int
}

func newMockCampaignRepo() *mockCampaignRepo {
	return &mockCampaignRepo{
		campaigns: make(map[string]*domain.Campaign),
		redis:     make(map[string]int),
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

func (m *mockCampaignRepo) WarmupRedisStock(ctx context.Context, id string, stock int) error {
	m.redis[id] = stock
	return nil
}

func (m *mockCampaignRepo) GetRedisStock(ctx context.Context, id string) (int, error) {
	if stock, ok := m.redis[id]; ok {
		return stock, nil
	}
	return -1, nil
}

func TestCampaignUsecase_Create(t *testing.T) {
	repo := newMockCampaignRepo()
	uc := usecase.NewCampaignUsecase(repo)
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
		if repo.redis[camp.ID] != 500 {
			t.Fatalf("expected redis stock 500, got %d", repo.redis[camp.ID])
		}
	})
}
