#!/usr/bin/env bash
# One-time local setup: builds the binaries and writes .env.local with your
# password hash, a session secret, and character settings. Re-run safe (it won't
# overwrite an existing .env.local).
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== poe-fissure local setup =="
go build -o ./poefissure-server ./cmd/poefissure-server
go build -o ./poefissure ./cmd/poefissure
echo "Built ./poefissure-server and ./poefissure"

if [[ -f .env.local ]]; then
  echo ".env.local already exists — edit it directly, or delete it to recreate."
  exit 0
fi

read -rsp "Choose a site password: " PW; echo
HASH="$(./poefissure-server hash "$PW")"
SECRET="$(./poefissure-server gen-secret)"
read -rp "Character name [Frozenmulligan]: " CHAR; CHAR="${CHAR:-Frozenmulligan}"
read -rp "League [Runes of Aldur]: " LEAGUE; LEAGUE="${LEAGUE:-Runes of Aldur}"
read -rp "Contact email (for the API User-Agent): " CONTACT
read -rp "POESESSID for trade pricing (optional, Enter to skip): " SESS

# Values are single-quoted so $ in the bcrypt hash / cookie is never expanded
# when dev.sh sources this file.
cat > .env.local <<EOF
APP_PASSWORD_HASH='$HASH'
SESSION_SECRET='$SECRET'
POE_CHARACTER='$CHAR'
POE_LEAGUE='$LEAGUE'
POE_CONTACT='$CONTACT'
POE_REALM='poe2'
POE_SESSID='$SESS'
EOF
chmod 600 .env.local
echo "Wrote .env.local (gitignored)."
echo
echo "Next:"
echo "  1. Mint your GGG token (opens a browser, Steam login works):"
echo "       ./poefissure auth login"
echo "  2. Start the app locally:"
echo "       ./scripts/dev.sh"
