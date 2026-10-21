package domain

import (
	"context"
	"errors"
	"math"
	"time"
)

var (
	ErrCustomerNotFound = errors.New("customer not identified")
	ErrInvalidPhone     = errors.New("invalid phone number")
)

// StockStatus represents the availability of a voucher product.
type StockStatus string

const (
	StockAvailable StockStatus = "AVAILABLE"
	StockSoldOut   StockStatus = "SOLD_OUT"
)

// CustomerState represents the eligibility of a customer for a voucher product.
type CustomerState string

const (
	CustomerEligible   CustomerState = "ELIGIBLE"
	CustomerClaimed    CustomerState = "CLAIMED"
	CustomerNotStarted CustomerState = "NOT_STARTED"
	CustomerEnded      CustomerState = "ENDED"
)

// Category API codes exposed to clients.
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

// MoneyToCents converts an IDR amount to cents (1 IDR = 100 cents).
// Values are floored to avoid over-reporting.
func MoneyToCents(idr float64) int64 {
	if idr <= 0 {
		return 0
	}
	return int64(math.Floor(idr * 100))
}

// Money holds a monetary amount with its currency.
type Money struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// NewIDRMoney creates a Money value in IDR from a Rp amount.
func NewIDRMoney(idr float64) Money {
	return Money{Amount: MoneyToCents(idr), Currency: "IDR"}
}

// CatalogItem is the internal model returned by the repository.
// It includes non-API fields (campaign IDs, session timing) needed for
// computing eligibility and stock status.
type CatalogItem struct {
	VoucherProductID  string
	CampaignID        string
	EventID           string
	CategoryID        string
	CategoryName      string
	Name              string
	DiscountValueIDR  float64
	PriceIDR          float64
	MinTransactionIDR float64
	RemainingStock    int
	SessionStart      time.Time
	SessionEnd        time.Time
	SessionStatus     string // ACTIVE, SCHEDULED, ENDED
}

// ToVoucherProduct converts a CatalogItem to the API DTO.
func (c *CatalogItem) ToVoucherProduct(isClaimed bool) VoucherProduct {
	stockStatus := StockAvailable
	if c.RemainingStock <= 0 {
		stockStatus = StockSoldOut
	}

	state := CustomerEligible
	if isClaimed {
		state = CustomerClaimed
	}

	return VoucherProduct{
		VoucherProductID: c.VoucherProductID,
		Category:         CategoryAPICode(c.CategoryName),
		Name:             c.Name,
		DiscountValue:    NewIDRMoney(c.DiscountValueIDR),
		Price:            NewIDRMoney(c.PriceIDR),
		MinTransaction:   NewIDRMoney(c.MinTransactionIDR),
		StockStatus:      stockStatus,
		CustomerState:    state,
	}
}

// VoucherProduct is the API response DTO for the voucher catalog.
type VoucherProduct struct {
	VoucherProductID string        `json:"voucherProductId"`
	Category         string        `json:"category"`
	Name             string        `json:"name"`
	DiscountValue    Money         `json:"discountValue"`
	Price            Money         `json:"price"`
	MinTransaction   Money         `json:"minTransaction"`
	StockStatus      StockStatus   `json:"stockStatus"`
	CustomerState    CustomerState `json:"customerState"`
}

// ProductCatalogRepository reads the voucher catalog data.
type ProductCatalogRepository interface {
	// ListVoucherProducts returns catalog items for live and upcoming sessions.
	ListVoucherProducts(ctx context.Context, now time.Time) ([]*CatalogItem, error)
	// ClaimedCategoriesByEvent returns a map of eventID -> set of categoryIDs
	// already claimed by the given customer.
	ClaimedCategoriesByEvent(ctx context.Context, customerID string) (map[string]map[string]bool, error)
}
