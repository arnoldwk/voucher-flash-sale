package events

import "time"

const (
	// ClaimStreamName is the Redis Stream key for flash sale claim events
	ClaimStreamName = "stream:voucher_claims"

	// ClaimGroupName is the Consumer Group name for voucher workers
	ClaimGroupName = "voucher_workers_group"
)

// VoucherClaimedEvent represents the event published upon a successful atomic flash sale claim.
type VoucherClaimedEvent struct {
	VoucherID      string    `json:"voucher_id"`
	CampaignID     string    `json:"campaign_id"`
	UserID         string    `json:"user_id"`
	VoucherCode    string    `json:"voucher_code"`
	DiscountAmount float64   `json:"discount_amount"`
	ClaimedAt      time.Time `json:"claimed_at"`
}
