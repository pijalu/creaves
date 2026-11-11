# Issue #199-17 — Heat source & O2 belong to the animal, not care entries

**Date:** 2026-09-21 · **Commit:** `8c08efe` · **Branch:** feature/open-issues-2026-10

## Observed
Heat source («Source de chaleur») and oxygen (O2) were inputs on each daily care
entry (`/cares/new`), although they stay constant for the animal's whole stay.
Staff had to re-enter them on every care.

## Expected
Heat source and O2 are attributes of the animal: entered once on the animal,
displayed on the animal page, no longer requested per care entry. Historical
care values remain untouched and visible (decision: leave history, no backfill).

## Fix
- **Migration** `20261023090000_animals_heat_source_oxygen` (additive only):
  `animals.heat_source varchar(200) NULL`, `animals.oxygen tinyint(1) NOT NULL DEFAULT 0`.
- **Model**: `Animal.HeatSource nulls.String`, `Animal.Oxygen bool` — bound
  automatically by `c.Bind(animal)` in the update action.
- **Animal edit form** (care tab, ×4 locales): `HeatSource` input + `Oxygen`
  checkbox after the feeding-times row.
- **Animal show page** (care tab, ×4 locales): conditional list items "Heat
  source" / "Oxygen" (FR «Source de chaleur»/«Oxygène», DE «Wärmequelle»/«Sauerstoff»,
  NL «Warmtebron»/«Zuurstof»), rendered only when set
  (`Valid && String != ""` — empty POST binds valid-empty string).
- **Care form** (×4 locales): removed `HeatSource`/`Oxygen` inputs and the
  heat-source autosuggest JS. Care DB columns and all care-support Go code
  (`createAutoSupportCare`, `SuggestionsHeatSource`, duplicate-guard
  fingerprint) intentionally kept; cares/show and animal-page care badges keep
  rendering historical values.

## Tests
- `TestAnimalHeatSourceOxygenOnAnimal`: form carries the fields → PUT replay
  sets heat source + O2 → DB verified → read mode shows both → cleared via
  form → DB verified → read mode hides both. Assertions pinned to read-mode
  markup (`<label class="small d-block">…</label>`) because the page's
  audit-log table echoes raw old/new values.
- `TestCareFormHasNoHeatSourceOxygen`: `/cares/new?animal_year_number=…`
  renders no `HeatSource`/`Oxygen` inputs.
- Parity check: no NEW drift on animals/_form, animals/show, cares/_form
  (all three remain in the frozen KNOWN-debt set).

## E2E (agent-browser, dev DB)
- `/animals/10213/edit` care tab: filled "Lampe infrarouge e2e" + O2 → Save →
  DB `heat_source='Lampe infrarouge e2e', oxygen=1`; show read mode listed
  "Heat source | Lampe infrarouge e2e" and "Oxygen | O2".
- FR locale: «Source de chaleur», «Oxygène» rendered.
- `/cares/new?animal_year_number=1904/26`: `hasHeat:false, hasO2:false`.
- Values cleared again → read mode hides both; dev animal left clean
  (`''/0`); language reset to en-US.

## Gates
go vet ✓ · staticcheck ✓ · gocognit 80 (baseline) · gocyclo 66 (baseline) ·
`GO_ENV=test go test -count=1 -race -cover ./...`: all packages ok except the
2 documented pre-existing grifts failures (parity NEW drift on
config/_sync_form, guest×3, sync_targets/_form; migrations replay seed
duplicate 'Relacher'). Actions coverage 50.8%.

## Residual risk
Low. Existing care rows retain heat/oxygen values in DB and UI history; new
cares store NULL/false. The `/suggestions/heat_source` endpoint now serves
only legacy care values until animal values accumulate — acceptable per
decision (no backfill).
