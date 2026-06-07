#!/usr/bin/env bash
set -euo pipefail

PG_PREFIX="$(brew --prefix)/opt/postgresql@16/bin"

echo "==> Installing Postgres 16 and Redis (if missing)"
brew list postgresql@16 >/dev/null 2>&1 || brew install postgresql@16
brew list redis >/dev/null 2>&1 || brew install redis

echo "==> Starting services"
brew services start postgresql@16
brew services start redis

echo "==> Waiting for Postgres to accept connections"
for i in {1..30}; do
  if "$PG_PREFIX/pg_isready" -q; then break; fi
  sleep 1
done

echo "==> Creating database (if missing)"
"$PG_PREFIX/createdb" ivyticketing 2>/dev/null || echo "database already exists"

echo "==> Installing Go tools (goose, sqlc)"
go install github.com/pressly/goose/v3/cmd/goose@latest
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

echo "==> Generating sqlc code"
(cd services/api && "$(go env GOPATH)/bin/sqlc" generate)

echo "==> Running migrations"
"$(go env GOPATH)/bin/goose" -dir database/migrations postgres \
  "postgres://localhost:5432/ivyticketing?sslmode=disable" up

echo "==> Installing web dependencies"
(cd apps/web && pnpm install)

echo "==> Setup complete. Run 'make dev' to start."
