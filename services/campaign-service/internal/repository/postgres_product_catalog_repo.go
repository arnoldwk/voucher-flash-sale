package repository

import (
	"context"
	"database/sql"
	"time"

	"voucher-flash-sale/services/campaign-service/internal/domain"
)

type PostgresProductCatalogRepo struct {
	db *sql.DB
}

func NewPostgresProductCatalogRepo(db *sql.DB) *PostgresProductCatalogRepo {
	return &PostgresProductCatalogRepo{db: db}
}

// catalogHorizon adalah jendela waktu ke depan untuk sesi yang masih akan
// ditampilkan sebagai "Akan Dijual".
const catalogHorizon = 7 * 24 * time.Hour

// ListVoucherProducts mengambil katalog voucher dari sesi yang sedang berjalan
// dan sesi-sesi berikutnya dalam horizon catalogHorizon.
//
// Sisa stock diambil dari campaigns.remaining_stock sebagai sumber kebenaran
// yang persisten; lapisan Redis menimpanya untuk stock yang sudah diklaim
// pada sesi berjalan.
func (r *PostgresProductCatalogRepo) ListVoucherProducts(ctx context.Context, now time.Time) ([]*domain.CatalogItem, error) {
	const query = `
        SELECT
            vp.id,
            c.id,
            c.flash_sale_event_id,
            cat.id,
            cat.name,
            p.name,
            p.discount_amount,
            vp.price,
            vp.min_transaction,
            c.remaining_stock,
            c.start_time,
            c.end_time,
            c.status
        FROM voucher_products vp
        JOIN campaigns c   ON c.id = vp.campaign_id
        JOIN products p    ON p.id = vp.product_id
        JOIN categories cat ON cat.id = p.category_id
        WHERE c.status <> 'DRAFT'
          AND c.end_time > $1
          AND c.start_time <= $2
        ORDER BY c.start_time ASC, cat.name ASC, vp.id ASC
    `

	rows, err := r.db.QueryContext(ctx, query, now, now.Add(catalogHorizon))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domain.CatalogItem
	for rows.Next() {
		item := &domain.CatalogItem{}
		err := rows.Scan(
			&item.VoucherProductID,
			&item.CampaignID,
			&item.EventID,
			&item.CategoryID,
			&item.CategoryName,
			&item.Name,
			&item.DiscountValueIDR,
			&item.PriceIDR,
			&item.MinTransactionIDR,
			&item.RemainingStock,
			&item.SessionStart,
			&item.SessionEnd,
			&item.SessionStatus,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ClaimedCategoriesByEvent mengembalikan kategori yang sudah diklaim customer
// per sesi flash sale. Kunci map adalah flash_sale_event_id.
func (r *PostgresProductCatalogRepo) ClaimedCategoriesByEvent(ctx context.Context, customerID string) (map[string]map[string]bool, error) {
	const query = `
        SELECT c.flash_sale_event_id, c.category_id
        FROM vouchers v
        JOIN campaigns c ON c.id = v.campaign_id
        WHERE v.user_id = $1
          AND v.status <> 'EXPIRED'
          AND c.flash_sale_event_id IS NOT NULL
    `

	rows, err := r.db.QueryContext(ctx, query, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]map[string]bool)
	for rows.Next() {
		var eventID, categoryID string
		if err := rows.Scan(&eventID, &categoryID); err != nil {
			return nil, err
		}
		if _, ok := result[eventID]; !ok {
			result[eventID] = make(map[string]bool)
		}
		result[eventID][categoryID] = true
	}
	return result, rows.Err()
}
