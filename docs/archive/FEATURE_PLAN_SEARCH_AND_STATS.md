# Feature Plan — Animal Search & Annual Statistics Report

Scope: `creaves/` (per-center app) and `creaves-console/` (consolidation app).
Languages: **fr (canonical), en-US, de, nl** — both apps localize via per-language
Plush template variants (`x.plush.html` = canonical, plus `.fr/.de/.nl` variants);
Creaves additionally uses `locales/*.yaml` keys for flash messages. Every new page
must ship 4 template variants in each app.

---

## 1. Review findings (current state, with evidence)

### Creaves

- **Animal list** `actions/animals.go:386 List()` — only supports "go to animal"
  (`animal_year_number`, `animal_id`). No field filtering. Paginated via
  `q := tx.PaginateFromParams(c.Params())`, ordered `ID desc`, enriched via
  `EnrichAnimals` (N+1; `EnrichAnimalsOptimized` exists and should be used).
- **Existing exports**:
  - `export/export.go` + `export/config.yaml` — YAML-driven raw-SQL CSV exports,
    uses `encoding/csv` (properly escaped) but writes the literal string `"null"`
    for SQL NULLs (`export.go:113-115`). Config already contains most statistical
    queries needed (see §3.2) but they are per-year grouped dumps, not a UI report.
  - `excel/` — Excel exports via excelize (out of scope).
  - `templates/registertable/registertable.plush.csv` — **broken CSV**: template
    string-concatenates `";"` separators with no CSV escaping (quotes/newlines in
    notes/reasons break rows; Plush `<%= %>` HTML-escapes instead of CSV-escaping).
    Same pattern in `registersnapshot` CSV. Rendered through
    `localrender.Csv` (`localrender/csv.go`).
- **Reports menu** (`templates/application.plush*.html`, Reports dropdown, lines
  ~64-76): Register, Snapshot, CSV exports, Excel exports. No stats page.
- **Year selector pattern** already exists: `actions/registertable.go:15-39`
  (`SQL_GET_REGISTER_YEARS`, `listRegisterYears`) — reuse.
- **Data model** (verified): search fields map to —
  - Year of entry → `animals.year` (or `YEAR(intakes.date)`; `animals.year` is the
    canonical register year)
  - Type → `animals.animaltype_id` → `animaltypes.name` (localized via `tname`)
  - Species → `animals.species` (canonical French name; localized via `tspecies`)
  - Reason for entry → `discoveries.entry_cause_id` → `entry_causes.cause/detail/nature`
  - Age → `animals.animalage_id` → `animalages.name`
  - Identification (ring) → `animals.ring` (free text, partial match)
  - Reason for exit → `outtakes.outtaketype_id` → `outtaketypes.name`
    (plus `outtaketypes.error` flag must exclude error rows, matching all
    existing export queries: `WHERE oo.error = 0 OR oo.error IS NULL`).

### Creaves Console

- **List** `actions/dashboard.go:111 ConsolidatedAnimalsIndex()` — already filters
  by `instance_id, status, species, animal_type, city, postal_code, year`.
  Missing: entry cause, age, ring, outtake reason. **All of these columns already
  exist** in `consolidated_animals` (`models/consolidated_animal.go:17-51`) →
  Console search needs **no schema change, no contract change**.
- **Reports** (`/reports`, `/reports/by_location|by_type|by_species` in
  `actions/dashboard.go:260+`): global or per-instance scope via
  `actions/report_scope.go`. Grouping uses canonical (French) values, display via
  `localizedGroupLabels` + `Translations` JSON column (`sync v2`).
- **No Reports link in the navbar** (`templates/application.plush*.html` — only
  Dashboard / Consolidated Animals / Admin). Must be added.
- **No CSV export at all** in the console. A shared CSV helper must be added
  (`encoding/csv`).
- **Stats report gap**: the required dimensions *class, AGW group, subsidies
  group, indigénat status, detailed reason for entry, nature of the cause* are
  **not present** in `consolidated_animals` nor in the webhook payload
  (`models/event_stream.go` both sides). → **Webhook contract change required in
  both repos** (§4).

### Percentages / duplicates

- All existing stats queries already use `COUNT(DISTINCT a.id)` —
  keep this everywhere ("must not take duplicate animals into account").
- The joins to `veterinaryvisits`/`travels` in `detail_register` show why
  dedup matters; new stats avoid such joins entirely.

---

## 2. CSV export fix (prerequisite, both apps)

**Problem**: `registertable.plush.csv` (and `registersnapshot` CSV) build CSV by
string concatenation — no escaping of `"`, `;`, newlines. `export/export.go` is
correct but emits `"null"` for NULLs.

**Plan**:

1. Add a shared CSV helper in Creaves, e.g. `actions/csv_export.go`:
   - `writeCSV(c buffalo.Context, filename string, header []string, rows [][]string)`
     using `encoding/csv` with `csv.Writer.Comma = ';'` (Excel-FR convention,
     matches existing exports), UTF-8 BOM prefix for Excel compatibility.
   - NULL → empty string (fix the `"null"` artifact in `export/export.go:113` too).
2. Replace `templates/registertable/registertable.plush.csv` (and snapshot CSV)
   rendering with Go-side CSV generation in `RegistertableIndexCSV`
   (`actions/registertable.go:42-73`) — build `[][]string` rows from enriched
   animals, drop `localrender.Csv` usage for these two endpoints.
3. Add the equivalent helper in console (`actions/csv_export.go`), `encoding/csv`
   based; use for all new exports there.
4. Tests: unit-test the helper (quotes, semicolons, newlines, UTF-8 accents,
   NULL handling). Creaves: `go test ./actions -run TestCSV`. Console:
   `CGO_ENABLED=1 go test -tags sqlite ./actions/...`.

---

## 3. Feature 1 — Animal search

### 3.1 Creaves — `/animals` search form + export

**Handler** (`actions/animals.go`, `AnimalsResource.List`):

- Keep existing `animal_year_number` / `animal_id` shortcuts.
- Add optional GET params (all combined with AND):
  | Param | Where clause | Notes |
  |-------|--------------|-------|
  | `year` | `animals.year = ?` | int |
  | `animaltype_id` | `animals.animaltype_id = ?` | UUID |
  | `species` | `animals.species = ?` | canonical name; select2 w/ `/suggestions/animal_species` |
  | `entry_cause_id` | join discoveries: `d.entry_cause_id = ?` | string ID |
  | `animalage_id` | `animals.animalage_id = ?` | UUID |
  | `ring` | `animals.ring LIKE ?` (`%v%`) | partial, case-insensitive collation is MySQL default |
  | `outtaketype_id` | join outtakes: `o.outtaketype_id = ?` AND `(oo.error = 0 OR oo.error IS NULL)` | UUID |
- Joins only when needed (entry cause / outtake type) to avoid row duplication;
  use `tx.Where("animals.discovery_id IN (SELECT id FROM discoveries WHERE entry_cause_id = ?)", …)`
  subquery style — simpler than JOIN + DISTINCT with Pop.
- Use `EnrichAnimalsOptimized` instead of `EnrichAnimals` (perf note in AGENTS.md).
- Load filter option lists: years (reuse `SQL_GET_REGISTER_YEARS`), animaltypes,
  animalages, entry_causes, outtaketypes — via existing `typehelper.go` loaders
  where available; add a cached loader for `entry_causes` and `outtaketypes` if
  missing (they already back admin pages).

**Export**: `GET /animals/search/export.csv` (registered explicitly in
`actions/app.go` next to line 120 resource, **before** the resource routes so
`/animals/search/export.csv` isn't swallowed by `:animal_id`) with the same
filters, **no pagination**, streamed via the new CSV helper. Columns (headers
localized per request language):
`Year, Number, Type, Species, Gender, Age, Ring, Intake date, Entry cause,
Entry cause detail, Exit date, Exit reason, Zone, Cage`.
Localized display values via existing `tname`/`tspecies` helpers.

**Templates**: extend `templates/animals/index.plush{,.fr,.de,.nl}.html`
(4 variants — base is French-canonical app: plain = en? **Note**: Creaves base
template language is French-canonical per `render.go` (`fr` → `""`); plain
`.plush.html` files are the French ones in practice — confirm during
implementation by checking `templates/animals/index.plush.html` vs `.fr` copy)
with a collapsible filter panel (GET form, sticky values), "Export CSV" button
preserving current query string. Add new locale keys (button/labels) to
`locales/animals.{fr,en-us,de,nl}.yaml` where flash messages/labels need them.

### 3.2 Console — `/consolidated_animals` search + export

**Handler** (`actions/dashboard.go:111`): add filters on existing columns:
`entry_cause` (LIKE or exact — decide: exact on stored canonical string),
`animal_age`, `ring` (LIKE), `outtake_type`. Feed the four new dropdown lists
via `SELECT DISTINCT …` like the existing species/type/city/year lists
(`dashboard.go:164-187`).

**Export**: `GET /consolidated_animals/export.csv` — same filters + scope
(`reportScope`), no pagination, via new CSV helper. Register route in
`actions/app.go` near line 171.

**Templates**: extend `templates/consolidated_animals/index.plush{,.fr,.de,.nl}.html`
(4 variants) with the new filters + export button. Display localized labels via
existing `LocalizedField(lang, field)` pattern.

---

## 4. Webhook contract change (required for Console stats report)

Console stats need species taxonomy + entry-cause detail/nature. Add to payload
(canonical French values, same rule as existing fields):

**Creaves** (`models/event_stream.go:43` `AnimalPayload`, `:57` `DiscoveryPayload`;
`actions/event_producer.go:70 buildEventPayloadWithTranslations`; resync builder
in `actions/webhook_resync.go`):

- `AnimalPayload`: + `SpeciesClass`, `SpeciesAGWGroup`, `SpeciesSubsideGroup`,
  `SpeciesNativeStatus` (from `species` table joined on
  `species.creaves_species = animals.species` — tolerate unknown species → empty).
- `DiscoveryPayload`: + `EntryCauseDetail`, `EntryCauseNature`
  (from `entry_causes`, already loaded as `animal.Discovery.EntryCause`).
- Extend `payload.Translations` map keys for the new fields so console can show
  localized group labels (mirror existing species/animal_type translations).

**Console** (`models/event_stream.go` — mirror structs;
`models/consolidated_animal.go`):

- **No migration needed** — the console has no production data yet. Add the new
  columns (`species_class, species_agw_group, species_subside_group,
  species_native_status, entry_cause_detail, entry_cause_nature`, all nullable
  varchar) **directly to the existing `create_consolidated_animals` /
  `add_consolidated_animal_fields` migrations** (edit the fizz files in place)
  and reset the dev/test DBs (`buffalo pop reset`). Add an index on
  `(year, instance_id)` for report queries. No new migration file, no down-file
  gymnastics.
- `UpdateFromPayload` + `applyState` (which clears fields on state snapshots —
  add the 6 new fields to the reset list at `consolidated_animal.go:108-113`).
- `LocalizedField` switch: add the new fields.

**Contract docs**: update payload JSON examples in both `AGENTS.md` files.

**Existing dev data**: after the schema reset, any rows still present from old
imports stay NULL for the new columns until a state resync (`/webhook_resync`).
Stats must group NULLs into an "Unknown" bucket so pre-resync data still totals
correctly.

**Compatibility**: `omitempty` everywhere → old Creaves → new Console and
new Creaves → old Console both keep working (fields simply absent/ignored).

---

## 5. Feature 2 — New statistics report (by selected year)

**This is a new report page** — not the existing annual statistics exports
(`export/config.yaml` per-year grouped dumps). The user picks a year (filter),
views the twelve tables below rendered on-screen, and can extract the results
(CSV export). Same functional spec in both apps; Creaves reads live tables,
Console reads `consolidated_animals` (after §4).

### 5.1 Routes & handler

**Creaves**: `GET /reports/annual?year=YYYY` + `GET /reports/annual/export.csv?year=YYYY`
(register in `actions/app.go` near line 159). New file `actions/reports_annual.go`.

**Console**: `GET /reports/annual?year=YYYY&instance_id=…` +
`GET /reports/annual/export.csv?…`. New file `actions/reports_annual.go`.
**Same twelve tables as Creaves, but scoped**: the user picks either
**all Creaves instances** (consolidated totals) or **one given instance**
(dropdown of instances, like the existing reports) — via the existing
`reportScope` helper (`actions/report_scope.go`). Default scope = all
instances.

### 5.2 Statistics set (year selector like registertable: dropdown of years present in DB, default = latest)

Total for year **T** = `COUNT(DISTINCT animal id)` of animals admitted in year Y
(Creaves: `animals.year = ?` + error-outtake exclusion; Console:
`year = ?` [+ scope]). Every table shows `category | count | % of T`
(%, 1 decimal; rows grouped NULL → "Unknown"/« Inconnu » bucket so columns sum to
100%). Categories:

| # | Report | Creaves source | Console source |
|---|--------|----------------|----------------|
| 1 | Top 20 entries by species | `animals.species` GROUP BY, LIMIT 20 | `species`, LIMIT 20 |
| 2 | By class | `species.class` | `species_class` (new col) |
| 3 | By AGW group | `species.agw_group` | `species_agw_group` (new) |
| 4 | By subsidies group | `species.subside_group` | `species_subside_group` (new) |
| 5 | By indigénat status | `native_statuses.status` via `species.native_status` | `species_native_status` (new) |
| 6 | By age | `animalages.name` via `animalage_id` | `animal_age` |
| 7 | By reason for exit — **by outtake type** | `outtaketypes.name` | `outtake_type` |
| 8 | By reason for exit — **by outcome rating** (alive/neutral/dead via `outtaketypes.rating`) | `outtaketypes.rating` | **not synced** → see note below |
| 9 | By reason for exit — **dead vs released** (`outtaketypes.dead` flag) | `outtaketypes.dead` | **not synced** → see note |
| 10 | Reason for entry | `entry_causes.cause` | `entry_cause` |
| 11 | Detailed reason for entry | `entry_causes.cause + detail` | `entry_cause_detail` (new) |
| 12 | Nature of the cause | `entry_causes.nature` | `entry_cause_nature` (new) |

The spec lists "By reason for exit" three times — rows 7–9 are the proposed
interpretation (existing exports `sortie_reason` / `sortie_types` /
`dead_register` support exactly this reading). **Confirm with the requester.**

For console rows 8–9, add `outtake_rating` (int) + `outtake_dead` (bool) to the
payload/outtake struct and `consolidated_animals` (same migration as §4) —
cheap to add while touching the contract. Otherwise console can only offer row 7.

**Creaves SQL pattern** (adapt existing `export/config.yaml` queries, e.g.
`animals_species` L366-378, `animals_class` L438-450, `AGW_group` L380-393,
`native_status` L480-493, `entry_age` L284-296, `sortie_reason` L298-309,
`entry_causes*` L495-540) — replace "all years grouped" with `WHERE a.year = ?`
parameter and add the `oo.error` exclusion consistently.

**Dedup**: one row per animal everywhere — base queries on `animals` (Creaves) /
`consolidated_animals` (Console), never join multi-row relations
(vet visits, travels); `COUNT(DISTINCT …)` as belt-and-braces.

### 5.3 UI

- New templates: `templates/reports/annual.plush{,.fr,.de,.nl}.html` in both apps
  (4 variants each).
- Year `<select>` (Creaves: reuse `listRegisterYears`; Console: existing
  `SELECT DISTINCT year` pattern from `ReportsBySpecies`, `dashboard.go:478`).
- **Console only**: instance `<select>` next to the year selector — "All
  centers" (default) + one option per registered Creaves instance (same
  pattern as existing `/reports` pages).
- One card/table per statistic: category, count, %; total row = T (100%).
- "Export CSV" button (top of page) preserving `year` (+ `instance_id`).
- Nav: add "Statistics (new)" item to the Reports dropdown in
  `templates/application.plush{,.fr,.de,.nl}.html` (Creaves — dropdown at
  lines 64-76) and add the whole Reports menu to the Console navbar
  (currently absent — new dropdown with Reports index + Annual statistics).

### 5.4 Export format

Single CSV file, sections separated by a blank line and a section title row:
`Section; Category; Count; Percentage` — or one file per section? **Proposal:
single file** with columns `Year, Section, Category, Count, Percent`, easier to
post-process. Headers localized. Via the §2 CSV helper (`;` separator, BOM,
proper escaping).

---

## 6. Implementation order (per app, cross-app where noted)

1. **CSV helper + escaping fix** (Creaves) — helper, rewrite register/snapshot
   CSV handlers, fix `"null"` in `export/export.go`, tests.
2. **Creaves search** — handler filters, option loaders, 4 template variants,
   export endpoint, locale keys.
3. **Creaves annual report** — `actions/reports_annual.go`, 12 queries, total +
   %, 4 template variants, nav links ×4, export.
4. **Contract change** — payload structs both repos, producer builders,
   console migration + `ApplyEvent`/`applyState`/`LocalizedField`, docs.
5. **Console CSV helper** + **Console search filters + export** (no schema
   dependency).
6. **Console annual report** — handler + queries on `consolidated_animals`,
   NULL→Unknown buckets, 4 template variants, navbar Reports menu ×4, export.
7. **Resync backfill** runbook for existing centers.

## 7. Testing & verification

Every number shown by the new pages must be validated against **known fixture
data** — not just "page renders". Unit tests prove query correctness; e2e tests
(agent-browser) prove the full stack (routes, templates, 4 languages, exports).

### 7.1 Unit tests

**Creaves** (MySQL dev+test) — `go test ./actions ./models`:

- **CSV helper** (`csv_export_test.go`): quotes, `;`, newlines, UTF-8 accents,
  NULL → empty (also covers `export/export.go` fix).
- **Search** (`animals_test.go`): build fixture set (≥2 years, ≥2 types,
  species with/without `species` taxonomy row, error-outtake rows, NULL age /
  ring / discovery). One test per filter + combination matrix (AND semantics),
  pagination preserved, `ring` partial match, error-outtakes excluded from
  `outtaketype_id` filter.
- **Stats queries** (`reports_annual_test.go`): fixture set with **known
  expected counts per category** — for each of the 12 tables assert exact
  counts and percentages (1 decimal), NULL → "Unknown" bucket, total row = T,
  `COUNT(DISTINCT)` dedup proven by fixtures with joined multi-row relations,
  error-outtakes excluded, empty year → empty tables (no crash, no NaN %).
- **Payload** (`event_producer_test.go` / extend existing): new fields present
  with canonical values, translations map extended, unknown species → empty
  strings, `omitempty` backwards compatibility (old payload JSON still parses).

**Console** (SQLite tests) — `CGO_ENABLED=1 go test -tags sqlite ./...`:

- **Event processor** (extend `event_processor_test.go`): new payload fields
  mapped to new columns, `animal_state` replace semantics clear the 6 new
  fields, translations stored.
- **Search filters** (extend dashboard tests): 4 new filters + scope.
- **Stats handler** (`reports_annual_test.go`): seed `consolidated_animals`
  directly with known values; assert exact counts/% per table; scope =
  all-instances vs single instance (seed 2 instances, verify totals differ and
  sum correctly); NULL → "Unknown".
- **CSV exports**: escaping + headers localized per request language.

### 7.2 E2E tests — agent-browser

Automated browser runs against a **seeded environment** with known data
(seed script committed, e.g. `buffalo task db:seed:e2e`, so expected numbers
are fixed and assertable). Both apps running (Creaves :3000, Console :3001),
console fed via real webhooks from Creaves (`webhook_e2e_test.go` pattern) so
the whole contract path is exercised.

**Creaves scenarios** (login admin/admin):

1. `/animals` — apply each filter + combinations; assert result table row
   count and specific animal numbers match fixture expectations.
2. Export CSV from a filtered search; assert file content (rows = on-screen
   results, headers in current language, proper escaping of fixture values
   containing `;` and `"`).
3. `/reports/annual` — switch years; assert every table's counts and %
   against the known fixture numbers; NULL category shows « Inconnu » /
   "Unknown" per language.
4. Annual export CSV — download, parse, assert all 12 sections and totals.
5. **All 4 languages**: repeat scenarios 1–4 spot-checks in fr, en-US, de, nl —
   labels localized, same numbers everywhere.
6. Nav links present in all 4 template variants.

**Console scenarios** (login, seeded instances A + B):

1. `/consolidated_animals` — new filters (entry cause, age, ring, outtake
   type); assert rows match fixture expectations.
2. Export CSV with filters + scope.
3. `/reports/annual` — **all centers** scope: totals = A + B; switch to
   **instance A**: totals = A only; **instance B**: totals = B only. Assert
   exact counts/% per table for each scope.
4. Annual export CSV per scope; assert contents.
5. **All 4 languages** spot-checks + navbar Reports menu ×4 variants.

**Contract e2e**: create/edit animals in Creaves UI → assert the event arrives
in console with the new fields populated (visible on consolidated animal
detail / list); run `/webhook_resync` → assert backfill of new columns.

### 7.3 Definition of done

- `go test ./...` green (Creaves), `CGO_ENABLED=1 go test -tags sqlite ./...`
  green (Console).
- Agent-browser e2e suite green in all 4 languages, both apps.
- Every displayed number traceable to a fixture expectation — no
  "renders without error" only assertions.

## 8. Open questions for requester

1. "By reason for exit" ×3 — confirm interpretation rows 7–9 (type / rating /
   dead-flag).
2. "Year of entry": register year (`animals.year`) — confirm, vs
   `YEAR(intakes.date)`.
3. Entry-cause filter on console: exact match on canonical string acceptable?
4. Export: single multi-section CSV (proposed) vs one file per statistic?
5. Should search/export be restricted (admin) or available to all logged-in
   users? (Current `/animals` and `/consolidated_animals` are session-authed,
   not admin-only — keep same.)
