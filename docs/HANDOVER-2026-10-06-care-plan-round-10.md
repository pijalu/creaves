# HANDOVER — Care plan round 10 (2026-10-06)

Status at handover: **all round-10 work committed, tree clean, dev DB migrated.**
Everything below is verifiable from `bugs.md`, the commits, and the running app
(`buffalo dev`, port 3000, admin/admin).

---

## 1. Committed work (chronological)

| Commit | Bug(s) | What |
|---|---|---|
| `4201099` | B10-7 | Scheduled cleanup/feeding occurrences within the 8 h future horizon render as open work (tab no longer empty until due−lookahead). Golden-DOM baselines re-recorded + daily date-drift mask fix. |
| `ca432cc` | B10-2…B10-6 | Per-animal `Cage · Zone · Espèce` loc lines (all row families + history column, collapses < 992px); toggle-with-time nomenclature `○ HH:MM ⇄ ✓ HH:MM` (feeding + single-occurrence pairs); dashboard ℹ popup in place, eye removed + `?item=` dead chain deleted, animal link → `#nav-treatment?med=<type>:<id>` deep link with tab activation + scroll + highlight; protocol tab: type bubble removed, entries kind-grouped under localized `.plan-kind-separator` titles, med series join the same row family (`medSeriesRow`); B10-6 superseded marks ("Covered by protocol") for today+future legacy treatments covered by converter-made plans + dosage enrichment migration. |
| `e088d86` | docs | bugs.md fix status + e2e evidence for B10-2…B10-7. |
| `fd7b13e` | B10-6/B10-4 follow-ups | Superseded matcher accepts the `drug (dosage)` composite (post-migration prompts carry the site); `?med=` deep link prefers TODAY's series (days render newest-first). Found during the visual pass. |
| `55b5d0b` | B10-8 | "Abused" medications become CARES: converter routes wound-care + unknown-drug series to CARE plans typed "Soin"; migration re-kinded the 22 existing converted plans (Traitement — → Soin —); superseded matcher understands care cores; converter complexity refactors (`treatmentSeriesRouting`, `groupTreatmentSeries`, `convertedPlanCores`). |

**Also fixed along the way:** the phase0b golden-DOM test's daily breakage
(short-date mask added); pre-existing `staticcheck` dead var removed.

## 2. Quality state

- `go test -count=1 ./...` green (actions suite green with `-race` as of
  `55b5d0b`'s parent; the B10-8 commit ran the actions suite + gates).
- `go vet ./...`, `staticcheck ./...` clean. `gocognit -over 15` /
  `gocyclo -over 12` clean for all touched files (remaining findings are
  pre-existing in untouched files).
- Migrations applied to the dev DB: `20261026100000_b10_6_converted_plan_dosage`
  (13 plans enriched) and `20261026110000_b10_8_abused_medication_to_care`
  (22 plans re-kinded to care). Both have down migrations; schema.sql dump
  refreshed by `pop migrate` (timestamp-only diff).
- Test-data cleanliness: the browser-driven apply/undo E2E left no rows (the
  one stray application + care row from the final care-apply test was deleted
  explicitly; verified 0 rows).

## 3. Known caveats / surprises for the next person

1. **The localized `templates/animals/show.plush.{fr,de,nl}.html` forks are
   LIVE per-locale templates** — earlier project docs claimed they were stale;
   that is wrong (the runtime picks the fork by locale). Every animal-page
   change must be ported to all four forks (done for round 10; the forks
   remain divergent in unrelated older markup, ~430 diff lines vs EN).
2. The care_plan partials (`templates/care_plan/*.plush.html`) are
   byte-identical forks by policy — copy EN → `.fr/.de/.nl` after edits.
3. Golden-DOM baselines: re-record with `PHASE0B_RECORD=1 go test ./actions
   -run TestCarePlanPhase0bDOMEquivalence` and eyeball the diff per kind;
   document every deliberate delta in the test header.
4. `care_animal_plans.name` prefixes ("Traitement — ", "Soin — ") and the
   "(à vérifier)"/"(conversion)" markers are load-bearing for the converter's
   idempotency guard and `DisplayName` stripping — keep them in sync with
   `conversionMarkers` and the converter guards.
5. The `?med=` deep link and `?src=` protocol trace live in ALL FOUR
   show.plush forks (inline `<script>`), plus the popup opener scripts.

## 4. Open items (logged in bugs.md, NOT started)

### B10-1 — "Done" timestamps render in UTC (pre-existing, untouched)
Server-side `Format(` on applied/terminal timestamps; needs client-side
localization like the toggle tooltips. Scope list in bugs.md.

### B10-9 — Entries older than the 8 h window still render on the work screen
Reported on `/care_plan?kind=medication` "and other". NOT investigated.
Start at `actions/preferences.go applyPreferenceCaps` + its call site
`CarePlanIndex` (actions/care_plan.go:81-97); check whether the complaint is
about open LATE rows vs the by-design History tier, and whether the caps are
actually loaded/seeded for the request. Note the JSON branch applies no caps
by design.

### B10-10 — Cleanup cage grouping redesign (user spec, verbatim)
> cages chores does not make much sense: One single button - the apply all
> should *repeat* the time toggle button - for grouped item, follow the same
> collapsible as feeding: One button for all, with time, collapsible with the
> list / per animal button. Avoid repeats of time!

Today a cleanup cage row renders one `○ HH:MM` toggle PER ANIMAL under each
time label (time repeats N times, no collapse). Make it feeding-parity:
- ONE batch button for the cage group that CARRIES the time
  (`○ HH:MM` + the existing corner count overlay), posting the existing
  `/care_plan/apply_batch`;
- collapsible header (count + earliest time) like `.plan-feed-header`;
- per-animal toggle rows inside the expanded list, each carrying its own
  time only where it differs (day qualifiers), otherwise the group time.
Touch points: `_plan_care_line.plush.html` (×4 forks),
`careViewsOf`/`foldCareTimeGroup` in actions/care_plan_viewmodel.go,
glyph pins in `care_plan_round7_todo_glyph_test.go` (`plan-cage-apply`
currently pinned to the bare fa-clock — B10-10 gives it a time), phase0b
baselines, and `flipBatchRow` in index.plush.html JS (it flips the per-animal
toggles — keep that contract).

## 5. Suggested next steps

1. B10-9 investigation (small, engine-side) then B10-10 (medium, template +
   viewmodel) — both fit the established round pattern: bugs.md entry → plan
   section → fix → gates → agent-browser e2e (4 locales) → commit per bug.
2. B10-1 (UTC timestamps) remains the oldest open UX complaint.
3. Consider archiving round 10 to `docs/archive/` once B10-9/B10-10 close
   (bugs.md convention), using `docs/care-plan-round-10-fix-plan.md` as the
   companion plan (append B10-8/B10-9/B10-10 sections there as they land).
