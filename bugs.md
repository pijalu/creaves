# Bugs — open list

Running bug list for the current round. Add every new report here, fix it with
a fix plan, validate, then move the resolved entries to `docs/archive/` when the
round is closed.

**Guideline** (same convention as the archived rounds):
1. Create a detailed fix plan for each bug — the plan must contain test approach and validation steps — execute the plan and validate the fix when all elements are in place.
2. Any issues found must be fixed and the fix plan updated accordingly.
3. Issues found during testing must be fixed and the fix plan updated accordingly.
4. Each bug is moved to `docs/archive/` when tested and closed with its plan.
5. **All changes must be tested, including e2e testing using the [agent-browser skill](../../.agents/skills/agent-browser/SKILL.md)** — no fix is complete without e2e evidence (commands, URL, captured output).
6. Use interactive shell/filmstrip to validate the output of the tool — verify the actual terminal output, not the plan.
7. Check code quality with each tool run separately (do not chain them with `;` or `&&`):
   - `go vet ./...`
   - `staticcheck ./...`
   - `gocognit -over 15 .`
   - `gocyclo -over 12 .`
   - `go test -count=1 -race -cover ./...`
8. Commit each fix with a clear and descriptive commit message.

**Session constraints:**
- **creaves contains real production data**: never drop/destroy data; schema changes must be additive/backward compatible (no destructive migrations).
- **Every UI change lands in all four locales** (`en-US`, `fr`, `de`, `nl`) — the `templates/**/*.plush.{html,fr,de,nl}.html` forks stay in sync.

---

## Open items

Round 9 — caregiver UX + data quality round: care-plan feeding/medication
feedback states, dashboard medication presentation, cleanup-rule activation,
default-rule naming, and reference-data localization. Reported against the
local dev instance (admin session) on 2026-10-05.

### R9-1 Feeding apply gives no feedback — row must go light-green with an undo

**Severity**: medium (usability: the caregiver gets no confirmation a feeding
was recorded). **Found**: 2026-10-05, on `/care_plan`.

**Defect**: applying a feeding (per-animal `.plan-feeding-one` or group
`.plan-feeding-apply`) records the care but the row keeps its original look —
there is no applied state. The medication slots turn green (`✓ HH:MM`
btn-success) and re-click to undo (R5-2c); the feeding row does not, and its
undo control (`.plan-unapply-btn`, `index.plush.html:416`) stays `d-none`.

**Expected** (all four `index.plush.*` forks):
- After a successful apply, the **applied** feeding row (or the applied
  animal chip in a multi-animal group) switches to a **light-green**
  background/state so it is easy to **see what was applied** — mirroring the
  medication "applied" visual language.
- The **apply button becomes an undo button** in place (same control,
  toggled): clicking it reverses the record (same `_apply_toggle` undo path
  as the history undo at `index.plush.html:573`) and restores the open state
  — no page reload. This matches the existing per-slot undo toggle: the
  hidden `.plan-unapply-btn` (`index.plush.html:416`) is the swap target.
- Group rows: only the animals actually applied turn green; the row stays
  actionable for the remaining open animals.

**Test approach**: e2e via agent-browser on `/care_plan?kind=feeding` — apply
a single-animal row and a multi-animal group, assert the light-green class +
visible undo, click undo, assert the open state returns and the DB
`cares`/application row is removed. Repeat across en/fr/de/nl.

**Status**: **fixed 2026-10-05**.

**Fix**: the applied state is toggled in the shared `planApply` helpers so every
apply path (single `.plan-feeding-one`, group batch `flipBatchRow`, undo)
gets it for free:
- `templates/care_plan/_apply_toggle.plush.html` (+ byte-identical fr/de/nl
  forks, propagated with `cp`): `markApplied` now adds `plan-animal-applied`
  to the closest `.plan-animal-row`; `markOpen` removes it. The existing
  apply↔undo button swap (`pairApply`/`pairUndo`) was already wired — the
  apply button becomes the undo button in place, no reload.
- `assets/css/care-plan.scss`: new rule
  `.plan-animal-row.plan-animal-applied { background: #d4edda; … }` (same
  `#d4edda`/`#155724` family as the existing applied med slot — one visual
  language for "applied"). No new user-facing text ⇒ no new `t()` keys.

**Validation (e2e, agent-browser, `/care_plan?kind=feeding`, en UI)**:
- Single animal 8429: apply → apply-check hidden, undo shown,
  `plan-animal-applied` present, row background `rgb(212, 237, 218)`
  (#d4edda); `care_plan_applications` row inserted (`source_type=rule`).
  Undo → apply shown, undo hidden, green removed (background transparent);
  DB row deleted.
- Group row (animals 8429/8430/8432/8433/8435/8513): "Apply cage" → confirm
  → all 6 `.plan-animal-row` got `plan-animal-applied` (6/6 green), group
  apply button hidden. The 6 recorded applications were left in the dev DB
  (valid test data from the restored Oct-2 dump, today's date).
- Webpack rebuilt; new CSS confirmed in
  `public/assets/application.72c03ba1d68a4fa50d8e.css`. Locale forks of
  `_apply_toggle` verified byte-identical (`md5 f6c159c…` ×4); CSS is shared.

**Quality gates**: `go vet ./...` exit 0; `staticcheck ./...` exit 0;
`gofmt -l .` flags only pre-existing test files (Go-version doc drift), none
touched by this fix. gocognit/gocyclo/go-test: **N/A — zero Go lines changed**
(scss + 4 template forks only); the DB-backed suite remains contaminated by
the restored production dump (pre-existing
`TestCarePlanConverterMarkerV2Refresh` failure, documented under R9-5).

### R9-2 Medication page needs a "Done" collapsible + per-slot green undo; "done" only when ALL repeats applied

**Severity**: high (core workflow: a partially-completed repeat treatment
currently has no correct home). **Found**: 2026-10-05, on
`/care_plan?kind=medication`.

**Current behaviour** (`actions/care_plan_viewmodel.go` `fillMedTiers`,
l.559–601): a medication series is placed in exactly one of the three open
tiers (late/now/later) by its **most urgent OPEN slot**
(`seriesTierOrMinus`). When every slot is terminal the series is dropped
entirely (`continue` at l.566/576) — it never lands in a "done" section. The
bottom **Historique** block (`index.plush.html:512–593`) is a separate,
non-tier, outline-button collapsible that mixes terminal + superseded rows
for every kind, not a medication "done" tier.

**Defect / required changes**:

1. **New "Done" collapsible** on `/care_plan?kind=medication`, styled and
   behaving like the **"Plus tard / Later"** tier
   (`index.plush.html:248–291`, `.plan-tier-later`, `#d4edda` background) but
   with a **darker green** and labelled **"Done"** (new
   `care_plan.tier.done` key in all four locales). Collapsed-by-default is
   acceptable; it must follow the same header/collapse markup.

2. **Done line look**: each fully-done animal × drug-series entry renders
   with the **same `_med_series` line layout** as the open tiers, with the
   **time toggle button(s) in green** (`✓ HH:MM` btn-success, the R4-1.1
   applied state).

3. **Toggle undo**: each green time button stays a live toggle — clicking it
   undoes that treatment occurrence (existing `plan-med-unapply` path) and
   the entry must return to its correct open tier without a reload.

4. **Popup treatment link**: the medication detail popup
   (`_item_detail_modal` / the `plan-detail-btn` data) must include a **link
   to the treatment** (the source/animal treatment page). The same link must
   also be visible in the **actual treatment popup on the animal page**
   (`care_plan_animal_page` treatment tab detail modal).

5. **KEY BUSINESS RULE — a treatment is "Done" ONLY when every repeated
   occurrence is applied.** Until then the entry stays in
   late/now/to-do(later), positioned by its **next open** time, and the
   animal can move **between** late/now/to-do during the day as occurrences
   are applied.

   **The "green" applies to the toggle buttons of the APPLIED slots**: on a
   series with multiple repeated treatments, the applied occurrences keep
   their **green `✓ HH:MM` toggle visible** next to the still-open ones, so
   the caregiver sees which doses are done at a glance. (This does NOT
   recolor the open slots — they keep their R4-1.1 urgency colours: late red
   / now yellow / later white. Green stays the applied-only signal.)

   Worked example (animal 1234, treatment at 08:00 / 12:00 / 18:00):
   - 11:00 — 08:00 is late → entry in **late**; apply 08:00 → 08:00 toggle
     turns green (✓ 08:00), entry **moves to "now"** (12:00 next), 12:00 +
     18:00 still to do.
   - 12:00 — apply 12:00 → 12:00 toggle green, entry **moves to "to-do
     (later)"** (18:00 next).
   - 17:30 — apply 18:00 → all three toggles green → entry **moves to
     "Done"**.

   This changes `fillMedTiers`: a series with a mix of applied + open slots
   must keep the applied slots visible (green toggles) alongside the open
   ones, and the tier is decided by the next open slot — only a 100%-applied
   series goes to the new Done section. `scopeSeriesToToday`/`seriesTierOrMinus`
   must not hide the applied-but-same-day slots.

**Test approach**: e2e via agent-browser on `/care_plan?kind=medication`
replaying the 1234 08:00/12:00/18:00 timeline (assert tier placement +
button colours at each step, and the final Done placement); assert the Done
collapsible look/label, the per-slot undo, and the treatment link in both
popups. Unit-test `fillMedTiers` for the partial-application tier rule. Sweep
en/fr/de/nl.

**Status**: **fixed 2026-10-05**.

**Fix** (branch `feature/care-expert`):

- **Done tier routing** — `actions/care_plan_viewmodel.go`:
  - New `DayPlanView` fields `MedDone []MedTierLine`, `MedDoneCount`,
    `MedDoneCountCap` (after `MedTiers`).
  - `fillMedTiers`: a fully-terminal series (`seriesTierOrMinus < 0`) goes to
    the new `collectDoneSeries` helper; a series that is NOT fully terminal
    overall but whose TODAY-scoped slots are all terminal ("done for today",
    future slots still open) is ALSO appended to `MedDone`. Tail sorts
    `MedDone` by `medDoneFirstDue` (earliest slot) and sets the count/cap.
  - `collectDoneSeries`: scopes the series to today; returns (series leaves
    the screen) only when nothing of it is due today; otherwise appends a
    single-series `MedGroupView` to `v.MedDone`.
  - `scopeSeriesToToday` `!ok` branch (no open slot ≤ endOfDay) now KEEPS
    today's terminal/done record instead of always emptying — the applied
    slot stays visible in Done rather than vanishing in the evening.
- **Done tier markup/CSS** — `templates/care_plan/index.plush.html` (+ fr/de/nl):
  `.plan-tier-done` (darker green `#b7dfc2`/`#9fd0ae`, header `#0f4a1f`) after
  `.plan-tier-later`; `#tier-done` collapsible (collapsed default, badge
  `view.MedDoneCountCap`), body loops `view.MedDone` reusing `_med_series`.
  Guarded by `view.Kind == "medication" && len(view.MedDone) > 0` (no empty tier).
- **Popup treatment link** — `_item_detail_modal.plush.*.html` ×4 gained a
  Treatment row (`#planDetailTreatmentDt/Dd/Link`); `_med_series.plush.*.html`
  ×4 `plan-detail-btn` carries `data-treatment-link` = `slot.ViewLink`
  (treatment record when applied, else the animal Treatment tab);
  `index.plush.*.html` `fillPlanDetail` and `animals/show.plush.*.html` both
  populate the row — the SAME link in the care-plan popup and the animal-page
  popup (shared modal). New `care_plan.detail.treatment` key in all 4 locales
  (Treatment / Traitement / Behandlung / Behandeling).
- `care_plan.tier.done` keys already existed (Done / Terminé / Erledigt / Klaar).

**Tests** (`actions/care_plan_round9_done_tier_test.go`, all PASS):
`TestFillMedTiersFullyAppliedSeriesGoesToDone`, `...PartialSeriesStaysOpenWithAppliedVisible`,
`...UndoneSeriesLeavesDone`, `...DoneForTodayWithFutureOpenSlots`,
`...DoneCountTracksSeries`. Updated `care_plan_round7_med_scope_test.go`
(`EmptyInEvening`, `DropsScopedOutSeries`) and
`care_plan_r415_animal_column_test.go` (count 3→4) for the new semantics.

**e2e (agent-browser, `/care_plan?kind=medication`, direct-run binary)**:
- Applied the Baycox 12:00 slot for animal 10214 → after refresh the series
  landed in **Done** (`data-done-count=1`, header "Done"), toggle green
  `✓ 12:00` `btn-success plan-med-unapply`, label "Baycox 5 % (PER OS) — 0.10 ml".
- **Undo**: clicked the green toggle → `care_plan_applications` row deleted
  (1→0), button flipped back to `○ 12:00` `btn-warning plan-med-apply`;
  re-tiered out of Done on reload.
- **Popup treatment link** (care-plan popup): `data-treatment-link` =
  `/treatments/305a8dac…?back=…`; Treatment dt/dd visible, href set, modal open.
- **Animal-page popup** (`/animals/10214`): modal opens, Treatment row shown,
  label "Citramox L.A. (48H) — 0.03 ml IM", fallback href `/animals/10214?back=#nav-treatment`.
- **Empty Done tier**: with nothing done, `#tier-done` is absent (no empty header).
- **Locale sweep** (all on the Done tier + popup): EN Done/Treatment ·
  FR Terminé/Traitement · DE Erledigt/Behandlung · NL Klaar/Behandeling.
  (`data-done-count=1` in every locale.)

**Quality gates** (run separately, on the R9-2 tree):
- `go vet ./...` — exit 0.
- `staticcheck ./...` — exit 0.
- `gocognit -over 15 .` — `fillMedTiers`=14, `collectDoneSeries`=1,
  `medDoneFirstDue`=4, `scopeSeriesToToday`=7; the only >15 in the file is the
  pre-existing `buildMedGroups`=31 (unchanged). Clean for R9-2 code.
- `gocyclo -over 12 .` — all R9-2 functions ≤ limit; `buildMedGroups`=19
  pre-existing/unchanged.
- `go test -count=1 -race -cover ./actions/` — all R9-2 / med-scope / animal-cell /
  seed tests PASS (60.4% coverage). Two failures are **pre-existing on a clean
  tree** against the restored prod dump, NOT caused by R9-2:
  `TestCarePlanConverterRoundTrip`, `TestCarePlanConverterMarkerV2Refresh`.

**Data note**: the e2e apply/undo round-trips on animal 10214 were undone via
the UI, leaving no orphan rows. The original debug fulfillment treatment
(`305a8dac…`) created during R9-2 diagnosis was removed by its own undo path
(undo deletes the application + its fulfillment record) — DB left consistent.

### R9-3 Dashboard medication lines show stray slot labels and don't stack on narrow screens

**Severity**: medium (readability + responsive layout on the landing page).
**Found**: 2026-10-05, on `/dashboard/`.

**Defect A — stray labels**: an animal with several treatments on one line
renders the slot **bucket dividers** between the buttons, e.g.
`<12:00>----soir----<18:00>`. These come from `_med_series.plush.html:67–69`
(`row.DividerBefore` → `.plan-med-bucket` with `t("care_plan.slot.*")`).
On a single dashboard line a bucket label is noise — the buttons should flow
without it.

**Defect B — no stacking**: when the viewport is too narrow to fit a full
medication line, the buttons do not re-flow under the medication label. The
`.plan-med-line` flex rules (A/B/C shapes, `assets/css/care-plan.scss`) were
tuned for the care-plan page; the dashboard cell does not wrap gracefully.

**Expected**:
- On `/dashboard/`, a single-line medication entry renders its buttons with
  **no bucket label** between them.
- If the line cannot fit, the buttons **stack under the medication** label
  (shape C) instead of overflowing; the layout should "stack" items to fit.
- The dashboard medication table should **inherit the care-plan
  `_med_series` / `.plan-med-line` styling** (it already reuses the partial
  at `dashboard.plush.html:124`) so the presentation matches the care-plan
  page's more natural sizing — verify the SCSS actually applies to the
  dashboard container and extend it if scoped too narrowly.

**Test approach**: e2e via agent-browser on `/dashboard/` with a
multi-treatment animal (assert no `soir`/`midi`/etc. label between buttons on
one line) and at several viewport widths (1600/1024/767 px) asserting the
buttons wrap under the label with no horizontal overflow. Sweep en/fr/de/nl.

**Status**: **fixed 2026-10-05**.

**Root cause**: the dashboard medication cell reused the shared `_med_series`
partial with no "compact" signal, so the R4-2.4 labelled bucket divider
(`.plan-med-bucket`) rendered between buttons (Defect A). And the global
compact single-line rules (`assets/css/care-plan.scss:330–355` —
`.plan-med-row .plan-med-line { flex-wrap: nowrap }`, `.plan-med-btns {
flex-wrap: nowrap }`, `.plan-med-label { white-space: nowrap; ellipsis }`)
apply to `.plan-med-row` on BOTH the care plan and the dashboard, so the
dashboard `<td>` could not re-flow (Defect B).

**Fix** (branch `feature/care-expert`):

- **Defect A (no bucket label on the dashboard)** — `_med_series.plush.html`
  (+fr/de/nl): the divider is gated on `row.DividerBefore && !medSeriesCompact`.
  The dashboard templates (4 forks) declare `<% let medSeriesCompact = true %>`
  next to the existing `medSeriesEye`; the care plan never sets it, so its
  dividers stay. The buttons still flow — only the caption is dropped.
- **Defect B (stacking)** — the dashboard cell wrapper became
  `<div class="… dash-med-cell">` (4 forks), and `assets/css/care-plan.scss`
  gained a `.dash-med-cell`-scoped override block (higher specificity than the
  rules it overrides, so the care plan is untouched): `.plan-med-line` wraps,
  `.plan-med-label` grows/wraps with no ellipsis clip (R4-7.24), and
  `.plan-med-btns` takes `flex: 1 1 100%` + `flex-wrap: wrap` so the buttons
  drop to their own line UNDER the label (shape C) instead of overflowing.
- Webpack bundle rebuilt (`npm run build` → `application.ce7c78f9…css`).

**Tests** (`actions/care_plan_series_test.go`, PASS): new
`TestMedSeriesCompactSuppressesBucketLabel` renders the SAME multi-bucket
series through all 4 `_med_series` forks — with `medSeriesCompact` (dashboard)
it asserts `plan-med-bucket` is ABSENT and both `○ 08:00`/`○ 18:00` buttons are
kept; without it (care plan) it asserts the divider STAYS. Existing
`TestMedSeriesPartialRenders` (care-plan divider present) still passes.

**e2e (agent-browser, `/dashboard/`, direct-run binary)**:
- Computed styles in `.dash-med-cell`: `plan-med-line flex-wrap=wrap`,
  `plan-med-btns flex-wrap=wrap flex-basis=100%`, `plan-med-label
  white-space=normal overflow=visible`; `nBuckets=0` inside dash cells.
- Viewport sweep: **1600px** `hOverflow=0 btnOverflowPx=0`; **1024px** and
  **767px** `hOverflow=0 btnOverflowPx=0` AND `anyBtnStackedUnderLabel=true`
  (buttons drop under the label — shape C, no horizontal scroll).
- Care-plan regression: `/care_plan?kind=medication` shows no `dash-med-cell`
  (override is dashboard-scoped); bucket logic unchanged there.
- Locale sweep en/fr/de/nl: med table renders 5 rows, `dashBuckets=0` in each.

**Quality gates** (run separately, on the R9-3 tree): `go vet ./...` exit 0;
`staticcheck ./...` exit 0; `gocognit -over 15` — new test fn =1; `gocyclo
-over 12` — new test fn =2; `go test -count=1 -race -cover ./actions/` — all
PASS (60.4%) except the two PRE-EXISTING converter failures
(`TestCarePlanConverterRoundTrip`, `TestCarePlanConverterMarkerV2Refresh`,
unchanged by R9-3). `TestCarePlanPagesAllLocales` / `TestLocaleKeyParity` PASS.

### R9-4 Cleanup care-rules don't trigger / don't appear in the care plan

**Severity**: high (a seeded, active rule produces no visible work).
**Found**: 2026-10-05, on
`/care_rules/0673d743-753a-4bc9-a913-f3c0560fc409/edit/` (a cleanup rule) —
the rule exists and is active, but no cleanup occurrence shows up in the
care plan.

**Context** (`actions/care_plan_seeds.go`): the cleanup enforcement is rule
**SR13 "Nettoyage des cages occupées"** (`Kind: careplan.KindCleanup`,
matcher **SM14** `zone_requires_cleanup = true`, ships **active**,
l.131–135, 153–155). Cleanup occurrences should be generated for occupied
cages of zones flagged `zones.requires_cleanup`.

**Investigation needed** — reproduce and localize before fixing. Candidate
causes to check, in order:
- Whether the SR13/SM14 seed actually ran and the rule row is `Active` in the
  DB (the edit page shows one rule; confirm it is SR13 and enabled).
- Whether the SM14 matcher (`zone_requires_cleanup = true`) matches any
  animal — i.e. whether any zone has `requires_cleanup` set and the matcher
  field is populated by the assembly.
- Whether the care-plan assembly pipeline
  (`actions/care_plan_service.go` / `care_plan_dayplan.go`) **generates and
  renders `KindCleanup` items** at all — a kind filter or a `kindMatch` /
  section guard may silently drop cleanup occurrences even when the rule
  fires.
- Whether cleanup has its own `kind=` view / section, or is expected on the
  default `/care_plan` page.

**Resolution — root cause: at the cutover almost no zone was flagged
`requires_cleanup` (11 of 12 unflagged in the 2026-10-02 baseline), so SM14
(`zone_requires_cleanup = true`) matched zero animals and the active SR13
produced zero occurrences.** The wiring itself is correct end-to-end:
`fillZoneRequiresCleanup` (`care_plan_service.go:278`) flags matcher contexts
from `zones.requires_cleanup`; the viewmodel renders `KindCleanup` under the
**Cleanup** kind (`care_plan_viewmodel.go:337,1231,1294`) on
`/care_plan?kind=cleanup`.

**Fix** (per direction: flag every zone during migration): data migration
`migrations/20261005090080_zones_requires_cleanup_default_true.up.fizz` —
`UPDATE zones SET requires_cleanup = 1 WHERE requires_cleanup = 0`. Every
existing zone joins the daily cleanup plan; an admin can still unflag a zone
via `/zones` (`RequiresCleanup` checkbox, `templates/zones/_form.plush.*.html`).

**Validation** — restored the 2026-10-02 production dump as base, ran
`buffalo pop migrate up` (25 migrations incl. the new one), booted the app,
logged in as admin, e2e via agent-browser on 2026-10-05:
- DB: all 12 zones `requires_cleanup=1` (baseline had 11 at 0).
- `/care_plan?kind=cleanup` → **"Cleanup 99+"** badge, per-cage
  "Apply cage (N)" buttons (228 apply controls), rule name "Nettoyage des
  cages occupées" rendered.
- Manual apply/undo of a cage cleanup records and reverses without error.

**Status**: fixed 2026-10-05 (migration, validated on the Oct-2 dump).

### R9-5 Default (seeded) rule descriptions carry a converter tag instead of "Règles par défaut"

**Severity**: low (cosmetic / data quality on reference rows).
**Found**: 2026-10-05, on the care-rules list / edit screens.

**Defect** (`actions/care_plan_seeds.go:144` and `:158`): seeded matchers and
rules are created with
`Description: fmt.Sprintf("Bibliothèque §7.4 %s [source: %s]", def.Key, ConverterTag)`
where `ConverterTag = "care_plan_converter"` (l.28). The user-facing
description therefore reads e.g.
`Bibliothèque §7.4 SR13 [source: care_plan_converter]`.

**Fix** (2026-10-05): two parts.
1. **Seed definition** — `actions/care_plan_seeds.go`: added
   `DefaultRuleDescription = "Règles par défaut"` (next to `ConverterTag`)
   and made `buildSeedMatcher` / `buildSeedRule` stamp it instead of
   `fmt.Sprintf("Bibliothèque §7.4 %s [source: %s]", …)`. Provenance stays
   recoverable via `created_by = ConverterTag`; the description is display
   text, not metadata.
2. **Backfill for existing rows** — migrations
   `20261005090090_backfill_default_rule_description_care_rules.up.fizz` and
   `20261005090100_backfill_default_rule_description_care_matchers.up.fizz`
   update rows whose description still matches
   `'%[source: care_plan_converter]'` (a hand-edited description never
   matches, so user edits win). Each is wrapped in a guarded stored
   procedure so it is a no-op when the table doesn't exist yet (baseline
   dumps predate `care_rules`).

**Regression pin**: `TestSeedBuildersStampDefaultDescription`
(`actions/care_plan_converter_test.go`) — every §7.4 seed rule/matcher
description equals `DefaultRuleDescription` and contains no converter tag.

**Validation** — restored the 2026-10-02 production dump, `buffalo pop
migrate up`, booted, e2e via agent-browser on 2026-10-05:
- `/care_rules` lists §7.4 defaults (Pesée hebdo juvéniles, Blessés —
  contrôle quotidien, …) with description **"Règles par défaut"**.
- DB: 0 §7.4 rows carry `[source: care_plan_converter]`; cluster conversion
  rules keep their own "Cluster alimentation ×N [source: …]" text (a
  separate, out-of-scope provenance string, not a §7.4 default).
- `go vet`/`staticcheck` clean; gocognit/gocyclo show no new flag on the
  seed builders.

**Note**: the cluster conversion rules (`Alimentation — … (conversion)`,
description "Cluster alimentation ×N [source: care_plan_converter]") are a
different, intentional provenance label for converted feeding clusters, not
the §7.4 default set. Left as-is; flag separately if those should also be
relabeled.

**Status**: fixed 2026-10-05.

### R9-6 Care rules / matchers must support localization (name in every language, incl. dropdowns)

**Severity**: high (breaks the project's all-language UI rule for a whole
reference-data area). **Found**: 2026-10-05, on the care-rules / care-matchers
view + edit screens.

**Defect**: care rules and care matchers expose only a single `Name`
(`models/care_rule.go:39`, `models/care_matcher.go`) with no localized
variants. The edit screen renders a single free-text
`<input name="Name" value="<%= rule.Name %>">` and the matcher dropdown
renders the raw `m.Name`
(`templates/care_rules/edit.plush.html:24–40`) — there is no translation
lookup and no per-language name field. Other system reference records
(zones, animal types, species, native statuses, …) resolve their display
name through the `tname`/`tbase` helper backed by the translations table
(`actions/tname.go`).

**Expected** (follow the existing system-record localization pattern):
- Care rules and care matchers must carry a **name in every supported
  language** (`en-US`, `fr`, `de`, `nl`), surfaced through the same
  translations-table / `tname` mechanism as other system records (add the
  tables to the translatable set; species/zones already use a custom field
  via `tnameDefaultField`).
- The **edit and view screens must expose the localized names** the same way
  other system records do (per-language name display/inputs), in all four
  template forks.
- **All seeded entries must ship localized names** (the §7.4 seed rules
  SR1–SR13 and matchers SM1–SM14 need `fr`/`en-US`/`de`/`nl` names in the
  seed/translation artifacts).
- **Localization applies to dropdowns too**: the **Care Rules matcher
  dropdown** (`edit.plush.html:37–40`) must render the **localized** matcher
  name via `tname`, not the raw base `Name` — same for any other
  rule/matcher select.

**Test approach**: unit-test the `tname` resolution for `care_rules` and
`care_matchers` across the four locales; e2e via agent-browser asserting the
edit/view screens and the matcher dropdown show the localized name in each
language; assert seeded rules carry localized names. Confirm all four
template forks render the per-language name inputs.

**Status**: fixed 2026-10-05.

**Fix** (existing `tname`/translations-table pattern, no schema change; the
canonical French name stays the base column):
- `actions/tname.go` — `translationBaseFields` and the
  `loadBaseTranslationMap` whitelist gained `care_rules:name` and
  `care_matchers:name`, so the per-request bulk preloader resolves both
  tables by record id.
- `actions/translations_helper.go` — form-whitelist gained
  `care_rules:{name,description}` and `care_matchers:{name}` so the shared
  `translations/_fields` partial + `setTranslationValues`/`saveTranslations`
  wiring applies (rule descriptions are translatable too; matcher names
  only).
- `actions/care_rules.go` / `actions/care_matchers.go` — New/Edit load
  existing translations, Create/Update persist them.
- Templates (all 4 forks each): `care_rules/index` renders
  `richPlanName(tname("care_rules", rule.ID, rule.Name), …)`;
  `care_matchers/index` renders `displayPlanName(tname(…))`;
  `care_rules/edit`+`new` render the matcher dropdown via
  `tname("care_matchers", m.ID, m.Name)` and embed the translations partial;
  `care_matchers/edit`+`new` embed the translations partial.
- `actions/care_plan_seeds.go` — `SeedMatcherNameTranslations()` /
  `SeedRuleNameTranslations()` map every seed key (SM1–SM14 + 4 derived,
  SR1–SR13) to its en-US/de/nl display names.
- `actions/care_plan_converter.go` — `convertSeedLibrary` refactored into
  `seedOneMatcher`/`seedOneRule` helpers; on insert each calls
  `saveSeedNameTranslations` (insert-only via `models.SaveTranslation`), so
  fresh seeds ship their localized names while existing rows and admin edits
  are never clobbered. Side effect: `convertSeedLibrary` complexity dropped
  from gocognit 24 / gocyclo 14 to under both gates.
- Dev DB backfill: 93 idempotent `INSERT … ON DUPLICATE KEY UPDATE value=value`
  statements (18 matchers + 13 rules × 3 locales, matched by canonical French
  name) applied to the dev database; re-apply is a no-op.

**Tests** (`actions/care_plan_round9_localize_test.go`):
`TestSeedNameTranslationsComplete` (every seed key carries en-US/de/nl
names), `TestSeedNameTranslationsNoStrayKeys` (no orphan translation keys),
`TestTnameCareRuleMatcherLocalization` (MySQL-backed: fr falls back to base;
en/de/nl resolve for both tables), `TestSeedLibraryEmitsNameTranslations`
(drives the real `convertSeedLibrary` on a purged seed library: 18 matchers +
13 rules each carry 3 locale names, tname spot-checks, second run is a strict
no-op; cleanup re-purges so the shared test DB keeps the empty seed library
`TestCarePlanConverterRoundTrip` requires).

**e2e** (agent-browser, admin session, rebuilt binary): all four locales
verified — care_rules index de shows "Reinigung belegter Käfige" /
"Baby-Igel — Zwangsfütterung" (conversion-era rows stay French by design);
care_matchers index de "Baby-Igel"/"Zu reinigende Zone", en-US "Baby
hedgehog"/"Zone to clean", nl "Babyegel"/"Te reinigen zone"; SR13 rule edit
in de: base name input keeps the canonical French "Nettoyage des cages
occupées", the matcher dropdown is localized ("Baby-Igel", "Zu reinigende
Zone", "Kein Matcher (alle Tiere)"), six pre-filled `tr_*` inputs
(`tr_en_US_name`=Cleaning of occupied cages, `tr_de_name`=Reinigung belegter
Käfige, `tr_nl_name`=Reiniging bezette kooien, plus empty descriptions);
SM14 matcher edit in de shows base "Zone à nettoyer" + 3 `tr_*` name inputs.

**Quality gates** (run separately): `go vet ./...` exit 0; `staticcheck
./...` exit 0; `gocognit -over 15 .` / `gocyclo -over 12 .` — no new flags,
`convertSeedLibrary` no longer flagged (pre-existing flags in `tname.go`,
`translations_helper.go`, `care_rules.go` unchanged from HEAD);
`go test -count=1 -race -cover ./...` — all packages PASS (actions 60.7%)
except the PRE-EXISTING `TestCarePlanConverterMarkerV2Refresh`, reproduced
identically on HEAD (stash-verified on a pristine `creaves_test`: fails at
`care_plan_converter_mysql_test.go:353` with and without the R9-6 tree).

**Environment note** (creaves_test rebuild, 2026-10-05): recreating
`creaves_test` from scratch hits two pre-existing migration-chain issues,
worked around locally (NOT code-fixed, out of scope): (1)
`20261008100000_create_attachment_blobs.up.sql` fails with Error 3780 —
fizz creates `attachments.id` as `char(36) utf8mb4_general_ci` while the raw
SQL FK targets the table default `utf8mb4_0900_ai_ci`; workaround: create the
table with matching collation by hand + `INSERT INTO schema_migration`. (2)
fizz-created tables land in `utf8mb4_general_ci` while raw-SQL migrations use
`utf8mb4_0900_ai_ci`, breaking collation-sensitive joins (event_streams DLQ
filters) with Error 1267; workaround: `ALTER TABLE … CONVERT TO CHARACTER SET
utf8mb4 COLLATE utf8mb4_0900_ai_ci` for all diverging tables with
`FOREIGN_KEY_CHECKS=0`.

---

Round 8 — revalidation sweep of the Round-7 commits (care-plan-round / fixes),
all findings reproduced live via agent-browser against the local dev instance
(admin session, en-US) on 2026-10-03.

### R8-1 `_apply_toggle` script block is a JS syntax error — every shared apply control dead

**Severity**: critical. **Found**: 2026-10-03, agent-browser on `/care_plan?kind=feeding`.

**Defect**: `templates/care_plan/_apply_toggle.plush.html` line 225
(= all four forks) wraps the `jsString(...)` call in literal quotes:

```js
wBox.textContent = "<%= jsString(t("care_plan.apply.last_weight")) %>: " + ...
```

`jsString` emits a full JS string literal INCLUDING quotes (json.Marshal of
the value), so the rendered script contains `"“Last recorded weight”: " + ...`
— a syntax error (`Unexpected identifier 'Last'`) that kills the ENTIRE
script block. Measured in the browser: `new Function(scriptText)` → parse
error; `window.planApply` === undefined.

**Blast radius** (all measured, en-US):
- `planApply` never defined → the care_plan index script (auto-refresh,
  batch apply, detail popup, late-record, history undo) crashes at
  `var csrf = planApply.csrf` — the whole page-level script dies.
- Group "Apply group (N)" buttons (`.plan-feeding-apply`, `.plan-cage-apply`)
  do nothing when clicked (no batch modal, no request) — verified on the
  VE28 row, 3 pending animals, feeding section.
- Skip/defer buttons, detail popup, batch modal, late-record buttons all
  dead on `/care_plan` for every kind.
- The medication slot toggle (a DIFFERENT script, `_plan_med_toggle`) still
  works — which is why single-slot apply/unapply tested fine.

**Fix**: drop the outer quotes in all four forks:
`wBox.textContent = <%= jsString(t("care_plan.apply.last_weight")) %> + ": " + ...`.

**Validation**: `new Function(scriptText)` parses; `window.planApply` is an
object; clicking a group apply opens the batch modal; apply records in DB.
Locale sweep after fix: fr/de/nl/en all render
`wBox.textContent = "<localized label>" + ": " + ...` and `planApply` is an
object on every locale.

**Status**: fixed 2026-10-03 (templates ×4 + regression pin in
`TestNoTemplateInterpolatesATranslationIntoAScript` rejecting quote-wrapped
`jsString` calls).

### R8-2 care-plan auto-refresh never fires; R4-4.1 30 s floor inverted

**Severity**: high (data staleness: day plan silently goes stale for users
who never click anything; the R4-4.1 protection is also defeated for users
who do).
**Found**: 2026-10-03, agent-browser on `/care_plan` — injected
`window.__probe` marker survived 95 s+ with no reload (period is 60 s),
no console errors, no modal open, `planApply.busy()` false.

**Defect**: `templates/care_plan/index.plush.html` line 626:

```js
if (Date.now() - window.planActionAge() < 30000) { return; } // R4-4.1 floor
```

`planActionAge()` already returns `Date.now() - lastActionAt`. The extra
`Date.now() -` inverts the guard in BOTH directions:

- Fresh page (`lastActionAt = 0`): `planActionAge()` = `Date.now()` →
  `Date.now() - planActionAge()` = 0 < 30000 → **defers on every 5 s tick
  forever** → the §10-CP6c auto-refresh NEVER fires until the user performs
  an action. Measured: marker survives 95+ s.
- Right after an action: `Date.now() - planActionAge()` = `lastActionAt`
  (≈1.79e12) ≥ 30000 → guard passes immediately → the reload can fire 1 s
  after the caregiver's change, wiping it from view — the exact failure
  R4-4.1 was written to prevent.

**Fix**: drop the extra subtraction:
`if (window.planActionAge() < 30000) { return; }`.

**Validation**: after fix, fresh page reloads at 60 s (injected marker
disappears); after `planApply.markAction()` the page must survive the next
30 s+ without reloading (marker persists past `nextReloadAt`).

**Status**: fixed 2026-10-03 (all four `index.plush.*` forks +
`TestAutoRefreshGuardUsesActionAgeDirectly` regression pin). Live-verified:
fresh page marker gone at 50–60 s; action marked at +32 s → page still alive
at +61 s (floor held), reloaded by +62–72 s (≥30 s after the action).

### Round-8 sweep — verified, no defect

Revalidated live via agent-browser on 2026-10-03 (en unless noted):

- R4-7.13 per-animal rule exclusion: set + restore round-trip on animal
  10315 (rows verified in `care_rule_exclusions`, cleaned up after).
- R4-7.15 medication animal column; R4-7.16 observation/care/weighing use
  the `.plan-med-line` format.
- R4-7.9 navbar: hamburger ≤767 px, inline ≥768 px, no horizontal overflow.
- R4-3.2 info-glyph alignment: spread 0 at 1600/1280/1024/800 px.
- R4-7.24 no mid-word cut cells on care_plan (486 cells) or med page (36).
- R4-7.25 slot semantics: `○ HH:MM` outline+"Apply" = open, `✓ HH:MM`
  btn-success = applied — apply/unapply round-trip on animal 10221.
- R4-7.11b fulfillment links: "View record" on applied cards → treatment
  page → "Back to the day plan" returns to `/care_plan?kind=medication`.
- R4-7.19/22 Protocol tab: single merged table, one line per protocol.
- R4-7.12 trace rows carry real actions; R4-7.18 density toggle gone on
  all kinds; R4-7.17 covered by the R8-1 parse sweep + locale test.
- R4-7.14/14b/14c feeding: red late dot (rgb 220,53,69), time sub-groups
  ordered ascending per feeding, collapsible animal list toggles.
- R4-7.21 count badges = occurrence count capped at 99+ (`BadgeCap`),
  matches rendered rows on observation (13/13, 10/10) and feeding
  (250→"99+", 99→"99").
- Locale spot-check after R8-1 fix: fr/de/nl/en all render the corrected
  script line and define `window.planApply`.

---

## Archived rounds

| Round | File |
|---|---|
| Round 7 (care plan / animal page UX) | `docs/archive/2026-10-03-care-plan-round-7-bugs.md` |
| Round 4 (care plan / treatment / dashboard UX) | `docs/archive/2026-10-02-care-plan-round-4-bugs.md` |
| Round 3 (care plan / treatment / navbar UX) | `docs/archive/2026-10-02-care-plan-round-3-bugs.md` |
| Round 2 (care plan UX) | `docs/archive/2026-10-01-care-plan-ux-round2-bugs.md` |

Round-7 companion plan: `docs/care-plan-round-7-fix-plan.md`.
Round-4 companion plan: `docs/care-plan-round-4-fix-plan.md`.