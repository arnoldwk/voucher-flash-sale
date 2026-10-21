package usecase

import (
	"context"
	"time"

	"voucher-flash-sale/services/campaign-service/internal/domain"

	"github.com/google/uuid"
)

type CreateCampaignInput struct {
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	TotalStock     int       `json:"total_stock"`
	DiscountAmount float64   `json:"discount_amount"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	ProductID      string    `json:"product_id"`
	EventID        string    `json:"event_id"`
	CategoryID     string    `json:"category_id"`
}

type CampaignUsecase struct {
	repo  domain.CampaignRepository
	cache domain.StockCache
}

func NewCampaignUsecase(repo domain.CampaignRepository, cache domain.StockCache) *CampaignUsecase {
	return &CampaignUsecase{repo: repo, cache: cache}
}

func (u *CampaignUsecase) Create(ctx context.Context, input CreateCampaignInput) (*domain.Campaign, error) {
	if input.TotalStock <= 0 {
		return nil, domain.ErrInvalidCampaignStock
	}

	if input.EndTime.Before(input.StartTime) || input.EndTime.Equal(input.StartTime) {
		return nil, domain.ErrInvalidCampaignDates
	}

	now := time.Now().UTC()
	campaign := &domain.Campaign{
		ID:               uuid.New().String(),
		ProductID:        input.ProductID,
		FlashSaleEventID: input.EventID,
		CategoryID:       input.CategoryID,
		Title:            input.Title,
		Description:      input.Description,
		SessionStock:     input.TotalStock,
		ClaimedCount:     0,
		RemainingStock:   input.TotalStock,
		DiscountAmount:   input.DiscountAmount,
		StartTime:        input.StartTime.UTC(),
		EndTime:          input.EndTime.UTC(),
		Status:           "ACTIVE",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := u.repo.Create(ctx, campaign); err != nil {
		return nil, err
	}

	// Warm hot path: metadata campaign + counter stock
	ttl := time.Until(campaign.EndTime) + time.Hour
	if ttl < time.Hour {
		ttl = time.Hour
	}

	meta := domain.CampaignMeta{
		CategoryID:     campaign.CategoryID,
		DiscountAmount: campaign.DiscountAmount,
		StartTime:      campaign.StartTime,
		EndTime:        campaign.EndTime,
	}
	if err := u.cache.PublishMeta(ctx, campaign.ID, meta); err != nil {
		return nil, err
	}
	if err := u.cache.Warmup(ctx, campaign.ID, campaign.RemainingStock, ttl); err != nil {
		return nil, err
	}

	return campaign, nil
}

func (u *CampaignUsecase) GetByID(ctx context.Context, id string) (*domain.Campaign, error) {
	campaign, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Override remaining stock dengan counter live di cache
	liveStock, err := u.cache.Stock(ctx, id)
	if err == nil && liveStock >= 0 {
		campaign.RemainingStock = liveStock
	}

	return campaign, nil
}

func (u *CampaignUsecase) List(ctx context.Context) ([]*domain.Campaign, error) {
	campaigns, err := u.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	if len(campaigns) > 0 {
		ids := make([]string, len(campaigns))
		for i, c := range campaigns {
			ids[i] = c.ID
		}
		stocks, err := u.cache.Stocks(ctx, ids) // satu round-trip MGET
		if err == nil {
			for _, c := range campaigns {
				if s, ok := stocks[c.ID]; ok && s >= 0 {
					c.RemainingStock = s
				}
			}
		}
	}

	return campaigns, nil
}
