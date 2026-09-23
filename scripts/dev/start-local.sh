#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="/root/ivyticketing/.env"
if [ -f "$ENV_FILE" ]; then
  set -a
  source "$ENV_FILE"
  set +a
fi

cleanup() { kill 0; }
trap cleanup EXIT

echo "==> Starting API on :${API_PORT:-8081} and web on :${PORT:-4321}"
(cd /root/ivyticketing/services/api && go run ./cmd/api) &
(cd /root/ivyticketing/apps/web && pnpm dev --host 0.0.0.0 --port 4321) &
wait
