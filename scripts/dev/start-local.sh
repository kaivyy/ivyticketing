#!/usr/bin/env bash
set -euo pipefail

cleanup() { kill 0; }
trap cleanup EXIT

echo "==> Starting API on :8080 and web on :4321"
(cd services/api && go run ./cmd/api) &
(cd apps/web && pnpm dev) &
wait
