-- =====================================================================
-- 103_seed_quotas.sql
-- Bootstrap session_category_quota rows for every active voucher
-- product linked to an ACTIVE campaign. quota mirrors
-- campaigns.remaining_stock at bootstrap time so the hot path and the
-- durable counter start aligned.
-- =====================================================================

INSERT INTO session_category_quotas (
    session_id, voucher_product_id, category, quota, reserved_count,
    confirmed_count, status
)
SELECT
    c.flash_sale_event_id,
    vp.id,
    cat.name,
    c.remaining_stock,
    0,
    0,
    'OPEN'
FROM voucher_products vp
JOIN campaigns c ON c.id = vp.campaign_id
JOIN products p ON p.id = vp.product_id
JOIN categories cat ON cat.id = p.category_id
WHERE c.status = 'ACTIVE'
  AND c.end_time > NOW()
ON CONFLICT (session_id, voucher_product_id) DO NOTHING;
