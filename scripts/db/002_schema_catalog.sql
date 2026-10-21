-- =====================================================================
-- 002_schema_catalog.sql
-- MASTER DATA: merchant (brand), category, product (katalog voucher)
-- =====================================================================

CREATE TABLE IF NOT EXISTS merchants (
    id          VARCHAR(64) PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS categories (
    id          VARCHAR(64) PRIMARY KEY,
    name        VARCHAR(128) NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS products (
    id               VARCHAR(64) PRIMARY KEY,
    merchant_id      VARCHAR(64) NOT NULL REFERENCES merchants(id)  ON DELETE RESTRICT,
    category_id      VARCHAR(64) NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,

    name             VARCHAR(255) NOT NULL,
    description      TEXT,

    -- Tiga nilai tampilan di UI:
    --   promo_value     -> "NILAI PROMO  Rp50.000"
    --   discount_amount -> "Hemat        Rp45.000"
    --   pay_amount      -> "Bayar        Rp5.000"
    promo_value      NUMERIC(12, 2) NOT NULL DEFAULT 0,
    discount_amount  NUMERIC(12, 2) NOT NULL DEFAULT 0,
    pay_amount       NUMERIC(12, 2) NOT NULL DEFAULT 0,

    terms            TEXT,   -- "Semua Outlet & App"
    image_url        TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ck_product_amounts CHECK (
        promo_value >= 0 AND discount_amount >= 0 AND pay_amount >= 0
    ),
    -- Tidak ada diskon bertingkat -> nilai promo = hemat + bayar
    CONSTRAINT ck_product_amounts_sum CHECK (
        discount_amount + pay_amount = promo_value
    )
);

CREATE INDEX IF NOT EXISTS idx_products_category ON products(category_id);
CREATE INDEX IF NOT EXISTS idx_products_merchant ON products(merchant_id);
