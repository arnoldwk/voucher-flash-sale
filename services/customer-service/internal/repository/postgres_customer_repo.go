package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"voucher-flash-sale/services/customer-service/internal/domain"
)

type PostgresCustomerRepo struct {
	db *sql.DB
}

func NewPostgresCustomerRepo(db *sql.DB) *PostgresCustomerRepo {
	return &PostgresCustomerRepo{db: db}
}

func (r *PostgresCustomerRepo) GetByPhone(ctx context.Context, phone string) (*domain.Customer, error) {
	query := `
		SELECT id, phone, is_verified, phone_verified_at, created_at, updated_at
		FROM customers
		WHERE phone = $1
	`
	row := r.db.QueryRowContext(ctx, query, phone)

	c := &domain.Customer{}
	err := row.Scan(
		&c.ID,
		&c.Phone,
		&c.IsVerified,
		&c.PhoneVerifiedAt,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrCustomerNotFound
		}
		return nil, err
	}

	return c, nil
}

func (r *PostgresCustomerRepo) GetByID(ctx context.Context, id string) (*domain.Customer, error) {
	query := `
		SELECT id, phone, is_verified, phone_verified_at, created_at, updated_at
		FROM customers
		WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)

	c := &domain.Customer{}
	err := row.Scan(
		&c.ID,
		&c.Phone,
		&c.IsVerified,
		&c.PhoneVerifiedAt,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrCustomerNotFound
		}
		return nil, err
	}

	return c, nil
}

func (r *PostgresCustomerRepo) Create(ctx context.Context, c *domain.Customer) error {
	query := `
		INSERT INTO customers (id, phone, is_verified, phone_verified_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(
		ctx,
		query,
		c.ID,
		c.Phone,
		c.IsVerified,
		c.PhoneVerifiedAt,
		c.CreatedAt,
		c.UpdatedAt,
	)
	return err
}

func (r *PostgresCustomerRepo) VerifyPhone(ctx context.Context, id string, verifiedAt time.Time) (*domain.Customer, error) {
	query := `
		UPDATE customers
		SET is_verified = TRUE, phone_verified_at = $1, updated_at = $2
		WHERE id = $3
		RETURNING id, phone, is_verified, phone_verified_at, created_at, updated_at
	`
	now := time.Now().UTC()
	row := r.db.QueryRowContext(ctx, query, verifiedAt, now, id)

	c := &domain.Customer{}
	err := row.Scan(
		&c.ID,
		&c.Phone,
		&c.IsVerified,
		&c.PhoneVerifiedAt,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrCustomerNotFound
		}
		return nil, err
	}

	return c, nil
}
