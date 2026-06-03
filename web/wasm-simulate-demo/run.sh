#!/usr/bin/env bash
# Run the browser simulate demo (from cre-cli repo root or this directory).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PORT="${WASM_DEMO_PORT:-9090}"
cd "$ROOT"
export WASM_DEMO_PORT="$PORT"
docker compose -f "$ROOT/web/wasm-simulate-demo/docker-compose.yml" up --build -d
echo "Demo: http://localhost:${PORT}/"
curl -sfI "http://127.0.0.1:${PORT}/" | grep -i content-type || true
