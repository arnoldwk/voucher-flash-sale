package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"voucher-flash-sale/services/customer-service/internal/domain"
)

type CustomerUsecase struct {
	repo domain.CustomerRepository
}

func NewCustomerUsecase(repo domain.CustomerRepository) *CustomerUsecase {
	return &CustomerUsecase{repo: repo}
}

// NormalizePhone converts phone numbers to canonical E.164 (628...) format.
func NormalizePhone(phone string) string {
	cleaned := strings.TrimSpace(phone)
	cleaned = strings.TrimPrefix(cleaned, "+")

	if strings.HasPrefix(cleaned, "08") {
		return "62" + cleaned[1:]
	} else if strings.HasPrefix(cleaned, "8") {
		return "62" + cleaned
	}
	return cleaned
}

func generateCustomerID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("c_01%s", strings.ToUpper(hex.EncodeToString(b)))
}

func (u *CustomerUsecase) GetOrCreateCustomer(ctx context.Context, rawPhone string) (*domain.Customer, error) {
	phone := NormalizePhone(rawPhone)
	if len(phone) < 8 {
		return nil, domain.ErrInvalidPhone
	}

	cust, err := u.repo.GetByPhone(ctx, phone)
	if err == nil {
		return cust, nil
	}

	if !errors.Is(err, domain.ErrCustomerNotFound) {
		return nil, err
	}

	// Auto-create new customer if not found
	now := time.Now().UTC()
	newCust := &domain.Customer{
		ID:         generateCustomerID(),
		Phone:      phone,
		IsVerified: false,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := u.repo.Create(ctx, newCust); err != nil {
		// In case of concurrent creation race condition, try fetching again
		if existing, fetchErr := u.repo.GetByPhone(ctx, phone); fetchErr == nil {
			return existing, nil
		}
		return nil, err
	}

	return newCust, nil
}

func (u *CustomerUsecase) VerifyCustomer(ctx context.Context, customerID string, verifiedAt time.Time) (*domain.Customer, error) {
	if strings.TrimSpace(customerID) == "" {
		return nil, errors.New("customerId is required")
	}
	if verifiedAt.IsZero() {
		verifiedAt = time.Now().UTC()
	}

	return u.repo.VerifyPhone(ctx, customerID, verifiedAt)
}
