# Care plan / animal page UX — Round 7 bugs

Live list for the round-7 feedback (2026-10-02). Status is marked per item;
everything marked **Done** is implemented, unit/render-tested and green —
what is still open is listed as **Open** with its remaining step.

Companion fix plan: `docs/care-plan-round-7-fix-plan.md`.

---

## R4-7.1 — Apply modal on the animal Protocol tab greys out the whole screen (**Done**)

**Where**: `/animals/10317?back=…#nav-plan` (any animal, any tab but the
Treatment one).

Clicking the green check of a feeding row opened the shared apply modal —
but the modal markup was rendered **inside the `#nav-treatment` tab pane**
(`templates/animals/show.plush.html`, `partial("care_plan/apply_toggle…")`).
From the Protocol tab that pane is `display:none`, so Bootstrap appended its
backdrop to `<body>` while the modal itself stayed invisible and unfocusable:
the screen greyed out and **nothing could be clicked** — no cancel, no
escape, no close.

**Fix**: both shared partials (`care_plan/plan_med_toggle`,
`care_plan/apply_toggle`) now render at **page level**, next to the plan
editor, outside every tab pane. Their listeners are delegated on
`document`, so DOM position is irrelevant. Applied in all four locale forks.

**Test**: render assertion — in `show.plush.*` the `planApplyModal` /
`planMedDosageModal` markup must appear AFTER the last `tab-pane` closes.

---

## R4-7.2 — Dead "missed" rows flood the day; the count belongs on the last row (**Done**)

**Where**: `/animals/…#nav-plan`, day cards.

A day with several feeding times showed every occurrence: the ones already
past due with nothing recorded rendered as dead rows with no apply button —
"only one apply" visible for a day that had four. The caregiver lost the
count of what was missed.

**Fix** (`actions/care_plan_animal_page.go`): an occurrence in status
`missing` **inside the last 24 h** leaves `day.Items` and only raises
`day.MissingCount` (capped at 5, `missingCountCap`). Older misses stay as
history. The template renders ONE pill beside the **last** remaining row's
description (`care_plan.animal_plans.missing`, `…missing_hint`, 4 locales).

---

## R4-7.3 — "Applicable protocol" and "Details" as two collapsed cards at the bottom (**Done**)

**Where**: `/animals/…#nav-plan`.

Two stacked collapsibles at the bottom of the tab answered one question
("why does this animal get this?").

**Fix**: ONE collapsed card **on top** of the tab merges the R4-6
traceability table (`#planTraceTable`, kept byte-identical for the a11y and
test surface) and the R3-7 protocol definitions table
(`#careAnimalPlansTable`). Collapsed by default; header keeps the source
count badge.

---

## R4-7.4 — The kind pill is glued to the red "late" border (**Done**)

**Where**: `/animals/…#nav-plan`, late rows.

`.plan-item-late` draws a 3px red left border; the row's first child (the
`nourriture` / kind pill) touched it and read as part of the border.

**Fix**: `assets/css/care-plan.scss` — `padding-left: 0.65rem` on
`.plan-item-late`.

---

## R4-7.5 — Per-animal check is noise for a lone animal; counter pill unreadable (**Done**)

**Where**: `/care_plan` (feeding cards, cage × diet).

With one animal in the cage the per-animal check duplicates the group
check; the corner counter badge (`badge-light`) was nearly invisible on the
row background; removing/adding the per-animal button shifted the row.

**Fix**: per-chip check buttons render only when `len(fcard.Chips) > 1`;
otherwise an invisible `.plan-apply-space` spacer keeps the footprint, so
the table columns stay aligned either way. The counter badge is
`badge-secondary` (darker) and renders only when the card holds several
animals. `aria-label` still carries the count (R4-4.2 a11y guarantee kept).
**Test**: `TestFeedingSingleAnimalGroupOnly` (new) + `TestFeedingSectionRender`
(extended) over all four forks.

---

## R4-7.6 — "Late" without a time: the caregiver cannot judge urgency (**Done**)

**Where**: `/care_plan`, feeding chips.

**Fix**: `FeedingChip` carries `DueHM` / `DueDayKey` / `DueShortDate`
(`feedingChipOf`), rendered as a muted `.plan-chip-due` span next to the
animal label.

---

## R4-7.11 — A green apply button reads as "already done" (**Done**)

Green (`btn-success` / `btn-outline-success`) is the **applied** colour in
this UI, so an apply affordance painted green looks recorded.

**Fix**: every apply control is now `btn-outline-secondary` (white
background, grey text) — `plan-apply-btn`, `plan-feeding-one`,
`plan-feeding-apply`, `plan-cage-apply`, in all forks. Terminal *applied*
states keep the green (`plan-med-*` slots, R4-1.1) and the modal **confirm**
buttons keep it too: confirming an action is not a state.

---

## R4-7.10 — `care_plan.status.done` rendered raw (**Done**)

`templates/animals/show.plush.*` uses `care_plan.status.done`, which was
never defined in any locale file, so the literal key was displayed.

**Fix**: `care_plan.status.done` added to all four `locales/care_plan.*.yaml`
(audit: every `care_plan.status.*` key used by a template now exists in all
four locales).

---

## Open items

### R4-7.7 — Compact medication shows tomorrow all day long (**Open**)

`/care_plan?view=compact&kind=medication` always lists the next days'
buttons. Required: show only what should be done **during the day** (the
table is empty in the evening), keep the buttons grouped — a dedicated
`tomorrow` bucket following morning/noon/evening — and show the *next*
entry only when the duration from now to it is shorter than the current
late one. Plan: §R4-7.7 of `docs/care-plan-round-7-fix-plan.md`.

### R4-7.8 — Button alignment in the medication table (**Open**)

Buttons must line up column-wise across rows (fixed slot-button width,
fixed bucket-label band). Plan: §R4-7.8.

### R4-7.9 — Hamburger shown while there is still room (**Open**)

`navbar-expand-lg` collapses the menu at <992px even when the bar has free
space. Plan: §R4-7.9 (review the breakpoint / item widths).

### R4-7.12 — Protocol trace repeats what the row already shows (**Open**)

The `{count} occurrence(s)` column adds nothing once the schedule and
content are visible; replace it with the edit/delete affordance.

### R4-7.13 — Per-animal exception to a global protocol (**Open**)

An animal-specific "skip this protocol" exception, toggled by a checkbox on
the protocol and listed as an exception on the animal. Needs an additive
table + engine rule (no destructive migration — see the session constraint).

### R4-7.11b — Link an activity to its logged item (**Open**)

An applied feeding/observation row should open the logged feeding (animal
feeding view) with `back=` routing back to the source.