GOPATH_BIN := $(shell go env GOPATH)/bin
DATABASE_URL ?= postgres://localhost:5432/ivyticketing?sslmode=disable

.PHONY: setup dev api web migrate-up migrate-down sqlc test lint fmt

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

lint:
	cd services/api && go vet ./...

fmt:
	cd services/api && go fmt ./...
	cd apps/web && pnpm exec prettier --write . || true
