-- =====================================================================
-- 005_schema_orders.sql
-- Transaction / order core: orders, session_category_quotas,
-- customer_entitlements for the transaction-service checkout flow.
-- =====================================================================

-- Reservation session quotas: durably tracks how many seats of a
-- voucher product in a flash-sale session are reserved vs confirmed.
CREATE TABLE IF NOT EXISTS session_category_quotas (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id        VARCHAR(64) NOT NULL,
    voucher_product_id VARCHAR(64) NOT NULL,
    category          VARCHAR(32) NOT NULL,
    quota             INT NOT NULL CHECK (quota >= 0),
    reserved_count    INT NOT NULL DEFAULT 0 CHECK (reserved_count >= 0),
    confirmed_count   INT NOT NULL DEFAULT 0 CHECK (confirmed_count >= 0),
    status            VARCHAR(32) NOT NULL DEFAULT 'OPEN', -- OPEN | CLOSED
    version           BIGINT NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (session_id, voucher_product_id)
);

CREATE INDEX IF NOT EXISTS idx_quotas_session_product ON session_category_quotas(session_id, voucher_product_id);
CREATE INDEX IF NOT EXISTS idx_quotas_category ON session_category_quotas(session_id, category);

-- Orders are the durable record of a reservation made through the
-- transaction-service checkout API.
CREATE TABLE IF NOT EXISTS orders (
    id                  VARCHAR(64) PRIMARY KEY,
    customer_id         VARCHAR(64) NOT NULL,
    voucher_product_id  VARCHAR(64) NOT NULL,
    session_id          VARCHAR(64) NOT NULL,
    category            VARCHAR(32) NOT NULL,
    price_amount        BIGINT NOT NULL,                    -- IDR cents
    currency            VARCHAR(8) NOT NULL DEFAULT 'IDR',
    status              VARCHAR(32) NOT NULL DEFAULT 'PENDING', -- PENDING | PAID | EXPIRED | CANCELLED
    reservation_deadline TIMESTAMPTZ NOT NULL,
    idempotency_key     UUID UNIQUE,
    version             BIGINT NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_orders_customer ON orders(customer_id);
CREATE INDEX IF NOT EXISTS idx_orders_session_category ON orders(session_id, category);
CREATE INDEX IF NOT EXISTS idx_orders_vp ON orders(voucher_product_id);
CREATE INDEX IF NOT EXISTS idx_orders_deadline ON orders(reservation_deadline)
    WHERE status = 'PENDING';
CREATE INDEX IF NOT EXISTS idx_orders_idempotency ON orders(idempotency_key);

-- One entitlement per customer per category per flash-sale session.
-- Links an active reservation to its customer so duplicates cannot be
-- reserved before payment completes.
CREATE TABLE IF NOT EXISTS customer_entitlements (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id   VARCHAR(64) NOT NULL,
    session_id    VARCHAR(64) NOT NULL,
    category      VARCHAR(32) NOT NULL,
    campaign_id   VARCHAR(64), -- audit only, nullable
    active_order_id VARCHAR(64) UNIQUE REFERENCES orders(id) ON DELETE SET NULL,
    status        VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',   -- ACTIVE | CONSUMED | RELEASED
    version       BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_entitlements_customer ON customer_entitlements(customer_id);
CREATE INDEX IF NOT EXISTS idx_entitlements_session_category ON customer_entitlements(session_id, category);
CREATE INDEX IF NOT EXISTS idx_entitlements_active_order ON customer_entitlements(active_order_id);
