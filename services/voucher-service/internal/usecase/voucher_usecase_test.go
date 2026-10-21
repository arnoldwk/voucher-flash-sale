package usecase_test

import (
	"context"
	"testing"
	"time"

	"voucher-flash-sale/pkg/events"
	"voucher-flash-sale/services/voucher-service/internal/domain"
	"voucher-flash-sale/services/voucher-service/internal/usecase"
)

type mockVoucherRepo struct {
	vouchersByCode   map[string]*domain.Voucher
	vouchersByUserID map[string][]*domain.Voucher
}

func newMockVoucherRepo() *mockVoucherRepo {
	return &mockVoucherRepo{
		vouchersByCode:   make(map[string]*domain.Voucher),
		vouchersByUserID: make(map[string][]*domain.Voucher),
	}
}

func (m *mockVoucherRepo) SaveClaimedVoucher(ctx context.Context, v *domain.Voucher) error {
	m.vouchersByCode[v.VoucherCode] = v
	m.vouchersByUserID[v.UserID] = append(m.vouchersByUserID[v.UserID], v)
	return nil
}

func (m *mockVoucherRepo) GetByCode(ctx context.Context, code string) (*domain.Voucher, error) {
	v, ok := m.vouchersByCode[code]
	if !ok {
		return nil, domain.ErrVoucherNotFound
	}
	return v, nil
}

func (m *mockVoucherRepo) ListByUserID(ctx context.Context, userID string) ([]*domain.Voucher, error) {
	return m.vouchersByUserID[userID], nil
}

func (m *mockVoucherRepo) Redeem(ctx context.Context, code string, redeemedAt time.Time) (*domain.Voucher, error) {
	v, ok := m.vouchersByCode[code]
	if !ok {
		return nil, domain.ErrVoucherNotFound
	}
	if v.Status == domain.StatusRedeemed {
		return nil, domain.ErrVoucherAlreadyRedeemed
	}

	v.Status = domain.StatusRedeemed
	v.RedeemedAt = &redeemedAt
	return v, nil
}

func TestVoucherUsecase(t *testing.T) {
	repo := newMockVoucherRepo()
	uc := usecase.NewVoucherUsecase(repo)
	ctx := context.Background()

	ev := events.VoucherClaimedEvent{
		VoucherID:      "v-1",
		CampaignID:     "c-1",
		UserID:         "user-100",
		VoucherCode:    "VCHR-TEST-100",
		DiscountAmount: 50.0,
		ClaimedAt:      time.Now(),
	}

	t.Run("process claim event", func(t *testing.T) {
		err := uc.ProcessClaimEvent(ctx, ev)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		vouchers, err := uc.ListUserVouchers(ctx, "user-100")
		if err != nil || len(vouchers) != 1 {
			t.Fatalf("expected 1 voucher for user, got %d", len(vouchers))
		}
		if vouchers[0].VoucherCode != "VCHR-TEST-100" {
			t.Fatalf("expected code VCHR-TEST-100, got %s", vouchers[0].VoucherCode)
		}
		if vouchers[0].Status != domain.StatusClaimed {
			t.Fatalf("expected status CLAIMED, got %s", vouchers[0].Status)
		}
	})

	t.Run("redeem voucher success", func(t *testing.T) {
		redeemed, err := uc.RedeemVoucher(ctx, "VCHR-TEST-100")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if redeemed.Status != domain.StatusRedeemed {
			t.Fatalf("expected status REDEEMED, got %s", redeemed.Status)
		}
		if redeemed.RedeemedAt == nil {
			t.Fatal("expected redeemed_at to be set")
		}
	})

	t.Run("redeem already redeemed voucher", func(t *testing.T) {
		_, err := uc.RedeemVoucher(ctx, "VCHR-TEST-100")
		if err != domain.ErrVoucherAlreadyRedeemed {
			t.Fatalf("expected %v, got %v", domain.ErrVoucherAlreadyRedeemed, err)
		}
	})
}
