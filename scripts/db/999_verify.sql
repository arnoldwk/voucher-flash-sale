-- =====================================================================
-- 999_verify.sql -- validasi data setelah seed
-- =====================================================================

\echo '=== 1. Master data counts ==='
SELECT 'merchants'    AS entity, COUNT(*) AS count FROM merchants
UNION ALL SELECT 'categories',    COUNT(*) FROM categories
UNION ALL SELECT 'products',      COUNT(*) FROM products
UNION ALL SELECT 'events',        COUNT(*) FROM flash_sale_events
UNION ALL SELECT 'stock_pools',  COUNT(*) FROM category_stock_pools
UNION ALL SELECT 'offers',        COUNT(*) FROM campaigns;

\echo ''
\echo '=== 2. Invariant: claimed_count <= session_stock ==='
SELECT id, session_stock, claimed_count, remaining_stock
FROM campaigns
WHERE claimed_count > session_stock;
-- harus kosong

\echo ''
\echo '=== 3. Invariant: remaining = session_stock - claimed_count ==='
SELECT id, session_stock, claimed_count, remaining_stock
FROM campaigns
WHERE remaining_stock <> session_stock - claimed_count;
-- harus kosong

\echo ''
\echo '=== 4. Alokasi stock sesi (pool / event_count) ==='
SELECT
    c.id,
    cat.name  AS kategori,
    p.total_stock,
    e.event_count,
    (p.total_stock / e.event_count)         AS expected,
    (p.total_stock % e.event_count)        AS remainder,
    c.session_stock
FROM campaigns c
JOIN flash_sale_events e       ON e.id = c.flash_sale_event_id
JOIN category_stock_pools p   ON p.category_id = c.category_id
                              AND p.period_start = e.period_start
                              AND p.period_end   = e.period_end
JOIN categories cat            ON cat.id = c.category_id
ORDER BY e.sequence, cat.name;

\echo ''
\echo '=== 5. Penawaran aktif (status ACTIVE + dalam window) ==='
SELECT COUNT(*) AS active_offer_count
FROM campaigns
WHERE status = 'ACTIVE' AND start_time <= NOW() AND end_time > NOW();

\echo ''
\echo '=== 6. Detail offer sesi 1 ==='
SELECT
    c.id                          AS campaign_id,
    cat.name                      AS kategori,
    p.name                        AS produk,
    c.session_stock,
    c.claimed_count,
    c.remaining_stock            AS stock_tersisa,
    (c.session_stock - c.claimed_count) AS available_slots
FROM campaigns c
JOIN categories cat  ON cat.id = c.category_id
JOIN products p      ON p.id = c.product_id
WHERE c.flash_sale_event_id = 'evt-nov-w1'
ORDER BY cat.name;

\echo ''
\echo '=== 7. Katalog lengkap (join master data) ==='
SELECT
    c.id                         AS campaign_id,
    e.sequence                   AS sesi,
    e.name                       AS event_name,
    m.name                       AS merchant,
    cat.name                     AS kategori,
    p.name                       AS produk,
    p.promo_value                AS nilai_promo,
    p.discount_amount            AS hemat,
    p.pay_amount                 AS bayar,
    p.terms,
    c.session_stock,
    c.remaining_stock
FROM campaigns c
JOIN flash_sale_events e  ON e.id = c.flash_sale_event_id
JOIN products p           ON p.id = c.product_id
JOIN merchants m          ON m.id = p.merchant_id
JOIN categories cat       ON cat.id = p.category_id
ORDER BY e.sequence, cat.name;

\echo ''
\echo '=== 8. Product per merchant ==='
SELECT
    m.name   AS merchant,
    COUNT(p.id) AS jumlah_produk,
    STRING_AGG(p.name, ', ' ORDER BY p.name) AS produk_list
FROM merchants m
LEFT JOIN products p ON p.merchant_id = m.id
GROUP BY m.id, m.name
ORDER BY COUNT(p.id) DESC;
