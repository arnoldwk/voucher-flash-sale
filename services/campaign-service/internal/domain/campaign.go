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
	ID               string    `json:"id"`
	ProductID        string    `json:"product_id,omitempty"`
	FlashSaleEventID string    `json:"flash_sale_event_id,omitempty"`
	CategoryID       string    `json:"category_id,omitempty"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	SessionStock     int       `json:"session_stock"`
	ClaimedCount     int       `json:"claimed_count"`
	RemainingStock   int       `json:"remaining_stock"`
	DiscountAmount   float64   `json:"discount_amount"`
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	Status           string    `json:"status"` // ACTIVE, SCHEDULED, ENDED, DRAFT
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CampaignRepository defines persistent storage operations for campaigns.
type CampaignRepository interface {
	Create(ctx context.Context, campaign *Campaign) error
	GetByID(ctx context.Context, id string) (*Campaign, error)
	List(ctx context.Context) ([]*Campaign, error)
	UpdateRemainingStock(ctx context.Context, id string, remainingStock int) error
}

// CampaignMeta holds campaign metadata stored in Redis for atomic claim validation.
type CampaignMeta struct {
	CategoryID     string
	DiscountAmount float64
	StartTime      time.Time
	EndTime        time.Time
}

// StockCache abstracts the hot-path in-memory counter layer (Redis).
type StockCache interface {
	// Warmup stores the initial stock counter and sets TTL.
	Warmup(ctx context.Context, campaignID string, stock int, ttl time.Duration) error
	// Stock returns the current cached stock. Returns -1 if not cached.
	Stock(ctx context.Context, campaignID string) (int, error)
	// PublishMeta stores campaign metadata for atomic claim validation.
	PublishMeta(ctx context.Context, campaignID string, meta CampaignMeta) error
	// Stocks returns stock for multiple campaigns in one round-trip (for List optimization).
	Stocks(ctx context.Context, campaignIDs []string) (map[string]int, error)
}
