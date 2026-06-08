#!/usr/bin/env bash
# Run poe-fissure locally over http://localhost:8080 (production-like: the Go
# server serves the built React app). Pass --build to rebuild the frontend.
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ "${1:-}" == "--build" || ! -d web/dist ]]; then
  echo "Building frontend..."
  (cd web && npm ci && npm run build)
fi

echo "Building server..."
go build -o ./poefissure-server ./cmd/poefissure-server

if [[ -f .env.local ]]; then
  set -a; source .env.local; set +a
else
  echo "No .env.local found — run ./scripts/setup-local.sh first." >&2
  exit 1
fi

export APP_DEV=1   # allow non-Secure cookies over http://localhost
export PORT="${PORT:-8080}"
export WEB_DIR="${WEB_DIR:-$PWD/web/dist}"
export SNAP_DIR="${SNAP_DIR:-$PWD/data/snapshots}"
# Reuse the token already minted by `./poefissure auth login` unless overridden.
default_token="$HOME/Library/Application Support/poefissure/token.json"
[[ -f "$default_token" ]] || default_token="$HOME/.config/poefissure/token.json"
export TOKEN_FILE="${TOKEN_FILE:-$default_token}"

: "${APP_PASSWORD_HASH:?missing in .env.local}"
: "${SESSION_SECRET:?missing in .env.local}"
: "${POE_CHARACTER:?missing in .env.local}"

echo "Serving on http://localhost:$PORT  (Ctrl-C to stop)"
exec ./poefissure-server serve
