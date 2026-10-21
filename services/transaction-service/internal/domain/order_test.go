package domain

import (
	"testing"
	"time"
)

func TestMoneyToCents(t *testing.T) {
	tests := []struct {
		name string
		idr  float64
		want int64
	}{
		{"zero", 0, 0},
		{"negative", -5000, 0},
		{"exact", 10000, 1000000},
		{"fraction floored", 10000.99, 1000099},
		{"small", 0.01, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MoneyToCents(tt.idr); got != tt.want {
				t.Errorf("MoneyToCents(%v) = %d, want %d", tt.idr, got, tt.want)
			}
		})
	}
}

func TestNewIDRMoney(t *testing.T) {
	m := NewIDRMoney(45000)
	if m.Amount != 4500000 || m.Currency != "IDR" {
		t.Errorf("got %+v", m)
	}
}

func TestOrderStatusLifecycle(t *testing.T) {
	now := time.Now().UTC()
	order := &Order{
		ID:                  "ord_01HQX",
		CustomerID:          "628123456789",
		VoucherProductID:    "vp_fnb",
		SessionID:           "evt-nov-w1",
		Category:            "FOOD_BEVERAGE",
		PriceAmount:         500000,
		Currency:            "IDR",
		Status:              OrderPending,
		ReservationDeadline: now.Add(5 * time.Minute),
		Version:             0,
	}
	if order.Status != OrderPending {
		t.Errorf("expected PENDING, got %s", order.Status)
	}
	order.Status = OrderExpired
	if order.Status != OrderExpired {
		t.Errorf("expected EXPIRED, got %s", order.Status)
	}
}

func TestOrderViewJSON(t *testing.T) {
	view := OrderView{
		OrderID:             "ord_01HQX",
		Status:              string(OrderPending),
		Price:               Money{Amount: 500000, Currency: "IDR"},
		ReservationDeadline: time.Date(2026, 10, 24, 5, 5, 0, 0, time.UTC),
	}
	if view.OrderID != "ord_01HQX" {
		t.Errorf("bad order id: %s", view.OrderID)
	}
	if view.Price.Amount != 500000 {
		t.Errorf("bad price: %d", view.Price.Amount)
	}
}

func TestDomainErrors(t *testing.T) {
	if ErrOutOfStock.Error() != "quota exhausted for this session" {
		t.Error("ErrOutOfStock message mismatch")
	}
	if ErrAlreadyEntitled.Error() != "customer already has an active entitlement for this category" {
		t.Error("ErrAlreadyEntitled message mismatch")
	}
}
