package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"voucher-flash-sale/services/voucher-service/internal/domain"
)

type PostgresVoucherRepo struct {
	db *sql.DB
}

func NewPostgresVoucherRepo(db *sql.DB) *PostgresVoucherRepo {
	return &PostgresVoucherRepo{db: db}
}

func (r *PostgresVoucherRepo) SaveClaimedVoucher(ctx context.Context, v *domain.Voucher) error {
	query := `
		INSERT INTO vouchers (id, campaign_id, user_id, voucher_code, discount_amount, status, claimed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (campaign_id, user_id) DO NOTHING
	`
	_, err := r.db.ExecContext(
		ctx,
		query,
		v.ID,
		v.CampaignID,
		v.UserID,
		v.VoucherCode,
		v.DiscountAmount,
		string(v.Status),
		v.ClaimedAt,
		v.CreatedAt,
		v.UpdatedAt,
	)
	return err
}

func (r *PostgresVoucherRepo) GetByCode(ctx context.Context, code string) (*domain.Voucher, error) {
	query := `
		SELECT id, campaign_id, user_id, voucher_code, discount_amount, status, claimed_at, redeemed_at, created_at, updated_at
		FROM vouchers
		WHERE voucher_code = $1
	`
	row := r.db.QueryRowContext(ctx, query, code)

	v := &domain.Voucher{}
	var statusStr string
	err := row.Scan(
		&v.ID,
		&v.CampaignID,
		&v.UserID,
		&v.VoucherCode,
		&v.DiscountAmount,
		&statusStr,
		&v.ClaimedAt,
		&v.RedeemedAt,
		&v.CreatedAt,
		&v.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrVoucherNotFound
		}
		return nil, err
	}
	v.Status = domain.VoucherStatus(statusStr)
	return v, nil
}

func (r *PostgresVoucherRepo) ListByUserID(ctx context.Context, userID string) ([]*domain.Voucher, error) {
	query := `
		SELECT id, campaign_id, user_id, voucher_code, discount_amount, status, claimed_at, redeemed_at, created_at, updated_at
		FROM vouchers
		WHERE user_id = $1
		ORDER BY claimed_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.Voucher
	for rows.Next() {
		v := &domain.Voucher{}
		var statusStr string
		err := rows.Scan(
			&v.ID,
			&v.CampaignID,
			&v.UserID,
			&v.VoucherCode,
			&v.DiscountAmount,
			&statusStr,
			&v.ClaimedAt,
			&v.RedeemedAt,
			&v.CreatedAt,
			&v.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		v.Status = domain.VoucherStatus(statusStr)
		list = append(list, v)
	}

	return list, rows.Err()
}

func (r *PostgresVoucherRepo) Redeem(ctx context.Context, code string, redeemedAt time.Time) (*domain.Voucher, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Select FOR UPDATE
	var currentStatus string
	checkQuery := `SELECT status FROM vouchers WHERE voucher_code = $1 FOR UPDATE`
	err = tx.QueryRowContext(ctx, checkQuery, code).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrVoucherNotFound
		}
		return nil, err
	}

	if currentStatus == string(domain.StatusRedeemed) {
		return nil, domain.ErrVoucherAlreadyRedeemed
	}

	updateQuery := `
		UPDATE vouchers
		SET status = $1, redeemed_at = $2, updated_at = $3
		WHERE voucher_code = $4
		RETURNING id, campaign_id, user_id, voucher_code, discount_amount, status, claimed_at, redeemed_at, created_at, updated_at
	`
	row := tx.QueryRowContext(ctx, updateQuery, string(domain.StatusRedeemed), redeemedAt, redeemedAt, code)

	v := &domain.Voucher{}
	var statusStr string
	err = row.Scan(
		&v.ID,
		&v.CampaignID,
		&v.UserID,
		&v.VoucherCode,
		&v.DiscountAmount,
		&statusStr,
		&v.ClaimedAt,
		&v.RedeemedAt,
		&v.CreatedAt,
		&v.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	v.Status = domain.VoucherStatus(statusStr)

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return v, nil
}
