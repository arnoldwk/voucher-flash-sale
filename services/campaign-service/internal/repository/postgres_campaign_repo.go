package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"voucher-flash-sale/services/campaign-service/internal/domain"

	"github.com/redis/go-redis/v9"
)

type PostgresCampaignRepo struct {
	db  *sql.DB
	rdb *redis.Client
}

func NewPostgresCampaignRepo(db *sql.DB, rdb *redis.Client) *PostgresCampaignRepo {
	return &PostgresCampaignRepo{
		db:  db,
		rdb: rdb,
	}
}

func (r *PostgresCampaignRepo) Create(ctx context.Context, c *domain.Campaign) error {
	query := `
		INSERT INTO campaigns (id, title, description, total_stock, remaining_stock, discount_amount, start_time, end_time, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.ExecContext(
		ctx,
		query,
		c.ID,
		c.Title,
		c.Description,
		c.TotalStock,
		c.RemainingStock,
		c.DiscountAmount,
		c.StartTime,
		c.EndTime,
		c.Status,
		c.CreatedAt,
		c.UpdatedAt,
	)
	return err
}

func (r *PostgresCampaignRepo) GetByID(ctx context.Context, id string) (*domain.Campaign, error) {
	query := `
		SELECT id, title, description, total_stock, remaining_stock, discount_amount, start_time, end_time, status, created_at, updated_at
		FROM campaigns
		WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)

	c := &domain.Campaign{}
	err := row.Scan(
		&c.ID,
		&c.Title,
		&c.Description,
		&c.TotalStock,
		&c.RemainingStock,
		&c.DiscountAmount,
		&c.StartTime,
		&c.EndTime,
		&c.Status,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrCampaignNotFound
		}
		return nil, err
	}

	return c, nil
}

func (r *PostgresCampaignRepo) List(ctx context.Context) ([]*domain.Campaign, error) {
	query := `
		SELECT id, title, description, total_stock, remaining_stock, discount_amount, start_time, end_time, status, created_at, updated_at
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
		err := rows.Scan(
			&c.ID,
			&c.Title,
			&c.Description,
			&c.TotalStock,
			&c.RemainingStock,
			&c.DiscountAmount,
			&c.StartTime,
			&c.EndTime,
			&c.Status,
			&c.CreatedAt,
			&c.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
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

func (r *PostgresCampaignRepo) WarmupRedisStock(ctx context.Context, id string, stock int) error {
	stockKey := fmt.Sprintf("campaign:stock:%s", id)
	return r.rdb.Set(ctx, stockKey, stock, 0).Err()
}

func (r *PostgresCampaignRepo) GetRedisStock(ctx context.Context, id string) (int, error) {
	stockKey := fmt.Sprintf("campaign:stock:%s", id)
	val, err := r.rdb.Get(ctx, stockKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return -1, nil // not in redis yet
		}
		return 0, err
	}

	return strconv.Atoi(val)
}
