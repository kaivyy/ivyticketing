GOPATH_BIN := $(shell go env GOPATH)/bin
DATABASE_URL ?= postgres://localhost:5432/ivyticketing?sslmode=disable
TEST_DATABASE_URL ?= postgres://localhost:5432/ivyticketing_test?sslmode=disable

.PHONY: setup dev api web migrate-up migrate-down sqlc test test-db-setup test-integration lint fmt

setup:
	bash scripts/dev/setup-local.sh

dev:
	bash scripts/dev/start-local.sh

api:
	cd services/api && go run ./cmd/api

web:
	cd apps/web && pnpm dev

migrate-up:
	$(GOPATH_BIN)/goose -dir database/migrations postgres "$(DATABASE_URL)" up

migrate-down:
	$(GOPATH_BIN)/goose -dir database/migrations postgres "$(DATABASE_URL)" down

sqlc:
	cd services/api && $(GOPATH_BIN)/sqlc generate

test:
	cd services/api && go test ./...

test-db-setup:
	$(GOPATH_BIN)/goose -dir database/migrations postgres "$(TEST_DATABASE_URL)" up

test-integration:
	cd services/api && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -tags=integration ./tests/integration/... -v

lint:
	cd services/api && go vet ./...

fmt:
	cd services/api && go fmt ./...
	cd apps/web && pnpm exec prettier --write . || true
