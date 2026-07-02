# TODO — poe-fissure

> Seed backlog synthesized from docs and commit history — only genuinely-open work (shipped items excluded).
> One checkbox = one GitHub issue. After import, GitHub Issues are the source of truth:
> https://github.com/pshima/poe-fissure/issues

## Backlog

- [ ] Implement a PoE2DB-backed ModResolver for real crafting tiers/ilvl-gates/weights
  - labels: craft, backend, enhancement
  - priority: medium
  - Body: `internal/craft/resolver.go` defines the `ModResolver` interface but only ships `nullResolver` (phase A — every lookup returns "unknown", so `internal/craft/rate.go` falls back to heuristics). Add the promised phase-B `poe2dbResolver` behind the same interface (no call-site changes): fetch per-base modifier facts (tier ranges, ilvl gates, roll weights, full mod pool) from PoE2DB and populate `Tier`/`IlvlGate`/`Weight`/`PoolFor`. Acceptance: a real resolver satisfies `ModResolver`, `rate.go` uses it when available, and crafting ratings reflect actual mod tiers/ilvl gates instead of heuristics.

- [ ] Validate PoB2 export against the real PoB2 GUI and fix XML/item-text generation
  - labels: pob, backend
  - priority: medium
  - Body: PoB2 code XML generation is documented as best-effort (only the codec in `internal/pob/code.go` is round-trip tested). Export a code (`poefissure pob Frozenmulligan`), paste into PoB2 → Import → From Code, and fix any items/tree/skills that fail to load cleanly in `internal/pob/build.go` (`BuildXML`, `ItemText`). Acceptance: an exported code imports into the PoB2 GUI with gear, passive tree, and skills all populating correctly.

- [ ] Expand upgrade mod-parsing regexes
  - labels: analysis, data
  - priority: low
  - Body: Mod regexes in `internal/analysis/stats.go` cover common mods only. Add patterns for mods that matter to real builds (e.g. chaos resistance on gear, hybrid/defensive mods) so the scorer and trade filters recognize them. Acceptance: added mod types are parsed into the correct `Stat*` keys with a test case per new pattern.

- [ ] Refine the broad weapon/offhand trade category fallback
  - labels: trade
  - priority: low
  - Body: `SlotCategory` in `internal/trade/url.go` returns "" (no category filter) for weapon/offhand slots, deferring to `WeaponCategory`, which only recognizes a fixed set of weapon bases. Broaden `WeaponCategory` coverage (more weapon base types, quiver/focus/shield offhands) so upgrade searches are category-scoped rather than broad. Acceptance: common poe2 weapon/offhand bases map to a `weapon.*`/`armour.*` category, with tests in `internal/trade/category_test.go`.

- [ ] Add a configurable poll interval flag to `char watch`
  - labels: cli
  - priority: low
  - Body: `char watch` hard-codes `interval := 60 * time.Second` in `cmd/poefissure/main.go`. Add a flag (e.g. `--interval`) so the poll cadence is user-settable while staying rate-limit aware. Acceptance: `poefissure char watch <name> --interval=<dur>` uses the supplied interval; default remains 60s.

- [ ] Add a `--budget` override flag to `upgrades`
  - labels: cli
  - priority: low
  - Body: The upgrade budget comes only from config (`a.cfg.Budget` in `cmd/poefissure/main.go`). Add a CLI flag to override amount/currency for one-off runs. Acceptance: `poefissure upgrades --budget=<n>` overrides the config budget for that invocation.

- [ ] Fix stale README claim that trade stat-ids aren't bundled
  - labels: docs
  - priority: low
  - Body: `README.md` ("Trade stat filters") states stat ids are "not bundled", but `internal/trade/default_stat_ids.json` is now embedded via `//go:embed` in `internal/trade/statids.go` (a common set of ~11 ids overlaid with the user's optional file). Update the README to say a default set ships and the user file only augments it. Acceptance: README matches actual behavior of `LoadStatIDsMerged`.

## Backlog / parked

- [ ] Real DPS/EHP scoring via a headless PoB2 engine
  - labels: analysis, backend
  - priority: low
  - Body: The upgrade scorer in `internal/analysis/scorer.go` is an explicit heuristic, not real DPS/EHP — no headless PoB math engine exists. Longer-term path is driving PoB2's Lua engine headlessly; the scorer interface is already isolated so it can be swapped. Parked (large, exploratory). Acceptance: a PoB-backed scorer produces real DPS/EHP numbers behind the existing scorer interface.

- [ ] Store the OAuth token in the OS keychain
  - labels: auth
  - priority: low
  - Body: Tokens are stored as a `0600` file (`internal/auth/store.go`). Optionally back token storage with the OS keychain for stronger at-rest protection. Parked (nice-to-have). Acceptance: token store can use the OS keychain, falling back to the `0600` file where unavailable.

- [ ] Register poe-fissure's own GGG OAuth client
  - labels: auth, ops
  - priority: low
  - Body: The tool currently rides on the public `pob` client id (works out of the box, but GGG could rotate/revoke it). The clean long-term path is emailing `oauth@grindinggear.com` for a dedicated public (PKCE) client with `authorization_code`+`refresh_token` grants and `account:characters` scope, then setting `client_id` in config (see README "Registering your own client"). Parked (external approval, ~1–4 weeks). Acceptance: an owned `client_id` authenticates login end to end.
