package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrCampaignNotFound     = errors.New("campaign not found")
	ErrInvalidCampaignDates = errors.New("end time must be after start time")
	ErrInvalidCampaignStock = errors.New("total stock must be greater than zero")
	ErrCampaignAlreadyEnded = errors.New("campaign has already ended")
)

// Campaign represents a flash sale campaign entity.
type Campaign struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	TotalStock     int       `json:"total_stock"`
	RemainingStock int       `json:"remaining_stock"`
	DiscountAmount float64   `json:"discount_amount"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	Status         string    `json:"status"` // ACTIVE, ENDED, DRAFT
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CampaignRepository defines data storage operations for campaigns.
type CampaignRepository interface {
	Create(ctx context.Context, campaign *Campaign) error
	GetByID(ctx context.Context, id string) (*Campaign, error)
	List(ctx context.Context) ([]*Campaign, error)
	UpdateRemainingStock(ctx context.Context, id string, remainingStock int) error
	WarmupRedisStock(ctx context.Context, id string, stock int) error
	GetRedisStock(ctx context.Context, id string) (int, error)
}
