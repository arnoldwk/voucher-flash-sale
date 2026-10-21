-- =====================================================================
-- 102_seed_voucher_products.sql
-- Seed voucher_products untuk API /product/v1/vouchers
-- Pattern: id = "vp_" + <category_slug>
-- =====================================================================

INSERT INTO voucher_products (
    id, campaign_id, product_id, price, min_transaction
) VALUES
    -- ===== SESI 1: LIVE (evt-nov-w1) =====

    -- F&B: 5rb payment, min tx 100rb
    ('vp_fnb',        'cmp-kenangan-live', 'prd-kenangan-toast', 5000,  100000),
    ('vp_fnb_starbucks','cmp-starbucks-next','prd-starbucks',       5000,  75000),
    ('vp_fnb_momo',   'cmp-starbucks-next','prd-momo-food',        5000,  50000),

    -- Fashion: 5rb payment, min tx 400rb
    ('vp_fashion',    'cmp-zara-live',    'prd-zara-all',          5000,  400000),
    ('vp_fashion_hm', 'cmp-hm-next',     'prd-hm-fashion',        5000,  300000),
    ('vp_fashion_uniqlo','cmp-hm-next',  'prd-uniqlo-basic',      5000,  350000),

    -- Elektronik: 5rb payment, min tx 200rb
    ('vp_gadget',    'cmp-tokopedia-live','prd-tokopedia-smartphone', 5000, 200000),
    ('vp_gadget_apple','cmp-tokopedia-live','prd-iphone-tradein',      5000, 500000),

    -- Beauty: 5rb payment, min tx 150rb
    ('vp_beauty',    'cmp-wardrobe-live','prd-wardrobe-skin',       5000, 150000),

    -- Retail: 5rb payment, min tx 300rb
    ('vp_retail',    'cmp-cgv-live',     'prd-cgv-movie',          5000, 300000),
    ('vp_retail_blessing','cmp-blessing-next','prd-blessing-retail',  5000, 500000),
    ('vp_retail_toped','cmp-blessing-next','prd-tokopedia-retail',   5000, 200000)
ON CONFLICT (campaign_id, product_id) DO NOTHING;
