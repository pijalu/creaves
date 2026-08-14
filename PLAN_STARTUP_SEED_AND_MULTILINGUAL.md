# Plan: Embedded Startup Seed + Multilingual (DE/NL) — Goal Cards

**Status**: PROPOSED. Each card below = one goal, executable in a **fresh context** (no prior knowledge assumed). Every card carries its own facts, commands, verification and handover.
**Usage**: create one goal per card (`freshContext: true`), `completionCriterion` = card's *Verify* section. Execute in dependency order.

## Global Context (true for every card — copy into each goal)

- Repo: workspace root; project dir **`creaves/`**. Stack: Go 1.18, Buffalo v0.18, Pop v6 ORM, MySQL/MariaDB, Plush templates, grifts CLI tasks.
- Run all commands from `creaves/`.
- Local MySQL: `localhost:3306`, user/pass `creaves`/`creaves`. Dev DB `creaves`, test DB `creaves_test` — **never modify these in verification steps**; use scratch DB `creaves_seedcheck` with:
  `GO_ENV=production DATABASE_URL='mysql://creaves:creaves@(localhost:3306)/creaves_seedcheck?parseTime=true&multiStatements=true&readTimeout=3s'` (production env in `database.yml` reads `DATABASE_URL` via `envOr`).
- `database.yml` DSNs include `multiStatements=true`.
- `grifts/init.go` boots the app (`buffalo.Grifts(actions.App())`); `models` package connects to DB at init (`log.Fatal` on failure) → any `go test` touching `models`/`grifts`/`actions` needs MySQL up and `GO_ENV=test` with `creaves_test` existing. `utils/` package has NO models import → safe for pure unit tests.
- Build check: `go build ./...`. Unit tests that need no DB: `go test ./utils/`.
- Reference dump: `creaves/creaves-startup.sql.gz` (62 KB mysqldump, French production reference data). Contains DROP+CREATE+INSERT for 7 tables: `animalages, animaltypes, caretypes, outtaketypes, drugs, species, dosages` (animaltypes = 13 rows, species ≈ 600). Column order in dump INSERTs matches `migrations/schema.sql` for all 7. Dump DDL uses `utf8mb4_0900_ai_ci` (MySQL 8-only) → **never execute dump DDL** (breaks MariaDB); INSERTs only.
- FK order: `dosages` → `animaltypes` + `drugs`. Seed order: `animalages, animaltypes, caretypes, outtaketypes, drugs, species, dosages`.
- `species.species` = Latin name (canonical, never translated); `species.creaves_species` = French common name. Dump species references `subside_groups` codes SG1–SG4 and `native_statuses` NS1–NS5 (no FK constraint; seeded by existing grifts).
- Existing seed: `grifts/db.go` task `db:seed` calls 15 `create_*` funcs; **all have count>0 skip-guards**; old built-in data is English placeholders. Tables seeded ONLY by grifts (must keep running): users(admin), traveltypes, zones, native_statuses, subside_groups, entry_causes, localities, configs.
- Verified model type names: `models.Animalage, models.Animaltype, models.Caretype, models.Outtaketype, models.Drug, models.Dosage, models.Species`.
- I18n: `mw-i18n/v2` (`actions/app.go:193`, default `en-US`), locale cookie `lang`, switch via `GET /language/:lang` (`actions/switchLanguage.go`). Locales: `locales/*.en-us.yaml` + `*.fr.yaml` (~30 domains; some en-us-only). Templates: 139 base (`*.plush.html`, hardcoded English) + 95 French duplicates (`*.plush.fr.html`); Buffalo/render auto-resolves per-lang template with **fallback to base** when variant missing. DB reference names shown raw via `selType.SelectLabel()` (`actions/typehelper.go`) and `<%= x.Name %>` in templates.
- Webhook contract (to creaves-console): payload fields `species`, `animal_type`, `zone`, outtake `type` carry raw DB display values.
- Git: branch `feature/kimi`, clean at plan time. Conventional commits. One commit per card unless noted.

---

# PHASE A — Task 1: embedded startup seed

## G1 — SQL dump parser (pure, unit-tested)
**Depends**: —
**Objective**: `utils/sqldump.go` extracts INSERT statements from a mysqldump stream, ignoring all DDL/comments; fully unit-tested.
**Context**:
- Dump format: statement starts with line `INSERT INTO \`<table>\` VALUES` (table = lowercase `[a-z_]+`), followed by one row per line ending `),`, final row ends `);`. Values may contain escaped quotes (`\'`), literal `\n` sequences, and `;` inside strings (never at line end except terminator).
- Everything else must be skipped: `--` comments, `/*!…*/` executable comments, `SET`, `DROP TABLE`, `CREATE TABLE … ) ENGINE=…`, `LOCK/UNLOCK TABLES`, `ALTER TABLE … DISABLE/ENABLE KEYS`.
**Steps**:
1. New file `creaves/utils/sqldump.go`: `func ExtractInsertStatements(r io.Reader) (order []string, stmts map[string][]string, err error)` — `bufio.Scanner` with 16 MB max buffer; start-regex `^INSERT INTO \x60([a-z_]+)\x60 VALUES\s*$`; accumulate until trimmed line ends with `;`; store complete multi-line statement per table; record first-seen table order; error on unterminated statement at EOF.
2. New file `creaves/utils/sqldump_test.go` with cases: DDL/comment skipping; multi-line INSERT reassembly; escaped quotes & `\n`; `;` inside string mid-line; two tables → grouping + order; unterminated statement → error; empty input.
**Verify**: `cd creaves && go test ./utils/` exits 0.
**Handover**: function signature `utils.ExtractInsertStatements` is consumed by G2 and G10.

## G2 — Embed dump + `seedStartup` grift + wire into `db:seed`
**Depends**: G1
**Objective**: `db:seed` seeds the 7 reference tables from the embedded dump before all existing seeds; idempotent per table; standalone task `db:seed:startup` exists.
**Steps**:
1. `git mv creaves/creaves-startup.sql.gz creaves/grifts/creaves-startup.sql.gz` (go:embed cannot reach parent dirs; `grifts/embedData.go` already embeds `*.csv` the same way). Verify no other refs: `grep -rn creaves-startup . --exclude-dir=node_modules`.
2. New file `creaves/grifts/startup_seed.go`:
   - `//go:embed creaves-startup.sql.gz` → `var startupSQLGz []byte`.
   - `var startupTables = []string{"animalages","animaltypes","caretypes","outtaketypes","drugs","species","dosages"}`; `startupModels = map[string]interface{}{...}` mapping each table to its verified model (Global Context).
   - `func seedStartup(c *grift.Context) error`: gunzip → `utils.ExtractInsertStatements`; execute tables in `startupTables` order then any extras in dump order; wrap all in `models.DB.Transaction(func(tx *pop.Connection) error …)`; per table: `cnt, err := tx.Q().Count(model)` — if >0 log `"<table>: N rows, skipping"` and continue; else `tx.RawQuery(stmt).Exec()` per statement (wrap errors with `github.com/pkg/errors.WithStack` → rollback).
3. Edit `creaves/grifts/db.go`: add `if err := seedStartup(c); err != nil { return err }` as the **first** call in `db:seed`; add `grift.Desc("seed:startup", …)` + `grift.Add("seed:startup", func(c *grift.Context) error { return seedStartup(c) })` in the same `db` namespace.
**Verify**: `cd creaves && go build ./... && go vet ./grifts ./utils` exits 0.
**Handover**: `seedStartup` is the single entry point; G10 will extend it with translation seeding. Dump now lives at `grifts/creaves-startup.sql.gz`.

## G3 — Real-DB verification of startup seed
**Depends**: G2
**Objective**: prove seeding works end-to-end on a scratch DB; idempotent; cleanup.
**Steps** (exact commands, from `creaves/`):
1. `mysql -ucreaves -pcreaves -e "DROP DATABASE IF EXISTS creaves_seedcheck; CREATE DATABASE creaves_seedcheck CHARACTER SET utf8mb4;"`
2. `export GO_ENV=production DATABASE_URL='mysql://creaves:creaves@(localhost:3306)/creaves_seedcheck?parseTime=true&multiStatements=true&readTimeout=3s'`
3. `buffalo pop migrate up`
4. `buffalo task db:seed` → expect log: 7 tables seeded; then existing grifts log "Already N records … skipping" for animaltypes/animalages/caretypes/outtaketypes/drugs/species.
5. Assert: `SELECT COUNT(*) FROM animaltypes;` = 13; `species` ≈ 600; `drugs`, `dosages`, `caretypes`, `outtaketypes`, `animalages` > 0; `SELECT name FROM animaltypes LIMIT 3;` → French names; `subside_groups` = 4; `native_statuses` = 5; `users` = 1 (admin).
6. Idempotency: `buffalo task db:seed` again → all 7 tables "skipping", counts unchanged. `buffalo task db:seed:startup` → all skip, exit 0.
7. `mysql -ucreaves -pcreaves -e "DROP DATABASE creaves_seedcheck;"`; unset env.
**Verify**: all assertions in step 5 pass; second seed run changes nothing; exit codes 0.
**Handover**: seed mechanism proven. G10 reuses same scratch-DB recipe.

## G4 — Document Task 1 + commit
**Depends**: G3
**Objective**: AGENTS.md reflects new seeding; work committed.
**Steps**:
1. Edit `creaves/AGENTS.md` → "Database Setup": `db:seed` first loads embedded production reference data from `grifts/creaves-startup.sql.gz` (7 tables, per-table skip-if-non-empty); document `buffalo task db:seed:startup`; note: dump INSERTs have no column names → adding NOT NULL columns without defaults to these 7 tables requires regenerating the dump.
2. Commit: `feat(seed): embed creaves-startup.sql.gz as first db:seed step` (includes G1–G4 files).
**Verify**: `grep -n "seed:startup" creaves/AGENTS.md` non-empty; `git log -1 --stat` shows commit.
**Handover**: Task 1 complete.

---

# PHASE B — UI locales DE/NL

## G5 — Generate de/nl locale skeletons
**Depends**: —
**Objective**: `locales/*.de.yaml` + `*.nl.yaml` exist for every `*.en-us.yaml`.
**Steps**:
1. Extend `creaves/translate.sh` (currently hardcoded en-us→fr copy-if-missing) to accept target locale arg: `./translate.sh de` copies each `locales/X.en-us.yaml` → `locales/X.de.yaml` if missing (same for `nl`); no arg = legacy fr behavior.
2. Run `./translate.sh de && ./translate.sh nl`.
**Verify**: `ls creaves/locales/*.de.yaml | wc -l` == `ls creaves/locales/*.en-us.yaml | wc -l` (≈30); same for `nl`.
**Handover**: skeleton files contain English text — content delivery is G22. Parity enforced by G6.

## G6 — Locale key-parity test + key audit
**Depends**: G5
**Objective**: automated test fails on key drift across en-us/fr/de/nl; all keys used in code exist in all 4 locales.
**Steps**:
1. New test (e.g. `creaves/locales/locales_test.go`, pure Go, yaml.v2/v3 already in go.mod — check imports; else `utils/`): load every `locales/*.yaml` via `locales.FS()` or embed path; group by domain file; assert identical `id` sets across locales per domain.
2. Audit used keys: `grep -rhn 'T.Translate(c, "' creaves/actions/ | grep -oE '"[^"]+"' | sort -u` and `grep -rh 't("' creaves/templates/ | grep -oE 't\("[^"]+"' | sort -u`; add any missing keys to all 4 locale files (English placeholder acceptable for de/nl).
**Verify**: `cd creaves && go test ./locales/` (or `./utils/`) exits 0.
**Handover**: parity test guards all future locale work (G19, G22).

## G7 — Language switcher DE/NL + template fallback proof
**Depends**: G5
**Objective**: users can switch to German/Dutch; pages render via base-template fallback with no errors.
**Context**: Buffalo/render resolves `name.plush.<lang>.html` by context lang (from cookie `lang`), falls back to base `name.plush.html` if variant missing → no de/nl template duplicates needed. Language links are hardcoded: `templates/application.plush.html` (~line 124, dropdown `<a … langPath({ lang: "fr"})>`), its twin `templates/application.plush.fr.html`, and login page `templates/auth/landing.plush.html` + `.fr.html` (~line 135, `fr-FR` link). mw-i18n auto-discovers languages from `locales/` files — no Go change needed; cookie values must be `de` / `nl` (match file suffixes).
**Steps**:
1. Add "In German"/"Auf Deutsch" (`lang: "de"`) and Dutch (`lang: "nl"`) entries in all 4 templates listed above (keep existing fr/en entries).
2. `go build ./...`.
3. E2E smoke: start `buffalo dev`, login admin/admin, set lang cookie to `de` (or use switcher), load `/animaltypes` (has `.fr.html` but no `.de.html`) → base template renders, HTTP 200, no template error in log; flash/labels from `de.yaml` where keyed.
**Verify**: build clean; manual/E2E check: 200 + base template on `/animaltypes` with `lang=de`; switcher shows 4 languages.
**Handover**: UI shell bilingual-ready; page text stays English until optional template refactor (future, out of scope).

---

# PHASE C — Multilingual DB data: translation table (approach B, locked)

**Design (locked)**: table `translations(id char(36) PK, table_name varchar(64), record_id varchar(36), field varchar(64), locale varchar(8), value text, created_at, updated_at)`; UNIQUE `(table_name, record_id, field, locale)`; INDEX `(table_name, record_id, locale)`. Existing `name`/`description` columns remain = canonical **French** (dump language) = fallback + sort key + webhook payload value (existing installs need no data migration). Fallback chain: requested locale → base column. Locales: `en-US, fr, de, nl` (`record_id` varchar(36) covers UUID PKs and species codes like `SP1`).

## G8 — Migration + Translation model
**Depends**: —
**Objective**: `translations` table exists (up/down fizz), `models.Translation` with validation.
**Steps**:
1. `buffalo pop generate fizz create_translations` → fill up/down fizz per locked schema.
2. New `creaves/models/translation.go`: struct with pop tags, `Translations` slice type, `Validate` (presence of table_name/record_id/field/locale/value; locale ∈ `SupportedLocales`); add `var SupportedLocales = []string{"en-US","fr","de","nl"}`.
3. Scratch DB: create `creaves_seedcheck` (G3 recipe), `buffalo pop migrate up`, inspect table; `buffalo pop migrate down` then up again → clean; drop scratch DB.
**Verify**: migration up/down/up exit 0; `\`SHOW CREATE TABLE translations\`` shows unique key; `go build ./...` clean.
**Handover**: table + model ready for G9/G10.

## G9 — Translation service (batched, no N+1)
**Depends**: G8
**Objective**: reusable read/write helpers in `models`, unit-tested.
**Steps**:
1. New `creaves/models/translations_service.go`:
   - `LoadTranslations(tx *pop.Connection, table, field, locale string, ids []string) (map[string]string, error)` — single `WHERE table_name=? AND field=? AND locale=? AND record_id IN (?)`.
   - `ResolveName(lang, base string, tr map[string]string, id string) string` — requested-locale value or base fallback.
   - `SaveTranslation(tx *pop.Connection, table, id, field, locale, value string) error` — upsert via SELECT-then-INSERT/UPDATE (pop has no portable ON DUPLICATE).
2. Tests `creaves/models/translations_test.go` on test DB (`GO_ENV=test`, `creaves_test` must exist): load/resolve/fallback/upsert/unique-key round-trip.
**Verify**: `cd creaves && GO_ENV=test go test ./models/ -run Translation` exits 0.
**Handover**: API consumed by G10 (seeding), G12/G13 (read path), G16+ (write path).

## G10 — Seed `fr` translations from the embedded dump
**Depends**: G2, G8, G9
**Objective**: startup seed also populates `translations` with French name/description values from dump data; works for fresh AND existing installs.
**Steps**:
1. Extend `creaves/grifts/startup_seed.go` (from G2): after base-table inserts, inside the same transaction, per source table insert `fr` translation rows read from the just-present rows (works whether base rows came from dump or pre-existed):
   - animaltypes, animalages, caretypes, outtaketypes: fields `name`, `description`.
   - drugs: `name`, `description`. dosages: `description`.
   - species: `creaves_species` only (`species` Latin column never translated).
   - Guard: skip a (table, locale) group if `SELECT COUNT(*) FROM translations WHERE table_name=? AND locale='fr'` > 0 → idempotent; existing production DBs get fr backfilled by running `buffalo task db:seed:startup` without touching base tables.
2. Scratch-DB verify (G3 recipe): fresh migrate + `db:seed` → `SELECT locale, COUNT(*) FROM translations GROUP BY locale;` = one `fr` row-group ≈ (13×2 animaltypes + ages + caretypes×2 + outtaketypes×2 + drugs×2 + dosages + ~600 species); re-run `db:seed` → counts unchanged.
**Verify**: counts query matches expectation; second run no-op; `go build ./...` clean.
**Handover**: fr translation data present everywhere. de/nl data = G11/G22.

## G11 — de/nl reference-data translation pipeline (export skeleton + SQL loader)
**Depends**: G10
**Objective**: translators get a fill-in SQL skeleton; finished de/nl SQL files are embedded and applied by the same seed mechanism.
**Steps**:
1. New grift `i18n:export:translations` (`grifts/i18n_export.go`): args `--lang=de|nl`; emits `translations_<lang>.sql` with one commented `INSERT INTO translations (…) VALUES ('<uuid>', '<table>', '<record_id>', '<field>', '<lang>', ''); -- base: <french value>` per missing row (reads base columns + fr translations).
2. Extend `grifts/embedData.go` or `startup_seed.go`: `//go:embed translations_*.sql` (optional glob, may match nothing); `seedStartup` applies any matched files after fr seeding, with same per-(table,locale) skip guard.
3. Verify: run export on scratch DB from G10 → file well-formed; place a 2-row hand-made `translations_de.sql`, re-run `db:seed:startup` → de rows present; re-run → no duplicates.
**Verify**: `buffalo task i18n:export:translations --lang=de` exits 0 and writes valid SQL; loader round-trip verified on scratch DB.
**Handover**: pipeline ready for translator delivery (G22).

---

# PHASE D — Read path (localized display)

## G12 — Localized select dropdowns (`typehelper.go`)
**Depends**: G9
**Objective**: all `<select>` labels for reference data render in current UI language with fr fallback.
**Context**: `actions/typehelper.go` — `selType{label,value}` + `*ToSelectables(ts)` converters (animalTypes, zones, outtakeTypes, caretypes, traveltypes, animalages, users). Language source: cookie `lang` (values: `en-US`? — normalize: treat `en`/`en-US` as `en-us`; base = `fr` = no lookup needed). Default installs (no lang cookie or fr) must issue **zero** extra queries.
**Steps**:
1. Add `func currentLang(c buffalo.Context) string` (normalize cookie; `""` or `"fr"` → base).
2. Change converter signatures to accept lang: `animalTypesToSelectables(ts *models.Animaltypes, lang string)` etc.; when lang non-base: collect IDs → `models.LoadTranslations(tx, table, "name", lang, ids)` once → label = `ResolveName`.
3. Call-site sweep: `grep -rn "ToSelectables(" creaves/actions/` — update every caller to pass `currentLang(c)`; converters need `tx` too — pass `c.Value("tx").(*pop.Connection)`.
**Verify**: `go build ./...` clean; `grep -rn "ToSelectables(ts)" creaves/actions/` (old 1-arg calls) empty; E2E: insert one de translation for an animaltype → dropdown shows German with `lang=de`, French otherwise.
**Handover**: pattern established for G13.

## G13 — `tname` template helper (localized names in templates)
**Depends**: G9
**Objective**: templates can render localized entity names via `<%= tname("animaltypes", x.ID, x.Name) %>`.
**Steps**:
1. Register helper where existing helpers live (find `bool2html` registration — `actions/render.go` or `app.go`): `tname(table string, id string, base string) string` — uses plush/buffalo context lang; per-request cache: map key `(table,field,locale)` → bulk-loaded map on first use (one query per table per request, lazy); store cache in `c.Value` via middleware-light helper closure.
2. Unit/smoke test: helper resolvable in render; fallback returns base when no translation.
**Verify**: `go build ./...`; render a test template using `tname` without error; E2E spot check with one de row.
**Handover**: consumed by G14 domain rollouts.

## G14a–G14e — Template rollout per domain (one goal each)
**Depends**: G13
**Objective per card**: replace raw `<%= x.Name %>` / `<%= x.Description %>` with `tname(...)` in these template groups (base `*.plush.html` AND `*.plush.fr.html` twins — same edit both files):
- **G14a**: animaltypes + animalages (10 files) + their select/display usage in animals forms.
- **G14b**: caretypes + outtaketypes + related cares/outtakes display (≈15 files).
- **G14c**: drugs + treatments + veterinaryvisits (dosage/drug names) (≈15 files).
- **G14d**: species pages + species display in animals index/show + landing + registertable + registersnapshot (field = `creaves_species`).
- **G14e**: zones, traveltypes/travels, native_statuses, subside_groups, entry_causes admin pages (localities excluded — proper nouns).
**Steps (each)**: locate raw usages via `grep -n "\.Name %}\|\.Description %}\|CreavesSpecies" creaves/templates/<domain>/`; replace with `tname("<table>", x.ID, x.Name)`; keep sorting unchanged (`ORDER BY name` base — deferred per-locale sort).
**Verify (each)**: `go build ./...`; E2E per AGENTS.md checklist: pages load, no 500/template errors, fr cookie unchanged rendering, de shows translated row where seeded.
**Handover**: after G14a–e all display surfaces localized.

## G15 — Localized species autocomplete
**Depends**: G9, G14d
**Objective**: `SuggestionsAnimalSpecies` (`actions/suggestions.go`) matches user input against current-locale common names too.
**Steps**: in handler, when lang non-base: load translations for `species`/`creaves_species`/lang matching `LIKE %q%`, merge with base `creaves_species` matches (dedupe by species ID); **return canonical `creaves_species` value as the selected value** (animals row must store canonical fr — webhook contract stability).
**Verify**: `go build ./...`; E2E: with de translation present, typing German name finds species and selects canonical value.
**Handover**: read path complete.

---

# PHASE E — Write path (admin CRUD for translations)

## G16 — Pilot: animaltypes admin with translations
**Depends**: G9, G13
**Objective**: admin can edit en-US/de/nl name+description for an animaltype; fr = base column (read-only note).
**Steps**:
1. New partial `creaves/templates/translations/_fields.plush.html`: inputs named `translations[<locale>][<field>]` for locales en-US, de, nl; prefill via a handler-provided map (load existing via `LoadTranslations`).
2. `actions/animaltypes.go` create/update: after base save (same tx), bind `translations` param map → `models.SaveTranslation` per non-empty value.
3. Add partial to `templates/animaltypes/_form.plush.html` + `.fr.html` twin.
4. New yaml keys for the partial labels ×4 locales (parity test G6 must stay green).
**Verify**: build; E2E: edit animaltype, add German name, save; switch to de → index/selects show German; fr shows base; DB rows in `translations` correct; parity test green.
**Handover**: proven pattern to replicate in G17.

## G17 — Roll out translation editing to remaining entities
**Depends**: G16
**Objective**: same translation UI on: animalages, caretypes, outtaketypes, traveltypes, drugs (name+description), dosages (description only), zones, native_statuses, subside_groups, entry_causes. (Localities excluded.)
**Steps**: per entity controller (`actions/<entity>.go`): replicate G16 binding; add partial to that entity's `_form.plush.html` (+`.fr.html`); entity list = checkoff in PR description.
**Verify**: build; E2E spot check 3 entities; `SELECT COUNT(*) FROM translations` grows after edits; parity test green.
**Handover**: —

## G18 — Species admin: translate `creaves_species` only
**Depends**: G16
**Objective**: species admin form edits common name per locale; Latin `species` column untouched.
**Steps**: add partial to `templates/species/_form.plush.html` (+fr twin) bound to field `creaves_species`; update `actions/species.go` save path.
**Verify**: build; E2E: add de common name for one species → animals pages + autocomplete (G15) show it under `lang=de`.
**Handover**: write path complete.

---

# PHASE F — Finishing

## G19 — Admin/UI yaml keys ×4 locales
**Depends**: G17, G18
**Objective**: every new UI string introduced by G16–G18 exists in en-us/fr/de/nl.
**Steps**: collect keys added during E-phase; ensure fr translations proper (not English placeholders); de/nl placeholder acceptable (G22 delivers content); keep G6 parity test green.
**Verify**: `go test ./locales/` green; `grep` shows keys in all 4 files.
**Handover**: —

## G20 — Webhook contract documentation (both repos)
**Depends**: G14d (species canonical decision visible in code)
**Objective**: document that payload display values are canonical base-locale (French) reference names regardless of UI language — no payload change.
**Steps**: add one paragraph to `creaves/AGENTS.md` payload spec + mirror in `creaves-console/AGENTS.md` contract section; note future option: `translations` map in payload (out of scope).
**Verify**: `grep -n "canonical" creaves/AGENTS.md creaves-console/AGENTS.md` non-empty.
**Handover**: —

## G21 — Full regression + E2E language matrix
**Depends**: all of B–E
**Objective**: whole suite green; manual E2E matrix per AGENTS.md mandatory checklist.
**Steps**: `buffalo test` (needs `creaves_test`); `go test ./...`; `buffalo dev` + Chrome DevTools MCP: for each lang (fr/de/nl): login, dashboard, animals index/new, treatments, admin animaltypes incl. translation save, language switch; assert no 500/404, no template errors, correct fallbacks.
**Verify**: suite exit 0; checklist ticked.
**Handover**: feature complete pending content.

## G22 — DE/NL content delivery (external dependency)
**Depends**: G5, G11
**Objective**: real German/Dutch text shipped.
**Steps**: hand translators (a) `locales/*.de.yaml`/`*.nl.yaml` skeletons (G5), (b) `translations_de.sql`/`translations_nl.sql` skeletons (G11); review + commit; run `db:seed:startup` on installs to apply SQL; decision needed: species common names (~600 rows) translate now or ship with fr fallback.
**Verify**: parity test green; E2E spot checks show real de/nl text.
**Handover**: DONE.

---

## Open questions (block only content, not code)
1. Fallback locale = French base columns — confirmed OK?
2. de/nl translation source: human translators vs machine-first draft?
3. Species common names de/nl (~600 rows): translate at launch or fallback?

## Rollback
- Task 1: revert commit; seeded data harmless (guards skip); fresh installs may `TRUNCATE` the 7 tables.
- Task 2: `buffalo pop migrate down` (drops `translations`) + code revert. All changes additive → low risk.
