# Fix archive — 2026-09-21 — Issue #199 items 1–3 (reception flow)

Source: https://github.com/pijalu/creaves/issues/199 ("Correctifs 21/09").
Branch: `feature/open-issues-2026-10`.

## #199-1 — Species auto-filled even when type has no default species

### Original report
> On `/reception/new` step 1, selecting an animal type with no default species
> still fills the Species field (first suggestion of `/suggestions/animal_species`
> = alphabetically-first species of the type). Expected: field stays empty unless
> the type defines a `default_species`.

### Fix
- `actions/suggestions.go`: new `SuggestionsAnimaltypeDefault` — returns the
  type's `default_species` as a 1-element JSON array (localized), or `[]` when
  the type has none.
- `actions/app.go`: route `GET /suggestions/animaltype_default_species`.
- `templates/reception/new.plush{,.fr,.de,.nl}.html`: the type-change handler
  now queries that endpoint instead of `/suggestions/animal_species`; the
  "don't overwrite a user-typed species" guard and
  `checkSpeciesTypeConsistency()` are unchanged.

### Tests / e2e
- `TestSuggestionsAnimaltypeDefault` (actions): type with default → 1 element;
  type without → `[]`. Passes.
- e2e (agent-browser, admin, `/reception/new`): selecting "Rodents" pre-fills
  "Red squirrel"; selecting "Large birds" leaves Species empty.

### Commit
- creaves `ff0d36a` fix(reception): auto-fill species only from type
  default_species (#199-1)

## #199-2 — Step 2→3: date calendar steals focus; cursor should land in "État Général"

### Original report
> Advancing from step 2 to step 3 of `/reception/new` opens the date flatpickr /
> focus is not in "État Général". Expected: no calendar auto-open, cursor in
> État Général.

### Fix
- `assets/js/wizard.js`: the step-nav click handler now focuses
  `$target.find('[data-autofocus]:visible:first')` when present, falling back to
  the legacy `input:eq(0)` behavior otherwise. The legacy first-input focus is
  what opened the intake-date flatpickr on entering step 3.
- `templates/reception/new.plush{,.fr,.de,.nl}.html`: the `Intake.General`
  TextAreaTag carries `data: {autofocus: "autofocus"}` (renders as
  `data-autofocus="autofocus"`).

### Tests / e2e
- e2e (agent-browser): step2→3 focus is `#animal-Intake.General`,
  `document.querySelectorAll('.flatpickr-calendar.open').length === 0`
  (previously focus = `#intakedate` with the calendar open); steps 0 and 1
  focus behavior unchanged.

### Commit
- creaves `ccb4cd9` fix(reception): focus État Général on step 3, no calendar
  auto-open (#199-2)

## #199-3 — Duplicate animal creation on double submit not fully blocked

### Original report
> 4 identical animals were created (same intake). Client-side submit blocking is
> not effective on the reception flow; no server-side duplicate guard on
> animal/intake creation. Expected: double submission creates exactly one
> animal; server-side guard rejects byte-identical intakes within the window.

### Fix
- `actions/duplicate_guard.go`: new `intakeFingerprintQuery` — same intake date,
  general state, wounds/parasites flags + details and remarks, null-safe `<=>`
  comparisons, 2-minute window (same helper as the issue-#100 guards).
- `actions/animals.go` (`AnimalsResource.Create`): after binding `AnimalCount`,
  binds a probe animal and calls `recentDuplicateExists`; on match it logs a
  warning and returns `duplicateSubmissionRedirect(c,
  "animal.duplicate.prevented", "/animals/")` — warning flash + redirect, no
  insert.
- Locale key `animal.duplicate.prevented` added in en/fr/de/nl.

### Tests / e2e
- `TestIntakeDuplicateGuardFingerprint` (actions): identical fingerprint →
  duplicate; different general state → no; NULL vs set wounds → no; entry older
  than the window → no. Passes (`GO_ENV=test go test ./actions`).
- e2e (agent-browser, admin, `/reception/new`): two identical POSTs of the
  wizard payload → first 303 (animal created), second 303 with guard warning
  log "Duplicate submission guard: skipping identical intake"; MySQL shows
  exactly 1 animal / 1 intake / 1 discovery. Test data cleaned up afterwards.
- Note: the pre-existing validation-failure path of `AnimalsResource.Create`
  re-renders `animals/new.plush.html` without the reception select context and
  500s ("selectAnimalTypes: unknown identifier") when the wizard payload fails
  validation — left as-is (out of scope, wizard validates client-side).

### Commit
- creaves `d4bf9d0` fix(reception): prevent duplicate animal creation on double
  submit (#199-3)
