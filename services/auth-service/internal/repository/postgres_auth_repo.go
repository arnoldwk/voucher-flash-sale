package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"voucher-flash-sale/services/auth-service/internal/domain"
)

type PostgresAuthRepo struct {
	db *sql.DB
}

func NewPostgresAuthRepo(db *sql.DB) *PostgresAuthRepo {
	return &PostgresAuthRepo{db: db}
}

func (r *PostgresAuthRepo) CreateOTPChallenge(ctx context.Context, c *domain.OTPChallenge) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Deactivate any existing ACTIVE challenges for this phone
	expirePrev := `
		UPDATE otp_challenges
		SET status = 'EXPIRED', updated_at = $1
		WHERE phone = $2 AND status = 'ACTIVE'
	`
	_, err = tx.ExecContext(ctx, expirePrev, time.Now().UTC(), c.Phone)
	if err != nil {
		return err
	}

	insertQuery := `
		INSERT INTO otp_challenges (id, phone, otp_code, status, attempt_count, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err = tx.ExecContext(ctx, insertQuery,
		c.ID, c.Phone, c.OTPCode, c.Status, c.AttemptCount, c.ExpiresAt, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *PostgresAuthRepo) GetLatestActiveChallenge(ctx context.Context, phone string) (*domain.OTPChallenge, error) {
	query := `
		SELECT id, phone, otp_code, status, attempt_count, expires_at, created_at, updated_at
		FROM otp_challenges
		WHERE phone = $1 AND status = 'ACTIVE'
		ORDER BY created_at DESC
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, query, phone)

	c := &domain.OTPChallenge{}
	err := row.Scan(
		&c.ID, &c.Phone, &c.OTPCode, &c.Status, &c.AttemptCount, &c.ExpiresAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrChallengeEmpty
		}
		return nil, err
	}

	return c, nil
}

func (r *PostgresAuthRepo) UpdateChallengeStatus(ctx context.Context, id, status string) error {
	query := `
		UPDATE otp_challenges
		SET status = $1, updated_at = $2
		WHERE id = $3
	`
	_, err := r.db.ExecContext(ctx, query, status, time.Now().UTC(), id)
	return err
}

func (r *PostgresAuthRepo) IncrementChallengeAttempt(ctx context.Context, id string) (int, error) {
	query := `
		UPDATE otp_challenges
		SET attempt_count = attempt_count + 1, updated_at = $1
		WHERE id = $2
		RETURNING attempt_count
	`
	var newAttempts int
	err := r.db.QueryRowContext(ctx, query, time.Now().UTC(), id).Scan(&newAttempts)
	return newAttempts, err
}

func (r *PostgresAuthRepo) CreateSession(ctx context.Context, session *domain.AuthSession) error {
	query := `
		INSERT INTO auth_sessions (id, customer_id, refresh_token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.ExecContext(ctx, query,
		session.ID, session.CustomerID, session.RefreshToken, session.ExpiresAt, session.CreatedAt,
	)
	return err
}
