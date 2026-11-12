# Bug 3 — Outtake rules: native-status filtering + location mode per outcome type

Date: 2026-09-16 · Scope: `creaves/` only (webhook payload unchanged) · Status: done

## Problem (bugs.md §3)

On `/outtakes/new` all outcome types were always offered, and the Location
field was always a free-text autocomplete. Required rules:

| Outcome type        | Excluded native statuses      | Location rule        |
|---------------------|-------------------------------|----------------------|
| OT1 Relacher        | NS2, NS3, NS4                 | free text            |
| OT4 Transferer      | NS3                           | pick from list       |
| OT6 Adoption        | NS1, NS3                      | pick from list       |
| all others          | —                             | no location          |

## Implementation

### 1. Migrations (additive)

- `20261004090000_outtaketype_rules.up.fizz`: `outtaketypes` gains
  `excluded_native_statuses` VARCHAR NULL (CSV of native-status IDs) and
  `location_mode` VARCHAR NOT NULL DEFAULT `'none'`; new table
  `outtake_location_options` (UUID PK, `name`, timestamps).
- `20261004091000_backfill_outtaketype_rules.up.sql`: sets the matrix above on
  existing rows (matched by name); others get `'none'` via the column default.

### 2. Model

- `models/outtaketype.go`: `ExcludedNativeStatuses nulls.String`,
  `LocationMode string`, constants `OuttakeLocationModeNone/Free/List`,
  `ExcludedNativeStatusList()`, `ExcludesNativeStatus(ns)`,
  CSV normalization in `Validate` (mode defaults to `none`, inclusion check).
- `models/outtake_location_option.go`: `ID uuid`, `Name string` (+ presence
  validation).

### 3. Reference CRUD: outtake locations

- `actions/outtake_location_options.go` — full CRUD behind
  `requireMaintainer`, translations on `name` via the
  `setTranslationValues`/`saveTranslations` pattern. Delete has no remap:
  outtakes store the location as plain text, so historical values are kept.
- Route `/outtake_location_options` (registered in `app.go`), 5 templates × 4
  locales, nav link in the Administration menu (all 4 layouts), locale keys
  `outtakeLocationOption.{created,updated,destroyed}.success` × 4.

### 4. Outtaketypes admin form

`templates/outtaketypes/_form.plush.*` × 4: `LocationMode` select
(none/free/list, translated) + multi-select `excluded_native_status_ids`
(translated labels via `tname`). `actions/outtaketypes.go` binds the
multi-select into the CSV field (`bindExcludedNativeStatuses`) and sets form
values on every render path (`setOuttaketypeRuleFormValues`).

### 5. Outtakes New / Create

`actions/outtakes.go`:

- `speciesNativeStatus(tx, species)` — looks up `species.creaves_species =
  animals.species` and returns its `native_status` ("" when unmapped).
- `filterOuttaketypesForNativeStatus(types, ns)` — drops types whose CSV
  contains `ns`; empty `ns` (unknown species) disables filtering.
- `setFilteredOuttakeFormData` — sets the filtered `selectOuttaketype` and the
  JS data (`outtakeLocationModesJSON`: typeID → mode;
  `outtakeLocationOptionsJSON`: `[{value: base name, label: translated}]`).
- `New`: after the animal is resolved, the radio list is filtered and the JS
  data is injected.
- `rejectOuttakeCreate` (used by `Create`): type must exist
  (`outtake.type.invalid`), must not exclude the species native status
  (`outtake.type.forbidden_by_native_status`), and the location must satisfy
  the mode (`outtake.location.invalid`). Violations → translated flash + 422
  re-render of the new form (`renderOuttakeNewRejected`).
- `enforceOuttakeLocationRule`: `none`/"" → location cleared; `list` →
  required and must match a base name in `outtake_location_options`;
  `free` → anything.

### 6. Templates × 4

`templates/outtakes/_form.plush.*`: when `outtakeLocationModesJSON` is set
(new form), a `#outtake-location-group` div with a free-text input and a
select (both `name="Location"`, mutually disabled) plus two
`<script type="application/json">` data tags; a vanilla-JS `change` handler on
the type radios switches between hidden (none), free text (+ existing
autocomplete) and select (list, value = base name, label = translation).
When the JSON is absent (edit form) the legacy autocomplete markup renders
unchanged.

### 7. Seed

`grifts/create_outtake_location_options.go` seeds Creaves / Refuge / VOC /
Zoo with stable UUIDs (idempotent by name); wired into `db:seed`.
`grifts/reference_translations.go` holds the en-US/de/nl names
(Wildlife rescue center / Shelter / VOC / Zoo, etc.) applied by
`applyReferenceTranslationsTx`. `grifts/create_outtaketype.go` carries the
rule matrix for fresh seeds.

## Tests

`actions/outtakes_rules_test.go`:

| Test | Covers |
|------|--------|
| `TestFilterOuttaketypesForNativeStatus` | full bug matrix: ns="" → all; NS1 → OT6 blocked; NS2/NS4 → OT1 blocked; NS3 → OT1+OT4+OT6 blocked; NS5 → all allowed |
| `TestOuttaketypeExcludesNativeStatus` | CSV parsing: whitespace, trailing comma, empty/NULL, "" ns never excluded |
| `TestEnforceOuttakeLocationRule` | none clears; legacy "" mode clears; list rejects missing/unknown, accepts reference option; free accepts anything (MySQL test DB) |
| `TestSpeciesNativeStatus` | lookup by `creaves_species`, unknown → "", "" → "" |
| `TestOuttakeCreateRejectsTypeForbiddenByNativeStatus` | full-stack POST (real app, login, CSRF): NS3 animal × type excluding NS3 → 422, nothing persisted, animal not linked |
| `TestOuttakeCreateRejectsLocationOutsideList` | list-mode type + bogus/missing location → 422, nothing persisted |

Also extended: `grifts/reference_translations_test.go` (4 new expected
records), shared helper `adminClientWithURL` extracted in
`configs_bug_test.go` (existing `adminClient` unchanged signature).

Test-DB note: `creaves_test` was created from a schema dump without
`schema_migration` rows; the two new migrations were applied to it manually
(equivalent SQL) and their versions recorded.

## Quality gates

- `go vet ./...` — clean.
- `staticcheck ./...` (v0.7.0-dev; released staticcheck cannot read Go 1.27
  export data) — no findings.
- `gocognit -over 15` / `gocyclo -over 12`: the repo has pre-existing
  violations (e.g. `buildEventPayloadInto` 100, `createOuttaketype` 17→20).
  Baseline vs. after for touched functions: `OuttakesResource.Create` 31→33
  cog / 18→20 cyc, `OuttakesResource.New` 15→17 cog / 10→11 cyc — small
  increments to legacy functions already over the limits; all new helpers are
  within limits (≤10 cog). No new gate violations introduced.
- `go test -count=1 ./...` — ok (actions 5.9s, all packages).
- `go test -count=1 -race -cover ./...` — ok (actions 58.3s, 33.1% cover; all
  packages pass).

## E2E evidence (agent-browser, dev DB, `buffalo dev` on :3000, admin/admin)

1. **Login** — `POST /auth/` → redirect to `/`. PASS.
2. **NS1 filtering** — `GET /outtakes/new?animal_year_number=130/26` (Renard
   roux, NS1): radios = DCD, Deceased, Duplicate, Euthanized, Dead on arrival,
   Released, Transferred. **Adoption absent**. PASS.
3. **NS3 filtering** — `…?animal_year_number=9991/26` (Ragondin, NS3): radios
   = DCD, Deceased, Duplicate, Euthanized, Dead on arrival. **Released,
   Transferred, Adoption all absent**. PASS.
4. **Location behaviour** (130/26, JS `change` handler):
   - Released (free) → group visible, `#outtakeLocationFree` shown+enabled,
     `#outtakeLocationList` hidden+disabled. PASS.
   - Transferred (list) → select shown with options
     `Wildlife rescue center=Creaves, Shelter=Refuge, VOC, Zoo` (en-US UI:
     translated labels, base-name values). PASS.
   - DCD (none) → group hidden, both inputs disabled, values cleared. PASS.
5. **Server-side rules via fetch POST /outtakes/**:
   - NS3 animal 980174 + Adoption UUID → **422** (forbidden type). PASS.
   - NS1 animal 8429 + Transferer + `Location=bogus-place` → **422**. PASS.
   - Same + `Location=Refuge` → success (redirect followed, animal page 200);
     DB: `outtakes.location='Refuge'`, `animals.outtake_id` set. PASS.
   - Fixtures removed afterwards (outtake deleted, animal unlinked, Ragondin
     animal deleted).
6. **Reference translations** — `/outtake_location_options`:
   fr: Creaves/Refuge/VOC/Zoo · en-US: Wildlife rescue center/Shelter/VOC/Zoo
   · de: Wildtierauffangstation/Auffangstation/VOC/Zoo · nl:
   Wildopvangcentrum/Opvang/VOC/Zoo. Nav link present + translated
   ("Uitgaanlocaties" under Beheer). PASS.
7. **Admin edit form** — `/outtaketypes/33a876d2…/edit` (Adoption):
   multi-select has `NS1 — Native` + `NS3 — Exotic species of concern`
   preselected, location_mode=`list` (not saved). Re-open 131/26 form →
   Adoption still absent for NS1. PASS.

No console errors observed.

## Out of scope / notes

- Webhook payload unchanged (outtake fields sent to the console as before).
- `outtaketypes/show` template not changed (form-only scope).
- `Create`'s pre-existing `verrs.HasAny()` re-render path (validation errors
  after rule checks) still renders the animal-selection page — unchanged
  pre-existing behaviour.
- Location select submits the **base name** (stable key); the label is
  translated. Server validates against base names, so edits of translations do
  not break historical data.
