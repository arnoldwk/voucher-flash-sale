package usecase_test

import (
	"context"
	"testing"
	"time"

	"voucher-flash-sale/services/customer-service/internal/domain"
	"voucher-flash-sale/services/customer-service/internal/usecase"
)

type mockCustomerRepo struct {
	customersByPhone map[string]*domain.Customer
	customersByID    map[string]*domain.Customer
}

func newMockCustomerRepo() *mockCustomerRepo {
	return &mockCustomerRepo{
		customersByPhone: make(map[string]*domain.Customer),
		customersByID:    make(map[string]*domain.Customer),
	}
}

func (m *mockCustomerRepo) GetByPhone(ctx context.Context, phone string) (*domain.Customer, error) {
	c, ok := m.customersByPhone[phone]
	if !ok {
		return nil, domain.ErrCustomerNotFound
	}
	return c, nil
}

func (m *mockCustomerRepo) GetByID(ctx context.Context, id string) (*domain.Customer, error) {
	c, ok := m.customersByID[id]
	if !ok {
		return nil, domain.ErrCustomerNotFound
	}
	return c, nil
}

func (m *mockCustomerRepo) Create(ctx context.Context, c *domain.Customer) error {
	m.customersByPhone[c.Phone] = c
	m.customersByID[c.ID] = c
	return nil
}

func (m *mockCustomerRepo) VerifyPhone(ctx context.Context, id string, verifiedAt time.Time) (*domain.Customer, error) {
	c, ok := m.customersByID[id]
	if !ok {
		return nil, domain.ErrCustomerNotFound
	}
	c.IsVerified = true
	c.PhoneVerifiedAt = &verifiedAt
	return c, nil
}

func TestCustomerUsecase(t *testing.T) {
	repo := newMockCustomerRepo()
	uc := usecase.NewCustomerUsecase(repo)
	ctx := context.Background()

	t.Run("phone normalization", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"08123456789", "628123456789"},
			{"+628123456789", "628123456789"},
			{"8123456789", "628123456789"},
			{"628123456789", "628123456789"},
		}

		for _, tc := range tests {
			res := usecase.NormalizePhone(tc.input)
			if res != tc.expected {
				t.Fatalf("NormalizePhone(%s) = %s, expected %s", tc.input, res, tc.expected)
			}
		}
	})

	t.Run("auto-create new customer", func(t *testing.T) {
		cust, err := uc.GetOrCreateCustomer(ctx, "08123456789")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cust.Phone != "628123456789" {
			t.Fatalf("expected normalized phone 628123456789, got %s", cust.Phone)
		}
		if cust.IsVerified {
			t.Fatal("expected isVerified to be false initially")
		}
		if cust.ID == "" {
			t.Fatal("expected non-empty customerId")
		}

		// Second call should return the same customer
		cust2, err := uc.GetOrCreateCustomer(ctx, "08123456789")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cust2.ID != cust.ID {
			t.Fatalf("expected same customerId %s, got %s", cust.ID, cust2.ID)
		}
	})

	t.Run("verify customer", func(t *testing.T) {
		cust, _ := uc.GetOrCreateCustomer(ctx, "08987654321")
		now := time.Now().UTC()

		verified, err := uc.VerifyCustomer(ctx, cust.ID, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !verified.IsVerified {
			t.Fatal("expected isVerified to be true")
		}
		if verified.PhoneVerifiedAt == nil {
			t.Fatal("expected phoneVerifiedAt to be set")
		}
	})
}
