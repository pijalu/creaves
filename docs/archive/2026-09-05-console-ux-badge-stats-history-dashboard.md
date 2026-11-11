# Fix archive — 2026-09-05 — console UX batch: badges, stats consistency, event history, dashboard parity

Batch closing bugs.md items 7, 8, 9 and 10 (console UX, MEDIUM/LOW).

## Original reports (bugs.md #7–#10)
> ## 7. creaves-console: outcome/status presentation — MEDIUM
> **Observed:** Outcome cell renders badges jammed together: "ReleasedNegative DCD"
> (no separator between status badge and outcome badge, `<small>` value glued);
> status column shows "Released" for animals whose outcome is DCD/Euthanized
> (negative outcomes); "DCD"/"Euthanasier" untranslated. Existing TODO to rename
> outtake→"outcome" and adopt negative/neutral/positive outcome semantics still open.
> **Expected:** visual separation between status and outcome badges; deceased animals
> not labelled "Released"; outcome terminology per existing TODO.
>
> ## 8. creaves-console: animal totals inconsistent across screens — MEDIUM
> **Observed:** dashboard "Animals by Instance" 2763, reports index "Total Animals"
> 2852, instance page "Animals" 2939, sync_management 3026-3286; dashboard/reports
> used different WHERE scopes.
> **Expected:** one shared stats source; all screens agree for a given instant.
>
> ## 9. creaves-console: animal detail event history + drill-down — LOW
> **Observed:** event history shows raw type `animal_state`, unformatted timestamp
> "2026-09-04 13:02:28", empty Source column; `/drill_down` is a 1:1 duplicate of the
> show page.
> **Expected:** localized/pretty event type, formatted date, meaningful source
> (import run / resync id), drill-down either removed or made useful.
>
> ## 10. creaves-console: EN dashboard lacks stat cards present in FR/DE/NL — MEDIUM
> **Observed:** EN `/` dashboard shows only by-status/by-instance tables + links;
> FR/DE/NL dashboards additionally show 4 stat cards.
> **Expected:** feature parity across all 4 localized layouts.

## Fix — item 7 (badge separation + outcome semantics)
- `models/consolidated_animal.go`: new `EffectiveStatus()` — returns `"died"` when
  the stored outcome is negative, else `CurrentStatus`. Legacy rows carrying
  `current_status='released'` plus a negative outcome now render as died, not
  Released.
- `actions/animal_sort.go`: new `statusClass(v, help)` plush helper —
  in_care→`success`, released→`primary`, died→`danger`, else `secondary` — computed
  from `EffectiveStatus`.
- `actions/render.go`: helper registered.
- `templates/consolidated_animals/index.plush.{html,fr,de,nl}.html` and
  `show.plush.{html,fr,de,nl}.html`: status badge
  `class="badge badge-<%= statusClass(animal) %> mr-1"` with
  `tstatus_localized(animal.EffectiveStatus())`; outcome badge given its own
  `mr-1` spacing; outtake type rendered as `<small class="d-block text-muted">`
  block instead of glued text. Badge classes/colors are locale-independent; only
  labels localize (Died/Décédé/Verstorben/Overleden, Negative/Négative/Negativ/Negatief).

## Fix — item 8 (single shared stats source)
- `actions/consolidated_stats.go` (new): `CountConsolidatedAnimals(tx, instanceID)`
  and `CountEventStreams(tx, instanceID)`; empty instance id = global count.
- Rewired every duplicate count site to the shared helpers:
  `DashboardIndex` + `ReportsIndex` (`dashboard.go`), `loadInstanceAdminView`
  (`instances.go`), `SyncManagementIndex` (`sync_management.go`). The refactor also
  removed the per-site `if scope.IsGlobal() {…} else {…}` branches (DashboardIndex
  gocognit 21→17, gocyclo 19→17 — improved, not worsened).

## Fix — item 9 (event history + drill-down)
- `actions/event_display.go` (new): localized event-type labels and badge classes in
  en-US/fr/de/nl (`Discovered`/`Découverte`/`Fund`/`Vondst`, `Released`/`Relâché`/
  `Freigelassen`/`Vrijgelaten`, `State snapshot`/`Instantané d'état`/
  `Zustands-Snapshot`/`Statussnapshot`, etc.); `eventSource` renders
  `"Resync <run-id[:8]>"` for events carrying `resync_run_id`, localized live label
  otherwise; `buildEventTimeline(events, lang)` produces a per-event payload diff
  (vs previous payload) over 14 tracked fields with localized field labels.
- `models/event_stream.go`: `ResyncRunID *uuid.UUID` column.
- `actions/webhook.go`: wire field `resync_run_id` (omitempty) parsed on fresh
  ingest; backfill onto already-stored live events on resync redelivery via
  `backfillResyncRunID` helper.
- `migrations/20260907010000_add_resync_run_id_to_event_streams.{up,down}.fizz` +
  `migrations/schema.sql` + sqlite test schema.
- `templates/consolidated_animals/show.plush.*`: formatted dates
  (`02/01/2006 15:04`), localized type badges, populated Source column.
- `templates/consolidated_animals/drill_down.plush.*`: page repurposed as
  "Event Timeline" (FR "Frise chronologique des événements", DE
  "Ereignis-Zeitstrahl", NL "Gebeurtenistijdlijn") rendering the payload diffs with
  `(empty)/(vide)/(leer)/(leeg)` placeholders.
- `creaves/actions/webhook_pusher.go`: producer fills `resync_run_id` on the wire
  for resync-delivered events; live events omit it (contract v2). Wire struct and
  `newWireEvent` mapping hoisted to package scope (keeps `deliverBatch` at its
  pre-existing complexity baseline 33/32).

## Fix — item 10 (EN dashboard stat cards) — verified stale
- Static inspection: all four `templates/dashboard/index.plush{,.fr,.de,.nl}.html`
  already contain the same four stat cards (`total_animals`, `total_events`,
  `unique_instances`, `active_webhook_keys`) — the drift was repaired during the
  item-6 i18n pass before this batch started. No template change required.
- Regression guard added: `actions/dashboard_template_parity_test.go`
  (`TestDashboardTemplatesStatCardParity`) parses all four dashboard templates and
  asserts the localized variants render exactly the en-US master's stat-card keys,
  in order. RED-proven by deleting one FR card (fails: "fr template renders 3 stat
  cards, en-US master has 4").

## Tests (all RED-first where feasible)
- `actions/badge_semantics_sqlite_test.go` (console, new): unit table for
  `EffectiveStatus` (released/positive → released; released/negative → died;
  in_care → in_care) + served index page: the row containing the seeded
  deceased-but-released animal shows a `badge-danger` "Died" badge, a separate
  outcome badge, `mr-1` separators and no jammed "ReleasedNegative". RED-proven
  (compile failure before `EffectiveStatus` existed).
- `actions/stats_consistency_sqlite_test.go` (console, new): mounts `/`, `/reports`,
  `/instances`, `/sync_management`, `/consolidated_animals` on one sqlite fixture
  (4 animals center-a + 2 center-b) and asserts the rendered total is 6 on every
  page at the same instant.
- `actions/event_history_sqlite_test.go` (console, new): served show page asserts
  localized labels ("Discovered"/"Released"/"State snapshot"), no raw `animal_state`,
  no raw `YYYY-MM-DD HH:MM:SS` timestamps, Source "Live update" and
  "Resync <run-id[:8]>" on the event carrying a run id; drill_down asserts timeline
  content. `TestWebhookStoresResyncRunID` posts a raw webhook envelope and asserts
  the stored event keeps its `resync_run_id` (querying by dedicated event id to
  stay immune to test.db leftovers).
- `actions/webhook_pusher_test.go` (creaves, new): `TestDeliverBatch_ResyncRunIDOnWire`
  and `TestDeliverBatch_LiveEventOmitsResyncRunID` pin the contract-v2 wire behavior.

## Validation
- Console: `go vet ./...` clean; `staticcheck ./...` clean (replaced deprecated
  `strings.Title` with unicode-safe `titleizeField`);
  `CGO_ENABLED=1 go test -tags sqlite -count=1 -race -cover ./...` green
  (actions 59.5%, models 58.5%).
- Console complexity: `gocognit -over 15 .` / `gocyclo -over 12 .` findings all at
  or below the pre-batch baseline (verified against a `git worktree` at 65f2806);
  the two functions this batch touched improved (DashboardIndex 21→17 cog / 19→17
  cyc). Remaining warnings are pre-existing and untouched: `UpdateFromPayload` 38/37,
  `installSafePopTxLogger` 25/13, `TestOutcomeHelpersAcceptValueAndPointer` 23,
  `LocalizedField` 21, `SyncManagementIndex` 16/14, `applyConsolidatedAnimalFilters`
  14, `UpsertByInstanceID` 13, `EventsDeleteCreate` 13.
- Creaves: `go vet ./...` clean; `staticcheck ./...` output byte-identical to the
  d7b4811 baseline (all findings pre-existing); `go test -count=1 ./...` green;
  `go test -count=1 -race -cover ./...` fully green (actions/excel/export/grifts/
  models/feeding/utils all ok — the previously documented timing-dependent pusher
  races did not reproduce).
- Browser (console :3001, agent-browser, all four locales via `/lang/?lang=…`):
  - Item 7: deceased animal row renders `badge-danger mr-1` "Died/Décédé/Verstorben/
    Overleden" status badge, separate "Negative/Négative/Negativ/Negatief" outcome
    badge, "DCD" as muted small block — no jammed "ReleasedNegative".
  - Item 8: dashboard stat card 10046 = reports "Total Animals" 10046 = instance row
    "Animals" 10046 = sync_management "Total animals in database: 10046"; events
    23886 equal on dashboard and instance row. Reports outcome split consistent
    (4199 released + 5598 died = 9797 released-status; outcomes 4042+157+5598).
  - Item 9: detail pages show "State snapshot/Instantané d'état/Zustands-Snapshot/
    Statussnapshot", dates "04/09/2026 20:40", Source "Live update/Mise à jour
    directe/Live-Update/Live update"; drill_down timelines localized with field
    diffs (Espèce/Art/Soort …, vide/leer/leeg).
  - Item 10: dashboards in en/fr/de/nl all render 4 stat cards with identical
    numbers (10046 / 23886 / 1 / 2) and localized labels.
- Dev database migrated in place with `buffalo pop migrate up` (1 migration:
  add_resync_run_id_to_event_streams); schema.sql re-dumped (column now appears in
  ALTER position with COLLATE — dump of the real migration result).

## Commits
- Console: `6c3500f` (item 7), `ccf2c4a` (item 8), `c6d3f8e` (item 9 console side),
  `a74e96a` (AGENTS.md contract doc), plus this batch's quality/parity follow-up.
- Creaves: `9da4a0b` (item 9 wire side), `9ef236f` (AGENTS.md contract doc),
  plus this batch's complexity refactor.
