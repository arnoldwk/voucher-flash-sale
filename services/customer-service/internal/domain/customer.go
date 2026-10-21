package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrCustomerNotFound = errors.New("customer not found")
	ErrInvalidPhone     = errors.New("invalid phone number")
)

type Customer struct {
	ID              string     `json:"customerId"`
	Phone           string     `json:"phone"`
	IsVerified      bool       `json:"isVerified"`
	PhoneVerifiedAt *time.Time `json:"phoneVerifiedAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type CustomerRepository interface {
	GetByPhone(ctx context.Context, phone string) (*Customer, error)
	GetByID(ctx context.Context, id string) (*Customer, error)
	Create(ctx context.Context, c *Customer) error
	VerifyPhone(ctx context.Context, id string, verifiedAt time.Time) (*Customer, error)
}
