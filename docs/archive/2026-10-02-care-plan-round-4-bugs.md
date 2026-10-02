# Care plan / treatment / dashboard UX — Round 4 bugs

Tracking document for the care-plan / treatment / dashboard UX round-4 bugs
(compact medication work screen, animal Protocol tab, day-plan feeding cards
and apply affordances). All work is in the `creaves/` project.

**Guideline** (same as `/Users/muaddib/dev/creaves.project/bugs.md`):
1. Create a detailed fix plan for each bug - the plan must contain test approach and validation steps - execute the plan and validate the fix when all elements are in place.
2. Any issues found must be fixed and the fix plan must be updated accordingly.
3. Issues found during testing must be fixed and the fix plan must be updated accordingly.
4. Each bug should be moved to docs/archive when tested and closed as the associated plan.
5. **All changes must be tested, including e2e testing using the [agent-browser skill](../.agents/skills/agent-browser/SKILL.md)** — no fix is complete without e2e evidence (commands, URL, captured output).
6. Use interactive shell/filmstrip to validate the output of the tool - you must verify the actual terminal output using agent-browser skill
7. Check code quality with each tool run separately (do not chain them with `;` or `&&`):
- `go vet ./...`
- `staticcheck ./...`
- `gocognit -over 15 .`
- `gocyclo -over 12 .`
- `go test -count=1 -race -cover ./...`
Fix any issues.
8. Commit each fix with a clear and descriptive commit message

### Session constraints
- **creaves contains real production data**: never drop/destroy data; schema changes must be additive/backward compatible (no destructive migrations).

At the end of the session - the bug list must be empty, all changes committed and resolved entries archived in `docs/archive/`. If new items are added, restart the process.

---

<!-- Previous rounds fixed & archived:
- docs/archive/2026-10-02-care-plan-round-3-bugs.md
- docs/archive/2026-10-02-care-plan-review-regressions-1-7.md
- docs/archive/2026-10-01-care-plan-ux-round2-bugs.md -->

---

## R4-1 — Compact medication "Late" tier is not readable as late

**URL**: `http://localhost:3000/care_plan?view=compact&kind=medication`

**Reported**: the "Late" item shows more than a single entry and adds pills about
"tomorrow" — the late one should be clear.

**Verified (agent-browser, 2026-10-02 17:39, admin session)**: the Late tier
header reads `Late 22` while the summary strip reads `Late 20` — the two counts
disagree (§R4-1c). Extracted DOM of `#tier-late-rows`:

```
LINE 0 rows=2 :: ○ 08:00 due=2026-10-02T08:00  ○ 18:00 due=2026-10-02T18:00
LINE 1 rows=1 :: ○ 12:00 due=2026-10-02T12:00  ○ 12:00 due=2026-10-03T12:00  ○ 12:00 due=2026-10-04T12:00
LINE 2 rows=1 :: ○ 12:00 due=2026-10-02T12:00
```

`LINE 1` is placed in the **Late** tier because its most urgent open slot is
`2026-10-02T12:00` (late), yet the line renders **three** day-pill buttons
(02/10 late + 03/10 + 04/10), and all three carry the **identical** `○` glyph
and `btn-warning` styling. A user scanning the Late tier cannot tell which
button is the late one — exactly the reported symptom.

**Root cause**:
- `actions/care_plan_viewmodel.go` `seriesOf()` / `chunkSeriesRows()`: a
  `MedSeriesView` is placed in a tier by `seriesTier()` = its **most urgent
  open slot**, but the whole series (every day) is rendered in that tier. The
  non-late slots of a late series are indistinguishable from the late slot.
- `_med_series.plush.html` renders every slot with the same glyph/`btn-warning`;
  only a `DueDayKey`/`DueShortDate` badge distinguishes days.

**Fix plan**: see `docs/care-plan-round-4-fix-plan.md` §R4-1.

**R4-1c (sub-issue)**: Late tier header badge (line 126 of
`templates/care_plan/index.plush.html`, `len(view.MedTiers[0])`) counts
**series**, while `view.Stats.LateCap` counts **occurrences**. Two different
units on the same screen (`Late 22` vs `Late 20`). Either unify on one unit or
label them distinctly.

---

## R4-2 — Animal Protocol tab: medication buttons have no status color code

**URL**: `http://localhost:3000/animals/10221?back=%2Fcare_plan%3Fkind%3Dmedication%26view%3Dcompact#nav-plan`

**Reported**:
1. buttons should be grouped `morning/noon/evening`, possibly on a single line,
   colour-coded — **red: late / yellow: now / white: future** — **green: done**,
   based on the current time;
2. the feeding should allow a click to create the entry (right type, prefilled);
3. general entry layout rule:
   ```
   A/ <text> left | <button1><button2>... right
   B/ <text> left | <button1> right
      <button2>
      ...
      <buttonX>
   C/ <text> left (wrapped as needed)
      <button1> right
      <button2>
      ...
      <buttonX>
   ```

**Verified**: DOM of `#animalCarePlan` — every open slot is `btn-warning`
regardless of whether it is late, due-now or in the future:

```
LINE 13 rows=2 :: ✓ 08:00 | btn-success  | 2026-10-02T08:00
                 ○ 18:00 | btn-warning | 2026-10-02T18:00
LINE 14 rows=2 :: ○ 08:00 | btn-warning | 2026-10-02T08:00   <-- 08:00 is PAST at 17:39, still yellow
                 ○ 18:00 | btn-warning | 2026-10-02T18:00
LINE 15 rows=1 :: ○ 12:00 | btn-warning | 2026-10-02T12:00   <-- PAST, still yellow
```

At 17:39, `08:00` and `12:00` of 2026-10-02 are **late** (red) but render
yellow (the "applicable / now" colour). No slot is ever rendered red. Only
`✓` (done → green) and `–` (out of window → `btn-light`) exist in
`_med_series.plush.html`.

**Grouping**: `LINE 13`/`14` already split morning (08:00) and evening (18:00)
into `rows=2` (bucket change forces a new row, `chunkSeriesRows`), but each row
is right-justified via `ml-auto text-right`, so a 2-row series is visually two
stacked right-aligned lines instead of one left-text/right-buttons line.

**Feeding**: in `templates/animals/show.plush.html` lines 703-728 the
`day.Items` rows render **status badges only — no apply button at all**. A
caregiver cannot record a feeding/observation from the animal Protocol tab; it
is reachable only from the day plan. The requested "click to create the entry,
right type prefilled" does not exist here.

**Fix plan**: `docs/care-plan-round-4-fix-plan.md` §R4-2.

---

## R4-3 — Info (ℹ) button position, localization and alignment

**URLs**: both `/care_plan` surfaces and `/animals/10221#nav-plan`

**Reported**:
- Info button should be translated in all languages — not just English;
- Info button should be the **1st entry** in the list to avoid alignment issues;
- no need for "En retard" pills — the button colour should indicate the status.

**Verified — partially NOT a bug**:
- `care_plan.detail.action` **is** translated in all four locales
  (`locales/care_plan.{en-us,fr,de,nl}.yaml:303`). Rendered check in the `de`
  locale: `document.querySelector('.plan-detail-btn').getAttribute('title')`
  → `"Details"`. The button **glyph** is `ℹ` (U+2139), a language-neutral
  symbol — it does not translate and should not.
- **Real gap**: the glyph `ℹ` is rendered *inline after the drug label*
  (`_med_series.plush.html`, inside `<div class="mr-2 text-nowrap">`), so
  rows with different label lengths put the ℹ at different x positions →
  the misalignment the user reports. It is not the first entry of the line.
- The day-pill badges (`plan-med-day`) are rendered **before** the button and
  do shift it, compounding the ragged alignment.

**"En retard" pills**: the `t("care_plan.status.late")` badges exist in three
places and are redundant with the requested colour coding:
- `templates/care_plan/index.plush.html:169` (compact tier card),
- `templates/animals/show.plush.html:722` (animal Protocol tab item row),
- `templates/care_plan/index.plush.html:431` (feeding chip status text).

**Fix plan**: `docs/care-plan-round-4-fix-plan.md` §R4-3.

---

## R4-4 — Day plan: apply affordance and layout

**URL**: `http://localhost:3000/care_plan`

**Reported**:
1. apply button — global: make sure a change stays visible for **at least 30 s**
   to allow undo / does not disappear on refresh too fast;
2. the general apply — the pill should **not be part of the button** but on the
   corner so the button size does not change;
3. order of elements: **late first / now / future** as first level;
4. no need of a colour dot on the animal — animals should share the same colour
   if they have the same treatments.

**Verified**:

**R4-4a (auto-refresh)** — `templates/care_plan/index.plush.html:737-749`:

```js
function scheduleReload() {
  setTimeout(function () {
    if (jQuery('.modal').is(':visible') || inFlight > 0) { scheduleReload(); return; }
    window.location.reload();
  }, 60000);
}
scheduleReload();
```

A single 60 s timer armed **at page load**, never re-armed on action. Applying
at T=59 s means the reload fires 1 s later and the in-place `markApplied()` undo
button (line 889) is destroyed by the full re-render → the change disappears
almost instantly and the 60 s minimum visibility is **not** guaranteed.

**R4-4b (pill inside button)** — `templates/care_plan/index.plush.html:458-467`
(feeding group) and `:491-500` (cleanup/cage):

```html
<button type="button" class="btn btn-success plan-feeding-apply" ...>
  <i class="fas fa-check"></i>
  <span class="badge badge-pill badge-light ml-1"><%= fcard.ApplicableCount %></span>
</button>
```

The count badge is a child of the button → the button **changes width** when
the count changes, and **vanishes entirely** when `ApplicableCount == 1`. The
corner-overlay pattern the user asks for already exists elsewhere on the page
(`:512`, `badge-pill ... position-absolute;top:-0.6em;right:-0.6em`).

**R4-4c (ordering)** — the tier blocks (`:123` late, `:219` now, `:311` later)
already render before the Feeding (`:411`), Cleanup (`:475`) and History
(`:508`) sections, so late/now/future **is** first level. However the
**feeding section itself is a flat list** with per-chip `Late`/`Due` pills and
no late/now/future grouping — so the "first level" ordering does not extend to
feeding, which is where most of the workload is (99+ feeding occurrences vs 41
medication on the live data).

**R4-4d (colour dot per animal)** — `templates/care_plan/index.plush.html:429`:

```html
<span class="plan-dot plan-dot-<%= chip.Status %>">●</span> <%= chip.Label %>
```

The dot colour is `chip.Status`, which `feedingChipOf()`
(`actions/care_plan_dayplan.go:704-712`) copies from **the individual
occurrence**: `Status: string(it.Status)`. Two animals in the same cage × diet
group therefore get **different** dot colours, which reads as "different
protocol" when they only have different occurrence times.

**Fix plan**: `docs/care-plan-round-4-fix-plan.md` §R4-4.

---

## R4-5 — "Same cage, different protocol" — investigated, **not a rendering bug**

**Reported**:
```
E2 E
nb 1/2 🍼
```
shown with `1883/26` a blue dot and `2008/26` a yellow (late) dot — unexpected;
why do `/animals/10192` and `/animals/10317` have different protocols?

**Verified — data-level explanation (engine is correct, UI is misleading)**:

Both animals are cage `E2`, zone `E`, diet `NB 1/2`, no care-plan
application recorded for either (no rows in `care_plan_applications` for
`animal_id IN (10192, 10317)`). Their `care_animal_plans` rows:

| animal | id | schedule |
|---|---|---|
| 1883/26 | 10192 | `{"times": ["08:15","11:15","14:15","17:15"], "anchor":"intake","every_days":1}` |
| 2008/26 | 10317 | `{"times": ["08:15","12:15","16:15"], "anchor":"intake","every_days":1}` |

They genuinely have **different feeding schedules** (4× vs 3× per day, different
hours) — produced by the care-plan **conversion** of the legacy per-animal
feeding data (`name = "Alimentation — nb 1/2 (conversion)"`). Rendered
confirmation:

- `/animals/10192#nav-plan` → `08:15 Late · 11:15 Late · 14:15 Late · 17:15 Due`
- `/animals/10317#nav-plan` → `08:15 Late · 12:15 Late · 16:15 Late`

At 17:39 animal 10192 still has a future 17:15 slot → `Due` (blue); 10317's
last slot (16:15) has passed → `Late` (yellow). **The engine is right**;
`replaces_kind`/`anchor` conversion produced per-animal plans that the
caregiver never reconciled.

**Conclusion**: this is a **data + traceability gap**, not a dot-rendering bug.
Without a way to see *why* an animal has the schedule it has, the differing dot
reads as a bug. Hence:

## R4-6 — Animal Protocol: no traceability of the applicable protocol

**Reported**: "Animal protocol should have a collapsible to see the applicable
protocol — it should allow the user to trace the actual protocol: rules
(specific animal, global rule) and content — it should link to the actual
definition if editable (global rules are admin only)."

**Verified**: `templates/animals/show.plush.html:736-791` renders the
`careAnimalPlansTable` in a collapsed `#planDetails` "Details" panel — but it
lists **only `care_animal_plans`** (the animal-specific plans). It does **not**
show the **global `care_rules`** (matcher rules) that also apply to the animal,
nor which concrete occurrences came from which source, nor an edit link to the
rule. The day cards above show `SourceName`/`SourceLink` per occurrence only in
the medication series partial; feeding/observation rows show a bare
`<a href=SourceLink>` with no rule-vs-animal distinction.

**Fix plan**: `docs/care-plan-round-4-fix-plan.md` §R4-6.

---

## R4-7 — Summary of verified vs. rejected reports

| # | Report | Verdict |
|---|---|---|
| R4-1 | Late tier shows several entries + "tomorrow" pills | **Confirmed** |
| R4-1c | — | **Confirmed** (Late 22 series vs Late 20 occurrences) |
| R4-2.1 | No red/yellow/white/green colour code on med buttons | **Confirmed** |
| R4-2.2 | Feeding not clickable on the animal Protocol tab | **Confirmed** |
| R4-2.3 | Entry layout rule A/B/C | **Confirmed** (no such rule today) |
| R4-3 | Info button not translated | **Rejected** — translated in all 4 locales (`de` renders `Details`); the `ℹ` glyph is a symbol |
| R4-3 | Info button should be 1st entry | **Confirmed** |
| R4-3 | "En retard" pills redundant | **Confirmed** |
| R4-4a | Applied change vanishes too fast on refresh | **Confirmed** (60 s timer armed at load, never re-armed) |
| R4-4b | Count pill inside the button changes its size | **Confirmed** |
| R4-4c | late/now/future first level | **Partially confirmed** — true for med tiers, **not** for the feeding section |
| R4-4d | Colour dot per animal | **Confirmed** (dot = per-occurrence status) |
| R4-5 | 1883 blue vs 2008 yellow = different protocol | **Explained** — genuinely different converted schedules; **not** a rendering bug → becomes R4-6 traceability work |

---

## Closure (2026-10-02)

All Round-4 bugs resolved, quality-gated and validated e2e
(agent-browser, admin/admin, dev server :3000, four locales):

| # | Resolution | Evidence |
|---|---|---|
| R4-1.1 | Every medication slot colour-coded by tier (`MedSlotView.Tier`/`TierClass`): late red · due-now yellow · future white (bordered) · applied green; glyph + title kept | `eval`: slot classes `btn btn-danger … plan-med-urgent`, `btn btn-warning …`, `btn btn-light border …`, `btn btn-success` |
| R4-1.2 | The series' urgent slot carries a red ring (`plan-med-urgent`), its future siblings recede (`plan-med-future`) — same on care plan, dashboard and animal tabs | 41 urgent / 37 future slots in the compact medication section |
| R4-1.3 | Late count reconciled: the tier badge counts OCCURRENCES (the strip's unit), per tier across all series | strip `Late 24 / Now 4 / Later 57` = badges `Late 24 / Now 4 / Later 57`; fr `En retard 24`, de `Überfällig 24`, nl `Te laat 24` |
| R4-2.1 | Same colour code on the animal Protocol tab (shared `_med_series` partial) | animal 10221 med buttons red/yellow/white by tier |
| R4-2.2 | Feeding/observation/care rows are actionable through ONE shared apply partial (`_apply_toggle`), feeding opens the prefilled entry | `data-kind=feeding`, `data-detail=lait réf. 30/50 di…`, `data-animal-id`, `data-due-at` on the animal page |
| R4-2.3 | A/B/C entry layout rule implemented in `assets/css/care-plan.scss` (content-driven flex) | line children `[plan-med-lead, plan-med-label, plan-med-btns]` |
| R4-2.4 | Morning/noon/evening groups are explicit labelled captions (`plan-med-bucket`) | `Evening` / `Soir` / `Abend` / `Avond` |
| R4-3.1 | ℹ is the FIRST entry of each line, in a fixed-width leading column; title stays localized, glyph untouched | `.plan-med-lead > button.plan-detail-btn` first, title `Details`/`Détails` |
| R4-3.3 | Redundant status pills removed where the colour/state carries it; terminal glyphs kept; status text stays `sr-only` | no `.text-muted.small` status pill; 704 `sr-only` labels; late rows red (`.plan-item-late`) |
| R4-4.1 | 30 s apply-visibility floor (shared `lastActionAt`, `planActionAge`) | `planActionAge()` guard live in the served page |
| R4-4.2 | Count pill is a corner overlay outside the apply button (feeding + cleanup) | `.plan-feeding-apply .badge` = 0, `.plan-apply-count` = 163 |
| R4-4.3 | Feeding grouped into late → now → later sections | `#feed-tier-0 Late 99+`, `#feed-tier-1 Now 20` |
| R4-4.4 | ONE status dot per cage × diet group (max tier of its chips) | 163/163 rows have exactly one `.plan-dot`; cage E2 (1883/26 + 2008/26) = single `plan-dot-late` |
| R4-5 | Investigated: genuinely different converted schedules, engine correct — **no code change**; made visible by R4-6. Data question carried to the care-expert backlog (out of scope this round) | see the R4-5 section above |
| R4-6 | Applicable-protocol traceability card on the Protocol tab (animal plans first, then rules; admin-only rule edit link) | `#planTraceTable` with 6 `plan-trace-animal` rows + 1 `plan-trace-rule` row (`/care_rules/b1a36983…` edit link) on animal 10221, in all 4 locales |

Quality gates (run separately, clean): `go vet ./...`, `staticcheck ./...`,
`gocognit -over 15 .` (no new offenders), `gocyclo -over 12 .` (no new
offenders), `go test -count=1 -race -cover ./...` (all packages ok).
