# Care-plan Goal 4 — continuation handover

**Handover date:** 2026-10-27  
**State:** Goal 4 is incomplete. Do not claim acceptance, close/archive Goal 4, or mark user-reported defects fixed.  
**Project:** `creaves/` (Buffalo/Go; see `creaves/AGENTS.md`).  
**Required UI locales:** `en-US`, `fr`, `de`, `nl`.

## User objective and safety boundary

Complete safe Goal 4 verification across `/care_plan` feeding, medication, cleanup, observation and `/animals/8635`, in four locales. Assert rendered localized headings and DOM, including Feeding tier summary placement and route/group-specific behavior. Never activate Apply/action controls or mutate animal-care records. Record broader-suite limitations honestly. Goal criteria require focused tests and diff checks to pass, and plan/handover/archive states to reflect unresolved gates truthfully.

Browser was already authenticated at `http://127.0.0.1:3000`. User identified remaining defects after the attempted Goal 4 checks. Work toward a correct review and eventual fixes; do not treat this handover as a complete solution.

## User-reported acceptance defects — all OPEN

**UPDATE 2026-10-27 (fix round): all five defects FIXED and validated** — reproduce-first on the approved fixture (clone `creaves_review`, port 3002, `admin`/`admin`, Apply allowed on the clone only), RED→GREEN regression tests, four-locale templates, agent-browser DOM evidence. `bugs.md` and the “Resolution record — 2026-10-27 fix round” section in `docs/care-plan-improvement-plan-2026-10-26.md` carry per-item proof. Honest residual: the cleanup single-animal path has no fixture cage, so it is covered by viewmodel/contract tests, not browser evidence. The 108-drift parity gate is green (zero whitelist); full `go test -count=1 ./...` passes. Original open-status list kept below for traceability.

Logged in `bugs.md` and in `docs/care-plan-improvement-plan-2026-10-26.md` under “User review: unresolved requested behaviors”. No direct targeted assertion or fix has been completed for these reports.

1. `/care_plan/?kind=feeding&group=cage` — toggle button lacks due-hour text.
2. `/care_plan/?kind=feeding&group=animal` — toggle button is incorrect; user wording was “to toggle button”, so exact desired form needs clarification. Do not silently infer.
3. `/care_plan/?kind=medication&group=animal` — animal cell is incorrectly sized and stale entry remains visible despite expected filtering.
4. `/care_plan/?kind=care&group=animal` — table appearance does not match other tables; task text appears in same color as animal text rather than distinct expected task styling.
5. `/care_plan/?kind=cleanup&group=cage` — single-animal cage shows an “O” Apply control; expanded multi-animal cage fails to put each animal on its own line.

These defects invalidate any assumption that aggregate tests or prior browser route visits establish Goal 2/3 acceptance. Add regression tests for exact URLs/group modes and appropriate one/many/filter fixtures before changing code. Follow all four localized template variants for any UI change.

## Worktree / preservation warning

Many unrelated care-plan implementation and test files are already modified/untracked in the worktree. Do not discard or reset them. At time of handover, `git status --short` included modified files under `actions/`, `templates/`, docs, `bugs.md`, plus untracked `actions/care_plan_goal3_layout_test.go`, `actions/care_plan_round10_caps_test.go`, archive docs, and `docs/care-plan-improvement-plan-2026-10-26.md`. Inspect `git status --short` and `git diff` before edits.

`bugs.md` was already modified before this handover; its present local version is much shorter than the tracked version. It now contains the five open reports and follow-up instructions, but its diff shows many other pre-existing deletions. Do not restore it from Git or overwrite prior work. Review this file’s actual diff before making further edits.

## Browser evidence gathered — strictly read-only

Executed with `agent-browser`, using direct navigation and `/lang/?lang=<locale>&url=<encoded-path>` locale switch only. Did not click Apply, action, or filter controls. No intentional record changes. The exact URL after navigation omitted trailing slash, e.g. `http://127.0.0.1:3000/care_plan?kind=feeding`.

- `/care_plan?kind=feeding`: rendered heading en-US `Feeding (per cage × diet)`, fr `Alimentation (par cage × régime)`, de `Fütterung (pro Käfig × Ration)`, nl `Voeding (per kooi × dieet)`. In page navigation the tab names changed correspondingly: `Feeding`, `Nourrissage`, `Fütterung`, `Voeding`. en-US in-tab tier links visible: `Late 99+`, `Now 55`, `Later 56`; French: `En retard 99+`, `Maintenant 55`, `Plus tard 56`. Prior handover stated Late/Now/Later contents `99+`, `55`, `56` inside Feeding; current navigation snapshots did not activate tiers. This proves placement of summary links, not correct toggle button due-hour behavior.
- `/care_plan?kind=cleanup`: visible heading en-US `Cage chores`, fr `Soins des cages`, de `Käfigpflege`, nl `Kooiverzorging`. Apply controls appeared in DOM but were not activated. No verified assertion of single/multi-animal rows.
- `/care_plan?kind=medication`: authenticated shell and content rendered, but kind-specific heading was not positively asserted in each locale. Dutch snapshot included `Later 6`, `Geschiedenis 4`. Do not count this as localized heading or stale-entry filtering verification.
- `/care_plan?kind=observation`: authenticated shell rendered, including Dutch `Dagplan`, `Observatie`, `Zone: Alle 0`; no observation-specific content heading was positively established across all locales. Do not count this as pass.
- `/animals/8635`: en-US `Animal number 326 (2026)` and `Cage S38 - West European Hedgehog (Hedgehogs and insectivorous mammals) - adult`; fr `Numéroanimal:326 (2026)` and `Cage S38 - Hérisson (Hérissons / Insectivore) - adulte`; de `Tiernummer 326 (2026)` and `Käfig S38 - Braunbrustigel (Igel und insektenfressende Säugetiere) - Erwachsen`; nl `Diernummer 326 (2026)` and `Kooi S38 - Egel (Egels en insectivore zoogdieren) - volwassen`. Route ID `8635` had display identity animal 326; this was observed, not treated as failure.

Previous handover message noted ports `:3001` unavailable; irrelevant to Creaves :3000 coverage.

## Checks actually run in this continuation

- `go test ./actions -run 'TestPhase3.*|TestCarePlanPagesAllLocales|TestGoal3Layout|TestCarePlanPhase0bDOMEquivalence' -count=1` — PASS (`ok creaves/actions 1.553s`). This is aggregate evidence, not direct proof for five acceptance cases.
- `go vet ./...` — PASS.
- `git diff --check` — PASS after documenting evidence and reports.
- `go test ./...` — FAIL in `creaves/grifts` structural locale parity. Output reports 108 known frozen-drift pairs; remaining packages shown passed. No test suppression or parity baseline changes made.

Earlier plan prose has an inconsistent prior record that says full suite passed; the later, current rerun failed. Use actual fresh rerun as status and clearly distinguish dated historical results.

## Next actions

**UPDATE 2026-10-27: actions 1–6 completed** in the fix round — fixture obtained (:3002 clone), defects reproduced then fixed with focused tests, four-locale parity, DOM revalidation with Apply activation on the clone, gates green (`go vet ./...`, `go test -count=1 ./...` incl. the previously failing 108-drift parity test, `git diff --check`), docs updated. Additionally the user's cage-matcher mandate was implemented (converter v3 + startup_v3 migration, “cage names are per centers / in doubt migrate to the animal / all centers”): clone validated and production remediated to a fully audited clean state — see `bugs.md` for the honest incident report (partial first build ran against production via the dev-server hot-reload; final state verified: 0 conversion/cage matchers, 18 seed matchers, hand-edited rules untouched) and the clone-only 12 duplicate plan pairs cleanup.

Original list kept for traceability:

1. Re-read `creaves/AGENTS.md` and current `git status --short`; preserve pre-existing changes.
2. Obtain a safe non-production authenticated fixture/session with representative feeding cage/animal rows, medication stale/filtered rows, care animal rows, and single-/multi-animal cleanup cages. Never test by applying care in production. Ask user for exact intended feeding `group=animal` toggle style (their report says “to toggle button”).
3. Inspect localized templates, relevant viewmodel/builders, and existing tests. Write focused regression tests for each exact report before editing; demonstrate RED. Scope templates/locales consistently; avoid unrelated cleanup.
4. Rerun focused tests and inspect DOM/text on safe fixture for exact group URLs in all four locales. Do not activate any Apply/action in browser.
5. Rerun `go vet ./...`, focused tests, `git diff --check`, and `go test ./...`. Do not call full-suite failure resolved unless all 108 drift pairs are resolved or project owners explicitly disposition that gate; report narrow exception accurately.
6. Update `bugs.md`, improvement plan, and handover with commands/results; do not archive unresolved defects or mark Goal 4 complete until all acceptance assertions and broad-gate disposition pass.

## Relevant files / documentation

- `creaves/AGENTS.md` — project rules and test environment.
- `creaves/bugs.md` — current open issue record (inspect diff; pre-existing wholesale edits noted above).
- `creaves/docs/care-plan-improvement-plan-2026-10-26.md` — goals, gates, browser evidence, reports, current limitations.
- `creaves/actions/care_plan_viewmodel.go`, preference logic in `creaves/actions/preferences.go` and `creaves/actions/care_plan.go`.
- `creaves/templates/care_plan/index.plush.html` and `.fr/.de/.nl.html`; relevant shared templates include `_plan_tier_feed_table`, `_plan_care_line`, medication row/toggle partials; confirm actual routing and locale fork behavior before changes.
- Focused existing tests: `actions/care_plan_goal3_layout_test.go`, `actions/care_plan_phase3_cleanup_parity_test.go`, `actions/care_plan_phase0b_dom_test.go`, `actions/care_plan_round10_caps_test.go`, `actions/care_plan_round7_todo_glyph_test.go`.

**Truthful status:** partial browser evidence exists; medication/observation locale assertions and all five defect validations remain incomplete. Goal 4 is OPEN, not accepted.
