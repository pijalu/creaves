# Bug 5 — Exports (excel/view): single year dropdown incl. "all years", selectable in the view

Resolved 2026-09-22 · commit `feb9876` · part of bugs.md issue pijalu/creaves#202 follow-up.

## Observed

- `/export/excel` and `/export/view` offered **one year input / download-button pair per
  export row** — multiple selections per page, confusing UI.
- The online view (`/export/view?query=X&year=Y`) rendered the chosen year but it
  **could not be changed in place** — user had to go back to the chooser.

## Expected

- One single year selection at the **top** of both chooser pages: a dropdown listing
  the years present in the DB plus an **"All years"** entry (default). All
  View/CSV/Excel links use that single selection.
- On the online view page the year must be an **in-place dropdown**; changing it
  reloads the view for the new year immediately. CSV download link follows it.

## Fix plan

- `actions/export.go`: new `exportYears()` helper — distinct years present in the
  relevant table, sorted desc, for the dropdown options.
- Chooser templates (`templates/export/index.plush.html`, `excel.plush.html`):
  single `<select id="exportYear">` at top of page (default `""` = all years);
  per-row year inputs removed; JS rewrites each export link to append
  `?year=<selected>` on click.
- View page (`templates/export/view.plush.html`): when the query has a year column,
  render the year as a `<select>` that submits on change → reload view with the
  chosen year; CSV link always carries the dropdown value.
- Locales (`locales/export.{en-us,fr,de,nl}.yaml`): `yearAll` ("All years" /
  "Toutes les années" / "Alle Jahre" / "Alle jaren") + `yearLabel`.

## Test approach & validation

- Go: `actions/export_view_test.go` — chooser UI contains single top dropdown,
  view year filter still works, garbage `year=` is a no-op, unknown query → 404.
- E2E (agent-browser, `http://127.0.0.1:3000`, admin/admin):
  - `/export/excel` builds `?year=2024` when a year is chosen; bare URL with
    "All years" (default).
  - `/export/view` chooser has **no** per-row year inputs left.
  - In-view dropdown reloads with the chosen year (`?year=2026`) and back to
    all years on empty selection.
  - fr labels `Année` / `Toutes les années` verified.
- Quality gates: `go vet ./...`, `staticcheck ./...`, `gocognit -over 15 .`,
  `gocyclo -over 12 .`, `go test -count=1 -race -cover ./...` — all green.

All validation recorded in commit `feb9876` message.
