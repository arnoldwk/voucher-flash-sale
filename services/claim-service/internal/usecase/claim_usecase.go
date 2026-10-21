package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"voucher-flash-sale/services/claim-service/internal/domain"

	"github.com/google/uuid"
)

type ClaimUsecase struct {
	repo domain.ClaimRepository
}

func NewClaimUsecase(repo domain.ClaimRepository) *ClaimUsecase {
	return &ClaimUsecase{repo: repo}
}

func (u *ClaimUsecase) Claim(ctx context.Context, req domain.ClaimRequest) (*domain.ClaimResult, error) {
	if strings.TrimSpace(req.CampaignID) == "" {
		return nil, errors.New("campaign_id is required")
	}
	if strings.TrimSpace(req.UserID) == "" {
		return nil, errors.New("user_id is required")
	}

	voucherID := uuid.New().String()
	randomBytes := make([]byte, 4)
	_, _ = rand.Read(randomBytes)
	voucherCode := fmt.Sprintf("VCHR-%s-%s", strings.ToUpper(hex.EncodeToString(randomBytes)), time.Now().Format("0405"))

	claimedAt := time.Now().UTC()
	discountAmount := 50.0 // Default campaign discount amount; can be configured per campaign

	err := u.repo.ExecuteAtomicClaim(
		ctx,
		req.CampaignID,
		req.UserID,
		voucherID,
		voucherCode,
		discountAmount,
		claimedAt,
	)
	if err != nil {
		return nil, err
	}

	return &domain.ClaimResult{
		VoucherID:      voucherID,
		CampaignID:     req.CampaignID,
		UserID:         req.UserID,
		VoucherCode:    voucherCode,
		DiscountAmount: discountAmount,
		ClaimedAt:      claimedAt,
	}, nil
}

func (u *ClaimUsecase) GetRemainingStock(ctx context.Context, campaignID string) (int, error) {
	return u.repo.GetRemainingStock(ctx, campaignID)
}
