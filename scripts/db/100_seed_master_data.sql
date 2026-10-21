-- =====================================================================
-- 100_seed_master_data.sql
-- MASTER DATA: merchant, category, product (katalog voucher)
-- =====================================================================

-- ---------------------------------------------------------------------
-- Merchant (brand / BTW)
-- ---------------------------------------------------------------------
INSERT INTO merchants (id, name) VALUES
    ('mch-kopi-kenangan',    'Kopi Kenangan'),
    ('mch-zara-map',         'Zara / MAP'),
    ('mch-tokopedia-gadget', 'Tokopedia Gadget'),
    ('mch-starbucks',        'Starbucks'),
    ('mch-hm',               'H&M'),
    ('mch-wardrobe-beauty', 'Wardrobe Beauty'),
    ('mch-cgv',              'CGV'),
    ('mch-blessing',         'Blessing'),
    ('mch-uniqlo',           'UNIQLO'),
    ('mch-apple-reseller',   'Apple Reseller')
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------
-- Category
-- ---------------------------------------------------------------------
INSERT INTO categories (id, name) VALUES
    ('cat-fn-b',       'F&B'),
    ('cat-fashion',    'Fashion'),
    ('cat-elektronik', 'Elektronik'),
    ('cat-kbeauty',    'Beauty'),
    ('cat-retail',     'Retail'),
    ('cat-leisure',    'Leisure')
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------------
-- Product (voucher yang bisa dibeli)
--   promo_value     = nilai voucher  ("NILAI PROMO")
--   discount_amount = nilai hemat     ("Hemat")
--   pay_amount      = harga yang dibayar user ("Bayar")
-- ---------------------------------------------------------------------
INSERT INTO products (
    id, merchant_id, category_id, name, description,
    promo_value, discount_amount, pay_amount, terms, image_url
) VALUES
    -- ===== 3 produk utama dari mockup =====
    ('prd-kenangan-toast', 'mch-kopi-kenangan', 'cat-fn-b',
     'Kopi Kenangan', 'Kenangan Mantan & Toast',
     50000, 45000, 5000, 'Semua Outlet & App', '/img/voucher/kopi-kenangan.png'),

    ('prd-zara-all', 'mch-zara-map', 'cat-fashion',
     'Zara / MAP', 'Semua Outlet & App',
     50000, 45000, 5000, 'Semua Outlet & App', '/img/voucher/zara-map.png'),

    ('prd-tokopedia-smartphone', 'mch-tokopedia-gadget', 'cat-elektronik',
     'Tokopedia Gadget', 'Voucher Smartphone Official',
     50000, 45000, 5000, 'Semua Outlet & App', '/img/voucher/tokopedia-gadget.png'),

    -- ===== produk tambahan untuk kelengkapan katalog =====
    ('prd-starbucks', 'mch-starbucks', 'cat-fn-b',
     'Starbucks', 'Free Upgrade Handcrafted Beverage',
     35000, 30000, 5000, 'Gerai Starbucks Indonesia', '/img/voucher/starbucks.png'),

    ('prd-momo-food', 'mch-tokopedia-gadget', 'cat-fn-b',
     'Tokopedia Now', 'Voucher Food Delivery Rp20.000',
     25000, 20000, 5000, 'Tokopedia Now', '/img/voucher/tokopedia-now.png'),

    ('prd-hm-fashion', 'mch-hm', 'cat-fashion',
     'H&M', 'Voucher Belanja Minimum Rp300.000',
     75000, 70000, 5000, 'H&M Online & Store', '/img/voucher/hm.png'),

    ('prd-uniqlo-basic', 'mch-uniqlo', 'cat-fashion',
     'UNIQLO', 'Voucher Basic Cotton 2 Pcs',
     80000, 75000, 5000, 'UNIQLO Indonesia', '/img/voucher/uniqlo.png'),

    ('prd-wardrobe-skin', 'mch-wardrobe-beauty', 'cat-kbeauty',
     'Wardrobe Beauty', 'Voucher Perawatan Skin',
     60000, 55000, 5000, 'Semua Gerai', '/img/voucher/wardrobe.png'),

    ('prd-blessing-retail', 'mch-blessing', 'cat-retail',
     'Blessing', 'Voucher Belanja Minimum Rp500.000',
     100000, 95000, 5000, 'Semua Gerai Blessing', '/img/voucher/blessing.png'),

    ('prd-tokopedia-retail', 'mch-tokopedia-gadget', 'cat-retail',
     'Tokopedia Retail', 'Voucher Belanja Rp200.000',
     90000, 85000, 5000, 'Tokopedia App', '/img/voucher/tokopedia-retail.png'),

    ('prd-cgv-movie', 'mch-cgv', 'cat-leisure',
     'CGV', 'Voucher Nonton Film 2D',
     60000, 50000, 10000, 'Semua Bioskop CGV', '/img/voucher/cgv.png'),

    ('prd-iphone-tradein', 'mch-apple-reseller', 'cat-elektronik',
     'Apple Reseller', 'Voucher iPhone Trade-In',
     500000, 495000, 5000, 'Authorized Reseller', '/img/voucher/apple.png')
ON CONFLICT (id) DO NOTHING;
