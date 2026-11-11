# Bug 4 — Reports: excel exports UI + run any report for a specific year (creaves + creaves-console)

**Status**: resolved 2026-10 — creaves commit `e81a9c8` (branch `feature/open-issues-2026-10`),
creaves-console commit `b94a5b5` (branch `main`), both pushed.

## Observed behavior

Excel exports were presented as a plain/ungrouped list with poor usability; reports could not be
run for a specific year (year selection missing or inconsistent between reports). Applied to both
`creaves` (`/export/excel` chooser was a bare `<ul>`; `/export/view` + `/export/csv` had no year
parameter) and `creaves-console` (`/reports` index, `by_location`, `by_type`, `/export/reports`
and `/export/excel` had no year filter).

## Expected behavior

- Excel exports organized in a nicer, better-looking UI (consistent layout, clear labels).
- Every report can be run for a specific year, consistently, in both projects and all four
  locales (en-US, fr, de, nl).

## Fix implemented

**Year-filter design (shared)**: every report query selects a year column under a per-query
alias (`"Année"` or lowercase `"année"`). The alias is declared per query
(`year_column` YAML field in creaves `export/config.yaml` + `excel/config/config.yaml`;
`YearColumn` field on the `exportQuery` registry in console `actions/export_queries.go`).
When a valid `?year=` (1900–2100, bind parameter) is present and the query declares a year
column, the SQL is wrapped: `SELECT * FROM (<sql>) AS year_filter WHERE year_filter.\`<alias>\` = ?`.
Invalid/absent years disable the filter. `animal_gavage` (creaves) has no year column and shows
no picker. Download filenames carry the year (`register_2024.csv`, `registre_detail_2024.xlsx`)
only when the filter actually applied.

**Creaves** (`e81a9c8`):
- `excel/excel.go` + `export/export.go`: `YearColumn` config field, `yearFilter`/`ParseYear`
  helpers, year-aware `RunQuery`/`FetchRows`, conditional filename suffix.
- `actions/export.go`: `ExportView` parses and exposes `year`.
- `/export/excel` rewritten as a Bootstrap table (Export | Year | Excel), per-row year input +
  download button + all-years link; fully i18n'd through a single template and new
  `locales/export.{en-us,fr,de,nl}.yaml` (the three locale template variants were removed).
- `/export/view` chooser: per-row year input applied by small JS to the View/CSV links;
  view page shows an active-year badge with an all-years reset and carries the year into the
  CSV link.

**Creaves-console** (`b94a5b5`):
- `actions/report_scope.go`: `ScopedWhereYear`, `parseReportYear`, `reportYearOptions`
  (dropdown built from `SELECT DISTINCT year FROM consolidated_animals`, scoped).
- `actions/dashboard.go`: `ReportsIndex`, `ReportsByLocation`, `ReportsByType` filter every
  stat query by the optional year (same pattern as the pre-existing `ReportsBySpecies`).
- `actions/export_reports.go`: `buildExportSQL` wraps with the year subquery; view shows a
  year badge + year input next to the center selector; CSV filenames carry `-<year>`.
- `excel/excel.go` (console pkg): same year subquery for `registre_detail`/`stat_communes`;
  `templates/reports/csv.plush.*` hub Excel cards carry a year input (GET form).
- All templates updated in the four locales (console keeps per-locale hardcoded variants).

## Tests

- creaves: `excel/excel_test.go` (`TestYearFilter`, `TestExcelConfigYearColumns`),
  `export/export_test.go` (`TestYearFilter`, `TestYearColumns_LoadedConfig` — every declared
  alias appears in its query; `animal_gavage` empty), `actions/export_view_test.go`
  (view/CSV/excel handlers filtered + unfiltered, filenames, chooser UI). All pass.
- console: `actions/export_reports_sqlite_test.go` — two-year fixture tests for view/CSV year
  filtering (row counts, filename suffix, invalid years ignored, every query runs with a year,
  index year inputs, dashboard year smoke tests); `actions/export_excel_sqlite_test.go` —
  `TestExportExcel_YearFilter` (pivot-cache row counts per year, scoped+year, invalid year),
  updated `TestReportsCSVIndex_RendersExcelLinks` for the new form markup. Full suite green
  (`CGO_ENABLED=1 go test -tags sqlite ./...`).

## E2E evidence (agent-browser)

- creaves (`:3000`, admin/admin):
  - `/export/excel` new table UI verified in en/fr/de/nl (headers Export/Année/Jahr/Jaar,
    placeholders "All years/Toutes les années/Alle Jahre/Alle jaren", localized
    Download/all-years actions).
  - `fetch("/export/excel?query=registre_detail&year=2024")` → 200,
    `filename="registre_detail_2024.xlsx"`.
  - `/export/view` chooser: 30 year inputs (31 rows minus `animal_gavage`).
  - `/export/view?query=register&year=2024` → badge "Year: 2024", CSV link
    `/export/csv?query=register&year=2024`; CSV content check: every data row starts with
    `2024` (2047 rows vs 10175 unfiltered).
- creaves-console (`:3001`, admin/admin123):
  - `/reports` year dropdown lists 2021–2026; `?year=2024` total 2047 vs 10175 unfiltered.
  - `by_location?year=2024`: 331 rows vs 624 unfiltered; `by_type?year=2024` renders with the
    dropdown.
  - `/export/reports`: 28 per-row year inputs; view `?year=2024` badge "Year: 2024", CSV link
    carries `&year=2024`; CSV 2048 lines vs 10176 unfiltered; every row year 2024.
  - `/reports/csv` hub: both Excel cards are GET forms with year inputs;
    `/export/excel?query=registre_detail&year=2024` → `filename="registre_detail_2024.xlsx"`.
  - Locale spot-checks: fr badge "Année : 2024", de index header "Jahr", dropdown labels
    localized in all four languages.

## Quality gates

Each run separately, both projects:
- `go vet ./...`: clean (both).
- `staticcheck ./...`: console shows only the two pre-existing `U1000` findings
  (`dataCacheReset`, `userCacheReset`), identical on the pre-change baseline; creaves clean.
- `gocognit -over 15 .` / `gocyclo -over 12 .`: no new hotspots (console identical to baseline;
  creaves `excel.RunQuery` was already listed, 18→20 gocognit, same function).
- `go test -count=1 -race -cover ./...`: console green (actions 59.0%, excel 63.4%, models
  56.1%). Creaves failures identical to the known 26-failure pre-existing baseline
  (24 actions + `TestTemplateVariantStructuralParity` + 1 env-flaky; template-parity NEW-drift
  list diffed against baseline — byte-identical, no new drift introduced).
