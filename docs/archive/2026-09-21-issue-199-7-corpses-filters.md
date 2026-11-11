# Issue #199-7 — `/reports/corpses`: filters, default unmarked, mark-once, translations

Archived: 2026-09-21 — commit `3d1d6c1` on branch `feature/open-issues-2026-10`.

## Observed
- No search/per-column filters on the corpse register.
- Marked and unmarked corpses mixed in one long list.
- Already-marked corpses could be re-marked/updated (checkbox + Mark
  button stayed active; server accepted overwrite).
- Mark/unmark flash messages hard-coded in English on FR/DE/NL.

## Fix
- `actions/reports_corpses.go`:
  - `corpseFilters` (f_number, f_species, f_destination, f_by +
    `marked` = unmarked|marked|all, default unmarked), applied in Go on
    the year-scoped rows (`keep`/`apply`, case-insensitive `containsFold`).
  - `ReportsCorpsesMark`: mark-once — already-marked outtakes are skipped
    (warning flash) instead of overwritten. Loop extracted to
    `markCorpseOuttakes`; redirect logic shared as `corpseRedirect`.
  - Flash messages via i18n keys `reports.corpses.marked.flash`,
    `reports.corpses.unmarked.flash`, `reports.corpses.marked.skipped`.
- `templates/reports/corpses.plush{,.fr,.de,.nl}.html`: GET filter bar
  above the table; marked rows render a disabled checkbox and no Mark
  button; check-all selects only enabled boxes.
- `locales/reports.{en-us,fr,de,nl}.yaml`: flash + filter labels.

## Verification
- Go tests: `TestCorpseRegisterFiltersAndMarkOnce` (default hides marked,
  marked=all shows, species filter narrows, re-mark is a no-op);
  `TestCorpseRegisterMark` updated to request `marked=all`.
- e2e (agent-browser, dev DB, year 2025 = 1429 dead rows): marked a row →
  default view 1429→1428 and destination hidden; `marked=all&f_number=1`
  shows row with disabled checkbox, no Mark button, admin Unmark; direct
  re-mark POST kept original destination + localized warning flash; FR
  filter labels and FR unmark flash verified. Test mark unmarked
  afterwards (data restored).
- Gates: `go vet`, `staticcheck` clean; gocognit 78 = baseline; gocyclo
  64 vs 63 baseline (+1: `TestCorpseRegisterMark` test growth, test-only).
  Full `go test -count=1 -race -cover ./...`: all pass except documented
  pre-existing grifts failures (parity drift, migrations replay).
