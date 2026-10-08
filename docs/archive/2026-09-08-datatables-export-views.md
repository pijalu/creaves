# bugs.md item — DataTables investigation (datatable sort/filter for dynamic exports)

**Status**: investigation complete → direction approved by user request (bugs.md
explicitly asks for DataTables) → implementing.

## Bug report (from bugs.md)

> creaves/creaves-console: datatable — tables sort/filter are not working well
> for dynamic exports, investigate usage of
> https://github.com/DataTables/DataTablesSrc.git for a better datatable
> implementation.

## Current state (before fix)

Both apps render dynamic export results (arbitrary query → arbitrary columns)
as a plain HTML `<table>` with ~70 lines of hand-rolled inline JS:

- `creaves/templates/export/view.plush.html` — `filterExportTable()` /
  `sortExportTable(col)` (patched 2026-09-08 for the 10k-row freeze:
  `table-layout:fixed` + single `replaceChildren()`).
- `creaves-console/templates/export/view.plush.{html,fr,de,nl}.html` — same
  functions, **older slow copy** (per-row `appendChild`, no
  `table-layout:fixed`); the freeze fix was never ported.

Pain points of the hand-rolled version:

1. All rows always live in the DOM (register export ≈ 10 018 rows × 14 cols ≈
   140 k cells): initial render is heavy, filter/sort mutate the whole DOM.
2. No pagination, no column-aware typing (dates, French decimal commas handled
   by one regex path), no sorted-column indicator, no result count.
3. Two diverging implementations to maintain (console copy already drifted).
4. i18n: zero localization of UI affordances.

## Investigation: DataTables (DataTablesSrc)

- **License**: MIT (free, OSS).
- **Current release**: `datatables.net@3.0.3` (+ `datatables.net-bs4@3.0.3`
  Bootstrap 4 styling, `datatables.net-plugins@3.0.2` i18n packs fr-FR/de-DE/
  nl-NL as CJS/ESM/JSON).
- **Dependency model**: DataTables 1.10+ requires jQuery. Both apps already
  ship jQuery (creaves: webpack `expose-loader`; console: CDN jQuery 3.6).
- **Why it fixes the pain**: DataTables keeps row data in an internal cache
  and only renders the visible page (`deferRender`): a 10 k-row table keeps
  ~25–50 `<tr>` in the DOM. Sorting/filtering operate on the data cache, not
  the DOM → no layout thrash, no freeze. Built-in: multi-column sort with
  indicators, full-text search, pagination, page-length menu, info line,
  type detection + custom types (French decimal comma via
  `DataTable.type('num-comma', …)`), 60+ official language packs.

### Alternatives considered

| Option | Verdict |
|--------|---------|
| Keep + patch hand-rolled JS | Rejected: already drifted between apps; keeps 140 k-cell DOM; reinventing pagination/typing. |
| `bootstrap-table` (already in creaves bundle, used on landing/feeding) | Viable for creaves (zero new dep) but console has no table plugin and the feature set (locale-aware numeric typing, huge-dataset rendering) is weaker than DataTables. Would leave the two apps on different implementations. |
| Server-side DataTables (`serverSide:true`) | Unnecessary: 10 k rows ≈ a few MB HTML; client-side with `deferRender` is fast and keeps handlers untouched. Revisit only if an export grows past ~50 k rows. |
| **DataTables client-side** | **Chosen**: one implementation for both apps, solves freeze root cause (small DOM), best i18n + typing. |

## Feasibility

**High.** No Go handler changes: handlers keep passing `cols`/`rows`; only the
two view templates and asset wiring change.

### Integration approach

**creaves** (webpack bundle, offline-capable — centers may have poor
connectivity):
1. `npm install datatables.net-bs4@^3.0.3 datatables.net-plugins@^3.0.2`
   (lockfile update).
2. `assets/js/application.js`: `require('datatables.net-bs4')` +
   `require('datatables.net-plugins/i18n/{fr-FR,de-DE,nl-NL}.js')`, pick by
   `lang` cookie (same pattern as the existing select2 i18n block).
3. SCSS: import `datatables.net-bs4/css/dataTables.bootstrap4.css` (webpack
   css-loader already configured) or `<link>` in the view template via
   `cssTag` — chosen: plain `cssTag("datatables…css")` is not available;
   import from `application.scss`.
4. `templates/export/view.plush.html`: drop the filter input +
   `sortExportTable`/`filterExportTable` script; add
   `$('#exportTable').DataTable({...})` init with `deferRender:true`,
   `pageLength:50`, `language` from the cookie-selected pack, custom
   `num-comma` type for French decimal commas. jQuery is exposed globally by
   the bundle, and `application.js` loads in `<head>` → inline init safe.

**creaves-console** (no webpack; CDN assets, mirrors existing jQuery/Bootstrap
CDN usage):
1. `templates/application.plush.html`: add DataTables 3.0.3
   `dataTables.bootstrap4.min.css` (in `<head>`) and
   `dataTables.bootstrap4.min.js` (after jQuery, end of body) via
   cdn.datatables.net.
2. `templates/export/view.plush.{html,fr,de,nl}.html`: drop the custom
   filter/sort script; init in a `DOMContentLoaded` handler (view templates
   render *before* the layout's jQuery `<script>` tag). Language object
   inlined per locale template (matches existing per-locale hardcoded UI
   text) — no extra JSON round-trip.
3. Same `num-comma` custom type.

### Effort estimate

| Step | Effort |
|------|--------|
| creaves: npm deps + bundle wiring + template | ~1 h |
| console: layout + 4 locale templates | ~1 h |
| Go tests update (assertions on new markup) | ~0.5 h |
| e2e validation both apps (agent-browser) | ~1 h |
| **Total** | **~half day** |

### Risks / mitigations

- **Large HTML payload still sent** (10 k rows in initial HTML): unchanged
  from today; acceptable (page already renders). Mitigation if it becomes a
  problem: `deferRender` keeps paint fast; server-side processing is the
  documented upgrade path.
- **Console CDN dependency**: console already depends on CDN for jQuery and
  Bootstrap — consistent with existing posture. creaves stays fully bundled.
- **DataTables 3 vs Bootstrap 4**: `datatables.net-bs4@3.0.3` is the official
  BS4 integration for DataTables 3 — verified on npm.
- **French decimal-comma numbers** sorted as text: mitigated by the
  `num-comma` type detector + order pre-processor (ported logic from the
  removed custom sorter).
- **Tests asserting old JS functions** must be updated (see below).

## Test approach

1. Update `creaves/actions/export_view_test.go`
   (`TestExportViewRendersHTMLTable`): remove `sortExportTable`/
   `filterExportTable` assertions; assert `#exportTable`, DataTables init
   (`DataTable(`), `deferRender`, no per-row sort links.
2. Console: check for equivalent export view test; add/adjust assertions the
   same way (`export_reports_sqlite_test.go`).
3. Quality gates per bugs.md guideline 7 (each tool separately):
   `go vet ./...`, `staticcheck ./...`, `gocognit -over 15 .`,
   `gocyclo -over 12 .`, `go test -count=1 -race -cover ./...`
   (console: `CGO_ENABLED=1 go test -tags sqlite ./...`).
4. e2e (agent-browser, guideline 5/6):
   - creaves `:3000` `/export/view?query=register` (≈10 k rows): table
     paginated (50 rows in DOM), sort toggle on a numeric column asc/desc
     correct, search "bernache" reduces row count, UI language follows
     `lang` cookie, no console errors.
   - console `:3001` `/export/reports/view?query=...`: same checks + center
     selector still works (form outside the table).

## Validation steps (closure)

- [x] Both apps: export view uses DataTables; old inline JS removed.
- [x] All quality gates green; webpack build (`npm run build`) green.
- [x] e2e evidence captured (commands, URLs, outputs).
- [ ] Commits with clear messages (one per app).
- [ ] This file moved to `docs/archive/`; bugs.md item removed.

## e2e evidence (agent-browser, 2026-09-08)

creaves `:3000` `/export/view?query=register` (temp admin, removed after):
- DataTables initialized; 50 `<tr>` in DOM (deferRender) out of 10 018 rows.
- Info line (fr default): "Affichage de 1 à 50 sur 10 018 entrées";
  search label "Rechercher :". German via lang cookie: "Suche:" /
  "1 bis 50 von 10.018 Einträgen".
- Sort col 0 (année): asc first cell "2021", desc "2026".
- Search "bernache": 32 visible rows, info "(filtrées depuis un total de
  10 018 entrées)" — matches pre-fix custom filter (32 rows).
- Postal-code column sort (pre-fix freeze trigger): 275 ms cold / 50 ms warm
  (was >150 s freeze).
- No JS console errors. Screenshot: /tmp/dt-creaves-register.png.

console `:3001` `/export/reports/view?query=register` (admin default password
reset to seed value `admin123` afterwards; DB is disposable):
- CDN DataTables initialized; 50 `<tr>` in DOM out of 10 046 rows.
- de template: "Suche:" / "1 bis 50 von 10,046 Einträgen"; fr template:
  "Rechercher :" / "Affichage de 1 à 50 sur 10,046 entrées".
- Sort col 0: asc "2021" / desc "2026"; search "bernache": 33 rows
  ("gefiltert von 10,046 Einträgen").
- Center selector form intact (2 options). No JS console errors.
- Screenshot: /tmp/dt-console-register.png.
