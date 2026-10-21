package usecase

import (
	"context"
	"time"

	"voucher-flash-sale/pkg/events"
	"voucher-flash-sale/services/voucher-service/internal/domain"
)

type VoucherUsecase struct {
	repo domain.VoucherRepository
}

func NewVoucherUsecase(repo domain.VoucherRepository) *VoucherUsecase {
	return &VoucherUsecase{repo: repo}
}

func (u *VoucherUsecase) ProcessClaimEvent(ctx context.Context, ev events.VoucherClaimedEvent) error {
	now := time.Now().UTC()
	claimedAt := ev.ClaimedAt
	if claimedAt.IsZero() {
		claimedAt = now
	}

	voucher := &domain.Voucher{
		ID:             ev.VoucherID,
		CampaignID:     ev.CampaignID,
		UserID:         ev.UserID,
		VoucherCode:    ev.VoucherCode,
		DiscountAmount: ev.DiscountAmount,
		Status:         domain.StatusClaimed,
		ClaimedAt:      claimedAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	return u.repo.SaveClaimedVoucher(ctx, voucher)
}

func (u *VoucherUsecase) ListUserVouchers(ctx context.Context, userID string) ([]*domain.Voucher, error) {
	return u.repo.ListByUserID(ctx, userID)
}

func (u *VoucherUsecase) RedeemVoucher(ctx context.Context, code string) (*domain.Voucher, error) {
	return u.repo.Redeem(ctx, code, time.Now().UTC())
}
