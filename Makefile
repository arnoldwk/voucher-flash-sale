.PHONY: all tidy fmt vet test build run-campaign run-claim run-voucher docker-up docker-down load-test

all: fmt vet test build

tidy:
	go mod tidy

fmt:
	gofmt -w -s .

vet:
	go vet ./...

test:
	go test -v -race ./...

build:
	go build -o bin/campaign-service ./services/campaign-service/cmd/server
	go build -o bin/claim-service ./services/claim-service/cmd/server
	go build -o bin/voucher-service ./services/voucher-service/cmd/server

run-campaign:
	go run ./services/campaign-service/cmd/server

run-claim:
	go run ./services/claim-service/cmd/server

run-voucher:
	go run ./services/voucher-service/cmd/server

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down -v

load-test:
	go run scripts/load_test.go
