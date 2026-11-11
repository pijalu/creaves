# Round 3 — care plan / treatment / navbar UX bugs (FIXED & VERIFIED)

Archived 2026-10-02. All seven bugs fixed, tested (go tests + e2e via
agent-browser in en/fr/de/nl), quality gates green, one commit per fix.

**Cross-cutting groundwork (do first):** shared toggle partial `templates/care_plan/_plan_med_toggle.plush.html` (+ JS/i18n) extracted from dashboard + care_plan inline scripts — consumed by care_plan index, dashboard (4 locales each) and animals/show (4 locales). Every UI change is applied to **all 4 locale template variants** and validated with **agent-browser in en, fr, de and nl** (`/lang/?lang=…&url=…`); any issue found during validation is fixed regardless of source and the relevant plan updated.

### Bug R3-1: Menu — progressive collapse
The navbar menu must collapse progressively based on available space:
1. **Space permitting**: show icon + text (current full mode).
2. **Not enough space**: show icons only, with a tooltip on each icon.
3. **Even less space**: fall back to hamburger menu.

**Assessment** (evidence: `templates/application.plush.html` + 3 localized variants):
- Current behavior is **two-stage, not three-stage**: `navbar-expand-xl` (line 23) collapses straight to hamburger below xl (1200px). Labels use `nav-label d-none d-xl-inline` (lines 42, 105, 113-148), so at <xl the labels would be icon-only *inside the collapsed hamburger* — but the inline CSS at lines 15-18 (`@media (max-width: 1199.98px) { #navbarSupportedContent .nav-label { display: inline !important; } }`, added for a previous "cut menu" bug) forces labels visible inside the hamburger panel.
- Result: there is **no intermediate icon-only state** — the bar jumps from icon+text directly to hamburger, and the breakpoint is fixed (xl) rather than based on actual available space.
- **No tooltip infrastructure exists**: no `tooltip()` initializer in `assets/js/application.js` or anywhere; Bootstrap 4.6 tooltips require explicit opt-in init. The `title="..."` attributes already present on the nav links (e.g. line 113) are the natural tooltip source.
- 10 top-level items (Admin dropdown, Reports dropdown, Dashboard, Feeding, Day plan, Log entries, TODOs, Animals, Reception, New dropdown) + search box (line 159) + right-side user menu (line 172) — overflow is real on medium widths with long localized labels (de/nl).
- Fix shape: three ranges (e.g. ≥xl icon+text; lg-only icon-only + initialized tooltips; <lg hamburger), or a JS-measured progressive collapse. Must be applied to all 4 `application.plush.{html,fr,de,nl}.html` variants; `guest.plush.*` likely unaffected (verify).

**Fix plan R3-1:**
1. In `templates/application.plush.html` (+ de/fr/nl variants): `navbar-expand-xl` → `navbar-expand-lg` (hamburger below lg); labels keep `d-none d-xl-inline` → expanded bar shows icon-only in [lg, xl), icon+text ≥xl, hamburger <lg.
2. Replace the `@media (max-width: 1199.98px)` override (lines 15-18) with `@media (max-width: 991.98px) { #navbarSupportedContent .nav-label { display: inline !important; } }` so labels stay visible inside the hamburger panel only.
3. Add tooltip init in `assets/js/application.js`: `$('.navbar .nav-link[title]').tooltip({placement:'bottom', boundary:'window', trigger:'hover'})`.
4. Guest layout: verify no nav-label usage → no change.

**Test approach R3-1:** e2e (agent-browser) at widths 1400/1050/800: (a) 1400 labels visible; (b) 1050 bar expanded + labels hidden + title attrs/tooltips present; (c) 800 toggler visible, collapsed, labels visible when expanded. All 4 locales via `/lang/?lang=…`.
**Validation steps R3-1:** no horizontal overflow at 1050 (`scrollWidth <= innerWidth`); locale sweep en/fr/de/nl.

### Bug R3-2: Care plan (`/care_plan`) — apply buttons & occurrence pill
1. Use a **checkmark icon** for apply buttons.
2. **Remove the pill showing the number of occurrences.**
3. When a group contains **more than 1 animal**: show a pill counter instead — one check next to each animal plus a global apply.
4. Once applied, the checkmark must become an **undo** control (clicking reverts the application).

**Assessment** (evidence: `templates/care_plan/index.plush.html` + 3 localized variants; JS in same file):
- Tier apply buttons are **text buttons**, not icons: `btn btn-success btn-sm ml-1 plan-apply-btn` containing `<%= t("care_plan.apply.action") %>` at lines 143-152 (late), 214-223 (now), 285-294 (later).
- The occurrence pill is the `+<%= card.RemainingCap %>` badge (`badge badge-pill badge-secondary`, title `care_plan.badge.more`) at lines 127, 202, 273 — rendered per tier card. This is the pill to remove; a pill counter must appear **only** when a group has >1 animal.
- Group/multi-animal apply exists only for **feeding groups** (line 369-372: `plan-feeding-apply` with `care_plan.card.apply_group (<%= fcard.ApplicableCount %>)`) and **cage rows** (line 396-399) — these are count-bearing text buttons, not check icons, and have no per-animal checks.
- **Undo exists only in the History section** (line 472: `plan-history-undo` outline button with `care_plan.action.undo`) — after applying from a tier, the row does not transform its checkmark into an undo; it disappears/moves. Required: applied state → checkmark becomes undo in place, single click reverts.
- Supporting infra already exists and is reusable: `_med_series.plush.html` implements exactly the apply→undo single-click toggle (`plan-apply-btn`/`plan-unapply-btn`, lines 41-63) via the same JS endpoints; the undo/unapply backend route is already exercised by history + med series.
- Viewmodel impact: `actions/care_plan_viewmodel.go` tier cards need an applied/undo state + per-animal breakdown for multi-animal groups (currently only `ApplicableCount`/counts exist).

**Fix plan R3-2:**
1. Apply buttons → checkmark icon: replace text `t("care_plan.apply.action")` inside `.plan-apply-btn` with `<i class="fas fa-check"></i>` (keep `title`) in tier rows (lines 143-152, 214-223, 285-294), feeding chip apply (~354), feeding group (369), cage (396) — all 4 locale variants.
2. Remove occurrence pill: delete `+<%= card.RemainingCap %>` badge blocks (lines 127, 202, 273). Keep compact folding; update S3 doc comment + any viewmodel tests asserting the pill.
3. Multi-animal groups: `ApplicableCount` as pill counter on the global check button; per-animal check icon per chip (extend `.plan-feeding-one`).
4. Applied → undo in place: extend `markApplied` to flip button to `fa-undo` + `plan-unapply-btn` (data attrs retained) instead of `disabled ✓`; add unapply dispatch (shared partial, groundwork). Batch rows: success → global button flips to undo that unapplies the batch sequentially.

**Test approach R3-2:** Go: viewmodel/http tests updated (pill removal, counter); e2e `/care_plan?kind=feeding` + a tier kind: check→undo round-trip without reload, counter pill only on multi-animal groups, no `+N` pill. 4 locales.
**Validation steps R3-2:** `get count/html/text` assertions; `care_plan_applications` row added/removed spot-check.

### Bug R3-3: Care plan — late/now/later background color code
Use a light background color code for the time-bucket groups:
- **late** → light red
- **now** → light yellow
- **later** → light green

**Assessment** (evidence: `templates/care_plan/index.plush.html`):
- Color coding exists today only via **button outline classes** on the collapse toggles (`btn-outline-danger` line 103, `btn-outline-warning` line ~178, `btn-outline-primary` line ~249) and via full-strength row classes (`table-danger` on late rows, line 111; analogous classes for now/later rows).
- The **group containers themselves have no background** — `#tier-late` (line 102), `#tier-now`, `#tier-later` are plain `<div class="mb-2">`.
- Fix shape: light-tint backgrounds (`#f8d7da` / `#fff3cd` / `#d4edda`, i.e. Bootstrap alert tints, or `bg-danger`/`bg-warning`/`bg-success` at low opacity) on the group wrapper including the collapse body; keep text/row contrast readable. CSS block already exists at the top of the file (lines 1-16) to extend; replicate to the 3 localized variants.
- Note: later = green is a **change of meaning** vs the current `btn-outline-primary` (blue) for later — update the summary strip badges (lines 38-44) for consistency.

**Fix plan R3-3** (same markup region as R3-4 — implemented together):
1. CSS in the existing `<style>` block (+3 locales): `.plan-tier-late{background:#f8d7da} .plan-tier-now{background:#fff3cd} .plan-tier-later{background:#d4edda}`; soften full-strength `table-danger`-style row classes inside tinted groups.
2. Summary strip: later badge `badge-primary` → `badge-success` (line 44).
3. Tint class on tier wrapper divs.

**Test approach R3-3:** e2e `get html` on `#tier-late/now/later` shows tint class; later strip badge `badge-success`. 4 locales.
**Validation steps R3-3:** class presence + computed bg spot-check via browser eval.

### Bug R3-4: Care plan — collapsible group styling
The collapsible late/now/later groups currently look like buttons. Add a proper **border around each group** so they read as grouped sections — should look better than a button.

**Assessment** (evidence: `templates/care_plan/index.plush.html`):
- Each tier is headed by a `<button class="btn btn-outline-* position-relative mb-1">` (lines 103, ~178, ~249) with a floating count pill — visually three loose buttons, no grouping affordance; the collapse body (`#tier-late-rows` line 107 etc.) is visually detached from its toggle.
- Fix shape: convert each tier into a bordered card-like section (border in the tier color, header row with tier name + count badge + chevron for collapse, body inside the same border) — pairs with R3-3's background tint. Keep the anchor ids (`#tier-late`, `#tier-now`, `#tier-later`, scroll-margin at line 15) and the summary-strip links working.

**Fix plan R3-4:**
1. Replace `<button class="btn btn-outline-*">` + detached `.collapse` with a bordered section: `.plan-tier.plan-tier-{late,now,later}` wrapper (border in tier color, radius), `.plan-tier-header` (cursor pointer, `data-toggle="collapse"`, title + count pill + chevron rotating on `aria-expanded`), `.plan-tier-body.collapse.show` holding the table.
2. Keep anchor ids + `scroll-margin-top`; default expanded.
3. All 4 locale variants; i18n keys unchanged.

**Test approach R3-4:** e2e: header click toggles `#tier-late-rows.show`; `#tier-later` anchor still scrolls. 4 locales.
**Validation steps R3-4:** combined with R3-3 e2e run.

### Bug R3-5: Care plan — medication line format
All medication rows must follow the format:

```
<animal> - <medication> - <toggle buttons with hours>
```

Example (fake data):

```
326/26 · Hérisson · S38 | Citramox L.A. (48H) — 0.06 ml IM | [12:00] [15:00] [18:00]
```

The hour buttons are **togglable** so the user can record/undo treatments with single clicks.

**Assessment** (evidence: `templates/care_plan/index.plush.html`, `templates/care_plan/_med_series.plush.html`, `actions/care_plan_viewmodel.go`):
- Current medication rendering in `/care_plan` is an **ordinary tier table row** (animal link td line 113-117, detail td line 119+, single text apply button) — the template header comment (lines 94-99) explicitly states "no per-hour button series, no separate collapsible medication section". There are **no hour buttons at all** on `/care_plan`.
- The requested component **already exists**: `_med_series.plush.html` renders `<series.Label>` + togglable hour-slot buttons (apply `○` → done `✓` → re-click undo; late `–` clickable; skipped `⊘`; deferred `⏸`) with `plan-apply-btn`/`plan-unapply-btn` and all data attributes — but it is currently used **only by the dashboard** (`templates/dashboard/dashboard.plush.html:124` + 3 localized variants), not by `/care_plan`.
- Data source exists: `MedGroupView` (`care_plan_viewmodel.go:155-159`, per-animal `AnimalID`/`AnimalLabel`, Series built by `buildMedGroups`) — the care-plan handler must build med groups for the active kind=medication and pass `medSeriesEye=false` (the partial skips the dashboard-only eye then).
- Fix shape: for kind=medication, replace tier table rows with one line per (animal, drug series) in the required `<animal> — <medication> | [HH:MM]…` format, reusing/extending `_med_series`; hour buttons stay single-click toggles (already the partial's contract). Animal label part (`326/26 · Hérisson · S38`) is `AnimalLabel`; medication part (`Citramox L.A. (48H) — 0.06 ml IM`) is `series.Label` — verify series.Label includes dosage/route, extend builder if not.

**Fix plan R3-5:**
1. Groundwork: shared partial `templates/care_plan/_plan_med_toggle.plush.html` (dashboard-style slot toggle JS + message modal + dosage modal; i18n via t()). Replace dashboard inline copies (4 dashboard locale templates) with the partial.
2. Viewmodel: expose `MedGroups []MedGroupView` on `DayPlanView` (built via existing `buildMedGroups(plan, zone, false)`); when kind=medication, tier sections render med lines instead of table rows — each series line placed by its most urgent open slot (`tierOrder`).
3. Template: in `care_plan/index.plush.html` (+3 locales) medication kind renders per tier: `<animal> — <series.Label> | slot buttons` via extended `_med_series.plush.html` (+3 locale variants) with an animal-label prefix flag (`medSeriesAnimal`).
4. Verify `series.Label` includes drug + dosage + route; extend label builder if missing.

**Test approach R3-5:** Go: viewmodel test (med groups in DayPlanView, label format); e2e `/care_plan?kind=medication`: line format, slot `○`→`✓`→`○` round-trip without reload, late confirm path; dashboard regression. 4 locales.
**Validation steps R3-5:** DB application rows; dashboard still renders via shared partial.

### Bug R3-6: Animal treatment tab — not togglable + wrong data source
URL: `/animals/<id>?back=...#nav-treatment`
1. **"Does not work" = the hour/clock badges are not togglable** (static spans/disabled buttons — user clarification; the whole screen content is wrong anyway and will be replaced, so fixing the current badges is NOT required).
2. Each planned treatment must be driven by the **care plan data source** — matching the care-plan medication source but limited to treatment details + toggle.
3. The view must show **history and future** occurrences.
4. For recurrent protocols **without an end date**, limit the future horizon to **5 days**.

**Assessment** (evidence: `templates/animals/show.plush.html` + 3 localized variants, `actions/animals.go:549-662`):
- Tab markup: `#nav-treatment` pane at line 420; data = `animal.Treatments.TreatmentEntriesMap()` date accordion (line 459) + per-date rows.
- Hour badges are **static**: done = `<span class="badge badge-success">` (line 561), skipped = disabled `btn-light` (565), pending = `badge-danger`/`badge-warning` spans (570-572); protocol rows identical pattern (610-618). No click handler, no apply/undo data attributes — hence "not togglable".
- Data source mismatch: the tab is fed by legacy `Treatments` entries plus **today-only** plan rows (`animalPlanTodayRows` in `animals.go` Show, line ~587) — there is no history/future horizon from the care-plan engine, and no 5-day future cap for open-ended recurrent protocols.
- Hash deep-link works (tab activation JS at lines 1080-1093) — not part of the bug.
- Fix shape: rebuild the pane on the care-plan engine (same occurrence/series builder as `/care_plan` medication — reuse `MedGroupView`/`_med_series`-style toggles), scoped to this animal; horizon = full history + future, future capped at 5 days when the protocol schedule has no end date (schedule parsing already available: `humanSchedule`, `planWindow` helpers in `actions/care_plan_display.go`). Apply/undo must hit the same endpoints as the care-plan page (existing `plan-apply-btn`/`plan-unapply-btn` JS lives in `care_plan/index.plush.html` — it must be extracted/shared or the animal page must include equivalent JS; currently show.plush.html has no plan-apply JS).

**Fix plan R3-6:**
1. Backend (`actions/care_plan_animal_page.go`, `animals.go` Show): animal-scoped plan window `[now-14d … now+5d]` (engine cap 14d past — document); future horizon `now+5d` for all sources (open-ended courses — `Schedule.OpenEnded()` — are the unbounded case; bounded courses end earlier naturally). Filter `plan.Items` to this animal + medication kind → date-grouped series of `MedSlotView` (extract reusable slot builder from `buildMedGroups`).
2. Template (`animals/show.plush.html` +3 locales): replace `#nav-treatment` accordion with per-date groups (history → future), each rendering treatment detail lines + hour toggles (`_med_series`-style). Keep "Add New treatment"; legacy manual treatments keep a compact read-only list below. Remove static clock badges + `TreatmentEntriesMap` usage.
3. JS: include shared `_plan_med_toggle` partial on the animal page.
4. Dead code: `animalPlanTodayRows` kept only for Dash-7 deep-link (`resolvePlanItemDetail`); drop `setTreatmentProtocolLinks` if no template references `treatmentProtocolLinks` (check all locale variants).

**Test approach R3-6:** Go: `care_plan_animal_page_test.go` — history included, future ≤5d, bounded course stops at end date, slot state mapping. e2e: animal with active protocol → past/today/future groups (future ≤5), toggle round-trip. 4 locales.
**Validation steps R3-6:** `get text` on pane; future date-group count ≤5; `care_plan_applications` check.

### Bug R3-7: Animal page — protocol tab redesign
- The actual protocol list must move into a **collapsed collapsible** titled "Details".
- The tab itself must contain the **complete care plan for the selected animal**.
- That embedded care plan must be enhanced to **avoid repeating animal details** (already known in context) and focus on content.

**Assessment** (evidence: `templates/animals/show.plush.html:638-683` + localized variants; `actions/care_plan_animal_page.go`):
- "Protocol" tab = the `#nav-plan` tab (fr label "Protocole", `locales/care_plan.fr.yaml:139`; tab at show line 35, pane at 638). Today it renders **only the `careAnimalPlans` table** (name/kind/content/active/window/replaces/schedule + edit/delete, lines 644-682) plus the plan editor modal — there is no care-plan *occurrence* content.
- Fix shape:
  1. Wrap the existing plans table in a `<details>`/Bootstrap collapse, **collapsed by default**, labeled "Details" (new i18n key, 4 locales).
  2. Add the animal's complete care plan to the pane: reuse the `/care_plan` view-model pipeline filtered to `animal_id` (per-kind sections with hour toggles for medication, feeding/care rows) — same togglable components as R3-5/R3-6 so all three screens share one interaction language.
  3. Animal-context variant: suppress the animal label column/link in every row (animal identity is the page context) — needs a flag in the view model / partial (e.g. `medSeriesEye`-style context flag or a dedicated `animalContext` boolean) so the shared partials drop `<animal>` from the line format.
- Shares JS requirements with R3-6 (apply/undo handlers on the animal page).

**Fix plan R3-7:**
1. Template `#nav-plan` pane (`animals/show.plush.html` +3 locales): wrap `careAnimalPlansTable` block in collapsed collapsible labeled "Details" (new i18n key `care_plan.animal_plans.details`, 4 YAMLs); body `collapse` without `show`.
2. Above it: the animal's complete care plan — extend R3-6 builder to all action kinds → `BuildAnimalCarePlanView(tx, animal, now)`: med series + feeding/care/weighing/observation toggle rows for `[history … now+5d]`.
3. Animal-context mode: `animalContext=true` flag in shared partials → omit animal label/column (identity is page context).
4. JS: shared toggle partial already included (R3-6).

**Test approach R3-7:** Go: builder test (all kinds present). e2e: `#nav-plan` shows care plan; Details collapsed by default and expands; edit/delete still work; no animal repetition in rows; key translated. 4 locales.
**Validation steps R3-7:** combined e2e with R3-6; locale sweep.

---

## Resolution summary (2026-10-02)

| Bug | Fix commit(s) | E2E evidence (agent-browser) |
|-----|---------------|------------------------------|
| R3-1 menu collapse | `3b759ad` (+ `1fc53e2` measured collapse) | 1400px: labels visible, fits; ≤1350: `.nav-compact` icon-only, `scrollWidth<=clientWidth` at every width 1000–1400; FR labels collapse earlier (longer), return at 1900; search box always reachable |
| R3-2 apply/undo, pill removal | `d908cb4` (+ shared partial `4d1ddd7`) | apply/undo round-trip on care_plan; DB `care_plan_applications` count tracks 1→2→1; no occurrence pills; in-place toggle, no reload |
| R3-3 tier tints | `c70ad82` | late/now/later sections render light red/yellow/green backgrounds; summary strip consistent |
| R3-4 collapsible groups | `c70ad82` | bordered tier sections with collapse headers replace button toggles; expand/collapse round-trip |
| R3-5 medication line format | `ca9f94a` | `/care_plan?kind=medication`: 41 lines / 84 slots as `<animal> — <medication> — hour toggles`; fr/de/nl day badges; feeding kind untouched (163 rows) |
| R3-6 treatment tab | `1733b8c` | 9577: 1 day / 1 slot, toggle round-trip DB 0→1→0; 8635: 6 day cards newest-first; 10208 (feeding-only): 0 days; history 14d + future 5d cap; 4 locales identical |
| R3-7 protocol tab | `bb197a9` (+ polish `4e4ea27`) | 8635 #nav-plan: 20 days, 6 slots, 6 items, Details collapsed→expands (edit/delete links work), ℹ popup in-page (no navigation); 10311 feeding-only plan: 7 day cards visible (was "no entries"); no top separator on first med line; toggle buttons right-justified; en/fr/de/nl, 0 missing translations |

### Issues found during fixing (all fixed, plans updated)

- **Engine window clamp** (R3-6 root cause): `BuildDayPlan` clamped
  `to = from + 14d`, truncating the 19-day TreatmentPlanWindow to zero
  future occurrences → tab rendered empty. Clamp now
  `maxTime(from, now) + 14d`; regression test
  `TestTreatmentPlanWindowSurvivesEngineClamp`.
- **Feeding-only plan rendered no entries** (R3-7, 10311): cage-grouped
  kinds (feeding/cleanup) were excluded from the per-day items — now
  every non-medication kind rides along as a compact line.
- **Navbar still overflowed** (R3-1 follow-up): breakpoint-only collapse
  let verbose locales cut the search box off — replaced by a measured
  probe (`scrollWidth` vs `clientWidth`) toggling `.nav-compact`.
- **Detail button navigated away**: the medication ℹ now opens the
  detail modal in-page on the animal show view (no view switch).
- **Pre-existing test failure** `TestUsersListFilters`: fixture landed
  on page 2 of the `role=user` listing (120 matches > per_page=100) —
  test now pages through the filter (`6ce39da`).

### Quality gates (2026-10-02, each run separately)

- `go vet ./...` — clean
- `staticcheck ./...` — clean
- `gocognit -over 15 .` — no function above baseline (refactors in
  `e45c38b`: `medSlotFor` dedupe, `setAnimalShowPlanData`,
  `betterTierSlot`, `animalTreatmentDayFor`)
- `gocyclo -over 12 .` — no function above baseline
- `go test -count=1 -race -cover ./...` — all packages ok (actions 59.3%)
