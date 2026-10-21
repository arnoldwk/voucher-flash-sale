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
}

type CampaignUsecase struct {
	repo domain.CampaignRepository
}

func NewCampaignUsecase(repo domain.CampaignRepository) *CampaignUsecase {
	return &CampaignUsecase{repo: repo}
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
		ID:             uuid.New().String(),
		Title:          input.Title,
		Description:    input.Description,
		TotalStock:     input.TotalStock,
		RemainingStock: input.TotalStock,
		DiscountAmount: input.DiscountAmount,
		StartTime:      input.StartTime.UTC(),
		EndTime:        input.EndTime.UTC(),
		Status:         "ACTIVE",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := u.repo.Create(ctx, campaign); err != nil {
		return nil, err
	}

	// Pre-warm stock into Redis for ultra-fast atomic deductions
	if err := u.repo.WarmupRedisStock(ctx, campaign.ID, campaign.TotalStock); err != nil {
		return nil, err
	}

	return campaign, nil
}

func (u *CampaignUsecase) GetByID(ctx context.Context, id string) (*domain.Campaign, error) {
	campaign, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Fetch live real-time stock from Redis if available
	liveStock, err := u.repo.GetRedisStock(ctx, id)
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

	for _, c := range campaigns {
		liveStock, err := u.repo.GetRedisStock(ctx, c.ID)
		if err == nil && liveStock >= 0 {
			c.RemainingStock = liveStock
		}
	}

	return campaigns, nil
}
