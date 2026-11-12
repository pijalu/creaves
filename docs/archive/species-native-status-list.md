# Bug 2 — Species view/edit: complete fields + native status list

Source: `bugs.md` item 2. The species table was not fully viewable/editable and the
native status had to become a select list fed from the `native_statuses` table.

## Analysis

The species form (`templates/species/_form.plush.*.html`) had two field bugs and two
missing fields:

- `CreavesGroup` / `Subside` were bound but the model fields are `AgwGroup` /
  `SubsideGroup` — so those inputs never persisted.
- `Huntable` (bool column) was absent from the form and from the show page.
- `NativeStatus` was treated as a **translated free-text field** (present in
  `setTranslationValues`/`saveTranslations` field lists) instead of a foreign reference
  to `native_statuses`. There was no select, and the show page rendered the raw value.

Pre-flight DB check confirmed `species.native_status` only ever contains `NS1..NS5`
(no free-text rows), so no data migration is required — the column already stores the
reference id.

## Plan

1. Form (`_form.plush.{html,fr,de,nl}.html`):
   - Rename `CreavesGroup` → `AgwGroup`, `Subside` → `SubsideGroup`.
   - Add `f.CheckboxTag("Huntable")`.
   - Add `f.SelectTag("NativeStatus", {options: selectNativeStatuses, value: selectedNativeStatus})`.
2. Handlers (`actions/species.go`):
   - `New`/`Edit` set `selectNativeStatuses` (via `nativeStatusesToSelectables`) and
     `selectedNativeStatus`.
   - Remove `native_status` from all `setTranslationValues`/`saveTranslations` field
     lists (it is a reference, not a translated string).
   - `Update` resets `Game` and `Huntable` to `false` before `c.Bind` (unchecked
     checkboxes are not posted, so they would otherwise stay `true`).
   - `Show` resolves a display label `nativeStatusLabel` from the selectables (fallback
     = raw id) — done handler-side rather than with the plush `tname` helper because the
     base-locale fallback would render the raw NS id.
3. Show (`show.plush.{html,fr,de,nl}.html`): add `Huntable` row + `Native status` row
   rendering `<%= nativeStatusLabel %>`.

## Test approach

- Go tests (`actions/species_form_test.go`):
  - `TestSpeciesCreateBindsAllFields` — POST create persists agw/subside/huntable/native_status.
  - `TestSpeciesUpdateResetsUncheckedFlags` — update with unchecked boxes resets them to false.
  - `TestNativeStatusesToSelectables` — helper returns id-valued options with resolved labels.
- e2e (agent-browser): edit SP19, confirm select options are the translated
  `native_statuses` labels with NS ids as values, change NS + Huntable + groups, save,
  and confirm the show page renders a `Huntable` row and the resolved `Native status` label.

## Quality gates

- `go vet ./...` — clean.
- `staticcheck ./...` — no new findings in `actions/`/`models/` (only pre-existing).
- `gocognit -over 15 .` / `gocyclo -over 12 .` — only pre-existing functions flagged;
  none of the touched species handlers.
- `go test -count=1 -race -cover ./...` — see final full-suite run recorded in the
  consolidation summary.

## e2e evidence

- Edit form `select[name="NativeStatus"]` options (English):
  `NS4=Domestic, NS2=Exotic, NS5=Exotic / Domestic, NS3=Exotic species of concern, NS1=Native`,
  selected `NS1` for SP19.
- After setting NS2 + Huntable + E2EAGW/E2ESUB and saving, DB row:
  `SP19 | agw_group=E2EAGW | subside_group=E2ESUB | game=0 | huntable=1 | native_status=NS2`.
- Show page rendered a `Huntable` row and `Native status` = `Exotic` (label, not `NS2`).
- SP19 restored to its original values afterwards.
