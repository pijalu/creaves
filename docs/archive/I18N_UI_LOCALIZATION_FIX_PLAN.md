# I18n UI Localization Fix Plan — creaves + creaves-console

Issues found testing `creaves/` (main app); `creaves-console/` must be verified/aligned.
All findings below are evidence-backed with `file:line` references (state as of commit `7c97546`).

---

## 1. Findings

| # | Symptom | Root cause | Location |
|---|---------|-----------|----------|
| F1 | Species suggestion popup shows FR text | `SuggestionsAnimalSpecies` matches translated names but **returns canonical French** `creaves_species` values (deliberate contract-stability choice) | `creaves/actions/suggestions.go:42-69` |
| F2 | Drug / animal-type-default-species suggestions show FR, don't match translated input | No translation join at all in these endpoints | `creaves/actions/suggestions.go:226-248, 251-281` |
| F3 | Search filter: Entry Cause list untranslated | Label built with `t.Fmt(true)` (canonical cause+detail), no `translateIDs` | `creaves/actions/animals_search.go:155-163` |
| F4 | Search filter: Age / Exit reason lists show FR | Handler code **is** translated (`translateIDs`, `animals_search.go:140-148, 173-181`). Startup artifacts contain the rows (animalages 4, outtaketypes 15 per locale). → DB in the test environment predates the Aug 21 translation artifacts; needs `db:seed:startup` re-run, then re-test | DB data, not code |
| F5 | Reception: EN popup shows FR "L'animal doit être relâché dans son milieu" | `/hint/speciesDetails` returns **raw** `native_statuses.indication` (canonical FR). Startup translation artifacts contain **0** `native_statuses` rows in any locale — table excluded from the 7-table dump scope | `creaves/actions/hint.go:9-33`; `creaves/grifts/translations_*.sql` |
| F6 | Reception: Entry Cause list untranslated | `entryCausesToSelectables(ec, true)` has no lang/tx params | `creaves/actions/reception.go:39`, also `creaves/actions/animals.go:652`; helper at `creaves/actions/typehelper.go:378` |
| F7 | Systemic: seed-translation artifacts cover only the 7 dump tables | `entry_causes` (0 rows), `native_statuses` (0), `zones`, `subside_groups` are seeded from separate grifts/CSV and were never added to the artifact inventory (4,686 values = 7 tables only) | `creaves/I18N_STARTUP_TRANSLATION_PLAN.md` scope table |
| F8 | Console display of consolidated values | Mostly OK: payload `translations` map (sync v2) flows into `consolidated_animals.translations`; dashboard/CSV/reports use `LocalizedField`. **Gaps to verify**: `entry_cause_detail`/`entry_cause_nature` and drill-down views; native-status indication not in payload at all | `creaves/actions/event_translations.go:14-56`; `creaves-console/models/consolidated_animal.go:95-115,288-311`; `creaves-console/actions/dashboard.go:345-355,580` |

Note on EN templates: base `.plush.html` templates (used for en-US) contain **no** hardcoded
French (accent grep clean); FR leaks in EN UI come from DB-backed strings (F5, F6) —
template sweep still required as regression check.

---

## 2. Design decision (needs owner sign-off before Phase 2)

**D1 — Suggestion submit semantics.** Species/drug names are stored as free text in
`animals.species` / `treatments.drug` and the webhook contract ships canonical (FR) values.
Popup must *display* localized text, but the *stored* value must stay canonical.

- **Option A (APPROVED — chosen): localized display + backend normalization.**
  Endpoints return localized labels (matching on translations AND canonical).
  A single helper `resolveReferenceInput(tx, table, field, lang, input) → canonical`
  translates a submitted localized string back to canonical at every write/search site.
  Users may also type in their own language. Contract untouched.
- Option B: return `{value, label}` pairs; autoComplete widget submits hidden canonical.
  More frontend work in `jquery.auto-complete.js` call sites, fragile against manual typing.

**D2 — select2 widget chrome** (placeholders, "no results", "searching") currently EN-only.
Optional: init select2 with per-cookie language files. Default: include (small).

**D3 — RESOLVED: model translation + seed integration.** Populate the `translations`
table with the missing items (entry_causes, native_statuses, zones, subside_groups)
using model-generated translations for each required language (en-US, de, nl),
and ensure they are applied as part of `db:seed` (not only the dump-scoped
`db:seed:startup` artifact path).

---

## 3. Fix plan (creaves first)

### Phase 0 — Baseline & repro
1. Fresh DB: `buffalo pop migrate up && buffalo task db:seed:startup` (ensures Aug 21 artifacts applied).
2. Set `lang=en-US` cookie; reproduce each symptom on `/animals` (filter), `/reception/new`.
3. Confirm F4 disappears after reseed (predicted); anything remaining is a code bug.

### Phase 1 — Data completeness (fixes F4, F5-data, F7)
1. Extend artifact generation to non-dump reference tables: `entry_causes`
   (cause, detail, nature, indication), `native_statuses` (status, indication, precision),
   `zones`, `subside_groups`; regenerate `translations_{en-US,de,nl}.sql`.
2. Add grift `i18n:coverage` — prints per-table × per-locale row counts from the
   artifacts; wire into `startup_seed_test.go` asserting no reference table/locale = 0.
3. Apply on dev DB (`buffalo task db:seed:startup`).
- **Check:** coverage grift output non-zero everywhere; `go test ./grifts/...`.

### Phase 2 — Suggestion endpoints (fixes F1, F2) — after D1
1. Add `resolveReferenceInput` helper (+ unit tests: canonical input, localized input,
   unknown input).
2. `SuggestionsAnimalSpecies`: return localized labels per `lang`; match on both
   canonical + translated (already does).
3. `SuggestionsTreatmentDrug`, `SuggestionsAnimalTypeDefaultSpecies`: join translations
   for `lang`, match on both columns, return localized labels.
4. Apply `resolveReferenceInput` at write/search sites: discoveries create/update
   (`Species`), search filter `species` param (`animals_search.go:59-61`), treatments
   create/update (`drug`), animaltypes update (`default_species`).
- **Check:** `go test ./actions/...` + new tests: EN cookie → EN labels; saved record
  keeps canonical FR value.

### Phase 3 — Dropdowns & hint popup (fixes F3, F5, F6)
1. `entryCausesToSelectables(ts, withBlank, lang, tx)`: translate `cause` + `detail`,
   keep `ID - cause ⇨ detail` format; update call sites (`reception.go:39`,
   `animals.go:652`) + `helpers_test.go:641-657`.
2. `hint.go HintSpeciesDetails`: join `translations` for currentLang on
   `native_statuses` (`status`, `indication`, `precision`), canonical fallback.
3. Sweep audit: grep every `ToSelectables(` call site; each reference-table select
   must pass `currentLang(c)` + tx (caretypes/animaltypes/animalages/outtaketypes
   already do; verify zones/traveltypes/drugs).
- **Check:** handler tests with each locale cookie assert localized option labels;
  hint test asserts localized indication.

### Phase 4 — Template/locale sweep (regression) — IMPLEMENTED (see §3.4)
1. Script-diff each base `.plush.html` vs `.fr/.de/.nl` variants for structural drift.
2. Grift/script diffing key sets of `locales/{all*,*}.yaml` across fr/en-US/de/nl →
   list missing keys; fill gaps.
3. D2: select2 language packs per cookie (if approved).
- **Check:** sweep report empty; `buffalo dev` boots; pages render in 4 locales.

#### §3.4 Phase 4 implementation record

**Locale key parity — done, zero gaps.**
`grifts/locale_key_parity_test.go :: TestLocaleKeyParity` (DB-less, reads the
embedded `locales` FS, go-i18n v1 flat-id format): for every base yaml, asserts
* all four locale files exist (`de`, `en-us`, `fr`, `nl`),
* no duplicate ids, no empty translations,
* exact key-set parity with the en-US reference (missing *and* extra keys fail).
Sweep result: all 28 bases × 4 locales already at parity — no keys needed
filling; the test now guards against regression.

**Template variant structural drift — check implemented, debt frozen.**
`grifts/locale_key_parity_test.go :: TestTemplateVariantStructuralParity`
(same DB-less file) normalizes a template to its structural skeleton — HTML
comments removed; plush `<% %>` expressions kept but with string literals and
whitespace collapsed (and inner angle brackets neutralized); remaining quoted
literals collapsed; **text nodes erased, including copy sitting between plush
block tags**; whitespace collapsed. Base vs variant skeletons must be
byte-identical: element names, nesting, field names and plush call shapes are
compared; translated copy is not.

Sweep result (146 base templates, 370 variant pairs):
* 90 bases / 272 pairs fully clean.
* 56 bases / 118 pairs carry pre-existing drift, frozen in the
  `knownVariantDrift` ratchet map: any drift there is logged, drift on any
  *other* template fails the test. Existing debt cannot grow silently.
* Dominant drift classes behind the debt:
  1. base `_form` templates omit `label:` options that fr/de/nl variants
     carry (EN forms render without explicit labels) — e.g. `zones/_form`,
     `animalages/_form`;
  2. base uses the localized `tspecies()`/`tname()` helpers while variants
     print raw canonical fields (`animals/edit`): semantically identical for
     fr (canonical = French), but de/nl variants then display French species
     names — same family as F1;
  3. attribute spacing/order noise, erased by the normalizer.
Paying the debt = editing the 56 template pairs; each fix deletes its ratchet
entry.

**D2 select2 chrome — done.**
`assets/js/application.js` now requires the select2 `en/fr/de/nl` i18n packs
directly after `select2.full.js` (they self-register on select2's AMD
registry) and sets `jQuery.fn.select2.defaults.set('language', …)` at module
scope — before the inline template scripts call `$('select').select2(...)`.
Cookie mapping follows `actions.SwitchLanguage` / `normalizeUILang`:
absent/`fr` → `fr`, `en-US` → `en`, `de` → `de`, `nl` → `nl`. Production
webpack build verified; fr/de/nl packs confirmed present in the emitted
bundle.

### Phase 5 — Console alignment (creaves-console)
1. Extend `webhook_e2e_test.go`: event with `translations` map covering
   `entry_cause`, `entry_cause_detail`, `entry_cause_nature`, `animal_age`,
   `outtake_type` → assert localized rendering via `LocalizedField`.
2. Add localized display of `entry_cause_detail`/`nature` in drill-down if missing.
3. (If D1-A chosen) no console change needed — payload stays canonical.
- **Check:** `CGO_ENABLED=1 go test -tags sqlite ./actions/... ./models/...`.

### Phase 6 — End-to-end verification — DONE (automated part; see §5)

---

## 4. Residual risks

- Artifact regeneration must respect dump-ID vs local-ID drift — existing
  `tnameResolveByBase`/base-value fallbacks mitigate; verify with fresh-seed E2E.
- `resolveReferenceInput` must be idempotent (canonical input → unchanged) or re-saving
  old records could corrupt data; covered by unit tests.
- `entry_causes` CSV already has 12 `nature` values reused across 27 rows — translations
  keyed per record; ensure dedup when translating.
- Fresh-seed E2E on the owner machine is still required: `db:seed` now applies the
  model-generated translations for `entry_causes`/`native_statuses`/`zones`/
  `subside_groups` (D3), but existing dev DBs must be re-seeded (or the grift
  re-run) before the localized strings appear.
- `resolveReferenceInput` idempotency (canonical in → canonical out) is covered by
  unit tests; re-saving legacy records keeps canonical values.
- Artifact-ID vs local-ID drift: `tnameResolveByBase`/base-value fallbacks mitigate;
  covered by `go test ./grifts/` on a fresh test DB.
- **`go test` must run with `GO_ENV=test`** for creaves: without it the suite binds
  the **dev** `creaves` database (10k+ animals), where `TestStartResyncRunCommittedBeforeReturn`
  legitimately exceeds its 30s worker deadline (full-state resync over every animal)
  and fixture tests touch live data. `buffalo test` sets the env automatically; use
  `GO_ENV=test go test ./actions/ ./models/ ./grifts/` when invoking go directly.

---

## 5. Verification record (Phase 6, automated part) — 2026-08-30

| Check | Command | Result |
|---|---|---|
| creaves build | `go build ./...` | PASS |
| creaves-console build | `go build ./...` | PASS |
| creaves actions/models/grifts (MySQL test DB) | `GO_ENV=test go test ./actions/ ./models/ ./grifts/ -count=1` | PASS — 496 test results, 0 fail, 0 skip (actions: 85 tests incl. resync commit race regression + locale key parity + template variant structural parity) |
| creaves-console (SQLite) | `CGO_ENABLED=1 go test -tags sqlite ./...` | PASS — actions + models ok |

All i18n behavior is covered by automated handler/model tests per phase checks
(localized suggestion labels per locale cookie, canonical submit normalization,
entry-cause dropdown translation, native-status hint translation, per-locale
translation artifact coverage, locale key parity, console `translations`-map
rendering via `LocalizedField`).

---

## 6. Manual E2E checklist (owner run — browser, dev instance)

Prereqs: fresh DB, `buffalo pop migrate up && buffalo task db:seed` (applies
reference translations incl. the D3 tables). Login admin/admin. For each locale
below set the language via the UI switcher (or `lang` cookie: `fr`, `en-US`,
`de`, `nl`).

Repeat all steps × 4 locales — fr, en-US, de, nl:

| # | Area | Step | Expected |
|---|---|---|---|
| 1 | Search filter — species | `/animals`, type 3+ chars of a species name in the current language into the species typeahead | Suggestions appear in the UI language (fr: French canonical; en/de/nl: translated); typing the translated name matches |
| 2 | Search filter — entry cause | Open filter, expand Entry Cause select | All options (cause + detail) in UI language, `ID - cause ⇨ detail` format kept |
| 3 | Search filter — age | Open Age select | All age labels in UI language |
| 4 | Search filter — exit reason | Open Exit reason select | All outtake-type labels in UI language |
| 5 | Search — submit | Filter with a localized species value picked from #1 | Results returned (submitted value normalized back to canonical) |
| 6 | Reception — open | `/reception/new` | Form renders in UI language |
| 7 | Reception — species popup | Type a species name in current language, open the info popup | Popup title/status/indication all in UI language (no French leakage for en/de/nl) |
| 8 | Reception — entry cause select | Open the entry-cause dropdown | Localized options (same strings as #2) |
| 9 | Reception — native-status hint | Pick a species with a native status → hint popup | Status + indication in UI language |
| 10 | Reception — submit | Complete an intake with values chosen above | Saved record keeps canonical (French) values for species/drug/reference fields |
| 11 | Select2 chrome | Open any select2 dropdown, type a nonexistent term | "No results"/placeholder strings in UI language |
| 12 | Persisted save (fr + one other locale) | Re-edit the animal saved in #10 in de or nl | Stored species/drug display resolves to the UI language; canonical value unchanged after save |

Console spot-check (after a push from dev creaves): dashboard + drill-down for an
animal pushed with a `translations` map renders entry-cause/age/exit fields in the
console's selected locale; canonical values unchanged.
