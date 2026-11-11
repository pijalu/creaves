# Fix archive — 2026-09-05 — i18n: taxonomy terms shown in raw French across console

## Original report (bugs.md #6, HIGH)
> ## i18n: taxonomy terms shown in raw French across console — HIGH
> **Observed (EN UI):** filter dropdowns (Species/Type/City/Cause/Age/Outcome type) and
> report tables (register, annual, by_species, by_type, snapshot) show canonical French
> values ("Accenteur alpin", "Moyens Oiseaux", "Indéterminé", "bébé", "Relacher").
> Animal detail localizes type/age/cause from stored translations but species falls back
> to French. Root cause found in creaves `actions/event_translations.go`:
> `payloadTranslationFieldsFor` uses `animal.Species` (French display name) as translation
> `record_id`, but `species` translations are keyed by species ids (SP###) → non-fr
> lookups always miss; only the fr base fallback fills `species`. Payload `translations`
> JSON confirmed missing `species` (and class/group keys) for en-US/de/nl. Mixed
> localization on `/reports/by_type` ("Hérissons et mammifères insectivores" vs "Small
> birds") confirms partial coverage.
>
> **Expected:** all term values (species, type, age, entry cause, outcome type, native
> status, subside group) display localized in fr/en/de/nl everywhere: dropdown labels,
> table cells, CSV exports (headers already localize).

## Root cause (two sides)
1. **creaves payload builder** — `payloadTranslationFieldsFor` keyed all five
   species-derived translation lookups (`species`, `species_class`,
   `species_agw_group`, `species_subside_group`, `species_native_status`) by
   `animal.Species` — the French display name stored on the animal — while the
   `translations` table keys species rows by the species business id (`species.ID`,
   e.g. `SP1`). Every en-US/de/nl lookup missed; only the fr base fallback (canonical
   values) ever filled the payload. Result: console stored translations never contained
   localized species data, so species stayed French in every UI language.
2. **console renderers** — even for fields that *were* translated (type, age, cause,
   outtake type), the index dropdowns rendered raw canonical values
   (`localizedGroupLabels` had no cases for `animal_age`, `entry_cause`,
   `outtake_type`), the register/snapshot tables rendered canonical fields directly,
   and CSV exports ignored the UI language. The fr/de/nl index template variants had
   also drifted from base: they still rendered `animal.EntryCause.String` in the cause
   cell.

## Fix — creaves (push side)
- `actions/event_translations.go`: `payloadTranslationFieldsFor` now takes the resolved
  `*models.Species` and keys all five species-derived fields by `species.ID`
  (empty id when species unknown → fr fallback only, as before).
  `loadPayloadTranslations` / `loadPayloadTranslationsPreloaded` pass the species
  through.
- `actions/event_producer.go` (`buildEventPayloadInto`): resolves the species row once
  per animal by `creaves_species` name (raw query with explicit column list — pop
  strict-maps raw columns, and the `order` column needs backticks) and passes it to the
  translations loader. The batched `translationPreloader` loads all needed species rows
  first and serves per-animal lookups without extra queries.

## Fix — creaves-console (receive side)
- `actions/dashboard.go`: `localizedGroupLabels` gained `animal_age`, `entry_cause`,
  `outtake_type` cases (previously returned empty maps); index handler now also serves
  `speciesLabels` / `animalTypeLabels` maps.
- `templates/consolidated_animals/index.plush.{html,fr,de,nl}.html`: species and type
  dropdown options render `tlabel_localized(...)` labels (canonical value kept in
  `value=`); fr/de/nl variants' cause cell switched to `tfield_localized(animal,
  "entry_cause")` (drift fix).
- `templates/reports/register.plush.{html,fr,de,nl}.html` and
  `templates/reports/snapshot.plush.{html,fr,de,nl}.html`: term cells
  (type, species, age, cause, outtake type) → `tfield_localized`.
- `actions/register_reports.go`: `RegisterExportCSV` / `SnapshotExportCSV` localize term
  values with `requestUILang(c)`.

## Tests
- creaves `TestBuildEventPayload_SpeciesAndOuttakeTranslationsAllLocales` (new): seeds a
  species row + en-US/de/nl translations for species name, class, agw group, subside
  group, native status and outtake type; asserts all six fields localized in
  en-US/de/nl and fr base fallbacks in the payload for both the per-animal and batched
  builder paths. Existing builder tests re-keyed to species ids and made unconditional.
- creaves-console `actions/i18n_localization_sqlite_test.go` (new): serves the real
  routes (index, register, snapshot, CSVs) from a sqlite fixture with nl/de
  translations; asserts `</td>`-anchored localized cells (Egel/Aanrijding/Volwassen…)
  and absence of French leakage — assertions anchored on `</td>` so dropdown options
  cannot satisfy them (RED-proven: reverting the nl index variant fails the test).

## Validation
- creaves: `go vet ./...` clean; `go test -tags sqlite -count=1 ./...` and plain
  `go test -count=1 ./...` green.
- creaves-console: `go vet` clean; `CGO_ENABLED=1 go test -tags sqlite -count=1 ./...`
  green including the 5 new `TestI18n_*` tests. (`-race` pusher-test data races in
  creaves are pre-existing, timing-dependent, and untouched by this change.)
- Live E2E: creaves resync run 1 pushed 9996/9996 events delivered, 0 failed; a second
  resync created exactly 3816 events (the animals whose payload hash changed after a
  dev-data translation repair) with 6230 skipped unchanged — hash-dedupe confirms only
  changed payloads re-pushed.
- Browser (console :3001): nl/de/en/fr index dropdowns and table cells show localized
  terms, no French leakage in nl/de; fr shows canonical French (correct); live NL
  register CSV fully localized (headers + values, e.g. Knaagdieren / Bruine rat /
  Vernietiging, verstoring van de habitat).
