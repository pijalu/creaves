# GitHub issue pijalu/creaves#199 — "Correctifs 21/09" — ALL 19 ITEMS RESOLVED

**Date completed:** 2026-09-21 · **Branch:** feature/open-issues-2026-10
**Source:** https://github.com/pijalu/creaves/issues/199

Every item below was fixed (or verified already-fixed), tested (Go tests +
agent-browser e2e), gated (go vet / staticcheck / gocognit / gocyclo /
`go test -count=1 -race -cover ./...`), committed, and archived.

| Item | Commit | Archive |
|------|--------|---------|
| #199-1 Reception step 1 — species auto-filled even when type has no default species | `ff0d36a` | [2026-09-21-issue-199-1-3-reception-fixes.md](2026-09-21-issue-199-1-3-reception-fixes.md) |
| #199-2 Reception step 2→3 — date calendar steals focus; cursor should land in "État Général" | `ccb4cd9` | [2026-09-21-issue-199-1-3-reception-fixes.md](2026-09-21-issue-199-1-3-reception-fixes.md) |
| #199-3 Reception/intake — duplicate animal creation not fully blocked | `d4bf9d0` | [2026-09-21-issue-199-1-3-reception-fixes.md](2026-09-21-issue-199-1-3-reception-fixes.md) |
| #199-4 Dashboard "Créer une sortie" buttons → landing tables | `47b4b23` (pre-existing, confirmed by reporter) | — |
| #199-5 Outtake form rework (duration bucket, remove +24/+48h, precise_location) | `8b820df` | [2026-09-21-issue-199-5-outtake-form.md](2026-09-21-issue-199-5-outtake-form.md) |
| #199-6 Corpse register table + mark/unmark in animal outtake tab | `3e26a6b` | [2026-09-21-issue-199-6-animal-corpse-tab.md](2026-09-21-issue-199-6-animal-corpse-tab.md) |
| #199-7 /reports/corpses — server-side filters, default unmarked, mark-once, localized flash | `3d1d6c1` | [2026-09-21-issue-199-7-corpses-filters.md](2026-09-21-issue-199-7-corpses-filters.md) |
| #199-8 Recurring todos weekly/monthly/yearly (done spawns next occurrence) | `0420160` | [2026-09-21-issue-199-8-recurring-todos.md](2026-09-21-issue-199-8-recurring-todos.md) |
| #199-9 Back from animal sheet returns to originating landing tab | `6691f9c` | [2026-09-21-issue-199-9-landing-tab-memory.md](2026-09-21-issue-199-9-landing-tab-memory.md) |
| #199-10 /animals — sort on entry cause + exit status, letter values, cause name | `5aeb597` | [2026-09-21-issue-199-10-animals-sort-exit-status.md](2026-09-21-issue-199-10-animals-sort-exit-status.md) |
| #199-11 OT7 outtake type — nullable rating, OT7=NULL, hidden when NULL | `e058d16` | [2026-09-21-issue-199-11-ot7-nullable-rating.md](2026-09-21-issue-199-11-ot7-nullable-rating.md) |
| #199-12 Admin exception — any outtake type with any species regardless of NS | `f8e4bd8` | [2026-09-21-issue-199-12-admin-ns-bypass.md](2026-09-21-issue-199-12-admin-ns-bypass.md) |
| #199-13 ready_for_release visible in read mode + landing; FR wording | `d432d70` | [2026-09-21-issue-199-13-ready-for-release-visibility.md](2026-09-21-issue-199-13-ready-for-release-visibility.md) |
| #199-14 /users admin overhaul (localized forms, single role selector, escalation closed, filters) | `7591c68` | [2026-09-21-issue-199-14-users-admin-overhaul.md](2026-09-21-issue-199-14-users-admin-overhaul.md) |
| #199-15 Media upload — show selected filename | `333e4d2` | [2026-09-21-issue-199-15-attachment-filename-feedback.md](2026-09-21-issue-199-15-attachment-filename-feedback.md) |
| #199-16 Align /outtakes/new design with /cares/new | `601ab71` | [2026-09-21-issue-199-16-outtakes-new-design-alignment.md](2026-09-21-issue-199-16-outtakes-new-design-alignment.md) |
| #199-17 Heat source & O2 on animal (migration+forms+show), removed from care form | `8c08efe` | [2026-09-21-issue-199-17-heat-oxygen-on-animal.md](2026-09-21-issue-199-17-heat-oxygen-on-animal.md) |
| #199-18 nav-care read mode — species diet frame hidden | `5432396` | [2026-09-21-issue-199-18-hide-species-diet-frame.md](2026-09-21-issue-199-18-hide-species-diet-frame.md) |
| #199-19 nav-discovery read mode — Raison below Cause d'entrée, FR label | `074bf91` | [2026-09-21-issue-199-19-discovery-raison-placement.md](2026-09-21-issue-199-19-discovery-raison-placement.md) |

## Resolved clarifications (decisions taken during the session)
- #199-5 stay-duration bucket: display-only aid, not stored, derived from dates.
- #199-8 recurrence: marking done spawns the next occurrence (+1 week/month/year) and closes the current one.
- #199-11 rating: nullable column, OT7 → NULL, hidden when NULL.
- #199-17 heat/O2: new animal fields, care inputs removed, no backfill of historical care values.
- #199-7 filters: server-side query-param filters.

## Final gate state (2026-09-21)
- `go vet ./...` ✓ · `staticcheck ./...` ✓ · `gocognit -over 15 .` = 80 (baseline) · `gocyclo -over 12 .` = 66 (baseline)
- `GO_ENV=test go test -count=1 -race -cover ./...`: all packages pass except 2 documented pre-existing grifts failures
  (`TestTemplateVariantStructuralParity` — frozen NEW-drift debt on config/_sync_form, guest×3, sync_targets/_form;
  `TestMigrationsReplayOnEmptyDatabase` — seed duplicate 'Relacher'). Actions coverage 50.8%.
- Transient fixture-collision flakes (duplicate `year/yearNumber` on `createAnimalSearchFixtures`) traced to
  ~470 orphaned "Testsp" fixture rows left by interrupted runs; residue cleaned from creaves_test.
