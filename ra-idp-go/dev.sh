#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
API_PID=

cleanup() {
  if [ -n "$API_PID" ] && kill -0 "$API_PID" 2>/dev/null; then
    kill "$API_PID" 2>/dev/null || true
    wait "$API_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

command -v go >/dev/null || {
  echo "go is required" >&2
  exit 1
}
command -v bun >/dev/null || {
  echo "bun is required" >&2
  exit 1
}

if [ ! -d "$ROOT_DIR/ui/node_modules" ]; then
  echo "Installing UI dependencies..."
  (cd "$ROOT_DIR/ui" && bun install --frozen-lockfile)
fi

echo "Starting ra-idp-go API at http://localhost:8081"
(
  cd "$ROOT_DIR"
  ADDR=:8081 \
    ISSUER=http://localhost:5173 \
    PERSISTENCE=memory \
    go run ./cmd/ra-idp-go
) &
API_PID=$!

echo "Starting UI at http://localhost:5173"
echo "Demo credentials: alice / demo-password-1234"
cd "$ROOT_DIR/ui"
bun run dev
