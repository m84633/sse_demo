.PHONY: run sqlc wire migrate-up migrate-down test-integration test-e2e lint

run:
	go run ./cmd/server

sqlc:
	sqlc generate

wire:
	wire ./cmd/server

migrate-up:
	@set -a; . ./.env; set +a; migrate -path ./migrations -database "$$MIGRATE_DB_URL" up

migrate-down:
	@set -a; . ./.env; set +a; migrate -path ./migrations -database "$$MIGRATE_DB_URL" down 1

test-integration:
	go test -tags=integration ./...

test-e2e:
	go test ./e2e

lint:
	golangci-lint run ./...
