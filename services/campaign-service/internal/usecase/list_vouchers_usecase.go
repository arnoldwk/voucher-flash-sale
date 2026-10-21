package usecase

import (
	"context"
	"regexp"
	"time"

	"voucher-flash-sale/services/campaign-service/internal/domain"
)

type ListVouchersInput struct {
	Phone string
}

type ListVouchersUsecase struct {
	repo  domain.ProductCatalogRepository
	cache domain.StockCache
}

func NewListVouchersUsecase(repo domain.ProductCatalogRepository, cache domain.StockCache) *ListVouchersUsecase {
	return &ListVouchersUsecase{repo: repo, cache: cache}
}

// Execute membangun daftar voucher product katalog untuk customer yang teridentifikasi
// via nomor telepon. Nomor telepon dinormalisasi: "08123456789" -> "628123456789".
func (u *ListVouchersUsecase) Execute(ctx context.Context, in ListVouchersInput) ([]*domain.VoucherProduct, error) {
	customerID, err := normalizePhone(in.Phone)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	items, err := u.repo.ListVoucherProducts(ctx, now)
	if err != nil {
		return nil, err
	}

	claimed, err := u.repo.ClaimedCategoriesByEvent(ctx, customerID)
	if err != nil {
		return nil, err
	}

	// Overlay stock live dari Redis. Postgres menyimpan snapshot persisten,
	// sedangkan Redis adalah counter decremented saat klaim. Bila campaign belum
	// ada di cache, nilai Postgres yang dipakai.
	u.applyLiveStock(ctx, items)

	result := make([]*domain.VoucherProduct, 0, len(items))
	for _, item := range items {
		isClaimed := false
		if cats, ok := claimed[item.EventID]; ok {
			if cats[item.CategoryID] {
				isClaimed = true
			}
		}
		vp := item.ToVoucherProduct(isClaimed)
		result = append(result, &vp)
	}

	return result, nil
}

// applyLiveStock menimpa RemainingStock dengan counter Redis bila tersedia.
func (u *ListVouchersUsecase) applyLiveStock(ctx context.Context, items []*domain.CatalogItem) {
	if len(items) == 0 {
		return
	}

	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.CampaignID)
	}

	stocks, err := u.cache.Stocks(ctx, ids)
	if err != nil {
		// Cache bukan sumber kebenaran; kegagalan Redis tidak boleh menggagalkan listing.
		return
	}

	for _, item := range items {
		if live, ok := stocks[item.CampaignID]; ok && live >= 0 {
			item.RemainingStock = live
		}
	}
}

// normalizePhone extracts digits from a phone number and normalizes Indonesian prefixes.
// 08xx -> 628xx, +62 -> 62.
var phoneCleanup = regexp.MustCompile(`[\s\-\(\)]+`)

func normalizePhone(phone string) (string, error) {
	if phone == "" {
		return "", domain.ErrInvalidPhone
	}

	// Hapus karakter non-digit
	digits := phoneCleanup.ReplaceAllString(phone, "")

	// Normalisasi prefix Indonesia
	switch {
	case len(digits) == 0:
		return "", domain.ErrInvalidPhone
	case len(digits) >= 2 && digits[0] == '0' && digits[1] == '8':
		// 08xx -> 628xx
		digits = "62" + digits[1:]
	case len(digits) >= 3 && digits[0] == '+' && digits[1] == '6' && digits[2] == '2':
		// +62 -> 62
		digits = digits[1:]
	}

	// Minimal panjang: negara(2) + lokal(6)
	if len(digits) < 8 {
		return "", domain.ErrInvalidPhone
	}

	return digits, nil
}
