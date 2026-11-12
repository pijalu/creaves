# Care plan / treatment / dashboard UX — Round 4 fix plan

Companion to `bugs.md` (Round 4). Every bug below was **verified with evidence**
in an authenticated agent-browser session (admin/admin) against the live dev
server on 2026-10-02, ~17:39–17:45.

**Non-negotiables for this round**
- **Production data**: `creaves` holds real data. No destructive SQL, no
  dropping/creating rows to "test". All verification is read-only
  (`agent-browser eval` on rendered DOM, `SELECT` on read paths).
- **All four locales**: every template change lands in
  `.plush.html`, `.plush.fr.html`, `.plush.de.html`, `.plush.nl.html` — the
  four files in `templates/care_plan/` and `templates/animals/` are
  *byte-identical forks today*; they must stay in sync (the locale fork
  convention). Any edit must be applied 4×.
- **Colour coding is additive, never a replacement for text**: the requested
  red/yellow/white/green is carried by CSS classes; the accessible label
  (`title`, `aria-label`, glyph) stays.

---

## Legend of verified evidence

| Source | What it showed |
|---|---|
| `/care_plan?view=compact&kind=medication` text | `Late 20 Now 4 Later 57` strip vs `Late 22` / `Now 2` / `Later 17` section headers |
| `#tier-late-rows` DOM | `LINE 1 rows=1 :: ○ 12:00 due=…10-02 ○ 12:00 due=…10-03 ○ 12:00 due=…10-04` — all three same glyph/class |
| `#animalCarePlan` DOM (10221) | every open slot `btn btn-warning`; `✓` slots `btn-success`; `0` apply-like buttons |
| `feedings` block on `/care_plan` | `E2 E / nb 1/2 🍼 / ● 1883/26 Due / ● 2008/26 Late` |
| `care_animal_plans` SQL (10192, 10317) | different `times` arrays (4× vs 3×) — genuinely different schedules |
| `care_plan_applications` SQL | no rows for 10192/10317 → dot diff is timing, not application |
| source `care_plan_viewmodel.go:402` `seriesTier` | tier = most urgent open slot of the series |
| source `care_plan_dayplan.go:704` `feedingChipOf` | `Status: string(it.Status)` → per-occurrence dot |
| source `index.plush.html:737` `scheduleReload` | 60 s timer armed at load, never re-armed on action |

---

## R4-1 — Late tier readability (compact medication)

**Bug**: A series in the Late tier renders *all* its days (late + tomorrow +
04/10) with identical `○` glyph and identical `btn-warning`, so the late slot
is indistinguishable.

**Design decision**: do NOT re-tier the series (a series legitimately spans
days). Instead make the *late* slot visually dominant and the future slots
recede, **within** the line — the same "urgency-first" reading already used by
the tiers themselves.

### R4-1.1 Colour-code every medication slot by tier
Add a `Tier int` field to `MedSlotView` (`actions/care_plan_viewmodel.go`) and
populate it in `medSlotFor()` from `tierOrder(careplan.PlanStatus(it.Status))`.
Then in `templates/care_plan/_med_series.plush.*` map `Tier` → class:

| Tier | Button | Class | Glyph |
|---|---|---|---|
| 0 late / missing | red | `btn-danger` | `○` (keep) |
| 1 due-now | yellow | `btn-warning` | `○` |
| 2 scheduled/future | white | `btn-light` (bordered) | `○` |
| 3 applied/done | green | `btn-success` | `✓` |

This *subsumes and replaces* the current `slot.Applicable` / `slot.LateAllowed`
branching for colour. Keep `LateAllowed` for the *clickability* decision
(`data-late="true"` + confirm) — colour and affordance are orthogonal.

Also drop the `tierOrder` overload: `Status == "skipped"/"deferred"` keep their
current `btn-light` + `⊘`/`⏸` treatment (terminal, not tiered).

### R4-1.2 Make the late slot visually dominant in a late series
The first slot of a series in the Late tier is the urgent one (`seriesTier`
returns it). Render it with an extra `plan-med-urgent` class → red ring /
`box-shadow` so it stands out from the future siblings in the same row. The
future day-pill slots in that line get `plan-med-future` → reduced opacity.

**Files**:
- `actions/care_plan_viewmodel.go` — `MedSlotView` + `medSlotFor()`
- `templates/care_plan/_med_series.plush.{html,fr,de,nl}.html` — class mapping + `plan-med-urgent`/`plan-med-future`
- `assets/css/application.scss` — new block (see R4-2.3)

### R4-1.3 Reconcile the Late count (Late 22 vs Late 20)
The section badge (`index.plush.html:126`) counts **series**
(`len(view.MedTiers[0])`), the summary strip (`:59`) counts **occurrences**
(`view.Stats.LateCap`). Pick **one unit for the badge** and make the strip
agree. Recommendation: keep the strip on **occurrences** (it is the workload
count and matches the 99+ caps), and change the section badge to the
occurrence count of that tier's series (sum of open slots), so a caregiver
sees the same number in both places. If a series count is wanted, render it as
a separate, explicitly-labelled secondary badge (`{n} series`).

**Test**: `go test ./actions -run TestCarePlanViewModel`, plus a new
`TestMedSeriesSlotTiering` asserting that for a fixture series spanning
yesterday/today/tomorrow the today-late slot has `Tier == 0` and the future
slots `Tier == 2`.

---

## R4-2 — Animal Protocol tab: colour-coded, grouped, actionable

**Bug**: no status colour on med buttons; feeding/observation rows have no apply
affordance at all; no A/B/C entry layout rule.

### R4-2.1 Colour code (shared with R4-1.1)
The animal Protocol tab renders the **same** `_med_series` partial
(`animals/show.plush.html:701`), so R4-1.1 fixes both surfaces at once. Verify
the `#nav-plan` med buttons go red for past-due slots (R4-2 evidence: `08:00`
and `12:00` on 2026-10-02 were `btn-warning` at 17:39 despite being late).

### R4-2.2 Make feeding/observation rows actionable
`animals/show.plush.html:703-728` renders `day.Items` rows with **status badges
only** (`0` apply-like buttons verified in `#animalCarePlan`). Add the apply
affordance mirroring the day-plan **feeding chip** button
(`care_plan/index.plush.html:437-447`, class `.plan-feeding-one`) and the
input-kind open-apply modal (`isInputKind` in `index.plush.html:911`).

- Feeding is an **input kind**: the click must open the shared apply modal
  **prefilled with the right type** (`data-kind="feeding"`,
  `data-detail="<food>"`, `data-animal-id`, `data-due-at`) — the requested
  "present the right type/prefilled entry".
- Non-feeding kinds (observation/care/weighing/cleanup) reuse the same button
  with their own `data-kind`.
- Because the animal page has **no** `.plan-feeding-one` handler today, either
  (a) extract the day-plan apply/undo handler into a shared partial
  (`care_plan/_apply_toggle.plush.*`) included by both pages, or (b) render
  these rows as **links** into the day plan pre-targeted at the occurrence.
  **Choose (a)** — it is one visual language, which is the stated goal
  (Jakob's / Similarity principle, already cited in the codebase).

**Safety**: verify against a fixture animal in tests, and in e2e **do not click
apply on a production animal** — assert the button renders, is enabled, and
carries the correct `data-*` (read-only DOM inspection), and exercise the actual
apply round-trip only in the Go HTTP tests against the test DB.

### R4-2.3 The A/B/C entry layout rule
New CSS in `assets/css/application.scss` under a `/* bugs.md R4-2.3 */` header,
applied to `.med-series-line` and the new animal item rows:

```
A/ <text left>            | <b1><b2>...<bX> right      (all buttons fit one line)
B/ <text left>            | <b1> right
   <b2> ... <bX>                    (buttons stack, still right-aligned)
C/ <text left, wrapped>   |
   <b1> right
   <b2> ... <bX>                    (text wraps, buttons in a column)
```

Implementation: `display:flex; flex-wrap:wrap;` on the line; the text block
`flex:1 1 auto; min-width:0;` (so it wraps in C); the button block
`flex:0 0 auto; margin-left:auto;` with `display:flex; flex-wrap:wrap;
justify-content:flex-end;`. The button block naturally collapses to one row when
it fits (A), stacks when it overflows (B), and sits under a wrapped text block
when the text takes the full width (C). Add a `container`/media-query guard so
the A→B breakpoint is driven by content width, not viewport.

Remove the current hard `ml-auto text-right` + per-bucket forced row break
(`chunkSeriesRows` forcing a new row on bucket change) where it fights the rule —
keep the bucket **divider** as the morning/noon/evening grouping cue, but let
the buttons flow in one right-aligned group (the user's "possibly on a single
line").

### R4-2.4 Morning/noon/evening grouping label
The divider exists but is **invisible** (`border-top` on a plain div with no
label). The user wants the group to read as morning/noon/evening. Add a small
localized bucket label on each divider using the existing keys
`care_plan.slot.morning|noon|evening` (already translated in all 4 locales) —
so grouping becomes *explicit* rather than implied by an empty rule.

**Test**: extend `TestAnimalShowPlanTodayRowsAllLocales`
(`actions/care_plan_i18n_html_test.go:330`) to assert, per locale, that
(a) the animal Plan tab contains at least one `plan-med-slot` with the
expected tier class, and (b) feeding item rows carry
`data-kind="feeding"`.

---

## R4-3 — Info button: position first, alignment, drop redundant pills

**Bug (verified)**: `care_plan.detail.action` **is** already translated in all 4
locales (rendered `Details` in `de`); the `ℹ` glyph is language-neutral and must
not be translated. The **real** defect is placement: the ℹ sits inline after the
label, so rows with different label lengths misalign it.

### R4-3.1 Move ℹ to be the first entry of the line
In `_med_series.plush.*`, move the `plan-detail-btn` **before** the drug label
(in a fixed-width leading column) so every row's ℹ lands at the same x — this
is the "1st entry in the list to avoid alignment issue" request. Keep the
`title` localized. The dashboard eye (`dash-med-view`, Dash-7) stays adjacent.

### R4-3.2 Fix day-pill-induced raggedness
`plan-med-day` badges precede the buttons and shift them. With R4-2.3's flex
layout + fixed-width label column, day pills move **with** their button and the
row becomes a clean grid. Consider a fixed min-width on the time button so
`○ 12:00` vs `✓ 12:00` never change the column width.

### R4-3.3 Remove the redundant "En retard" status pills
Delete the late/now/later **status badges** where the **button colour** now
carries the status (R4-1.1):
- `templates/care_plan/index.plush.html:169` (compact tier card) — the
  `care_plan.status.late` badge; keep only when it conveys something the colour
  cannot (e.g. `RecordedLate` ⏱ marker, which stays).
- `templates/animals/show.plush.html:722` (animal item row `Tier == 0` badge) —
  replaced by a red state.
- `templates/care_plan/index.plush.html:431` (feeding chip status **text**) —
  per R4-4d the dot is removed too; keep a visually-hidden accessible label so
  screen readers still get the status.

**Constraint**: never remove the *only* signal of a status. Terminal states
(`skipped ⊘`, `deferred ⏸`, `superseded ↪`, `RecordedLate ⏱`) keep explicit
glyphs — they are not colour-encodable.

**Test**: add a render assertion that no `care_plan.status.late` text badge
co-occurs with an open `plan-med-slot` in the compact medication section.

---

## R4-4 — Day plan apply affordance, ordering, dots

### R4-4.1 Auto-refresh must not erase a fresh change (< 30 s visibility)
`index.plush.html:737-749` arms a single 60 s timer at page load and never
re-arms it. Applying at T=59 s is undone by the reload 1 s later.

**Fix**:
1. Track `lastActionAt` (set in `markApplied`, `markOpen`, `instantApply`,
   `instantUnapply`, and the history undo/late-record handlers).
2. In `scheduleReload`, defer the reload while `now - lastActionAt < 30_000`
   (the requested 30 s floor) — reschedule for the remainder.
3. Keep the modal / `inFlight` guards.

Optionally show a subtle "undo available" affordance during that window. A
`setInterval` that re-checks is simpler than chained `setTimeout` recursion and
avoids drift; use one timer, `setInterval(…, 5000)` with a due-time check.

**Test**: e2e — apply an occurrence, wait 35 s, assert the row is still there in
its applied state; then wait for the reload and assert it moved to history.
(`agent-browser wait 35000`.)

### R4-4.2 Count pill must not be inside the button
`index.plush.html:458-467` (feeding group) and `:491-500` (cleanup/cage) embed
`<span class="badge badge-pill">count</span>` as a button child → the button
**changes width** as the count changes and disappears at count==1.

**Fix**: move the count to a corner overlay, reusing the pattern already in the
file at `:512` — wrap the button in a `position-relative d-inline-block`
container and render the badge `position-absolute; top:-0.6em; right:-0.6em`.
Render it at **any** count ≥ 1 (not just > 1) so the button never resizes.
Add `aria-label` carrying the count for a11y.

**Test**: DOM assertion that `.plan-feeding-apply` has **no** child `.badge`
after the fix, and that the container has the overlay badge.

### R4-4.3 late/now/future as first level — extend to feeding
Medication tiers already render first (`:123`, `:219`, `:311`) before Feeding
(`:411`). But the **feeding** section is a flat list of cage×diet cards with
per-chip `Late`/`Due` pills and no tier grouping, and it holds most of the
workload (99+ feeding vs 41 medication on live data).

**Fix**: group the feeding cards into `late → now → later` sections matching the
medication tier visual language (`plan-tier-*` classes already exist in the
`<style>` block at `index.plush.html:20-34`). Compute the tier per **card** from
its chips' most urgent status (reuse `tierOrder`). Keep the zone/cage sort
*inside* each tier.

### R4-4.4 No per-animal colour dot when treatments match
`index.plush.html:429` renders `<span class="plan-dot plan-dot-<chip.Status>">`
where `chip.Status` is the **individual occurrence's** status
(`care_plan_dayplan.go:704-712`) → two animals in the same cage×diet group get
different dot colours (the `1883/26` blue vs `2008/26` yellow case).

**Fix**: replace the per-animal dot with a **per-group** status. The card
already has a natural group status = the most urgent chip status; render **one**
dot per card (next to the cage×diet label) using that group status, and drop the
per-animal dot entirely. Per-animal nuance stays available by hovering the chip
(`title`), which already carries `t("care_plan.status." + chip.Status)`.

This also resolves the user's confusion directly: identical cage×diet → identical
colour, always.

**Test**: unit test on `feedingViewOf`/`feedingViewsOf` asserting a card's
exposed group status is the max-tier of its chips, and a render test asserting
`#feedings .plan-dot` count equals the number of cards (not the number of
chips).

---

## R4-5 — (investigation, no code change)

**Finding**: animals 1883/26 (id 10192) and 2008/26 (id 10317) are both cage `E2`
/ zone `E` / diet `NB 1/2`, with **genuinely different** feeding schedules
produced by the care-plan conversion:

| id | schedule |
|---|---|
| 10192 | `["08:15","11:15","14:15","17:15"]` (4×/day) |
| 10317 | `["08:15","12:15","16:15"]` (3×/day) |

Neither has any row in `care_plan_applications`. The dot difference is purely
occurrence timing (10192 has a future 17:15 slot → `due`; 10317's last slot
16:15 has passed → `late`). **The engine is correct** — no rendering bug.

**Action**: none in code. The user's confusion is a *traceability* gap →
R4-6. Optionally flag to the data owner that the conversion produced
per-animal schedules that disagree within a shared cage (a care-expert
question, out of scope for this round's UI fixes).

---

## R4-6 — Animal Protocol traceability collapsible (new feature)

**Bug**: `animals/show.plush.html:736-791` (`#planDetails`) lists only
`care_animal_plans` (animal-specific). It does **not** show the global
`care_rules` that also apply, nor which occurrences came from which source, nor
an edit link to a rule.

**Fix**:

### R4-6.1 Backend — a "applicable protocol" projection
New handler/viewmodel in `actions/care_animal_plans.go` (or a new
`actions/care_protocol_trace.go`):

```go
type ProtocolSourceView struct {
    SourceType string // "animal" | "rule"
    SourceID   string
    Name       string
    Editable   bool   // animal plans: true; global rules: current user is admin
    EditURL    string // /care_animal_plans/{id} or /care_rules/{id}
    Content    string // humanised action payload
    Schedule   string // humanised schedule (existing humanSchedule)
    ReplacesKind bool
}
type ProtocolTraceView struct {
    AnimalID  int
    Sources   []ProtocolSourceView  // sorted: animal plans first, then rules
}
```
Populate from the **same** engine the day plan uses (`care_plan_service.go` /
`DayPlan`), so the trace and the rendered plan can never disagree. Filter to
sources that actually produced at least one occurrence for this animal.

### R4-6.2 UI — collapsible on the animal Protocol tab
Replace/extend the `#planDetails` card body with a two-part trace:
1. **Applicable sources** — one row per `ProtocolSourceView`: badge
   `Animal` / `Rule`, the source name (linked when `Editable`), the humanised
   content + schedule, and a "Replaces kind" check when set.
2. **Occurrences → source** — for each rendered day card, keep the existing
   `SourceLink` but label it explicitly (`from rule "M-x"` / `from animal plan
   "…"`) so a caregiver can trace a due time back to its definition.

Permission: **global rules are admin-only** — non-admin users see the rule name
and content but no edit link (and `/care_rules/{id}` edit is already
admin-gated server-side; this is about not showing a dead link).

**Test**: `TestAnimalProtocolTraceAllLocales` — per locale, GET the animal page
with an animal having both an animal plan and a matching global rule; assert
both appear, that the rule edit link is present for admin and absent for a
non-admin, and that no row is shown for a rule that produced no occurrence.

---

## Execution order

1. **R4-1.1 + R4-2.1** (tier colouring, one change, both surfaces) — highest
   value, unblocks the visual work.
2. **R4-3.3 + R4-4.4** (drop redundant pills, group dot) — small, independent.
3. **R4-2.3 + R4-2.4 + R4-3.1 + R4-3.2** (layout + ℹ first + bucket labels) —
   one template/CSS pass.
4. **R4-4.1** (auto-refresh floor) — isolated JS, needs a 35 s e2e wait.
5. **R4-4.2** (pill on corner) — isolated.
6. **R4-4.3** (feeding tiers) — touches grouping, do after the layout lands.
7. **R4-2.2** (feeding apply on animal tab) — largest; needs the shared
   handler partial.
8. **R4-6** (protocol trace) — new backend + UI, independent of the above.

## Quality gates (each run separately, per bugs.md)

```
go vet ./...
staticcheck ./...
gocognit -over 15 .
gocyclo -over 12 .
go test -count=1 -race -cover ./...
```

## Validation matrix

| Surface | URL | Locales |
|---|---|---|
| Compact medication | `/care_plan?view=compact&kind=medication` | en-US, fr, de, nl |
| Day plan (all) | `/care_plan` | en-US, fr, de, nl |
| Animal Protocol | `/animals/10221?back=…#nav-plan` | en-US, fr, de, nl |
| Animal Protocol (feeding) | `/animals/10192#nav-plan`, `/animals/10317#nav-plan` | en-US, fr, de, nl |

Every surface must be captured with `agent-browser get text body` **and** a DOM
`eval` dump of the button classes — screenshots alone are not evidence.

## Out of scope this round

- The conversion data divergence between 10192 and 10317 (R4-5) is a care-expert
  / data question; R4-6 makes it *visible* but does not change the conversion.
- Any schema change. Everything here is template + CSS + view-model/handler Go.
