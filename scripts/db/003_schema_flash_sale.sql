-- =====================================================================
-- 003_schema_flash_sale.sql
-- Sesi flash sale (mingguan) + pool stock per kategori per periode
-- =====================================================================

-- Satu baris = satu sesi flash sale (mis. "Minggu 1" dalam sebulan)
CREATE TABLE IF NOT EXISTS flash_sale_events (
    id           VARCHAR(64) PRIMARY KEY,
    name         VARCHAR(255) NOT NULL,

    -- Periode alokasi (mis. November 2026)
    period_start TIMESTAMPTZ NOT NULL,
    period_end   TIMESTAMPTZ NOT NULL,

    -- Urutan sesi dalam periode + jumlah total sesi
    -- session_stock = pool.total_stock / event_count
    sequence     INT NOT NULL CHECK (sequence >= 1),
    event_count  INT NOT NULL CHECK (event_count >= 1),

    -- Jendela waktu sesi ini
    start_time   TIMESTAMPTZ NOT NULL,
    end_time     TIMESTAMPTZ NOT NULL,
    status       VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',

    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ck_event_window    CHECK (end_time > start_time),
    CONSTRAINT ck_event_sequence  CHECK (sequence <= event_count),
    CONSTRAINT ck_event_period    CHECK (period_end >= period_start)
);

CREATE INDEX IF NOT EXISTS idx_events_window ON flash_sale_events(start_time, end_time);
CREATE INDEX IF NOT EXISTS idx_events_period ON flash_sale_events(period_start, period_end);


-- Total stock pool per kategori per periode
CREATE TABLE IF NOT EXISTS category_stock_pools (
    id           VARCHAR(64) PRIMARY KEY,
    category_id  VARCHAR(64) NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    period_start TIMESTAMPTZ NOT NULL,
    period_end   TIMESTAMPTZ NOT NULL,
    total_stock  INT NOT NULL CHECK (total_stock >= 0),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_pool UNIQUE (category_id, period_start, period_end)
);

CREATE INDEX IF NOT EXISTS idx_pools_period ON category_stock_pools(period_start, period_end);


-- Link offer -> sesi + kategori
ALTER TABLE campaigns
    ADD COLUMN IF NOT EXISTS flash_sale_event_id VARCHAR(64)
        REFERENCES flash_sale_events(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS product_id VARCHAR(64)
        REFERENCES products(id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS category_id VARCHAR(64)
        REFERENCES categories(id) ON DELETE RESTRICT;

-- Satu kategori hanya boleh punya 1 offer per sesi
CREATE UNIQUE INDEX IF NOT EXISTS uq_event_category_offer
    ON campaigns(flash_sale_event_id, category_id)
    WHERE flash_sale_event_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_campaigns_event    ON campaigns(flash_sale_event_id);
CREATE INDEX IF NOT EXISTS idx_campaigns_product  ON campaigns(product_id);
CREATE INDEX IF NOT EXISTS idx_campaigns_category ON campaigns(category_id);


-- Kuota: maks 1 voucher per KATEGORI per pengguna
ALTER TABLE vouchers
    ADD COLUMN IF NOT EXISTS category_id VARCHAR(64)
        REFERENCES categories(id) ON DELETE RESTRICT;

ALTER TABLE vouchers DROP CONSTRAINT IF EXISTS uq_campaign_user;
ALTER TABLE vouchers ADD CONSTRAINT uq_category_user UNIQUE (category_id, user_id);

CREATE INDEX IF NOT EXISTS idx_vouchers_category ON vouchers(category_id);
