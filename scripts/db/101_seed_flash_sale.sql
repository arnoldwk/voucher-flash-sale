-- =====================================================================
-- 101_seed_flash_sale.sql
-- DATA operasional: sesi flash sale, pool stock, offer (campaign)
-- =====================================================================

-- ---------------------------------------------------------------------
-- Sesi flash sale November 2026 (4 sesi mingguan)
--   Sesi 1 = SEDANG BERJALAN  (NOW-2h s/d NOW+4h)
--   Sesi 2 = AKAN DIJUAL      (NOW+7d)
--   Sesi 3, 4 = terjadwal
-- ---------------------------------------------------------------------
INSERT INTO flash_sale_events (
    id, name, period_start, period_end, sequence, event_count,
    start_time, end_time, status
) VALUES
    ('evt-nov-w1', 'Flash Sale November 2026 - Minggu 1',
     '2026-11-01', '2026-11-30', 1, 4,
     NOW() - INTERVAL '2 hour', NOW() + INTERVAL '4 hour', 'ACTIVE'),

    ('evt-nov-w2', 'Flash Sale November 2026 - Minggu 2',
     '2026-11-01', '2026-11-30', 2, 4,
     NOW() + INTERVAL '7 day',  NOW() + INTERVAL '9 day',  'ACTIVE'),

    ('evt-nov-w3', 'Flash Sale November 2026 - Minggu 3',
     '2026-11-01', '2026-11-30', 3, 4,
     NOW() + INTERVAL '14 day', NOW() + INTERVAL '16 day', 'ACTIVE'),

    ('evt-nov-w4', 'Flash Sale November 2026 - Minggu 4',
     '2026-11-01', '2026-11-30', 4, 4,
     NOW() + INTERVAL '21 day', NOW() + INTERVAL '23 day', 'ACTIVE')
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------
-- Pool stock per kategori per periode (November 2026)
--   session_stock = total_stock / event_count (4)
-- ---------------------------------------------------------------------
INSERT INTO category_stock_pools (id, category_id, period_start, period_end, total_stock) VALUES
    ('pool-nov-fn-b',       'cat-fn-b',       '2026-11-01', '2026-11-30', 1200),  -- /4 = 300
    ('pool-nov-fashion',     'cat-fashion',    '2026-11-01', '2026-11-30', 1000),  -- /4 = 250
    ('pool-nov-elektronik', 'cat-elektronik', '2026-11-01', '2026-11-30',  400),  -- /4 = 100
    ('pool-nov-kbeauty',   'cat-kbeauty',    '2026-11-01', '2026-11-30',  800),  -- /4 = 200
    ('pool-nov-retail',     'cat-retail',     '2026-11-01', '2026-11-30', 1500),  -- /4 = 375
    ('pool-nov-leisure',    'cat-leisure',    '2026-11-01', '2026-11-30',  600)   -- /4 = 150
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------
-- OFFER (campaign) -- sesi 1: LIVE, 5 kategori -> "5 Penawaran Aktif"
--   session_stock   = pool / 4
--   claimed_count   = sudah diklaim pada sesi ini
--   remaining_stock = session_stock - claimed_count
-- ---------------------------------------------------------------------
INSERT INTO campaigns (
    id, product_id, flash_sale_event_id, category_id,
    title, description,
    session_stock, claimed_count, remaining_stock, discount_amount,
    start_time, end_time, status
) VALUES
    -- 1 Kopi Kenangan / F&B -- 1200/4 = 300
    ('cmp-kenangan-live', 'prd-kenangan-toast', 'evt-nov-w1', 'cat-fn-b',
     'Kopi Kenangan', 'Kenangan Mantan & Toast',
     300, 0, 300, 45000,
     NOW() - INTERVAL '2 hour', NOW() + INTERVAL '4 hour', 'ACTIVE'),

    -- 2 Zara / MAP / Fashion -- 1000/4 = 250
    ('cmp-zara-live', 'prd-zara-all', 'evt-nov-w1', 'cat-fashion',
     'Zara / MAP', 'Semua Outlet & App',
     250, 0, 250, 45000,
     NOW() - INTERVAL '2 hour', NOW() + INTERVAL '4 hour', 'ACTIVE'),

    -- 3 Tokopedia Gadget / Elektronik -- 400/4 = 100, sudah 77 terjual -> sisa 23 (mockup)
    ('cmp-tokopedia-live', 'prd-tokopedia-smartphone', 'evt-nov-w1', 'cat-elektronik',
     'Tokopedia Gadget', 'Voucher Smartphone Official',
     100, 77, 23, 45000,
     NOW() - INTERVAL '2 hour', NOW() + INTERVAL '4 hour', 'ACTIVE'),

    -- 4 Wardrobe Beauty / Beauty -- 800/4 = 200
    ('cmp-wardrobe-live', 'prd-wardrobe-skin', 'evt-nov-w1', 'cat-kbeauty',
     'Wardrobe Beauty', 'Voucher Perawatan Skin',
     200, 0, 200, 55000,
     NOW() - INTERVAL '2 hour', NOW() + INTERVAL '4 hour', 'ACTIVE'),

    -- 5 CGV / Leisure -- 600/4 = 150
    ('cmp-cgv-live', 'prd-cgv-movie', 'evt-nov-w1', 'cat-leisure',
     'CGV', 'Voucher Nonton Film 2D',
     150, 0, 150, 50000,
     NOW() - INTERVAL '2 hour', NOW() + INTERVAL '4 hour', 'ACTIVE'),

    -- ================= SESI 2 -- AKAN DIJUAL =================
    ('cmp-starbucks-next', 'prd-starbucks', 'evt-nov-w2', 'cat-fn-b',
     'Starbucks', 'Free Upgrade Handcrafted Beverage',
     300, 0, 300, 30000,
     NOW() + INTERVAL '7 day', NOW() + INTERVAL '9 day', 'SCHEDULED'),

    ('cmp-hm-next', 'prd-hm-fashion', 'evt-nov-w2', 'cat-fashion',
     'H&M', 'Voucher Belanja Minimum Rp300.000',
     250, 0, 250, 70000,
     NOW() + INTERVAL '7 day', NOW() + INTERVAL '9 day', 'SCHEDULED'),

    ('cmp-blessing-next', 'prd-blessing-retail', 'evt-nov-w2', 'cat-retail',
     'Blessing', 'Voucher Belanja Minimum Rp500.000',
     375, 0, 375, 95000,
     NOW() + INTERVAL '7 day', NOW() + INTERVAL '9 day', 'SCHEDULED')
ON CONFLICT (id) DO NOTHING;
