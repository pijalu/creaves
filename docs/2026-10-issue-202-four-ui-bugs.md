# Fix plan — GitHub issue pijalu/creaves#202 (4 bugs)

All work in `creaves/`. Constraints: production data — no destructive changes; all
UI changes applied to every supported language (base/en-US, fr, de, nl).

Quality gates (run per fix, not chained):
`go vet ./...`, `staticcheck ./...`, `gocognit -over 15 .`, `gocyclo -over 12 .`,
`go test -count=1 -race -cover ./...`

E2E: `agent-browser` CLI against `http://127.0.0.1:3000` (admin/admin), capture
command output + URL as evidence. Mobile-width check via browser viewport resize.

---

## Bug 1 — Dashboard frames not full-width on smartphone (`/dashboard/`)

**Root cause**: every dashboard section renders a bare `<table class="table ...">`
inside the layout's `div.container`. `.table` is `width: 100%` of the container
content box, but wide tables (min-content wider than a 375px viewport) overflow
the container → page-level horizontal scroll (measured: scrollWidth 727px at
375px viewport; tables 487/712/639px vs 345px content box) — frames appear
squeezed left / clipped on smartphones.

**Fix** (`templates/dashboard/dashboard.plush.{,fr.,de.,nl.}html`, 4 files × 8 tables):
- wrap each section `<table>` in `<div class="table-responsive">` (Bootstrap 4
  pattern: section-level horizontal scroll on narrow screens, full-width layout
  preserved; `w-100` redundant since `.table` is already `width: 100%`).

**Test approach**: template-only change → covered by e2e (no Go unit test).
**Validation**: e2e — login, open `/dashboard/`, `get html` each section wrapper
asserts `table-responsive` + `w-100` present on all 8 tables; resize viewport to
375px (iPhone) and assert layout wraps (table inside scroll wrapper), desktop
144px unchanged; page renders without 500/template error in all 4 languages
(`/lang/?lang=fr|de|nl`).

## Bug 2 — User show view displays raw `true`/`false` (`/users/<id>`)

**Root cause**: `templates/users/show.plush.html` lines for `Admin`, `Approved`,
`Shared` print `<%= user.Field %>` directly → Go boolean renders as `true/false`.
Other boolean role flags already use the `bool2html` ✓/× icon helper.

**Fix**:
- new plush helper `boolLabel(v bool, yes, no string) string` in
  `actions/render.go` (returns `yes` when true else `no`) + unit test
  `actions/render_test.go`.
- locale keys `users.yes` / `users.no` added to `locales/users.{en-us,fr,de,nl}.yaml`.
- template: `<%= boolLabel(user.Admin, t("users.yes"), t("users.no")) %>` for the
  three fields.

**Test approach**: Go unit test for `boolLabel`; render test via e2e.
**Validation**: e2e — open `/users/<id of admin user>`, assert page text contains
localized yes/no and not `true`/`false`; repeat with `/lang/?lang=de` (Ja/Nein),
`fr` (Oui/Non), `en-US` (Yes/No), `nl` (Ja/Nee).

## Bug 3 — Misleading helper text on `/export/excel/`

**Root cause**: `templates/export/excel.plush.html` reuses the `/export/` index
key `export.index.lead` whose sentence mentions "consultez-le en ligne" — wrong
for the Excel page (download only).

**Fix**:
- new locale key `export.excel.lead` in `locales/export.{en-us,fr,de,nl}.yaml`,
  download-only sentence per language.
- `templates/export/excel.plush.html` line 5 uses `t("export.excel.lead")`.

**Test approach**: template + locale only → e2e. Grep asserts index page still
uses `export.index.lead`.
**Validation**: e2e — open `/export/excel/` in each language, assert new sentence
shown and old "consult/view online" sentence absent; `/export/` (CSV view page)
still shows original sentence.

## Bug 4 — Missing filters on discoverers report (`/reports/discoverers`)

**Root cause**: `ReportsDiscoverersIndex` only passes `year`;
`SQL_DISCOVERER_ROWS` has no name/city/postal predicates; template form has only
the year select.

**Fix**:
- `actions/reports_discoverers.go`: `discovererFilters{Name, City, PostalCode}`
  struct; `listDiscovererRows(tx, year, filters)`; SQL gains three optional
  `? = '' or <col> like ?` conjuncts (name matches `concat(firstname,' ',lastname)`,
  city/postal prefix-insensitive `like` with `%term%`); handler reads+trims
  params, sets them back for the template; CSV export honours the same filters.
- `templates/reports/discoverers.plush.html`: filter form gains name/city/postal
  inputs + Apply button; CSV link carries active filters. Also fixes the typo
  `col-1O` → `col-10` on the year select wrapper.
- locale keys `reports.discoverers.filter.*` (name, city, postal_code, apply,
  reset) in `locales/reports.{en-us,fr,de,nl}.yaml`.
- Go test: filter behaviour of `listDiscovererRows` against seeded fixture rows
  (name substring match, city, postal, empty filters = all).

**Test approach**: new unit test `actions/reports_discoverers_test.go` + e2e.
**Validation**: e2e — open `/reports/discoverers`, count rows, enter a name
substring of a known discoverer, apply, assert only matching rows remain; clear
filters, assert full list returns; CSV export with filter contains only matching
rows; repeat one filtered view in `de` locale for label localization.

---

## Execution order

1. Bug 1 → gates → e2e → commit
2. Bug 2 → gates → e2e → commit
3. Bug 3 → gates → e2e → commit
4. Bug 4 → gates → e2e → commit
5. Final full gates + full e2e regression pass → archive this file to
   `docs/archive/` → empty `# To fix` in `bugs.md`.

## Issues found during work

- Bug 2: `templates/users/show.plush.html` has full locale variants
  (`show.plush.fr.html`, `.de.html`, `.nl.html`) — the fix had to be applied to
  all four files, not only the base template (all-language UI rule).
- Bug 1: measured root cause was page-level horizontal overflow (scrollWidth
  727px at 375px viewport), not width:auto shrink as first suspected; plan
  updated (plan §Bug 1).
