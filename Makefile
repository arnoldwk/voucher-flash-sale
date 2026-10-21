.PHONY: all tidy fmt vet test test-cover build run-campaign run-claim run-voucher run-customer run-auth run-all token docker-up docker-down load-test db-up db-down db-reset db-seed db-verify db-shell

# Auto-load .env (jika ada) untuk seluruh target run-*.
-include .env
export

all: fmt vet test build

tidy:
	go mod tidy

fmt:
	gofmt -w -s .

vet:
	go vet ./...

test:
	go test -v -race ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | tail -1

build:
	go build -o bin/campaign-service ./services/campaign-service/cmd/server
	go build -o bin/claim-service ./services/claim-service/cmd/server
	go build -o bin/voucher-service ./services/voucher-service/cmd/server
	go build -o bin/customer-service ./services/customer-service/cmd/server
	go build -o bin/auth-service ./services/auth-service/cmd/server

run-campaign:
	go run ./services/campaign-service/cmd/server

run-claim:
	go run ./services/claim-service/cmd/server

run-voucher:
	go run ./services/voucher-service/cmd/server

run-customer:
	go run ./services/customer-service/cmd/server

run-auth:
	go run ./services/auth-service/cmd/server

# Generate a dev JWT for testing protected endpoints (reads JWT_SECRET from .env).
token:
	go run ./scripts/token

# run-all starts all 5 services in background.
run-all:
	@echo "Starting all services (logs at /tmp/voucher-services.log)..."
	$(MAKE) run-campaign > /tmp/campaign.log 2>&1 &
	$(MAKE) run-claim > /tmp/claim.log 2>&1 &
	$(MAKE) run-voucher > /tmp/voucher.log 2>&1 &
	$(MAKE) run-customer > /tmp/customer.log 2>&1 &
	$(MAKE) run-auth > /tmp/auth.log 2>&1 &
	@echo "All services started. Logs: /tmp/{campaign,claim,voucher,customer,auth}.log"
	@echo "Stop with: pkill -f 'voucher-flash-sale/services'"

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down -v

load-test:
	go run scripts/load_test.go

# ---- Database ----

db-up:
	docker compose up -d postgres redis

db-down:
	docker compose down

db-reset:
	docker compose down -v
	docker compose up -d postgres redis
	@sleep 3

db-seed:
	@echo "Applying schema..."
	@docker compose exec -T postgres psql -U postgres -d flashsale -v ON_ERROR_STOP=1 < scripts/db/001_schema_core.sql
	@echo "Applying catalog schema..."
	@docker compose exec -T postgres psql -U postgres -d flashsale -v ON_ERROR_STOP=1 < scripts/db/002_schema_catalog.sql
	@echo "Applying flash sale schema..."
	@docker compose exec -T postgres psql -U postgres -d flashsale -v ON_ERROR_STOP=1 < scripts/db/003_schema_flash_sale.sql
	@echo "Applying voucher products schema..."
	@docker compose exec -T postgres psql -U postgres -d flashsale -v ON_ERROR_STOP=1 < scripts/db/004_schema_voucher_products.sql
	@echo "Seeding master data..."
	@docker compose exec -T postgres psql -U postgres -d flashsale -v ON_ERROR_STOP=1 < scripts/db/100_seed_master_data.sql
	@echo "Seeding flash sale data..."
	@docker compose exec -T postgres psql -U postgres -d flashsale -v ON_ERROR_STOP=1 < scripts/db/101_seed_flash_sale.sql
	@echo "Seeding voucher products..."
	@docker compose exec -T postgres psql -U postgres -d flashsale -v ON_ERROR_STOP=1 < scripts/db/102_seed_voucher_products.sql
	@echo "Done."

db-verify:
	docker compose exec -T postgres psql -U postgres -d flashsale -v ON_ERROR_STOP=1 < scripts/db/999_verify.sql

db-shell:
	docker compose exec postgres psql -U postgres -d flashsale
