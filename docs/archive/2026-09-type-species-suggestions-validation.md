## 1. ✅ DONE — Autocomplete cache ignores linked inputs (Species)

**Symptom:** on `/reception/new`, selecting a type (e.g. Canidé) then typing `%` in
Species lists canids; switching to another type (chauves-souris) and typing `%` again
still lists canids.

**Cause (part 1):** jquery.auto-complete caches per input *value* only; Species
suggestions also depend on `#animal-AnimaltypeID`.

**Fix:** `cache: 0` added to the Species `autoComplete({...})` block in:

- `creaves/templates/reception/new.plush.html`
- `creaves/templates/reception/new.plush.fr.html`
- `creaves/templates/reception/new.plush.de.html`
- `creaves/templates/reception/new.plush.nl.html`

(Other multi-input fields — postal code/city, discoverer names/address — already had
`cache: 0` in all 4 locales. `Discovery.Location` and `Discoverer.Country` are
single-input and keep the default cache on purpose.)

## 2. ✅ DONE — Refocus does not re-query (plugin bug)

**Cause (part 2, root):** in `creaves/assets/js/jquery.auto-complete.js`, the focus
handler triggers a *synthetic* `keyup.autocomplete` whose `e.which` is `undefined`.
`$.inArray(undefined, [13,27,35,36,37,38,39,40])` returns `0`, so `!~0` is `false`
and the keyup handler bails: no re-query on refocus, stale dropdown HTML reshown.

**Fix:** guard changed to `if (!e.which || !~$.inArray(e.which, ...))` (line ~132).

Also fixes the same stale-refocus behavior for PostalCode/City fields (minChars: 0).

**Verified (agent-browser, FR):** webpack rebuilt the plugin; Canidé → `%` listed
canids; switched to chauves-souris → refocus Species (no retyping) → list showed bats
(Pipistrelle, etc.), no canids. Items 1+2 confirmed fixed together.

## 3. ✅ DONE — Server rejects mismatched type/species with HTTP 422

**Symptom:** saving an animal with species "Lapin de Garenne" and a non-matching type
fails: `422 - species "Lapin de Garenne" does not match selected animal type`.

**Decision (user):** let the user save invalid type/species combos; only show visible
warnings on new/update forms. Do **not** hard-block.

**Change:** `creaves/actions/animal_species_consistency.go`

- `completeAndValidateSpeciesType`: replace the final
  `return fmt.Errorf("species %q does not match selected animal type", ...)` (line 48)
  with a warning `log.Printf(...)` + `return nil` (same soft-block pattern already used
  for unmapped species). Update the function comment.
- Keep the blank-type + multiple-mappings branch rejected (cannot infer; UI select is
  required anyway).
- Callers (`creaves/actions/animals.go` lines ~623 Create, ~804 Update) keep calling it
  unchanged.
- The show-page banner already exists: `speciesTypeMismatch` →
  `templates/animals/show.plush.*.html` ("type and species do not match") — no change.

**Test update:** `creaves/actions/animal_species_consistency_test.go` lines 52–55:
mismatching animal must now be **accepted** (nil error, submitted type preserved).
`TestCompleteAndValidateSpeciesTypeMultipleMappingsRejected` stays unchanged.

## 4. ✅ DONE — reception/new: visible warnings + wizard confirm dialog

Per locale (4 files: `creaves/templates/reception/new.plush[.fr|.de|.nl].html`):

1. Add `<div id="speciesUnknownHint" class="alert alert-danger d-none">…</div>` next to
   existing `#speciesTypeHint` (line ~113):
   - en: `unknown species` · fr: `espèce inconnue` · de: `unbekannte Art` ·
     nl: `onbekende soort`
2. In the `$("#animal-Species").on("change input", …)` handler:
   - `$.getJSON('/suggestions/species_type', …).fail(...)` → show
     `#speciesUnknownHint`, hide `#speciesTypeHint`.
   - success branches (auto-set type / mismatch / match) → hide/show hints
     consistently (mismatch → show `#speciesTypeHint`, hide `#speciesUnknownHint`;
     otherwise hide both).
   - empty term → hide both hints.
3. Wizard confirm before leaving step 1 (bind at template parse time so it runs
   *before* the wizard.js `nextBtn` handler bound on `document.ready`):

   ```js
   $('#step-1 .nextBtn').on('click', function(e){
       var mismatch = !$('#speciesTypeHint').hasClass('d-none');
       var unknown  = !$('#speciesUnknownHint').hasClass('d-none');
       if ((mismatch || unknown) && !confirm("…")) {
           e.stopImmediatePropagation();
           e.preventDefault();
           return false;
       }
   });
   ```

   Confirm text:
   - en: `The species is unknown or does not match the selected type. Continue anyway?`
   - fr: `L'espèce est inconnue ou ne correspond pas au type sélectionné. Continuer quand même ?`
   - de: `Die Art ist unbekannt oder passt nicht zum ausgewählten Typ. Trotzdem fortfahren?`
   - nl: `De soort is onbekend of komt niet overeen met het geselecteerde type. Toch doorgaan?`

**Status:** applied to all 4 locales; all inline `<script>` blocks pass `node --check`.

**Binding fix during verification:** direct `.on('click')` bound at parse time is wiped
on document.ready in this stack (only delegated handlers survive), and a delegated
handler fires *after* the wizard's direct nextBtn handler (cannot veto). Fix: bind
direct inside `$(function(){ ... })` — registered here, it runs after wizard.js
setupWizard's ready handler, so the confirm handler executes first and
`stopImmediatePropagation()` cancels the advance.

**Verified (agent-browser, FR):** mismatch → `#speciesTypeHint` visible, Next shows
confirm (accept → advances); unknown species → `#speciesUnknownHint` visible, Next
shows confirm (cancel → stays on step 1); matching combo → no confirm, advances.
`/suggestions/species_type` returns 404 for unknown species, 200 for known.

## 5. ✅ DONE — animals/_form (update): full reception/new type/species approach

**Request:** the edit (update) form must get the *same* type/species approach as
reception/new, including suggestions — not just a mismatch banner.

Update form current state (`creaves/templates/animals/_form.plush[.fr|.de|.nl].html`):

- Species autocomplete queries `/suggestions/animal_species` **without**
  `animaltype_id` (unfiltered, `minChars: 1`, default cache) — reception/new filters
  by selected type, refocus-lists the type's species, and uses `cache: 0`.
- No `species_type` lookup: no auto-set of type from species, no mismatch hint,
  no `is-invalid` marking for unknown species.
- Only `#speciesHint` (`/hint/speciesDetails`) with "Unknown species !" danger alert.
- Reception/new also shows `#speciesTypeHint` / `#speciesUnknownHint` and (item 4)
  asks confirmation before leaving the step.

Per locale, port from reception/new (after item 4 lands — copy its final markup/JS):

1. Species autocomplete: pass `animaltype_id: $('#animal-AnimaltypeID').val()`,
   `minChars: 0`, `cache: 0`, `onSelect` triggers change; empty term + no type →
   `response([])`.
2. Type select change handler: when Species empty, prefill with first species of the
   newly selected type (same `/suggestions/animal_species` q="" pattern).
3. `#animal-Species` change/input handler: `species_type` lookup → auto-set type when
   blank, show `#speciesTypeHint` on mismatch, `#speciesUnknownHint` on 404, hide both
   on match/empty; keep `is-invalid` marking + `#nativeGroupHint` behavior equivalent
   to reception/new (keep existing `#speciesHint` speciesDetails info as-is).
4. Add `#speciesTypeHint` + `#speciesUnknownHint` divs (localized strings as in
   item 4) near the Species input.
5. Submit-time confirm: on form submit, if mismatch or unknown hint visible →
   `confirm(...)` (same localized strings as item 4); cancel aborts submit. (Update
   form has no wizard, so hook the submit instead of `.nextBtn`.)

**Status:** all 5 steps applied to en/fr/de/nl; inline scripts pass `node --check`.
Submit confirm + type-change prefill are bound inside `$(function(){ ... })` (see
item 4 binding note).

**Verified (agent-browser, FR, `/animals/1/edit`):** mismatch → `#speciesTypeHint`
visible; unknown → `#speciesUnknownHint` visible; submit cancel stays on edit page;
submit accept saves and the show page renders the `speciesTypeMismatch` banner;
Species autocomplete filtered by type (Canidés → canids); type-change prefills
Species (Canidés → "Chien viverrin"). Test data restored to original afterwards.

## 6. ✅ DONE — /species admin list: use species suggest for search

**Page:** `http://localhost:3000/species` (species maintenance list).

**Request:** the search field should use the species suggestion endpoint
(`/suggestions/animal_species` autocomplete, as on reception/new) instead of plain
free-text search, so it's easier to find species (incl. localized names).

**Notes:**

- Check `creaves/templates/species/` list template + handler for the current search
  wiring before editing.
- Autocomplete source: `$.getJSON('/suggestions/animal_species', { q: term }, …)`
  (already locale-aware via `currentLang` on the server; single-input → default cache
  OK).
- All-language rule applies (4 locale templates).

**Verified (agent-browser, FR `/species`):** typing "Ren" suggests species incl.
"Renard roux"; selecting it auto-submits `/species?q=Renard+roux` and filters the
list to 1 row. Applied to en/fr/de/nl; inline scripts pass `node --check`.

## 8. ✅ DONE — TestLoadConfigMultipleConfigsCoexist fails (pre-existing, test pollution)

**Found during item 7 quality gate** (`go test -count=1 -race -cover ./...` →
`FAIL creaves/actions`). Reproduced on base commit `6a04fb8` — **not a regression**
from this work.

**Symptom:** `configs_bug_test.go:326: LoadConfig picked <X>, want oldest active <Y>`.

**Root cause:** `LoadConfig` short-circuits on the global `CurrentConfig` cache
(`if CurrentConfigGet() != nil { return ... }`). In the full-package run against the
shared MySQL test DB, an earlier test leaves `CurrentConfig` set, so `LoadConfig`
returns that stale config instead of the test's freshly-seeded oldest active row. The
test clears the cache once at start, but a concurrently/earlier-running test re-sets
it before the assertion. Test-isolation issue.

**Fix applied (commit 6296365):** two test-isolation gaps —
1. Back-date the seeded "oldest active" config (`first.CreatedAt = now-24h`): the
   shared test DB already held an *older* active row (auto-created `BigMac.local`
   default, created_at 08:05), which `LoadConfig` legitimately picked over the test's
   freshly-created `first`. (The initially suspected `CurrentConfig` cache pollution
   was ruled out — clearing it alone did not fix the failure.)
2. Re-clear `CurrentConfigSet(nil)` right before the `LoadConfig` call so the lookup
   under test always hits the DB.

`LoadConfig` semantics unchanged. `go test -count=1 ./actions` now passes (ok 9.4s).

## 7. ✅ DONE — End-to-end verification (per AGENTS.md checklist)

- [x] `buffalo dev` rebuilds Go + assets without errors (plugin rebuilt; pages render)
- [x] `/reception/new`: switch type after listing species → refocus refetches (bug 2) — bats shown after switching to chauves-souris
- [x] Mismatch (Canidé + Lapin de Garenne): hint visible, Next → confirm dialog;
      cancel stays on step 1, OK proceeds
- [x] Unknown species: unknown hint visible, Next → confirm dialog
- [x] Save with mismatch succeeds (no 422) on create **and** update — update verified end-to-end (`/animals/1`); create path shares `completeAndValidateSpeciesType` soft-accept (unit-tested, commit 6a04fb8)
- [x] Update form shows mismatch warning — `#speciesTypeHint` on `/animals/1/edit`
- [x] Show page banner still appears for mismatched animals — verified on `/animals/1/`
- [x] `go test ./actions -run TestCompleteAndValidateSpeciesType` — ok
- [x] Repeat on FR (and spot-check DE/NL) — locale parity — FR full; DE/NL mismatch+unknown hints + confirm-cancel verified; en confirm verified

**Quality gates (item 7, each run separately):**
- `go vet ./...` — clean (exit 0)
- `staticcheck ./...` — no findings
- `gocognit -over 15 .` / `gocyclo -over 12 .` — violations are pre-existing legacy complexity (event_producer, sqldump, startup_seed, …); none in code touched by this work (`animal_species_consistency` absent from both lists)
- `go test -count=1 -race -cover ./...` — surfaced pre-existing `TestLoadConfigMultipleConfigsCoexist` failure → tracked & fixed (item 8); full `./actions` suite now `ok`

---

### Browser repro notes (Chrome/agent-browser)

- Selecting a type auto-fills Species with the first matching species when empty
  (type-change handler) — clear the field before typing `%` to reproduce the stale-list
  bug.
- `/suggestions/animal_species?q=%25&animaltype_id=<Canidés-id>` returns
  `["Chien viverrin","Loup","Renard roux"]` on the seeded dev DB.
- `agent-browser keyboard type` does not reliably emit `keyup` for the plugin's
  handler; dispatch `jQuery.Event("keyup")` with `which=53` when scripting the repro.
