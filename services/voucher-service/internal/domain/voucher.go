package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrVoucherNotFound        = errors.New("voucher not found")
	ErrVoucherAlreadyRedeemed = errors.New("voucher has already been redeemed")
	ErrVoucherExpired         = errors.New("voucher is expired")
)

type VoucherStatus string

const (
	StatusClaimed  VoucherStatus = "CLAIMED"
	StatusRedeemed VoucherStatus = "REDEEMED"
	StatusExpired  VoucherStatus = "EXPIRED"
)

type Voucher struct {
	ID             string        `json:"id"`
	CampaignID     string        `json:"campaign_id"`
	UserID         string        `json:"user_id"`
	VoucherCode    string        `json:"voucher_code"`
	DiscountAmount float64       `json:"discount_amount"`
	Status         VoucherStatus `json:"status"`
	ClaimedAt      time.Time     `json:"claimed_at"`
	RedeemedAt     *time.Time    `json:"redeemed_at,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

type VoucherRepository interface {
	SaveClaimedVoucher(ctx context.Context, v *Voucher) error
	GetByCode(ctx context.Context, code string) (*Voucher, error)
	ListByUserID(ctx context.Context, userID string) ([]*Voucher, error)
	Redeem(ctx context.Context, code string, redeemedAt time.Time) (*Voucher, error)
}
