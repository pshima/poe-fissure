# poe-fissure — Next Steps

Pick-up notes as of 2026-06-04. Everything below the "What's done" line is built,
tested (`go vet ./...` + `go test ./...` green), and verified offline against the
fixture. The blocker to going live is the GGG OAuth approval (see step 1).

---

## ▶ Start here when you come back

1. **Send the OAuth registration email (the only hard blocker).**
   - To: `oauth@grindinggear.com` — **write it yourself**, GGG rejects LLM-generated requests.
   - Request: PoE account name (owner of Frozenmulligan), app name `poe-fissure`,
     **Public client**, grant types `authorization_code` + `refresh_token`,
     scope `account:characters` (justify: read own character for build tracking/upgrades),
     redirect URI `http://localhost:49152/callback`.
   - Full draft is in `README.md` → "Getting access". Approval ~1–4 weeks.

2. **When the `client_id` arrives**, create the config and drop it in:
   ```sh
   cp config.example.yaml "$HOME/Library/Application Support/poefissure/config.yaml"
   # edit: set client_id (and confirm contact, character, league)
   ```

3. **Run the live flow** (this is the first real integration test):
   ```sh
   go build -o poefissure ./cmd/poefissure
   ./poefissure auth login          # browser opens; Steam login works here
   ./poefissure char list           # confirm Frozenmulligan appears on realm poe2
   ./poefissure char get Frozenmulligan   # fetch + snapshot + print gear
   ```
   - **If `char get` errors**, the most likely cause is the exact league/realm or the
     character-name path encoding. Check the raw response and adjust
     `internal/poeapi/characters.go`.

4. **Validate PoB2 export against the real GUI** (highest-risk piece):
   ```sh
   ./poefissure pob Frozenmulligan
   ```
   Paste the code into PoB2 → Import → From Code. If items/tree/skills don't load
   cleanly, fix the XML/item-text generation in `internal/pob/build.go`
   (`BuildXML` and `ItemText`). The codec itself (`code.go`) is round-trip tested.

5. **Populate trade stat ids** so upgrade URLs include stat filters:
   - Create `~/.config/poefissure/trade_stat_ids.json` mapping internal stat keys
     (the `Stat*` consts in `internal/analysis/stats.go`) → PoE2 trade stat ids
     (e.g. `"life": "explicit.stat_3299347043"`).
   - Get ids from the trade2 data/stats endpoint or by inspecting a query on the
     trade site. Unmapped stats are silently omitted today.
   ```sh
   ./poefissure upgrades --goals dps,survival,resists,attributes
   ```

---

## ✅ What's done (no action needed)

- **OAuth PKCE public-client flow** — `internal/auth` (pkce, oauth, store, manager).
- **API client + rate limiting** — `internal/poeapi` (honors `X-Rate-Limit-*` / `Retry-After`).
- **Canonical Item/Character schema** — `internal/schema` (incl. poe2 skills/runeMods/passives).
- **Local JSON snapshot history** — `internal/storage`.
- **PoB2 code encode/decode + build XML** — `internal/pob` (codec tested; XML best-effort).
- **Heuristic upgrade scorer** — `internal/analysis` (DPS, survivability, resist/attr caps, budget).
- **Safe trade2 URL generation** — `internal/trade` (query in `q` param; no scraping).
- **CLI** — `cmd/poefissure` (`auth`, `char`, `pob`, `upgrades`).

---

## 🔧 Known limitations / backlog (improve later)

- **Scorer is a heuristic, not real DPS/EHP.** No headless PoB engine exists. If you
  want accurate numbers, the longer-term path is driving PoB2's Lua engine headlessly;
  the scorer interface is isolated so it can be swapped.
- **Trade stat-id map is empty by default** (see step 5) — ids aren't bundled because
  they go stale.
- **Weapon category is left broad** in `internal/trade/url.go` (`SlotCategory`) to avoid
  guessing wrong weapon subtypes — refine once you know the build's weapon class.
- **`char watch` interval is hard-coded to 60s** in `cmd/poefissure/main.go` — make it a flag if needed.
- **No `--budget` flag yet**; budget comes from config. Add a CLI override if handy.
- **Mod regexes** in `internal/analysis/stats.go` cover common mods only; add patterns
  as you hit ones that matter for the build (e.g. chaos res on gear, hybrid defenses).
- **Token storage** is a `0600` file; consider OS keychain later.

---

## Quick reference

- Config: `~/Library/Application Support/poefissure/config.yaml` (macOS).
- Token: `~/Library/Application Support/poefissure/token.json` (`0600`).
- Snapshots: `~/Library/Application Support/poefissure/snapshots/<character>/*.json`.
- Build/test: `go build ./...`, `go vet ./...`, `go test ./...`.
- Fixture for offline work: `internal/schema/testdata/character_frozenmulligan.json`.
