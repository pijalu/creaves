# Care Plan improvements — implementation plan and schedule

**Prepared:** 2026-10-26  
**Scope:** Follow-up UX/behavior improvements reported on `/care_plan`.  
**Schedule basis:** Relative estimates in engineering workdays; no calendar due dates were supplied. Estimates exclude review/merge queue time. Run sequentially because later UI/E2E checks depend on earlier filtering and viewmodel behavior.

## Confirmed behavior contract

1. **Preference-window filtering applies to every plan kind.** Per-kind `late_show_hours` and `future_show_hours` determine which open occurrences can be acted on. Hide an occurrence and its action when no applicable Apply action remains; omit an animal row with no applicable actions. At an 8-hour configured limit, exclude the exact boundary and beyond: late age `>= cap`, scheduled future distance `>= cap`. Terminal history is not open work and remains governed by existing History behavior.
2. **Yesterday is calendar-day based.** Show the yesterday badge and its Apply action only when that occurrence passes the same applicable preference-window rules. Do not infer elapsed-24-hour semantics.
3. **Cleanup cage view.** Keep cage-level Apply-all, include due time in its button label, and apply to remaining eligible animals only. Show an empty animal cell for a single-animal cage; for multiple animals show animal number and one time-bearing Apply control per line. Show a partial-completion red/green diagonal only on cage-level Apply-all, with a fixed split acceptable and proportional fill optional. Preserve per-animal Apply in `group=animal` when eligible.
4. **Table consistency.** Each table’s animal column adapts to its own longest cell; do not force one table’s content width onto unrelated tables. Care rows should use the medication animal-row visual style.
5. **Feeding tier navigation.** Place Late/Now/Later summary indicators within Feeding tab content. Selecting a tier collapses other tiers in that active tab only; do not collapse tiers on other tabs.
6. **Preserve B10-9 disposition.** B10-9 remains closed—not reproduced. Do not change cap behavior beyond the newly confirmed inclusive boundary contract unless a concrete over-cap OPEN HTML occurrence is demonstrated.

## Dependency-ordered goals and schedule

| Sequence | Goal / outcome | Estimate | Depends on | Primary touchpoints |
|---|---|---:|---|---|
| 1 | **Preference-filter semantics and occurrence visibility.** Centralize/reuse eligibility logic for late, now, future and yesterday; use inclusive cutoffs; ensure each rendered row/animal has at least one valid Apply; apply consistently to all plan kinds. Add unit/handler regression tests for exact cutoff, just-inside cutoff, future cap, day-boundary yesterday, terminal rows, empty rows, and each supported plan kind. | 2–3 days | None | `actions/preferences.go`, `actions/care_plan.go`, plan viewmodel/builders, focused action tests |
| 2 | **Cleanup cage grouping and partial Apply-all.** Make cage Apply label carry time; construct only eligible remaining-item refs; render single-animal empty cell and multi-animal numbered/time controls; represent partial state on cage-level control. Keep/restore eligible animal-mode Apply. Test zero/one/many animals, fully open/partially applied/fully applied cage, mixed preferences and batch re-verification. | 2–3 days | 1 | Care viewmodel, `CarePlanApplyBatch`, `_plan_care_line.plush.html` and locale variants, glyph and phase0b golden tests, batch tests |
| 3 | **Per-table layout, care-row parity and Feeding navigation.** Implement adaptive longest-cell width per table; align care animal row style with medication; move tier summary within Feeding tab and scope tier-collapse interaction to active tab. Add DOM/layout assertions and navigation tests. | 1–2 days | 1 | Care-plan template/CSS, four localized index forks, shared row partials, JS/tier tests and golden baselines |
| 4 | **Integrated locale, regression and authenticated E2E sweep; documentation/closure.** Run targeted tests then broader suite/gates available to this checkout; verify all user-visible changes in `en-US`, `fr`, `de`, `nl`; inspect DOM/text and click only safe navigation/filter controls in authenticated E2E. Never press Apply or change production data. Record observed results and unresolved limitations; archive handover only after B10-9/B10-10 status is accurate. | 2–3 days | 1–3 | Four locale forks, focused and full test suites, `git diff --check`, agent-browser, fix plan/archive/handover |

**Total estimate:** 7–11 engineering workdays, excluding review/merge waiting. Schedule checkpoints: end of day 3, preference-filter contract/test gate; end of day 6, cleanup and partial-batch gate; end of day 8, layout/navigation gate; end of day 11, locale/E2E/regression closeout. Re-estimate after Goal 1 if preference filtering requires broader domain changes.

## Per-goal completion gates

**Goal 1 implementation progress (2026-10-27):** `applyPreferenceCaps` now excludes late/missing items at age `>= late_show_hours` and scheduled items at distance `>= future_show_hours`, before viewmodel projection. New tests cover exact/just-inside/beyond boundaries, terminal History retention, each of six action kinds, and an over-cap yesterday occurrence producing no actionable care row. Focused `go test ./actions -run 'TestPreferenceCaps|TestCarePlanPagesAllLocales|TestTemplateVariantStructuralParity' -count=1`, full `go test ./actions -count=1`, `go test ./grifts -run TestTemplateVariantStructuralParity -count=1`, `go test ./...`, `go vet ./...`, and `git -C creaves diff --check` passed.

- **Goal 1:** Boundary tests prove `< cap` included and `>= cap` excluded for late and future; calendar-yesterday fixture proves badge/action filtering; no zero-action rows; terminal history unaffected; tests pass and `git diff --check` clean. Current progress covers cap filtering before projection, boundary/all-kind/terminal/yesterday-empty-row unit tests; remaining integration evidence for per-kind handler preferences and no zero-action rows in every plan kind still required.
- **Goal 2:** Tests prove batch request contains eligible remaining animals only; partial/empty/single/multi cage DOM is correct; cage button presents time; animal grouping retains applicable Apply; four locale templates structurally aligned; phase0b goldens reviewed.
- **Goal 3:** DOM/CSS tests show animal width is shared within each table and content-adaptive; Care row matches medication reference; Feed tier indicators sit inside tab; selection collapses siblings only within active tab.
- **Goal 4:** Required focused tests, localization/template checks, and available broader quality gates pass; authenticated E2E asserts routes, text, DOM and locale outputs without Apply when authorized session/fixture is available. If unavailable, document exact blocker and keep Goal 4 incomplete; plan, bugs/archive and handover state must agree without claiming browser evidence.

## Risks and constraints

- The user’s newly specified `>=` boundary differs from the prior B10-9 test’s inclusive-at-cap behavior. Goal 1 must update that test and docs intentionally; it must not be treated as a discovered production incident.
- UI filtering must agree between rendered Apply affordances, counts/badges and batch refs; otherwise hidden work could still be applied or visible rows could be non-actionable.
- Batch endpoint already re-verifies each item. Preserve that concurrency/stale-item safety; do not treat client-side filtering as authorization.
- “Longest cell” means per individual table, not globally. Validate long localized animal labels and narrow viewports to avoid overflow.
- E2E environment is production-backed. Use read-only observations and synthetic DOM/test fixtures; do not mutate records with Apply controls.
- Do not claim full live persisted-occurrence E2E for a behavior unless a safe fixture exists. Document synthetic/unit evidence separately from persisted-record evidence.

## Goal 4 rerun status (2026-10-06)

**Automated evidence from this run:**
- `go test ./actions -run 'TestPhase3.*|TestCarePlanPagesAllLocales|TestGoal3Layout' -count=1` — PASS (`ok creaves/actions 1.239s`).
- `CGO_ENABLED=1 go test -tags sqlite ./actions -run 'TestPhase3.*|TestCarePlanPagesAllLocales|TestGoal3Layout' -count=1` — PASS (`ok creaves/actions 0.457s`).
- `go test ./grifts -run TestTemplateVariantStructuralParity -count=1` — FAIL: reports frozen/known structural drift in existing unrelated locale templates (for example `animalages/_form.plush.html` vs `.fr.html`; 108 known-drift pairs). No template-parity code or tests altered to suppress this.
- Initial `go test ./...` — FAIL: `TestCarePlanPhase0bDOMEquivalence` exposed expected-old vs actual-new DOM deltas for accumulated Goal 2/3 template changes. Regenerated its four kind snapshots with `PHASE0B_RECORD=1 go test ./actions -run '^TestCarePlanPhase0bDOMEquivalence$' -count=1`; reviewed diffs, then reran that DOM test — PASS. Latest `go test ./...` — FAIL only at `creaves/grifts` template parity, with 108 known/frozen unrelated locale-drift pairs; all remaining packages passed.
- `go vet ./...` — PASS.
- `git diff --check` — PASS.

**Browser reachability in this run:** unauthenticated `http://127.0.0.1:3000/care_plan?kind=feeding` returns HTTP 302; `127.0.0.1:3001` refuses connection. No credentials or safe non-production authenticated fixture were supplied, so did not attempt sign-in or activate any Apply/action control. The available browser/E2E status remains incomplete; prior read-only authenticated-route evidence is historical handover evidence, not a fresh Goal 4 run.

**Remaining blockers:** safe authenticated session or non-production fixture required for live routes/locales; 108 existing unrelated template-drift pairs fail the full suite's structural parity check. Goal 3 DOM baseline has been regenerated and its focused test passes. Keep Goal 4 open; do not claim full integration/E2E verification or archive closure.

## Goal 4 verification record (2026-10-06; read-only browser probes repeated 2026-10-27)

**Automated evidence (not browser E2E; rerun 2026-10-27):**
- `go test ./actions -run 'TestPhase3.*|TestCarePlanPagesAllLocales' -count=1` — PASS (`ok creaves/actions 1.314s`).
- `go test ./grifts -run TestTemplateVariantStructuralParity -count=1` — PASS (`ok creaves/grifts 0.509s`).
- `go test ./...` — PASS across packages (Go output included cached package results).
- `go vet ./...` — PASS.
- `git diff --check` from `creaves/` — PASS.
- Locale structural parity is test evidence only; it does not establish browser-rendered locale text or authenticated route behavior.

**Browser evidence (read-only):**
- Initial unauthenticated `agent-browser` probe of `/care_plan?locale=fr` redirected to `/auth/new`; snapshot showed Login/Register navigation, Login heading and credential fields. A subsequent user-authorized login used exactly `admin` / `admin`; no credential variants were tried.
- Authenticated cleanup route loaded in all four locales using `/lang/?lang=<locale>&url=%2Fcare_plan%3Fkind%3Dcleanup`: English heading `Day plan`, German `Tagesplan`, French `Plan de la journée`, Dutch `Dagplan`. Locale text in navigation also changed (`Übersicht`, `Tableau de bord`, `Overzicht`). This verifies rendered page locale, not every task label.
- Authenticated route opens: `/care_plan?kind=cleanup`, `/care_plan?kind=feeding&group=animal`, `/care_plan?kind=medication`, `/care_plan?kind=observation`, and `/animals/8635`. Snapshot on animal route showed `Animal number 326 (2026)` and hedgehog cage/species heading. Other route queries loaded common Day plan shell; this probe did not establish persisted-record correctness or exhaustive route-specific content assertions.
- No Apply/toggle/action control activated; no production data intentionally changed. `127.0.0.1:3001` had refused connection in the earlier probe.

**Exact limitation / status:** Authenticated read-only routes and localized headings now verified, but integrated persisted-record/behavior assertions across routes and locale-specific page content remain incomplete. Browser access appears production-backed; no safe fixture exists to demonstrate occurrence behavior without mutating data. Do not claim full authenticated E2E or close Goal 4/archive handover until remaining safe assertions and status reconciliation are supported by evidence.

## User review: unresolved requested behaviors (2026-10-27)

User reports the deliverable still fails these concrete acceptance cases; treat as open defects, not validated or fixed:
1. `/care_plan/?kind=feeding&group=cage`: toggle button lacks due-hour text.
2. `/care_plan/?kind=feeding&group=animal`: toggle button presentation is still incorrect (user described as “to toggle button”; exact desired appearance needs confirmation during safe fixture validation).
3. `/care_plan/?kind=medication&group=animal`: animal cell sizing remains incorrect and stale entry remains visible despite expected filtering.
4. `/care_plan/?kind=care&group=animal`: table styling differs from other tables; task text and animal text share color rather than task using expected distinct treatment.
5. `/care_plan/?kind=cleanup&group=cage`: single-animal cage still shows an “O” apply control; expanded multi-animal cage does not put each animal on its own line.

These were not directly asserted by Goal 4 browser sweep: that sweep intentionally avoided activating controls and did not capture exact per-route/group DOM for all five scenarios. Focused aggregate tests passing do not validate these acceptance cases. Goal 2/3 and Goal 4 acceptance gates remain unresolved pending targeted regression tests, localized rendering checks, and DOM verification against safe data fixtures. No claim made that defects have been corrected.


Browser session `agent-browser` at `127.0.0.1:3000` was already authenticated as `admin`; route changes used direct navigation and `/lang/?lang=<locale>&url=<encoded-route>` only. No Apply, action, or filter control was activated; no record mutation attempted.

**Exact observed routes and visible text:**
- `/care_plan?kind=feeding`: `en-US` heading `Feeding (per cage × diet)`; French `Alimentation (par cage × régime)`; German `Fütterung (pro Käfig × Ration)`; Dutch `Voeding (per kooi × dieet)`. Tab labels localized respectively `Feeding`, `Nourrissage`, `Fütterung`, `Voeding`. English in-tab tier links `Late 99+`, `Now 55`, `Later 56`; French `En retard 99+`, `Maintenant 55`, `Plus tard 56`. Existing cached handover evidence observed tier content within Feeding: Late 99+, Now 55, Later 56. Current browser snapshot confirms Feeding heading and in-tab UI; no tiers activated.
- `/care_plan?kind=cleanup`: headings `Cage chores` (en-US), `Soins des cages` (fr), `Käfigpflege` (de), `Kooiverzorging` (nl); tab/nav text also localized.
- `/care_plan?kind=medication`: route retained and rendered authenticated care-plan content. Snapshot had medication entries in English and Dutch (e.g. Dutch `Later 6`, `Geschiedenis 4`), but no kind-specific heading surfaced in filtered snapshot; French and German medication-kind content likewise not positively established. Do not count as localized heading pass.
- `/care_plan?kind=observation`: authenticated plan shell rendered. Dutch showed `Dagplan`, `Observatie`, and `Zone: Alle 0`; other locales showed localized plan shell/tab labels but no observation-specific content heading. Do not count as localized heading pass.
- `/animals/8635`: headings were `Animal number 326 (2026)` / `Cage S38 - West European Hedgehog (Hedgehogs and insectivorous mammals) - adult` (en-US); `Numéroanimal:326 (2026)` / `Cage S38 - Hérisson (Hérissons / Insectivore) - adulte` (fr); `Tiernummer 326 (2026)` / `Käfig S38 - Braunbrustigel (Igel und insektenfressende Säugetiere) - Erwachsen` (de); `Diernummer 326 (2026)` / `Kooi S38 - Egel (Egels en insectivore zoogdieren) - volwassen` (nl). Browser rendered localization, although animal number differs from route id as expected display identity.

**Checks rerun:** `go test ./actions -run 'TestPhase3.*|TestCarePlanPagesAllLocales|TestGoal3Layout|TestCarePlanPhase0bDOMEquivalence' -count=1` PASS (`1.553s`); `go vet ./...` PASS; `git diff --check` PASS. `go test ./...` FAILS at `creaves/grifts` structural locale parity with 108 frozen known-drift pairs; other package outputs passed. No suppression or baseline changes made.

**Disposition:** Goal 4 remains OPEN. Required complete kind-specific rendered-heading assertions across every locale are not evidenced for medication and observation; their pages show sparse/empty localized shells. Broader-suite gate remains blocked by 108 known locale template drifts. This is partial authenticated read-only evidence, not full E2E or closure. B10-9/B10-10 closure/archive status remains unresolved; do not mark complete or archive handover on this evidence.

## Resolution record — 2026-10-27 fix round (supersedes the open status above)

All five reported regressions and the cage-matcher mandate were fixed and validated. `bugs.md` carries the per-item evidence; this section records the round-level outcome.

**Fixture:** clone `creaves_review` served on port 3002 (`GO_ENV=production PORT=3002 DATABASE_URL=…creaves_review… ./bin/creaves-review`), webhook/event-stream disabled, login `admin`/`admin`, Apply activation allowed there only. Production DB `creaves` untouched by the defect fixes.

**Defect fixes (RED→GREEN tests + four-locale templates + agent-browser DOM evidence on :3002):**
1. feeding `group=cage` Apply button carries due-hour text (was clock icon);
2. medication `group=animal`: animal-cell sizing + preference-window stale-entry filtering; sub-item 3b (“hier” relative label) validated on `/animals/10214` Traitement tab — due times render as `○ 12:00` text, superseded treatments show “Repris par le protocole”;
3. care `group=animal` table matches medication row styling with distinct task-text treatment;
4. cleanup `group=cage`: no “O” control on single-animal cages; multi-animal cages put each animal on its own line. Residual: the fixture holds no single-animal cleanup cage, so that path rests on viewmodel/contract tests (stated honestly).

**Parity gate:** all 108 frozen locale-template drift pairs resolved; `grifts.TestTemplateVariantStructuralParity` green with a zero whitelist; full `go test -count=1 ./...` green.

**Cage-matcher elimination (converter v3 + startup_v3 migration):** feeding conversion is per-animal plans only; the migration retires converter-owned feeding rules in favor of verbatim per-animal plans (diet-equality gate), drops over-sweep coverage, unlinks and deletes unreferenced matchers, keeps hand-edited rules. Clone run: 10 rules retired, +102 plans, 165 over-sweep dropped, 10 matchers deleted, 18 seed matchers remain. Production run completed via remediation after a partial first build (details and honest incident report in `bugs.md`): final audited state — 8 rules retired, +84 plans, 80 over-sweep dropped, 0 conversion/cage matchers, 7 hand-edited rules untouched. Regression coverage: `TestConvertDataEmitsNoCageMatchers` (source fence), `TestSeedMatcherExpressionsParse` (no `cage `), `TestCarePlanMigrationV3MigratesRulesToAnimals`, `TestCarePlanMigrationV3CompletionPassWithoutActiveRules`, RoundTrip MySQL test.

**Gates (final):** `go vet ./...` PASS; `go test -count=1 ./...` PASS; `git diff --check` PASS.
