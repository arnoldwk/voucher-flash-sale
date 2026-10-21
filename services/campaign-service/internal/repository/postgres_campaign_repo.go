package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"voucher-flash-sale/services/campaign-service/internal/domain"
)

type PostgresCampaignRepo struct {
	db *sql.DB
}

func NewPostgresCampaignRepo(db *sql.DB) *PostgresCampaignRepo {
	return &PostgresCampaignRepo{db: db}
}

func (r *PostgresCampaignRepo) Create(ctx context.Context, c *domain.Campaign) error {
	query := `
		INSERT INTO campaigns (
			id, title, description,
			session_stock, claimed_count, remaining_stock, discount_amount,
			start_time, end_time, status, created_at, updated_at,
			product_id, flash_sale_event_id, category_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`
	_, err := r.db.ExecContext(
		ctx,
		query,
		c.ID,
		c.Title,
		c.Description,
		c.SessionStock,
		c.ClaimedCount,
		c.RemainingStock,
		c.DiscountAmount,
		c.StartTime,
		c.EndTime,
		c.Status,
		c.CreatedAt,
		c.UpdatedAt,
		nullIfEmpty(c.ProductID),
		nullIfEmpty(c.FlashSaleEventID),
		nullIfEmpty(c.CategoryID),
	)
	return err
}

func (r *PostgresCampaignRepo) GetByID(ctx context.Context, id string) (*domain.Campaign, error) {
	query := `
		SELECT id, title, description,
		       session_stock, claimed_count, remaining_stock, discount_amount,
		       start_time, end_time, status, created_at, updated_at,
		       product_id, flash_sale_event_id, category_id
		FROM campaigns
		WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)

	c := &domain.Campaign{}
	var productID, eventID, categoryID sql.NullString
	err := row.Scan(
		&c.ID,
		&c.Title,
		&c.Description,
		&c.SessionStock,
		&c.ClaimedCount,
		&c.RemainingStock,
		&c.DiscountAmount,
		&c.StartTime,
		&c.EndTime,
		&c.Status,
		&c.CreatedAt,
		&c.UpdatedAt,
		&productID,
		&eventID,
		&categoryID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrCampaignNotFound
		}
		return nil, err
	}

	c.ProductID = productID.String
	c.FlashSaleEventID = eventID.String
	c.CategoryID = categoryID.String
	return c, nil
}

func (r *PostgresCampaignRepo) List(ctx context.Context) ([]*domain.Campaign, error) {
	query := `
		SELECT id, title, description,
		       session_stock, claimed_count, remaining_stock, discount_amount,
		       start_time, end_time, status, created_at, updated_at,
		       product_id, flash_sale_event_id, category_id
		FROM campaigns
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.Campaign
	for rows.Next() {
		c := &domain.Campaign{}
		var productID, eventID, categoryID sql.NullString
		err := rows.Scan(
			&c.ID,
			&c.Title,
			&c.Description,
			&c.SessionStock,
			&c.ClaimedCount,
			&c.RemainingStock,
			&c.DiscountAmount,
			&c.StartTime,
			&c.EndTime,
			&c.Status,
			&c.CreatedAt,
			&c.UpdatedAt,
			&productID,
			&eventID,
			&categoryID,
		)
		if err != nil {
			return nil, err
		}
		c.ProductID = productID.String
		c.FlashSaleEventID = eventID.String
		c.CategoryID = categoryID.String
		list = append(list, c)
	}

	return list, rows.Err()
}

func (r *PostgresCampaignRepo) UpdateRemainingStock(ctx context.Context, id string, remainingStock int) error {
	query := `
		UPDATE campaigns
		SET remaining_stock = $1, updated_at = $2
		WHERE id = $3
	`
	_, err := r.db.ExecContext(ctx, query, remainingStock, time.Now(), id)
	return err
}

// nullIfEmpty mengembalikan nil untuk string kosong agar kolom FK nullable
// menerima NULL, bukan string kosong yang tidak valid sebagai FK.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
