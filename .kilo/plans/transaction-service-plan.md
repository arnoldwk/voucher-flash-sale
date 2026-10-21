# Plan: transaction-service (Core Checkout / Orders)

## Objective

Implement a new microservice `transaction-service` that exposes
`POST /v1/orders` to reserve a voucher product for a customer. The reservation:

1. Atomically checks & decrements the per-session category quota (Redis hot path, Postgres truth).
2. Ensures the customer does not already have an active entitlement for the same category (1 voucher per category per customer).
3. Persists a `PENDING` order with a 5-minute reservation deadline.
4. Returns the contract:

```json
{
  "status": true,
  "code": "CREATED",
  "message": "Order reserved successfully",
  "errorMessage": null,
  "errorData": null,
  "data": {
    "orderId": "ord_01HQX...",
    "status": "PENDING",
    "price":  { "amount": 10000, "currency": "IDR" },
    "reservationDeadline": "2026-10-24T05:05:00Z"
  }
}
```

Or, on quota exhaustion:

```json
{
  "status": false,
  "code": "STOCK_UNAVAILABLE",
  "message": "Checkout failed",
  "errorMessage": "Voucher stock is empty for this session.",
  "errorData": { "category": "FOOD_BEVERAGE" },
  "data": {}
}
```

---

## References

- Sequence diagram: `data-support/sequenc-checkout-dan-payment.png`
- Class diagram: `data-support/class-diagram-trx.png`
- User-supplied API contract: literal, including 409 / `STOCK_UNAVAILABLE` with `errorData.category`.
- Voucher class diagram: `data-support/class-diagram-voucher.png` (defines `voucher_products`, `sessions`, `vouchers`, `redemptions`). We share read-only views of `voucher_products` but introduce new tables owned by transaction-service.

---

## Design Decisions

| Concern | Decision | Rationale |
|---|---|---|
| Service placement | **New** `services/transaction-service` (port `8086`) | Sequence diagram names `transaction-svc`; Clean Architecture stays uniform across all 5 services; keeps the reservation hot path independent from catalog reads. |
| Stock source of truth | **Redis hot path + Postgres truth** (both) | Hot path mirrors the existing `campaign:stock:{id}` counter (atomic Lua decrement); Postgres `session_category_quotas` is the durable record and counter-arg for correctness. The Lua script becomes the single command that decrements Redis **and** writes a transient reservation marker. Postgres is updated with the order insert in the same transaction. |
| Entitlement model | **New** `customer_entitlements` table owned by transaction-service | Matches the class diagram exactly (`activates` 1..0..1 → `orders.active_order_id`). Reusing `vouchers` (created post-payment) is too late — we must block the reservation before payment. |
| Endpoint shape | `POST /v1/orders` with `voucherProductId` in body | User's explicit API contract wins; body matches existing patterns in this repo (e.g. `/api/v1/campaigns`). |
| Reservation TTL | **5 minutes** (300s) | Sequence diagram says "Timer 5m". Stored as `reservation_deadline` and enforced by a sweeper that releases quota + entitlement on expiry. |
| Idempotency | Header `Idempotency-Key` on `POST /v1/orders` → `orders.idempotency_key UNIQUE` | Sequence diagram explicitly shows `{Idempotency-Key}` on the order create call. |
| Authentication | Bearer JWT (`pkg/middleware.AuthBearer`) | Reuse the existing middleware; require `JWT_SECRET` in env. |
| Phase scope | **`POST /v1/orders` only** | Payment step (`POST /v1/orders/{orderId}/payments` → Xendit stub) is out of scope and will be planned separately once the payment-svc Xendit contract is confirmed. |
| Envelope | `pkg/apiresponse.Envelope` (existing); `errorData` populated with `{category}` on `STOCK_UNAVAILABLE` | Matches existing patterns from campaign-service catalog endpoint. |

---

## Domain Model (mirrors `class-diagram-trx.png` exactly)

### Tables (new SQL file `scripts/db/005_schema_orders.sql`)

```sql
CREATE TABLE orders (
  id                    VARCHAR(64) PRIMARY KEY,           -- ord_<ulid-like>
  customer_id           VARCHAR(64) NOT NULL,               -- normalized phone e.g. "6281..."
  voucher_product_id    VARCHAR(64) NOT NULL
        REFERENCES voucher_products(id) ON DELETE RESTRICT,
  session_id            VARCHAR(64) NOT NULL,               -- flash_sale_event_id
  category              VARCHAR(32) NOT NULL,
  price_amount          BIGINT NOT NULL,                    -- IDR cents
  currency              VARCHAR(8)  NOT NULL DEFAULT 'IDR',
  status                VARCHAR(32) NOT NULL DEFAULT 'PENDING', -- PENDING|PAID|EXPIRED|CANCELLED
  reservation_deadline  TIMESTAMPTZ NOT NULL,
  idempotency_key       UUID UNIQUE,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  version               BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_orders_customer         ON orders(customer_id);
CREATE INDEX idx_orders_session_category ON orders(session_id, category);
CREATE INDEX idx_orders_deadline         ON orders(reservation_deadline)
    WHERE status = 'PENDING';

CREATE TABLE session_category_quotas (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id        VARCHAR(64) NOT NULL,                  -- flash_sale_event_id
  voucher_product_id VARCHAR(64) NOT NULL
        REFERENCES voucher_products(id) ON DELETE CASCADE,
  category          VARCHAR(32) NOT NULL,
  quota             INT NOT NULL CHECK (quota >= 0),
  reserved_count    INT NOT NULL DEFAULT 0 CHECK (reserved_count >= 0),
  confirmed_count   INT NOT NULL DEFAULT 0 CHECK (confirmed_count >= 0),
  status            VARCHAR(32) NOT NULL DEFAULT 'OPEN',   -- OPEN|CLOSED
  version           BIGINT NOT NULL DEFAULT 0,
  UNIQUE (session_id, voucher_product_id)
);

CREATE TABLE customer_entitlements (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id       VARCHAR(64) NOT NULL,
  session_id        VARCHAR(64) NOT NULL,
  category          VARCHAR(32) NOT NULL,
  campaign_id       VARCHAR(64),                           -- nullable, audit only
  active_order_id   VARCHAR(64) UNIQUE
        REFERENCES orders(id) ON DELETE SET NULL,
  status            VARCHAR(32) NOT NULL DEFAULT 'ACTIVE', -- ACTIVE|CONSUMED|RELEASED
  version           BIGINT NOT NULL DEFAULT 0,
  UNIQUE (customer_id, session_id, category)
);
```

Schema & seed wiring:
- `scripts/db/005_schema_orders.sql` — new tables + indexes.
- Extend `Makefile` `db-seed` to apply `005_schema_orders.sql`.
- Extend `docker-compose.yml` `postgres` volume mount picks it up on fresh init.
- Bootstrap row per active `voucher_product` into `session_category_quotas` with `quota = campaigns.remaining_stock` at the moment the service starts (mirroring how the campaign service warms `campaign:stock:*` from Postgres remaining_stock).

---

## Component Layout

```
services/transaction-service/
├── cmd/server/main.go
├── internal/
│   ├── config/config.go
│   ├── domain/order.go                 # Order, SessionCategoryQuota, CustomerEntitlement entities & errors
│   ├── repository/
│   │   ├── postgres_order_repo.go      # orders + entitlements + quotas (PG truth)
│   │   └── redis_reservation_cache.go  # campaign:stock Lua DECR + reservation expiration
│   ├── usecase/
│   │   ├── reserve_order_usecase.go
│   │   └── reserve_order_usecase_test.go
│   └── delivery/http/
│       ├── order_handler.go
│       ├── order_handler_test.go
│       └── reservation_expiry_worker.go  # background sweeper that releases quota + entitlement on expiry
└── Dockerfile (build/package/Dockerfile.transaction)
```

---

## Redis Keys

| Key | Type | Purpose |
|---|---|---|
| `campaign:stock:{campaign_id}` | string (int) | Mirrors existing claim-service hot path. Decremented at reservation time (atomic Lua). |
| `reservation:order:{order_id}`  | string + TTL  | Reservation marker with TTL = remaining seconds until `reservation_deadline`. Enables idempotent reservation & sweeper. |
| `entitlement:cust:{customer_id}:session:{session_id}:cat:{category}` | string = order_id | Fast existence check for "already entitled". Mirrors `customer_entitlements` row for hot reads. |

Atomic Lua script (`reserveOrderLuaScript`) — single round-trip:

1. `local stock = tonumber(redis.call('GET', campaign:stock:{id}))`
2. If `nil` or `stock <= 0` → return `-2` (out of stock).
3. If key exists at `entitlement:cust:...:cat:...` → return `-3` (already entitled).
4. `DECR campaign:stock:{id}`
5. `SET reservation:order:{newID} {meta} EX <ttl>`
6. `SET entitlement:cust:...:cat:... {orderID} EX <ttl + 60s_grace>`
7. Return `1`.

Rollback path (cancelled/expired): `INCR campaign:stock:{id}` + `DEL entitlement:...`.

> **Convergence**: the Redis script protects the hot path from oversell; the
> Postgres `session_category_quotas` table is the durable counter that is
> updated inside the same transaction as the `orders` insert. A periodic
> reconciler (out of scope for phase 1) can re-seed Redis from Postgres.

---

## Reservation Usecase Flow

```
ReserveOrder(ctx, input { voucherProductId, phoneNumber, idempotencyKey, customerId }):
  1. Normalize phone (reuse normalizePhone from list_vouchers_usecase.go).
  2. Resolve voucherProduct → {campaign_id, session_id, category, price_cents, min_tx}.
     (Read from Postgres voucher_products + campaigns + categories.)
  3. Idempotency check:
     - SELECT orders WHERE idempotency_key = $1 → return existing order.
     - If found but status != PENDING, return original.
  4. Lua RESERVE (Redis): atomic decrement + entitlement write.
     - If returns -2: pick & return STOCK_UNAVAILABLE with errorData = {category}.
     - If returns -3: return ALREADY_ENTITLED (409).
  5. Postgres TX (defer-rollback on any failure):
     a) SELECT session_category_quotas FOR UPDATE
        → check (reserved_count < quota).
        If full → ROLLBACK; do Redis rollback; return STOCK_UNAVAILABLE.
     b) UPDATE reserved_count = reserved_count + 1, version = version + 1.
     c) INSERT orders row (PENDING, reservation_deadline = now + 5min).
     d) INSERT customer_entitlements (ACTIVE, active_order_id = orders.id).
     e) COMMIT.
  6. On commit failure → Redis rollback (INCR + DEL) → return 500.
  7. Return OrderView DTO.
```

### Domain errors

```go
var (
    ErrOutOfStock          = errors.New("quota exhausted")           // → STOCK_UNAVAILABLE 409
    ErrAlreadyEntitled     = errors.New("already entitled")          // → ALREADY_ENTITLED 409
    ErrVoucherProductMissing = errors.New("voucher product not found or not in active session")
    ErrReservationWindowClosed = errors.New("voucher session is closed or not yet open")
    ErrIdempotencyConflict    = errors.New("idempotency key reused with different payload")
)
```

---

## Handler & Envelope

`POST /v1/orders` → `OrderHandler.CreateOrder`:

```go
type createOrderRequest struct {
    VoucherProductID string `json:"voucherProductId"`
    PhoneNumber      string `json:"phone_number"`
}
```

| HTTP | apiresponse.Code | errorData | Use |
|---|---|---|---|
| 201 | `CREATED` | n/a | Reservation succeeded. |
| 400 | `INVALID_REQUEST` | n/a | Missing/invalid body, normalization failed. |
| 404 | `NOT_FOUND` | n/a | voucherProductId not active / unknown. |
| 409 | `STOCK_UNAVAILABLE` | `{category}` | Quota exhausted. |
| 409 | `ALREADY_ENTITLED`  | `{category}` | Same customer already has entitlement for this category in the session. |
| 409 | `IDEMPOTENCY_CONFLICT` | n/a | Same key, different payload. |
| 401 | `UNAUTHORIZED` | n/a | JWT invalid (middleware). |

The handler composes via `pkg/apiresponse.Error(w, statusCode, code, message)`; on STOCK_UNAVAILABLE we additionally populate `errorData` by switching to the typed `Envelope{ErrorData: map[string]any{"category": category}}` write (small helper added to `pkg/apiresponse` — non-breaking).

Wire-up mirrors campaign-service:
```go
mux := http.NewServeMux()
orderH.RegisterRoutes(mux)
mux.Handle("/v1/orders", http.StripPrefix("/v1",
    pkgmiddleware.AuthBearer(orderMux)))
```
(Inner mux registers `POST /orders`. With JWT middleware, the path becomes `/v1/orders`.)

---

## Background Worker

`reservation_expiry_worker.go` runs inside the service:

- Every 10s (configurable) it does:
  ```sql
  SELECT id, voucher_product_id, session_id, category
  FROM orders
  WHERE status = 'PENDING' AND reservation_deadline <= NOW()
  LIMIT 200 FOR UPDATE SKIP LOCKED;
  ```
- For each: release `session_category_quotas.reserved_count -= 1`, mark order `EXPIRED`, mark entitlement `RELEASED`, rollback Redis (INCR + DEL).

This is the only safe way to keep `reserved_count` from drifting; the Lua script's `entitlement` key auto-expires as a backstop.

---

## Test Strategy

Following the project's TDD preferences:

1. **Domain tests** (`domain/order_test.go`):
   - Money/price formatting (cents conversion reuse `MoneyToCents`).
   - Status lifecycle transitions.
   - Error sentinels (`errors.Is`).

2. **Usecase tests** (`usecase/reserve_order_usecase_test.go`) — mocked repos:
   - Happy path → quota decremented, order PENDING, deadline = now+5m.
   - Lua `-2` → STOCK_UNAVAILABLE returned without touching PG.
   - Lua `-3` → ALREADY_ENTITLED returned.
   - PG `reserved_count >= quota` race → rollback Redis + STOCK_UNAVAILABLE.
   - Idempotency-key replay → returns original order, no new insert.
   - Idempotency-key conflict (different payload) → 409.
   - Voucher outside session window → 400.

3. **Handler tests** (`delivery/http/order_handler_test.go`) — full envelope shape:
   - 201 success body matches contract byte-for-byte (`orderId`, `status: "PENDING"`, `price`, `reservationDeadline`).
   - 400 invalid phone.
   - 401 no JWT.
   - 409 STOCK_UNAVAILABLE with `errorData.category` populated.
   - 409 ALREADY_ENTITLED.
   - 409 idempotency conflict.

4. **Repository test** (integration — gated on `INTEGRATION=1`):
   - `session_category_quotas` row created from seed.
   - Concurrent reservations (10 goroutines × 1 quota) ⇒ exactly 1 succeeds, 9 receive STOCK_UNAVAILABLE.

5. **Worker test**:
   - Pre-seeded expired order → mark EXPIRED + release row.

---

## Wiring (mirrors campaign-service)

`cmd/server/main.go`:

```go
db, _ := database.ConnectPostgres(...)
rdb, _ := pkgredis.ConnectRedis(...)

orderRepo  := repository.NewPostgresOrderRepo(db)        // orders, quotas, entitlements
stockCache := repository.NewRedisReservationCache(rdb)   // Lua script + helpers
uc := usecase.NewReserveOrderUsecase(orderRepo, stockCache)
h := deliveryhttp.NewOrderHandler(uc)
deliveryhttp.StartExpiryWorker(ctx, db, stockCache, 10*time.Second)

mux := http.NewServeMux()
inner := http.NewServeMux()
h.RegisterRoutes(inner)
mux.Handle("/v1/", http.StripPrefix("/v1", pkgmiddleware.AuthBearer(inner)))

// graceful shutdown
```

Env (`config.go`):
- `TRANSACTION_SERVICE_PORT=8086`
- `POSTGRES_*`, `REDIS_ADDR`, `JWT_SECRET` (must match auth-service)
- `RESERVATION_TTL_SECONDS=300`
- `EXPIRY_WORKER_INTERVAL=10s`

`docker-compose.yml`:
- Add `transaction-service` service on `8086`, depends on `postgres` + `redis`, uses `golang:1.23-alpine` builder + `JWT_SECRET`.

`Makefile`:
- `build` target builds the new binary.
- `run-transaction` target.

`.env.example`:
- Add `TRANSACTION_SERVICE_PORT=8086`.

---

## Files to Create / Modify

### New
- `services/transaction-service/` (entire tree above)
- `build/package/Dockerfile.transaction`
- `scripts/db/005_schema_orders.sql`
- `scripts/db/103_seed_quotas.sql` — bootstrap rows for seeded active campaigns (derived from `voucher_products`).
- `pkg/apiresponse/error_with_data.go` — small typed-helper for `errorData` writes (additive).

### Modified
- `docker-compose.yml` — new service entry.
- `Makefile` — `build`, `run-transaction`, `run-all`, `db-seed`.
- `.env.example` — port + env var.

---

## Step-by-step Implementation Order

1. **Schema & seed**
   - Add `005_schema_orders.sql` with three tables + constraints.
   - Add `103_seed_quotas.sql` mirroring `campaigns.remaining_stock` for active sessions.
   - Wire into `db-seed`, `docker-compose` mount.

2. **Domain layer (TDD: red→green→refactor)**
   - `domain/order.go` entities & errors.
   - `domain/order_test.go` covering invariants, price formatting, status transitions.

3. **Redis reservation cache**
   - `redis_reservation_cache.go` with the Lua script + rollback helpers.
   - Unit tests with miniredis (or skipped if not vendored; otherwise live Redis in `INTEGRATION=1`).

4. **Postgres repository**
   - `postgres_order_repo.go` with `CreateOrderTx` (FOR UPDATE on quota + insert order + insert entitlement, all in one tx).
   - `FindByIdempotencyKey`, `ExpirePendingOrders`.

5. **Usecase** (TDD-first)
   - `reserve_order_usecase.go` orchestration per the flow above.
   - Tests for each branch of the decision tree.

6. **Handler & envelope helpers**
   - `order_handler.go` with strict request decoding + envelope helpers.
   - Tests verifying exact JSON shape (success + STOCK_UNAVAILABLE with `errorData`).

7. **Reservation expiry worker**
   - `reservation_expiry_worker.go` periodically releases expired reservations.
   - Integration test for expiry path.

8. **Wiring & config**
   - `config.go`, `main.go`, `Dockerfile.transaction`.
   - Update `docker-compose.yml`, `Makefile`, `.env.example`.

9. **End-to-end verification**
   - `gofmt -w -s .` clean.
   - `go vet ./...` clean.
   - `go test -race ./...` green.
   - Smoke test against running docker-compose: GET healthz, POST /v1/orders returns 201 CREATED, run reservation until quota is exhausted and verify STOCK_UNAVAILABLE with `errorData.category: "FOOD_BEVERAGE"`.

---

## Out of Scope (parking lot)

- `POST /v1/orders/{orderId}/payments` → Xendit stub.
- `payment-svc` (does not exist yet in this repo).
- `GET /v1/internal/v1/orders/{orderId}/state` internal endpoint.
- Voucher issuance / `vouchers` row creation (happens post-payment, by voucher-service).
- Cross-service reconciliation between Redis stock and Postgres quota (we document the divergence window).
- Discount-pricing engine beyond `price` (no coupon-on-coupon, no payment fees).

---

## Acceptance Criteria

- `go build ./...`, `go vet ./...`, `go test -race ./...` pass.
- `gofmt -l .` returns nothing.
- Smoke test: with `vp_fnb` quota = `5`, 5 sequential `POST /v1/orders` return 201, the 6th returns 409 with body exactly:
  ```json
  {"status":false,"code":"STOCK_UNAVAILABLE","message":"Checkout failed","errorMessage":"Voucher stock is empty for this session.","errorData":{"category":"FOOD_BEVERAGE"},"data":{}}
  ```
- Idempotency: identical `Idempotency-Key` returns the same `orderId` and creates exactly one `orders` row.
- Same phone retry on the same `vp_fashion` after a successful reservation returns 409 `ALREADY_ENTITLED` with `errorData.category = "FASHION_LIFESTYLE"`.
- Reservation expiry: an order with `reservation_deadline` in the past is moved to `EXPIRED` by the worker, freeing one slot in `session_category_quotas.reserved_count`.
