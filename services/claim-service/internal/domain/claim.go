package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrOutOfStock        = errors.New("campaign is sold out")
	ErrAlreadyClaimed    = errors.New("user has already claimed a voucher in this campaign")
	ErrCampaignNotActive = errors.New("campaign is not active")
	ErrRateLimited       = errors.New("too many requests, please slow down")
)

type ClaimRequest struct {
	CampaignID string `json:"campaign_id"`
	UserID     string `json:"user_id"`
}

type ClaimResult struct {
	VoucherID      string    `json:"voucher_id"`
	CampaignID     string    `json:"campaign_id"`
	UserID         string    `json:"user_id"`
	VoucherCode    string    `json:"voucher_code"`
	DiscountAmount float64   `json:"discount_amount"`
	ClaimedAt      time.Time `json:"claimed_at"`
}

type ClaimRepository interface {
	ExecuteAtomicClaim(ctx context.Context, campaignID, userID, voucherID, voucherCode string, discountAmount float64, claimedAt time.Time) error
	GetRemainingStock(ctx context.Context, campaignID string) (int, error)
}
