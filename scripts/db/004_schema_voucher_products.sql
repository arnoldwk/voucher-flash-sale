-- =====================================================================
-- 004_schema_voucher_products.sql
-- Tabel voucher_products (jembatan products <-> campaigns) + pricing
-- untuk API /product/v1/vouchers
-- =====================================================================

-- min_transaction: minimum transaksi untuk bisa pakai voucher
ALTER TABLE products
    ADD COLUMN IF NOT EXISTS min_transaction NUMERIC(12, 2) NOT NULL DEFAULT 0;

-- voucher_products: setiap campaign bisa punya beberapa varian produk
-- (misalnya: F&B dengan nominal berbeda)
CREATE TABLE IF NOT EXISTS voucher_products (
    id              VARCHAR(64) PRIMARY KEY,
    campaign_id     VARCHAR(64) NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    product_id      VARCHAR(64) NOT NULL REFERENCES products(id)  ON DELETE RESTRICT,

    -- harga dalam RUPIAH (di API dikonversi ke cents: * 100)
    price           NUMERIC(12, 2) NOT NULL DEFAULT 0,
    min_transaction NUMERIC(12, 2) NOT NULL DEFAULT 0,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_vp_campaign_product UNIQUE (campaign_id, product_id),
    CONSTRAINT ck_vp_price CHECK (price >= 0),
    CONSTRAINT ck_vp_min_tx CHECK (min_transaction >= 0)
);

CREATE INDEX IF NOT EXISTS idx_vp_campaign  ON voucher_products(campaign_id);
CREATE INDEX IF NOT EXISTS idx_vp_product  ON voucher_products(product_id);

-- Tambahan di vouchers: link ke voucher_product (untuk traceability API)
ALTER TABLE vouchers
    ADD COLUMN IF NOT EXISTS voucher_product_id VARCHAR(64)
        REFERENCES voucher_products(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_vouchers_vp ON vouchers(voucher_product_id);
