package usecase_test

import (
	"context"
	"testing"
	"time"

	"voucher-flash-sale/services/claim-service/internal/domain"
	"voucher-flash-sale/services/claim-service/internal/usecase"
)

type mockClaimRepo struct {
	stock   map[string]int
	claimed map[string]map[string]bool // campaignID -> userID -> bool
}

func newMockClaimRepo() *mockClaimRepo {
	return &mockClaimRepo{
		stock:   make(map[string]int),
		claimed: make(map[string]map[string]bool),
	}
}

func (m *mockClaimRepo) ExecuteAtomicClaim(
	ctx context.Context,
	campaignID, userID, voucherID, voucherCode string,
	discountAmount float64,
	claimedAt time.Time,
) error {
	users, ok := m.claimed[campaignID]
	if !ok {
		users = make(map[string]bool)
		m.claimed[campaignID] = users
	}

	if users[userID] {
		return domain.ErrAlreadyClaimed
	}

	stock, exists := m.stock[campaignID]
	if !exists {
		return domain.ErrCampaignNotActive
	}
	if stock <= 0 {
		return domain.ErrOutOfStock
	}

	m.stock[campaignID] = stock - 1
	users[userID] = true
	return nil
}

func (m *mockClaimRepo) GetRemainingStock(ctx context.Context, campaignID string) (int, error) {
	if s, ok := m.stock[campaignID]; ok {
		return s, nil
	}
	return 0, domain.ErrCampaignNotActive
}

func TestClaimUsecase_Claim(t *testing.T) {
	repo := newMockClaimRepo()
	uc := usecase.NewClaimUsecase(repo)
	ctx := context.Background()

	campaignID := "camp-123"
	repo.stock[campaignID] = 2

	t.Run("empty user_id error", func(t *testing.T) {
		_, err := uc.Claim(ctx, domain.ClaimRequest{CampaignID: campaignID, UserID: ""})
		if err == nil {
			t.Fatal("expected error for empty user_id")
		}
	})

	t.Run("successful claim for user1", func(t *testing.T) {
		res, err := uc.Claim(ctx, domain.ClaimRequest{CampaignID: campaignID, UserID: "user-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.VoucherCode == "" || res.VoucherID == "" {
			t.Fatal("expected non-empty voucher details")
		}
		if repo.stock[campaignID] != 1 {
			t.Fatalf("expected stock 1, got %d", repo.stock[campaignID])
		}
	})

	t.Run("duplicate claim from same user rejected", func(t *testing.T) {
		_, err := uc.Claim(ctx, domain.ClaimRequest{CampaignID: campaignID, UserID: "user-1"})
		if err != domain.ErrAlreadyClaimed {
			t.Fatalf("expected %v, got %v", domain.ErrAlreadyClaimed, err)
		}
	})

	t.Run("successful claim for user2 exhausts stock", func(t *testing.T) {
		_, err := uc.Claim(ctx, domain.ClaimRequest{CampaignID: campaignID, UserID: "user-2"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.stock[campaignID] != 0 {
			t.Fatalf("expected stock 0, got %d", repo.stock[campaignID])
		}
	})

	t.Run("out of stock rejection for user3", func(t *testing.T) {
		_, err := uc.Claim(ctx, domain.ClaimRequest{CampaignID: campaignID, UserID: "user-3"})
		if err != domain.ErrOutOfStock {
			t.Fatalf("expected %v, got %v", domain.ErrOutOfStock, err)
		}
	})
}
