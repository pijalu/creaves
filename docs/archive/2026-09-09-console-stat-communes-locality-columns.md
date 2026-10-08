# Bug 9 — Console stat_communes Excel export missing location columns

Archived: 2026-09-09. Fixed in creaves `d58532b` + creaves-console `d3032af`.

## Observed

`http://localhost:3001/export/excel?query=stat_communes&instance_id=LaGrange`
produced an incomplete workbook compared to the creaves equivalent
(`http://localhost:3000/export/excel?query=stat_communes`). Creaves exports all
fields:

```
Année  N°  Espèce  Date d'entrée  Cause de découverte  lieux de découverte  localité  Code postal  Commune  Province  Région  Pays  Cantonnement  Direction
```

Console was missing:

```
Commune  Province  Région  Pays  Cantonnement  Direction
```

These are geographic/administrative fields derived from the discovery city on
the creaves side (localities reference table). The webhook `animal_state`
payload did not carry them, so the console export emitted empty-string
placeholders.

## Fix — resolve-on-producer (extend webhook payload)

Each creaves instance resolves its own `localities` reference table at event
production time; the console stays reference-data-free. Payload-only contract
extension — `CanonicalStateContent` hashes deliberately unchanged on both
sides, so in-flight ack logic is unaffected.

**creaves** (`d58532b`):
- `models/event_stream.go`: `DiscoveryPayload` += `LocalityCommune`,
  `LocalityProvince`, `LocalityRegion`, `LocalityCountry`,
  `LocalityCantonnement`, `LocalityDirection` (json `locality_*`).
- `actions/event_translations.go`: `translationPreloader` indexes localities
  by `Locality` name for the animals' discovery-city set; `localityFor(city)`.
- `actions/event_producer.go`: `buildEventPayloadInto` resolves
  `discovery.city == localities.locality` (mirrors the stat_communes export
  `LEFT JOIN`), fills the 6 fields; unknown city → all empty (NULL-equivalent).
- `actions/event_producer_test.go`: `TestBuildEventPayload_LocalityResolution`
  asserts all 6 fields + unknown-city-empty case.
- `AGENTS.md`: webhook payload example updated.

**creaves-console** (`d3032af`):
- `models/event_stream.go`: same 6 payload fields.
- `models/consolidated_animal.go`: 6 `nulls.String` columns
  (`discovery_commune` … `discovery_direction`); `UpdateFromPayload` maps
  non-empty values; `applyState` resets them with the other discovery fields.
- `migrations/20260909010000_add_locality_fields_to_consolidated_animals.*.fizz`.
- `excel/config/config.yaml` stat_communes: `IFNULL(a.discovery_commune,"")`
  etc. replace the `""` placeholders; header aliases unchanged (xlsx pivot
  templates bind to them).
- `actions/event_processor_test.go`: SQLite test schema += 6 columns.
- `models/consolidated_animal_test.go`: mapping test, empty-stays-NULL test,
  applyState-reset test.
- `AGENTS.md`: payload example + schema table updated.

## Migration-path finding (content-hash skip)

The console dedupes `animal_state` events by producer `StateHash`
(`actions/event_processor.go:110`), and creaves skips unchanged animals when
queueing resync events. Because the 6 new fields are intentionally NOT in the
hash, an incremental resync after the upgrade delivers nothing
(`events_skipped_unchanged: 10047`) and even a delivered state is a no-op.
Backfill requires either a console-side rebuild of the instance data or a
**forced full rebuild** resync from creaves (`/webhook_resync`, "Force full
rebuild" checkbox → `StartResync(force=true)`), which re-queues all states.

## Validation

### Unit / integration tests
- creaves: `go test ./actions -run TestBuildEventPayload` — ok (incl. new
  locality-resolution test).
- creaves-console: `CGO_ENABLED=1 go test -tags sqlite ./...` — all ok (incl.
  3 new mapping tests).

### Quality gates (run separately)
- creaves: `go vet` clean; `staticcheck` — only pre-existing baseline warnings
  (identical with/without change, verified via stash); `gocognit`/`gocyclo`
  pre-existing complexity on `buildEventPayloadInto` (84→100 / →49) and
  `newTranslationPreloader` (37→51 / →27), same if-guard pattern as existing
  per-field resolution — noted per guideline 7;
  `go test -count=1 -race -cover ./...` all ok.
- creaves-console: `go vet` clean; `staticcheck` clean; `gocognit`/`gocyclo`
  pre-existing `UpdateFromPayload` (53→59 / 58) — same pattern, noted;
  `CGO_ENABLED=1 go test -count=1 -race -tags sqlite ./...` all ok.

### E2E (agent-browser + MySQL inspection)

1. Console dev DB rebuilt (`DROP DATABASE consolidation; CREATE DATABASE …`;
   `buffalo pop migrate`; `buffalo task db:seed`; LaGrange instance row +
   webhook key re-inserted with bcrypt hash of the raw key from creaves
   `config.settings`).
2. creaves UI `http://localhost:3000/webhook_resync` → checked "Force full
   rebuild" → Start resync. Status JSON:
   `"status":"completed","total_animals":10047,"events_created":10047,
   "events_delivered":10047,"events_failed":0`.
3. Console DB:
   `SELECT COUNT(*) total, SUM(discovery_commune IS NOT NULL) FROM consolidated_animals`
   → `10047 / 9680` (367 unknown cities stay NULL, mirroring the creaves
   LEFT JOIN; creaves source query: 9971/10085 matched).
4. Console export `http://localhost:3001/export/excel?query=stat_communes`
   (agent-browser open → `~/Downloads/stat_communes.xlsx`): sheet6 header row
   `Année, N°, Espèce, Date d'entrée, Cause de découverte, lieux de
   découverte, localté, Code postal, Commune, Province, Région, Pays,
   Cantonnement, Direction`; 10047 data rows; sample row
   `2021, 1, Hérisson, …, Jodoigne, 1370, JODOIGNE, BRABANT WALLON, Wallonie,
   Belgique, NIVELLES, MONS`.
5. Creaves export `http://localhost:3000/export/excel?query=stat_communes`:
   identical header + same first-row locality values (`JODOIGNE / BRABANT
   WALLON / Wallonie / Belgique / NIVELLES / MONS`) — column parity between
   the two apps confirmed.
