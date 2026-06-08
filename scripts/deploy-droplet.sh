#!/usr/bin/env bash
# Deploy poe-fissure to the blt-infra DO droplet (droplet-sfo Cloudflare Tunnel).
# Assembles the production .env from your LOCAL secrets (never printing them),
# ships the source, and brings up the compose stack. Run AFTER `terraform apply`
# in blt-infra has created the poe-fissure.bltech.app ingress rule.
#
# Usage (from the poe-fissure repo root):  ./scripts/deploy-droplet.sh
set -euo pipefail

DROPLET="${DROPLET:-root@138.197.214.133}"
APPDIR="${APPDIR:-/opt/poefissure}"
INFRA="${INFRA:-$HOME/workplace/blt-infra}"
REPO="$(cd "$(dirname "$0")/.." && pwd)"
LOCAL_ENV="$REPO/.env.local"
TOKEN_JSON="$HOME/Library/Application Support/poefissure/token.json"

[ -f "$LOCAL_ENV" ]  || { echo "missing $LOCAL_ENV (run scripts/setup-local.sh)"; exit 1; }
[ -f "$TOKEN_JSON" ] || { echo "missing token.json (run ./poefissure auth login)"; exit 1; }
command -v jq >/dev/null || { echo "jq required"; exit 1; }

echo "→ Reading the droplet tunnel token from terraform..."
TUNNEL_TOKEN="$(cd "$INFRA" && source .env && terraform -chdir=terraform/cloudflare output -raw droplet_tunnel_token)"
[ -n "$TUNNEL_TOKEN" ] || { echo "empty tunnel token — did 'terraform apply' run?"; exit 1; }

REFRESH="$(jq -r '.refresh_token // empty' "$TOKEN_JSON")"
[ -n "$REFRESH" ] || { echo "no refresh_token in token.json — re-run ./poefissure auth login"; exit 1; }

# Carry the app vars over from .env.local and assemble the prod .env locally.
set -a; source "$LOCAL_ENV"; set +a
ENVTMP="$(mktemp)"; trap 'rm -f "$ENVTMP"' EXIT
cat > "$ENVTMP" <<EOF
TUNNEL_TOKEN=$TUNNEL_TOKEN
APP_PASSWORD_HASH=$APP_PASSWORD_HASH
SESSION_SECRET=$SESSION_SECRET
POE_CHARACTER=$POE_CHARACTER
POE_LEAGUE=$POE_LEAGUE
POE_REALM=${POE_REALM:-poe2}
POE_CONTACT=${POE_CONTACT:-}
POE_SESSID=${POE_SESSID:-}
GGG_REFRESH_TOKEN=$REFRESH
EOF

echo "→ Ensuring $APPDIR exists (deploy-owned)..."
ssh "$DROPLET" "install -d -o deploy -g deploy -m 755 $APPDIR"

echo "→ Shipping source (excluding secrets/build artifacts)..."
# Excludes are anchored to the transfer root with a leading '/' so they match ONLY
# the repo-root items — an unanchored 'data' / 'poefissure-server' would also wrongly
# exclude internal/craft/data and cmd/poefissure-server.
rsync -avz --delete \
  --exclude='/.env' --exclude='/.env.local' --exclude='/app.env' --exclude='/.git/' \
  --exclude='/data/' --exclude='/web/node_modules/' --exclude='/web/dist/' \
  --exclude='/poefissure' --exclude='/poefissure-server' \
  --rsync-path='sudo -u deploy rsync' \
  "$REPO/" "$DROPLET:$APPDIR/"

echo "→ Installing $APPDIR/app.env (600, deploy-owned)..."
ssh "$DROPLET" "install -o deploy -g deploy -m 600 /dev/stdin $APPDIR/app.env && rm -f $APPDIR/.env" < "$ENVTMP"

echo "→ Building + starting the stack on the droplet..."
ssh "$DROPLET" "sudo -u deploy bash -lc 'cd $APPDIR && docker compose -f docker-compose.prod.yml up -d --build'"

echo
echo "✓ Deployed. Checks:"
echo "    ssh $DROPLET 'sudo -u deploy docker compose -f $APPDIR/docker-compose.prod.yml ps'"
echo "    ssh $DROPLET 'sudo -u deploy docker logs poefissure-cloudflared --tail 20'"
echo "    curl -I https://poe-fissure.bltech.app   # 200 once DNS propagates"
