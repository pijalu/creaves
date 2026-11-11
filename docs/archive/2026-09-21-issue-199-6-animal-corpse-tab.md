# Issue #199-6 — Corpse register on the animal sheet (outtake tab)

Archived: 2026-09-21 — commit `3e26a6b` on branch `feature/open-issues-2026-10`.

## Observed
Corpse destination tracking (issue #149) was only reachable via
`/reports/corpses`. On the animal sheet there was no trace of whether the
corpse had been disposed of / sent to an organism.

## Expected
For animals whose outtake type is Dead=true, the outtake tab shows a
"Corpse register" block: destination, destination date, marking user;
a Mark action (modal with organism autocomplete + date) when unmarked,
and an Unmark action (admin only) when marked.

## Fix
- `actions/animals.go` (Show): when `animal.Outtake.Type.Dead`, set
  `corpseOuttake=true` and always set `corpseMarkedBy` (empty string when
  nobody marked it — Plush errors on unknown identifiers).
- `actions/reports_corpses.go`: `ReportsCorpsesMark` / `ReportsCorpsesUnmark`
  honor an optional `back` param via `safeRedirectTarget`, falling back to
  `/reports/corpses?year=…`.
- `templates/animals/show.plush.html` (+ `.fr/.de/.nl`): corpse table,
  Mark button + modal (`#corpseTabMarkModal`) posting to
  `/reports/corpses/mark` with hidden `back=/animals/<id>#nav-outtake`,
  admin Unmark form posting to `/reports/corpses/unmark`. Localized
  headings/labels in all 4 languages.
- `actions/reports_corpses_test.go`: `TestCorpseRegisterMarkBackRedirect`
  covers the back-param redirect and unsafe-target fallback.

## Verification
- e2e (agent-browser, dev DB): animal 2507/25 (DCD) — table visible, Mark
  modal → POST → redirect back to `/animals/<id>#nav-outtake` with
  "1 corpse(s) marked" flash; row shows destination/date/admin login +
  Unmark; Unmark clears fields and restores Mark button. Released animal
  8299 shows no corpse block. FR labels verified. Test data restored to
  NULL after the cycle.
- Gates: `go vet`, `staticcheck` clean; gocognit 78 (baseline), gocyclo
  64 vs 63 baseline (`AnimalsResource.Show` 14→15, already over gate);
  `go test -count=1 -race -cover ./...` — all pass except the two
  documented pre-existing grifts failures
  (`TestTemplateVariantStructuralParity`, `TestMigrationsReplayOnEmptyDatabase`).
