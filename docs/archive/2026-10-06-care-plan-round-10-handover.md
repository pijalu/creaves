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

### B10-1 — browser-local done timestamps
**Implemented and verified in checkout.** Applied timestamps are emitted as RFC-3339 instants and formatted with `Date.toLocaleTimeString` in all four locale forks on animal Treatment badges/tooltips, legacy-treatment badges, and care-plan History. `TestBuildDayPlanViewMedTiers` pins the UTC RFC-3339 value; focused localization/template tests pass. Authenticated read-only smoke loaded `/care_plan` and `/animals/8635`; those production-backed screens contained no completed timestamp. Non-mutating browser E2E appended synthetic `<time class="js-local-time" datetime="2026-10-06T13:13:00Z">` on `/treatments` and ran the exact formatter; session `b10` timezone `Europe/Brussels` rendered `03:13 PM` (matching `toLocaleTimeString`). Formatter behavior verified without writes; live persisted-record rendering remains unverified. No production data changed.

### B10-9 — Entries older than the 8 h window still render on the work screen
**CLOSED — not reproduced on available instance.** `CarePlanIndex` loads per-kind saved caps and applies them before viewmodel construction; strict-over-limit late/missing and scheduled rows are dropped, exact boundaries remain. Authenticated read-only `/preferences` showed 8 h late/future + 1 h now values for all six kinds. `/care_plan?kind=medication` showed four History rows and no old open rows; the uncapped JSON read model's older rows were terminal items, retained in History by design. Boundary regression test passes. No cap change warranted. Reopen only with identified late/missing OPEN row beyond persisted kind cap on HTML route, with source, due timestamp and preference.

### B10-10 — Cleanup cage grouping redesign (user spec, verbatim)
> cages chores does not make much sense: One single button - the apply all
> should *repeat* the time toggle button - for grouped item, follow the same
> collapsible as feeding: One button for all, with time, collapsible with the
> list / per animal button. Avoid repeats of time!

**CLOSED — fixed; focused tests and available read-only E2E pass.** Cleanup cage rows collapse occurrence toggles under count headers; batch button carries `○ HH:MM`; animal-mode action toggles retain time. Feeding animal mode no longer repeats identity/time in second cell. Four shared care partial forks have exact parity. Authenticated session `b10` showed 98 cleanup tasks, `Bac noir E` count 2, `S10 S` count 11, and 09:00 batch controls; feeding animal view showed identity/detail separation. Live fr/de/nl route headings localized. Earliest-time choice among distinct times remains unverified because all observed groups displayed 09:00; no Apply used.

## 5. Suggested next steps

1. B10-9 closed as not reproduced; B10-10 fixed with documented tests and read-only E2E, with earliest-time selection caveat.
2. B10-1 implemented, four-locale templates and focused tests pass; synthetic browser fixture confirms UTC-to-browser-local formatting without data mutation. No persisted applied row available for read-only E2E.
3. After remaining queued round-10 objectives are validated, archive this handover under `docs/archive/` per `bugs.md` convention; retain companion plan with all evidence/caveats.
