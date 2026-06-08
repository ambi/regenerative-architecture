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

command -v bun >/dev/null || {
  echo "bun is required" >&2
  exit 1
}

if [ ! -d "$ROOT_DIR/node_modules" ]; then
  echo "Installing API dependencies..."
  (cd "$ROOT_DIR" && bun install --frozen-lockfile)
fi

if [ ! -d "$ROOT_DIR/ui/node_modules" ]; then
  echo "Installing UI dependencies..."
  (cd "$ROOT_DIR/ui" && bun install --frozen-lockfile)
fi

echo "Starting ra-idp API at http://localhost:3000"
(
  cd "$ROOT_DIR"
  PORT=3000 \
    ISSUER=http://localhost:5173 \
    PERSISTENCE=memory \
    EVENT_SINK=console \
    OBSERVABILITY=noop \
    DEMO_USER_PASSWORD=demo-password-1234 \
    bun run dev
) &
API_PID=$!

sleep 1
if ! kill -0 "$API_PID" 2>/dev/null; then
  wait "$API_PID" 2>/dev/null || true
  exit 1
fi

echo "Starting UI at http://localhost:5173"
echo "Demo credentials: alice / demo-password-1234"
cd "$ROOT_DIR/ui"
bun run dev
