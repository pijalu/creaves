# Issue #199-18 — Hide species diet frame from animal care tab read mode

**Date:** 2026-09-21 · **Commit:** `5432396` · **Branch:** feature/open-issues-2026-10

## Observed
`/animals/<id>#nav-care` read mode displayed a frame listing every feeding
regime of the species (the "Diet by life stage" block added for issue #145),
cluttering the animal sheet.

## Expected
Read mode keeps only the animal's own alimentation field and its feeding
times. Edit mode unchanged. The feeding-guides admin CRUD
(`/feeding_guides`) and the `/suggestions/feeding_guide` autocomplete used by
the care form stay.

## Fix
- Removed the `if (hasFeedingGuides)` block ("Diet by life stage" /
  «Régime alimentaire par stade» / «Fütterung nach Lebensstadium» /
  «Voeding per levensfase») from `templates/animals/show.plush{,.fr,.de,.nl}.html`.
- Removed the `setAnimalFeedingGuides` context loader call from
  `AnimalsResource.Show` and the now-unused function from
  `actions/feeding_guides.go` (`feedingGuideViews` stays for the admin index).
- Edit mode needed no change: `_form.plush.*.html` never referenced
  `feedingGuides`.

## Tests
- New `TestAnimalShowCareTabHasNoSpeciesDietFrame`: creates a feeding guide
  for the fixture animal's species, then asserts the show page still contains
  the "Feeding" label but none of the 4 localized diet-frame labels and no
  guide text leak.
- Existing feeding-guides admin tests (CRUD, admin-only, suggestions) pass
  unchanged.
- Parity check: no NEW drift on animals/show (pair stays in frozen
  KNOWN-debt set).

## E2E (agent-browser, dev DB)
- Inserted a temporary guide row for "Cygne tuberculé" (species of dev animal
  10213).
- EN read mode: `#nav-care` labels = ["Feeding"] only; no "Diet by life
  stage", no guide-text leak.
- FR read mode: labels = ["Alimentation"]; no «Régime alimentaire par stade».
- Edit mode: Feeding + HeatSource inputs present, no diet frame (unchanged).
- Temp row deleted (0 guides left); language reset to en-US.

## Gates
go vet ✓ · staticcheck ✓ · gocognit 80 (baseline) · gocyclo 66 (baseline) ·
`GO_ENV=test go test -count=1 -race -cover ./...`: all packages ok except the
2 documented pre-existing grifts failures; actions 50.8% coverage.
(One transient fixture-collision flake `TestCareDuplicateGuardFingerprint`
from a leftover row of an earlier run — passes on re-run, unrelated.)

## Residual risk
Low. The frame was display-only; no data removed. Species diets remain
editable under `/feeding_guides` and reachable via the care-form suggestion
endpoint.
