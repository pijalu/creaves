# Type/Species Suggestions & Validation — Bug Follow-up

Tracking document for the reception/new suggestion-cache bug and the type/species
mismatch handling changes. All work is in the `creaves/` project.

**Guideline** (same as `/Users/muaddib/dev/creaves.project/bugs.md`):
1. Create a detailed fix plan for each bug - the plan must contain test approach and validation steps - execute the plan and validate the fix when all elements are in place.
2. Any issues found must be fixed and the fix plan must be updated accordingly.
3. Issues found during testing must be fixed and the fix plan must be updated accordingly.
4. Each bug should be moved to docs/archive when tested and closed as the associated plan.
5. **All changes must be tested, including e2e testing using the [agent-browser skill](../.agents/skills/agent-browser/SKILL.md)** — no fix is complete without e2e evidence (commands, URL, captured output).
6. Use interactive shell/filmstrip to validate the output of the tool - you must verify the actual terminal output using agent-browser skill
7. Check code quality with each tool run separately (do not chain them with `;` or `&&`):
- `go vet ./...`
- `staticcheck ./...`
- `gocognit -over 15 .`
- `gocyclo -over 12 .`
- `go test -count=1 -race -cover ./...`
Fix any issues.
8. Commit each fix with a clear and descriptive commit message

### Session constraints
- **creaves contains real production data**: never drop/destroy data; schema changes must be additive/backward compatible (no destructive migrations).

At the end of the session - the bug list must be empty, all changes committed and resolved entries archived in `docs/archive/`. If new items are added, restart the process.

**All-language rule:** every template change below must be applied to all 4 locales:
base (en), `.fr`, `.de`, `.nl`.

---

## 1. ✅ DONE (code) / ⏳ VERIFY — Autocomplete cache ignores linked inputs (Species)

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

## 2. ✅ DONE (code) / ⏳ VERIFY — Refocus does not re-query (plugin bug)

**Cause (part 2, root):** in `creaves/assets/js/jquery.auto-complete.js`, the focus
handler triggers a *synthetic* `keyup.autocomplete` whose `e.which` is `undefined`.
`$.inArray(undefined, [13,27,35,36,37,38,39,40])` returns `0`, so `!~0` is `false`
and the keyup handler bails: no re-query on refocus, stale dropdown HTML reshown.

**Fix:** guard changed to `if (!e.which || !~$.inArray(e.which, ...))` (line ~132).

Also fixes the same stale-refocus behavior for PostalCode/City fields (minChars: 0).

**Verification pending:** webpack rebuild (`buffalo dev` watches assets; confirm
`public/assets/jquery.auto-complete.*.js` regenerated), then browser repro:
Canidé → `%` → switch to chauves-souris → refocus Species → list must show bats.

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

## 4. ✅ DONE (code) / ⏳ VERIFY — reception/new: visible warnings + wizard confirm dialog

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

**Status:** applied to all 4 locales; all 10 inline `<script>` blocks per file pass
`node --check`. Browser verification pending (item 7).

## 5. ⏳ TODO — animals/_form (update): full reception/new type/species approach

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

## 6. ⏳ TODO — /species admin list: use species suggest for search

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

## 7. ⏳ TODO — End-to-end verification (per AGENTS.md checklist)

- [ ] `buffalo dev` rebuilds Go + assets without errors
- [ ] `/reception/new`: switch type after listing species → refocus refetches (bug 2)
- [ ] Mismatch (Canidé + Lapin de Garenne): hint visible, Next → confirm dialog;
      cancel stays on step 1, OK proceeds
- [ ] Unknown species: unknown hint visible, Next → confirm dialog
- [ ] Save with mismatch succeeds (no 422) on create **and** update
- [ ] Update form shows mismatch warning
- [ ] Show page banner still appears for mismatched animals
- [ ] `go test ./actions -run TestCompleteAndValidateSpeciesType`
- [ ] Repeat on FR (and spot-check DE/NL) — locale parity

---

### Browser repro notes (Chrome/agent-browser)

- Selecting a type auto-fills Species with the first matching species when empty
  (type-change handler) — clear the field before typing `%` to reproduce the stale-list
  bug.
- `/suggestions/animal_species?q=%25&animaltype_id=<Canidés-id>` returns
  `["Chien viverrin","Loup","Renard roux"]` on the seeded dev DB.
- `agent-browser keyboard type` does not reliably emit `keyup` for the plugin's
  handler; dispatch `jQuery.Event("keyup")` with `which=53` when scripting the repro.
