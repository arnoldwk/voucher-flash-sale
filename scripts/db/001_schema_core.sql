-- =====================================================================
-- 001_schema_core.sql
-- Tabel inti: campaigns (offer flash sale) dan vouchers (hasil klaim)
-- =====================================================================

CREATE TABLE IF NOT EXISTS campaigns (
    id                 VARCHAR(64) PRIMARY KEY,
    title              VARCHAR(255) NOT NULL,
    description        TEXT,

    -- Alokasi stock sesi (pool kategori / jumlah event dalam periode)
    session_stock      INT NOT NULL DEFAULT 0 CHECK (session_stock >= 0),
    -- Jumlah yang sudah diklaim pada sesi ini
    claimed_count      INT NOT NULL DEFAULT 0 CHECK (claimed_count >= 0),
    -- Sisa stock sesi
    remaining_stock    INT NOT NULL DEFAULT 0 CHECK (remaining_stock >= 0),

    discount_amount    NUMERIC(12, 2) NOT NULL DEFAULT 0,

    start_time         TIMESTAMPTZ NOT NULL,
    end_time           TIMESTAMPTZ NOT NULL,
    status             VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',

    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ck_campaign_window CHECK (end_time > start_time),
    -- Invariant inti: claimed tidak boleh melebihi alokasi sesi
    CONSTRAINT ck_campaign_stock  CHECK (claimed_count <= session_stock)
);

CREATE INDEX IF NOT EXISTS idx_campaigns_status ON campaigns(status);
CREATE INDEX IF NOT EXISTS idx_campaigns_window ON campaigns(start_time, end_time);


CREATE TABLE IF NOT EXISTS vouchers (
    id               VARCHAR(64) PRIMARY KEY,
    campaign_id      VARCHAR(64) NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    user_id          VARCHAR(64) NOT NULL,
    voucher_code     VARCHAR(64) NOT NULL UNIQUE,
    discount_amount  NUMERIC(12, 2) NOT NULL DEFAULT 0,
    status           VARCHAR(32) NOT NULL DEFAULT 'CLAIMED', -- CLAIMED, REDEEMED, EXPIRED
    claimed_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    redeemed_at      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_vouchers_user_id ON vouchers(user_id);
CREATE INDEX IF NOT EXISTS idx_vouchers_campaign_id ON vouchers(campaign_id);
CREATE INDEX IF NOT EXISTS idx_vouchers_code ON vouchers(voucher_code);
CREATE INDEX IF NOT EXISTS idx_vouchers_status ON vouchers(status);
