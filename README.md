# poe-fissure

A Go tool that queries your **Path of Exile 2** character through the official API,
stores snapshots locally using GGG's canonical item schema, exports
[Path of Building 2](https://pathofbuilding.community/) codes, and generates
ToS-safe trade-site search URLs for gear upgrades.

Built for character **Frozenmulligan**, realm `poe2`, league **Runes of Aldur**.

## Two ways to run it

- **Web app** (`cmd/poefissure-server` + `web/`) — a password-gated React UI to view
  your gear, find upgrades with **live trade prices**, and get **step-by-step crafting
  advice** for a pasted item (0.5 / Runes of Aldur). Deploy it on a VM with Docker +
  Caddy — see [`deploy/README.md`](deploy/README.md). No GGG client registration: the
  server uses a refresh token minted locally and your POESESSID for trade.
- **CLI** (`cmd/poefissure`) — the original local commands (`auth`, `char`, `pob`,
  `upgrades`, `export`); also how you mint the refresh token for the server.

## Status

- ✅ OAuth 2.1 Authorization Code + PKCE (public client) — the same flow PoB2 uses.
- ✅ Canonical `Item`/`Character` schema, local JSON snapshot history.
- ✅ Rate-limit-aware API client (honors `X-Rate-Limit-*` / `Retry-After`).
- ✅ PoB2 import-code generation (deflate + URL-safe base64 XML).
- ✅ Heuristic upgrade scorer (DPS, survivability, resist/attribute caps, budget).
- ✅ Safe trade2 URL generation (no automated scraping).
- ✅ **Works out of the box** via the shared `pob` public client — no GGG approval needed.

## Getting access

The official API has no self-serve registration. But you don't need your own
registration to start: this tool defaults to `client_id: pob`, the public client
Path of Building ships. Because it's a PKCE **public** client it has no secret — the
id is plaintext in [PoB's source](https://github.com/PathOfBuildingCommunity/PathOfBuilding-PoE2/blob/dev/src/Classes/PoEAPI.lua),
shipped openly to millions of users — and GGG accepts any loopback redirect port for
public clients, so the same id authenticates this tool's own `http://localhost:<port>`
callback. The default config just works:

```sh
poefissure auth login   # opens the browser; Steam login works on pathofexile.com
```

You only request the `account:characters` scope (read your own characters/inventory),
a subset of what `pob` is approved for.

**Caveat:** `pob` is Path of Building's registered app identity, not ours. For a
personal, read-only, your-own-character tool this is low-harm — it's the same client
id and scope PoB hands every user — but GGG could rotate or revoke it, which would
break login until you switch ids. Do **not** point this at write scopes or abusive
request volumes under someone else's client.

### Registering your own client (optional, the clean long-term path)

To stop riding on `pob`, email `oauth@grindinggear.com` for your own. GGG rejects
low-effort/LLM-generated requests, so **write it yourself** and show you've read the
[developer docs](https://www.pathofexile.com/developer/docs). Request: Public client
(PKCE, no secret); grant types `authorization_code` + `refresh_token`; scope
`account:characters`; redirect URI `http://localhost:49082` (bare loopback, no path —
GGG exact-matches it, so register the exact port you set as `redirect_port`). When
approved, set `client_id` in your config to the new id. Approval takes ~1–4 weeks.

## Setup

```sh
go build -o poefissure ./cmd/poefissure
cp config.example.yaml "$HOME/Library/Application Support/poefissure/config.yaml"   # macOS
# defaults work as-is (client_id: pob); edit only to set your own client_id/contact
```

## Usage

```sh
poefissure auth login                 # browser OAuth (Steam login works)
poefissure auth status
poefissure char list                  # all your poe2 characters
poefissure char get Frozenmulligan    # fetch + store a snapshot, print gear
poefissure char watch Frozenmulligan  # poll on an interval (rate-limit aware)
poefissure pob Frozenmulligan         # print a PoB2 import code
poefissure upgrades --goals dps,survival,resists,attributes
```

## Trade stat filters (optional)

Trade2 stat filters need GGG's internal stat ids, which are **not** bundled (they go
stale). Generated URLs always include item category + price + online status; to add
stat filters, create `~/.config/poefissure/trade_stat_ids.json` mapping internal stat
keys to trade ids, e.g.:

```json
{ "life": "explicit.stat_3299347043", "fire_res": "explicit.stat_3372524247" }
```

Keys are the `Stat*` constants in `internal/analysis/stats.go`. Unmapped stats are
quietly omitted (the URL still works).

## Design notes & limitations

- **Trade is ToS-safe by design**: poe-fissure builds a query and emits a
  `pathofexile.com/trade2` URL you open in a browser. It makes **no** requests to
  GGG's undocumented trade endpoints.
- **No headless PoB math**: there is no programmatic PoB2 DPS/EHP engine. The Go
  scorer is a transparent heuristic ranker; paste the exported PoB2 code into the
  PoB2 GUI for exact numbers. The scorer interface is designed so a real engine could
  replace it later.
- Tokens are stored at `~/.config/poefissure/token.json` with `0600` permissions and
  are never logged or committed.

## Layout

```
cmd/poefissure       CLI
internal/config      YAML config
internal/auth        OAuth PKCE flow + token store/manager
internal/poeapi      API client + rate limiter + character endpoints
internal/schema      canonical Item/Character objects
internal/storage     local JSON snapshot history
internal/pob         PoB2 code encode/decode + build XML
internal/analysis    mod parsing + upgrade scorer
internal/trade       trade2 query builder + safe URL generation
```
