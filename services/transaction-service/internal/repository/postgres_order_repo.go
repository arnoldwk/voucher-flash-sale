package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"voucher-flash-sale/services/transaction-service/internal/domain"
)

type PostgresOrderRepo struct {
	db *sql.DB
}

func NewPostgresOrderRepo(db *sql.DB) *PostgresOrderRepo {
	return &PostgresOrderRepo{db: db}
}

// FindVoucherProduct resolves the voucher product and its campaign/session
// metadata so the usecase can decide whether a reservation is allowed.
func (r *PostgresOrderRepo) FindVoucherProduct(ctx context.Context, voucherProductID string) (*domain.ProductRead, error) {
	const query = `
        SELECT
            vp.id,
            c.id,
            c.flash_sale_event_id,
            cat.id,
            cat.name,
            p.name,
            vp.price,
            c.remaining_stock,
            c.start_time,
            c.end_time,
            c.status,
            q.id
        FROM voucher_products vp
        JOIN campaigns c   ON c.id = vp.campaign_id
        JOIN products p    ON p.id = vp.product_id
        JOIN categories cat ON cat.id = p.category_id
        LEFT JOIN session_category_quotas q
          ON q.session_id = c.flash_sale_event_id
         AND q.voucher_product_id = vp.id
        WHERE vp.id = $1
          AND c.status = 'ACTIVE'
          AND c.end_time > NOW()
          AND c.start_time <= NOW()
    `
	row := r.db.QueryRowContext(ctx, query, voucherProductID)

	p := &domain.ProductRead{}
	var sessionStatus string
	err := row.Scan(
		&p.ID,
		&p.CampaignID,
		&p.SessionID,
		&p.CategoryID,
		&p.CategoryName,
		&p.Name,
		&p.PriceIDR,
		&p.RemainingStock,
		&p.StartTime,
		&p.EndTime,
		&sessionStatus,
		&p.QuotaID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrVoucherProductMissing
		}
		return nil, err
	}
	p.SessionStatus = sessionStatus
	return p, nil
}

// FindByIdempotencyKey returns an existing order for the given idempotency
// key, if any. Used by the usecase to dedupe retried reservation requests.
func (r *PostgresOrderRepo) FindByIdempotencyKey(ctx context.Context, key string) (*domain.Order, error) {
	const query = `
        SELECT id, customer_id, voucher_product_id, session_id, category,
               price_amount, currency, status, reservation_deadline,
               idempotency_key, version, created_at, updated_at
        FROM orders
        WHERE idempotency_key = $1
    `
	row := r.db.QueryRowContext(ctx, query, key)
	o := &domain.Order{}
	var statusStr string
	err := row.Scan(
		&o.ID,
		&o.CustomerID,
		&o.VoucherProductID,
		&o.SessionID,
		&o.Category,
		&o.PriceAmount,
		&o.Currency,
		&statusStr,
		&o.ReservationDeadline,
		&o.IdempotencyKey,
		&o.Version,
		&o.CreatedAt,
		&o.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	o.Status = domain.OrderStatus(statusStr)
	return o, nil
}

// InsertOrderAndEntitlement atomically:
//  1. Locks the quota row FOR UPDATE and verifies reserved_count < quota.
//  2. Bumps reserved_count and version.
//  3. Inserts the order (PENDING) with its reservation deadline.
//  4. Inserts the customer entitlement linked to the order.
//
// The caller is responsible for rolling back Redis if this transaction fails.
func (r *PostgresOrderRepo) InsertOrderAndEntitlement(ctx context.Context, order *domain.Order, quotaID string, entitlement *domain.CustomerEntitlement) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Lock quota row to serialize concurrent reservations against the same quota.
	var reservedCount, quota int
	var version int64
	checkQuery := `
        SELECT reserved_count, quota, version
        FROM session_category_quotas
        WHERE id = $1
        FOR UPDATE
    `
	err = tx.QueryRowContext(ctx, checkQuery, quotaID).Scan(&reservedCount, &quota, &version)
	if err != nil {
		return err
	}
	if reservedCount >= quota {
		return domain.ErrOutOfStock
	}

	// Bump reserved counter optimistically using version.
	bumpQuery := `
        UPDATE session_category_quotas
        SET reserved_count = reserved_count + 1,
            version = version + 1,
            updated_at = NOW()
        WHERE id = $1 AND version = $2
    `
	result, err := tx.ExecContext(ctx, bumpQuery, quotaID, version)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return domain.ErrOutOfStock
	}

	// Insert order.
	insertOrder := `
        INSERT INTO orders (
            id, customer_id, voucher_product_id, session_id, category,
            price_amount, currency, status, reservation_deadline,
            idempotency_key, version, created_at, updated_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
    `
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, insertOrder,
		order.ID,
		order.CustomerID,
		order.VoucherProductID,
		order.SessionID,
		order.Category,
		order.PriceAmount,
		order.Currency,
		string(domain.OrderPending),
		order.ReservationDeadline,
		order.IdempotencyKey,
		order.Version,
		now,
		now,
	)
	if err != nil {
		return err
	}

	// Insert entitlement linking customer -> active order.
	insertEnt := `
        INSERT INTO customer_entitlements (
            id, customer_id, session_id, category, campaign_id,
            active_order_id, status, version, created_at, updated_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    `
	_, err = tx.ExecContext(ctx, insertEnt,
		entitlement.ID,
		entitlement.CustomerID,
		entitlement.SessionID,
		entitlement.Category,
		entitlement.CampaignID,
		order.ID,
		string(domain.EntitlementActive),
		entitlement.Version,
		now,
		now,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ExpirePendingOrders marks PENDING orders whose reservation_deadline has
// passed as EXPIRED, decrements reserved_count, and releases entitlements.
// Returns the list of expired order IDs.
func (r *PostgresOrderRepo) ExpirePendingOrders(ctx context.Context) ([]string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Grab expired pending orders to process.
	selectQuery := `
        SELECT id, voucher_product_id, session_id, category
        FROM orders
        WHERE status = $1 AND reservation_deadline <= NOW()
        ORDER BY reservation_deadline ASC
        LIMIT 200 FOR UPDATE SKIP LOCKED
    `
	rows, err := tx.QueryContext(ctx, selectQuery, string(domain.OrderPending))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expired []string
	for rows.Next() {
		var orderID, vpID, sessionID, category string
		if err := rows.Scan(&orderID, &vpID, &sessionID, &category); err != nil {
			return nil, err
		}
		expired = append(expired, orderID)

		// Release entitlement linked to this order.
		_, _ = tx.ExecContext(ctx, `
            UPDATE customer_entitlements
            SET status = $1, updated_at = NOW()
            WHERE active_order_id = $2 AND status = $3
        `, string(domain.EntitlementReleased), orderID, string(domain.EntitlementActive))

		// Decrement reserved_count for the quota of this product/session/category.
		_, _ = tx.ExecContext(ctx, `
            UPDATE session_category_quotas
            SET reserved_count = GREATEST(reserved_count - 1, 0),
                updated_at = NOW()
            WHERE voucher_product_id = $1 AND session_id = $2 AND category = $3
        `, vpID, sessionID, category)
	}

	// Mark the orders as expired.
	if len(expired) > 0 {
		_, _ = tx.ExecContext(ctx, `
            UPDATE orders
            SET status = $1, updated_at = NOW()
            WHERE status = $2 AND reservation_deadline <= NOW()
        `, string(domain.OrderExpired), string(domain.OrderPending))
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return expired, nil
}
