# Deploying poe-fissure

A single Docker image (React frontend + Go API), single-user, password-gated. **No
GGG client registration** — the server fetches your character with a refresh token
you mint locally, and prices trades with your POESESSID cookie.

Two supported targets:
- **DO droplet via the blt-infra `droplet-sfo` Cloudflare Tunnel** (recommended;
  Cloudflare handles TLS, no host ports) — see "Droplet deploy" below.
- **Standalone VM with Caddy** (auto-HTTPS on your own domain) — see "Standalone
  VM" further down.

---

## Droplet deploy (blt-infra `droplet-sfo` tunnel)

Uses `docker-compose.prod.yml` at the repo root (app + a `cloudflared` sidecar on a
private network). Follows the blt-infra "DO Droplet Apps" pattern.

### 1. Register the hostname in blt-infra (Terraform)

In `blt-infra/terraform/cloudflare/terraform.tfvars`, add to
`droplet_tunnel_ingress_rules`:

```hcl
{ hostname = "poe-fissure.bltech.app", service = "http://poefissure:8080" }
```

Then `source .env && terraform -chdir=terraform/cloudflare apply`. This creates the
tunnel ingress rule + CNAME automatically.

### 2. Prepare secrets on the droplet

Mint the GGG refresh token locally (`./poefissure auth login`), grab your POESESSID,
and have your local `.env.local` set (from `scripts/setup-local.sh`).

### 3. Ship the source and bring it up

Just run the helper from the repo root — it assembles `/opt/poefissure/app.env` from
your local secrets (tunnel token from terraform, refresh token from `token.json`,
password/POESESSID from `.env.local`), rsyncs the source, and builds + starts the
stack:

```bash
./scripts/deploy-droplet.sh
```

Verify (the script prints these): `docker compose ps` (both `Up`), `docker logs
poefissure-cloudflared` ("Connection registered"), and `curl -I
https://poe-fissure.bltech.app` → 200 once DNS propagates.

Note: secrets live in `/opt/poefissure/app.env` (not `.env`) and are passed to the
containers via `env_file` with no `${}` interpolation, so the `$` in the bcrypt hash
is safe. The rsync excludes are anchored to the repo root (`/data/`,
`/poefissure-server`) so they don't clobber `internal/craft/data` or
`cmd/poefissure-server`.

---

## Standalone VM (Caddy auto-HTTPS)

Uses `deploy/docker-compose.yml` + `deploy/Caddyfile` for a VM that owns ports
80/443 on your own domain.

## 1. Provision a VM

A small instance is plenty (1 vCPU / 1 GB). Install Docker + the compose plugin:

```sh
curl -fsSL https://get.docker.com | sh
```

Point a DNS **A record** (e.g. `poe.example.com`) at the VM's public IP, and open
ports **80** and **443**.

## 2. Mint a GGG refresh token (once, on your own machine)

The server never does interactive OAuth. Do the browser login locally and copy the
refresh token over:

```sh
go build -o poefissure ./cmd/poefissure
./poefissure auth login          # browser opens; Steam login works
```

Then copy the `refresh_token` value from the token file:
- macOS: `~/Library/Application Support/poefissure/token.json`
- Linux: `~/.config/poefissure/token.json`

## 3. Get your POESESSID (for trade pricing — optional)

Log in to `pathofexile.com` in your browser → DevTools → Application → Cookies →
copy the **POESESSID** value. (It expires periodically; when it does, the app shows
an "update POESESSID" error and you re-paste a fresh one.)

## 4. Configure

On the VM, clone the repo, then:

```sh
cp deploy/.env.example deploy/.env
# generate the login password hash and session secret using the image:
docker compose -f deploy/docker-compose.yml run --rm app hash 'your-password'
docker compose -f deploy/docker-compose.yml run --rm app gen-secret
```

Edit `deploy/.env`: set `DOMAIN`, `ACME_EMAIL`, `APP_PASSWORD_HASH`,
`SESSION_SECRET`, `POE_CHARACTER`/`POE_LEAGUE`/`POE_CONTACT`, `GGG_REFRESH_TOKEN`,
and (optionally) `POE_SESSID`.

## 5. Launch

```sh
docker compose -f deploy/docker-compose.yml up -d --build
```

Visit `https://your-domain`, log in with your password, and you'll see your gear.
Caddy provisions the TLS cert on first request.

## Operating it

- **Logs:** `docker compose -f deploy/docker-compose.yml logs -f app`
- **Update:** `git pull && docker compose -f deploy/docker-compose.yml up -d --build`
- **Refresh POESESSID:** edit `deploy/.env`, then
  `docker compose -f deploy/docker-compose.yml up -d` (recreates the app container).
- **Data** (snapshots, token) lives in the `data` volume; it survives rebuilds.

## What runs where

| Concern | Mechanism |
|---|---|
| Site login | single password → signed session cookie (`APP_PASSWORD_HASH`, `SESSION_SECRET`) |
| Character data | GGG API via non-interactive refresh-token grant (`GGG_REFRESH_TOKEN`, `pob` client) |
| Trade pricing | on-demand `api/trade2` with `POE_SESSID`, rate-limit aware |
| Crafting advisor | fully offline (embedded 0.5 knowledge base) |
| HTTPS | Caddy, automatic certificate for `DOMAIN` |

## Trade stat filters (optional)

To attach stat filters to trade searches, drop a `trade_stat_ids.json` mapping into
the data volume at `/data/trade_stat_ids.json` (see the repo README). Unmapped stats
are omitted and reported in the result.
