# Bug 9 + Bug 10 — Export view UI polish (archived 2026-09-08)

## Bug 9 — Per-column filter UI placement + placeholders (export view)

**Observed**: the per-column filter feature (export view, both apps) had
poor UI: the "Filters" toggle button sat in the top-right toolbar next to
Exports/CSV (creaves) resp. CSV/All reports (console) — crammed and
detached from the table it controls; in creaves it rendered above an input
box. Filter input placeholders were the generic word "Filter" in every
column.

**Fix**:
- Moved `#toggleColumnFilters` into a dedicated left-aligned row directly
  above the table (all 5 templates: console en/fr/de/nl + creaves).
- Each filter input now renders `placeholder="<column name>"` (aria-label
  keeps the localized "Filter <col>" prefix).

## Bug 10 — "Back to list" link placement (export view)

**Observed**: the link back to the export list sat far from the page title,
mixed with the download actions (creaves: `← Exports` in the right toolbar;
console: `← All reports` button in the right column).

**Fix**:
- Console: back anchor moved inside the `<h1>` before the title text
  (localized title attr: Back to reports / Retour aux rapports / Zurück zu
  den Berichten / Terug naar rapporten); removed from the right `col-md-4`.
- Creaves: back anchor moved inside the `<h3>` before the title text;
  removed from the `text-right` toolbar.

## Validation

- Unit tests (both repos) assert: button renders before the table,
  placeholders show rendered column names (`Espèce`), no generic "Filter"
  placeholder, back link inside the title element, toolbar back button gone.
- Suites green: `go test ./...` (creaves) and
  `CGO_ENABLED=1 go test -tags sqlite ./...` (console).
- Quality gates: `go vet` clean both repos; `staticcheck` clean on console,
  pre-existing unrelated warnings only on creaves (grifts dot imports,
  models log reimport); `go test -race -cover` green both repos.
- e2e (agent-browser, console :3001, register view): back link inside the
  title navigates to `/export/reports`; toolbar holds only "Download CSV";
  toggle reveals 13 inputs with column-name placeholders (Année, N°,
  Espèce, ...); filtering Espèce=Hérisson narrows 10,046 → 3,540 rows.

## Commits

- creaves-console `e396cf8` — fix(export): filter button above table,
  column-name placeholders, back link in title (bugs.md 9+10)
- creaves `8d78d3c` (branch feature/i18n) — same title
