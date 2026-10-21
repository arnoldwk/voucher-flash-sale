package domain

import (
	"context"
	"errors"
	"math"
	"time"
)

var (
	ErrOutOfStock              = errors.New("quota exhausted for this session")
	ErrAlreadyEntitled         = errors.New("customer already has an active entitlement for this category")
	ErrVoucherProductMissing   = errors.New("voucher product not found or not in an active session")
	ErrReservationWindowClosed = errors.New("voucher session is closed or not yet open")
	ErrIdempotencyConflict     = errors.New("idempotency key reused with a different payload")
	ErrInvalidPhone            = errors.New("invalid phone number")
)

// OrderStatus represents the lifecycle of an order reservation.
type OrderStatus string

const (
	OrderPending   OrderStatus = "PENDING"
	OrderPaid      OrderStatus = "PAID"
	OrderExpired   OrderStatus = "EXPIRED"
	OrderCancelled OrderStatus = "CANCELLED"
)

// QuotaStatus represents the operational status of a session quota.
type QuotaStatus string

const (
	QuotaOpen   QuotaStatus = "OPEN"
	QuotaClosed QuotaStatus = "CLOSED"
)

// EntitlementStatus represents the lifecycle of a customer entitlement.
type EntitlementStatus string

const (
	EntitlementActive   EntitlementStatus = "ACTIVE"
	EntitlementConsumed EntitlementStatus = "CONSUMED"
	EntitlementReleased EntitlementStatus = "RELEASED"
)

// Order is the durable record of a voucher product reservation.
type Order struct {
	ID                  string      `json:"id"`
	CustomerID          string      `json:"customer_id"`
	VoucherProductID    string      `json:"voucher_product_id"`
	SessionID           string      `json:"session_id"`
	Category            string      `json:"category"`
	PriceAmount         int64       `json:"-"` // cents; exposed via OrderView
	Currency            string      `json:"currency"`
	Status              OrderStatus `json:"status"`
	ReservationDeadline time.Time   `json:"reservation_deadline"`
	IdempotencyKey      string      `json:"-"`
	Version             int64       `json:"-"`
	CreatedAt           time.Time   `json:"-"`
	UpdatedAt           time.Time   `json:"-"`
}

// SessionCategoryQuota is the durable per-session stock counter.
type SessionCategoryQuota struct {
	ID               string      `json:"-"`
	SessionID        string      `json:"session_id"`
	VoucherProductID string      `json:"voucher_product_id"`
	Category         string      `json:"category"`
	Quota            int         `json:"quota"`
	ReservedCount    int         `json:"reserved_count"`
	ConfirmedCount   int         `json:"confirmed_count"`
	Status           QuotaStatus `json:"status"`
	Version          int64       `json:"-"`
	CreatedAt        time.Time   `json:"-"`
	UpdatedAt        time.Time   `json:"-"`
}

// CustomerEntitlement links one customer to one active order per
// category per session.
type CustomerEntitlement struct {
	ID            string            `json:"-"`
	CustomerID    string            `json:"customer_id"`
	SessionID     string            `json:"session_id"`
	Category      string            `json:"category"`
	CampaignID    string            `json:"campaign_id"`
	ActiveOrderID string            `json:"active_order_id"`
	Status        EntitlementStatus `json:"status"`
	Version       int64             `json:"-"`
	CreatedAt     time.Time         `json:"-"`
	UpdatedAt     time.Time         `json:"-"`
}

// OrderView is the API response DTO returned to clients.
type OrderView struct {
	OrderID             string    `json:"orderId"`
	Status              string    `json:"status"`
	Price               Money     `json:"price"`
	ReservationDeadline time.Time `json:"reservationDeadline"`
}

// Money is a reusable monetary value object.
type Money struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// NewIDRMoney creates a Money value in IDR from a Rp amount.
func NewIDRMoney(idr float64) Money {
	return Money{Amount: MoneyToCents(idr), Currency: "IDR"}
}

// MoneyToCents converts an IDR amount to cents (1 IDR = 100 cents).
// Values are floored to avoid over-reporting.
func MoneyToCents(idr float64) int64 {
	if idr <= 0 {
		return 0
	}
	return int64(math.Floor(idr * 100))
}

// ReserveOrderInput carries everything needed to attempt a reservation.
type ReserveOrderInput struct {
	VoucherProductID string `json:"voucher_product_id"`
	PhoneNumber      string `json:"phone_number"`
	IdempotencyKey   string `json:"-"`
	CustomerID       string `json:"-"`
}

// ProductRead is the resolved voucher-product read model returned by
// the repository to the usecase.
type ProductRead struct {
	ID             string
	CampaignID     string
	SessionID      string
	CategoryID     string
	CategoryName   string
	Name           string
	PriceIDR       float64
	RemainingStock int
	SessionStatus  string
	StartTime      time.Time
	EndTime        time.Time
	QuotaID        string
}

// OrderRepository is the port to the durable order/quota/entitlement store.
type OrderRepository interface {
	// FindVoucherProduct resolves the product and campaign/session metadata.
	FindVoucherProduct(ctx context.Context, voucherProductID string) (*ProductRead, error)
	// FindByIdempotencyKey returns an existing order for the key if present.
	FindByIdempotencyKey(ctx context.Context, key string) (*Order, error)
	// InsertOrderAndEntitlement persists order + quota bump + entitlement
	// in a single transaction, locking the quota row with FOR UPDATE.
	InsertOrderAndEntitlement(ctx context.Context, order *Order, quotaID string, entitlement *CustomerEntitlement) error
	// ExpirePendingOrders marks PENDING orders past their deadline as EXPIRED,
	// decrements reserved_count and releases entitlements. Returns affected order IDs.
	ExpirePendingOrders(ctx context.Context) ([]string, error)
}

// StockCache is the hot-path Redis cache shared with claim-service/campaign-service.
// Returns "stock", -1 if absent.
type StockCache interface {
	Decrement(ctx context.Context, campaignID string) (int, error)
	Increment(ctx context.Context, campaignID string) (int, error)
	Get(ctx context.Context, campaignID string) (int, error)
	SetReservationMarker(ctx context.Context, reservationKey string, value string, ttl time.Duration) error
	DelReservationMarker(ctx context.Context, reservationKey string) error
	SetEntitlementMarker(ctx context.Context, entitlementKey string, value string, ttl time.Duration) error
	DelEntitlementMarker(ctx context.Context, entitlementKey string) error
	// Reserve atomically decrements stock and writes reservation + entitlement markers.
	// Returns 1 on success, -2 out of stock, -3 already entitled.
	Reserve(ctx context.Context, stockKey, entitlementKey, reservationKey, orderID, orderJSON string, ttl time.Duration) (int, error)
	// ReleaseReservation rolls back stock, reservation marker, and entitlement marker.
	ReleaseReservation(ctx context.Context, stockKey, reservationKey, entitlementKey string) (int, error)
}

// Category API codes exposed to clients (mirrors campaign-service domain).
const (
	CategoryFoodBeverage         = "FOOD_BEVERAGE"
	CategoryFashionLifestyle     = "FASHION_LIFESTYLE"
	CategoryGadgetElectronics    = "GADGET_ELECTRONICS"
	CategoryBeautyHealth         = "BEAUTY_HEALTH"
	CategoryGeneralRetail        = "GENERAL_RETAIL"
	CategoryEntertainmentLeisure = "ENTERTAINMENT_LEISURE"
)

var categoryAPICodes = map[string]string{
	"F&B":        CategoryFoodBeverage,
	"Fashion":    CategoryFashionLifestyle,
	"Elektronik": CategoryGadgetElectronics,
	"Beauty":     CategoryBeautyHealth,
	"Retail":     CategoryGeneralRetail,
	"Leisure":    CategoryEntertainmentLeisure,
}

// CategoryAPICode maps a database category name to its API code.
// Unknown categories fall back to GENERAL_RETAIL.
func CategoryAPICode(dbName string) string {
	if code, ok := categoryAPICodes[dbName]; ok {
		return code
	}
	return CategoryGeneralRetail
}
