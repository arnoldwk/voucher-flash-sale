package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"voucher-flash-sale/services/transaction-service/internal/domain"
)

type ReserveOrderUsecase struct {
	repo  domain.OrderRepository
	cache domain.StockCache
}

func NewReserveOrderUsecase(repo domain.OrderRepository, cache domain.StockCache) *ReserveOrderUsecase {
	return &ReserveOrderUsecase{repo: repo, cache: cache}
}

// ReserveOrderResult is what the usecase returns to the handler.
type ReserveOrderResult struct {
	OrderID             string       `json:"orderId"`
	Status              string       `json:"status"`
	Price               domain.Money `json:"price"`
	ReservationDeadline time.Time    `json:"reservationDeadline"`
}

// ReserveOrder attempts to reserve a voucher product for the customer.
//
// Flow:
//  1. Normalize phone -> customer id.
//  2. Resolve voucher product (active session only).
//  3. Idempotency dedupe on orders.idempotency_key.
//  4. Atomic Lua reservation (decrement Redis stock + write markers).
//  5. Postgres tx: FOR UPDATE quota -> bump reserved_count -> insert order
//     -> insert entitlement. Rollback Redis if tx fails.
//  6. Return the OrderView-compatible result.
func (u *ReserveOrderUsecase) ReserveOrder(ctx context.Context, in domain.ReserveOrderInput) (*ReserveOrderResult, error) {
	customerID, err := normalizePhone(in.PhoneNumber)
	if err != nil {
		return nil, err
	}
	in.CustomerID = customerID

	// Idempotency: when no key is supplied, generate one so the unique
	// constraint is satisfied without enabling dedupe for this request.
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = newUUID()
	}

	product, err := u.repo.FindVoucherProduct(ctx, in.VoucherProductID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	ttl := 5 * time.Minute
	reservationDeadline := now.Add(ttl)

	// Idempotency dedupe: if the key was already used, return the original order.
	if in.IdempotencyKey != "" {
		existing, err := u.repo.FindByIdempotencyKey(ctx, in.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return &ReserveOrderResult{
				OrderID:             existing.ID,
				Status:              string(existing.Status),
				Price:               domain.Money{Amount: existing.PriceAmount, Currency: existing.Currency},
				ReservationDeadline: existing.ReservationDeadline,
			}, nil
		}
	}

	orderID := newOrderID()
	apiCategory := domain.CategoryAPICode(product.CategoryName)
	reservationKey := fmt.Sprintf("reservation:order:%s", orderID)
	stockKey := fmt.Sprintf("campaign:stock:%s", product.CampaignID)
	entitlementKey := fmt.Sprintf("entitlement:cust:%s:session:%s:cat:%s", customerID, product.SessionID, apiCategory)

	orderJSON := fmt.Sprintf(`{"order_id":"%s","category":"%s"}`, orderID, apiCategory)

	// Atomic Lua reservation: decrement Redis stock + write reservation + entitlement markers.
	code, err := u.cache.Reserve(ctx, stockKey, entitlementKey, reservationKey, orderID, orderJSON, ttl)
	if err != nil {
		// Redis infrastructure failure: do not touch Postgres.
		return nil, err
	}

	switch code {
	case 1:
		// Granted. Proceed to durable Postgres write.
	case -2:
		// Out of stock.
		return nil, &StockError{
			Category: apiCategory,
			Err:      domain.ErrOutOfStock,
		}
	case -3:
		// Already entitled for this category in the session.
		return nil, &AlreadyEntitledError{
			Category: apiCategory,
			Err:      domain.ErrAlreadyEntitled,
		}
	default:
		return nil, fmt.Errorf("unexpected reserve code: %d", code)
	}

	// Build order + entitlement entities.
	order := &domain.Order{
		ID:                  orderID,
		CustomerID:          customerID,
		VoucherProductID:    in.VoucherProductID,
		SessionID:           product.SessionID,
		Category:            apiCategory,
		PriceAmount:         domain.MoneyToCents(product.PriceIDR),
		Currency:            "IDR",
		Status:              domain.OrderPending,
		ReservationDeadline: reservationDeadline,
		IdempotencyKey:      in.IdempotencyKey,
		Version:             0,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	entitlement := &domain.CustomerEntitlement{
		ID:            newUUID(),
		CustomerID:    customerID,
		SessionID:     product.SessionID,
		Category:      apiCategory,
		CampaignID:    product.CampaignID,
		ActiveOrderID: orderID,
		Status:        domain.EntitlementActive,
		Version:       0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Durable write in one transaction (quota lock + order + entitlement).
	if err := u.repo.InsertOrderAndEntitlement(ctx, order, product.QuotaID, entitlement); err != nil {
		if errors.Is(err, domain.ErrOutOfStock) {
			// Quota full in PG: rollback the Lua reservation so Redis stays aligned.
			if _, rollErr := u.cache.ReleaseReservation(ctx, stockKey, reservationKey, entitlementKey); rollErr != nil {
				return nil, rollErr
			}
			return nil, &StockError{Category: apiCategory, Err: domain.ErrOutOfStock}
		}
		// Any other failure: rollback Lua reservation too.
		if _, rollErr := u.cache.ReleaseReservation(ctx, stockKey, reservationKey, entitlementKey); rollErr != nil {
			return nil, rollErr
		}
		return nil, err
	}

	return &ReserveOrderResult{
		OrderID:             order.ID,
		Status:              string(order.Status),
		Price:               domain.Money{Amount: order.PriceAmount, Currency: order.Currency},
		ReservationDeadline: order.ReservationDeadline,
	}, nil
}

// StockError is returned when quota is exhausted.
type StockError struct {
	Category string
	Err      error
}

func (e *StockError) Error() string { return e.Err.Error() }
func (e *StockError) Unwrap() error { return e.Err }

// AlreadyEntitledError is returned when customer already has entitlement.
type AlreadyEntitledError struct {
	Category string
	Err      error
}

func (e *AlreadyEntitledError) Error() string { return e.Err.Error() }
func (e *AlreadyEntitledError) Unwrap() error { return e.Err }

// newOrderID generates an order id like ord_<hex>.
func newOrderID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "ord_" + hex.EncodeToString(b)
}

// newUUID generates a random uuid hex string for entitlement id.
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

var phoneCleanup = regexp.MustCompile(`[\s\-\(\)]+`)

// normalizePhone extracts digits from a phone number and normalizes Indonesian
// prefixes: 08xx -> 628xx, +62 -> 62. Mirrors campaign-service logic.
func normalizePhone(phone string) (string, error) {
	if phone == "" {
		return "", domain.ErrInvalidPhone
	}
	digits := phoneCleanup.ReplaceAllString(phone, "")
	if len(digits) == 0 {
		return "", domain.ErrInvalidPhone
	}
	switch {
	case len(digits) >= 2 && digits[0] == '0' && digits[1] == '8':
		digits = "62" + digits[1:]
	case len(digits) >= 3 && digits[0] == '+' && digits[1] == '6' && digits[2] == '2':
		digits = digits[1:]
	}
	if len(digits) < 8 {
		return "", domain.ErrInvalidPhone
	}
	return digits, nil
}
