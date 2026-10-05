# D5 — Animal `#nav-plan` non-medication rows use a third, divergent format

**Date**: 2026-10 (review round). **URL**: `/animals/{id}#nav-plan`.
**Severity**: medium (same work, third look — caregivers must re-learn state
reading between the day plan and the animal page). **Guideline**: §2 (toggle
colour semantics), §3 (one line per item — time grouping), §4.3 (animal
identity omitted on the animal's own page).

## Observed (measured)

On the animal page Plan tab, medication days reuse the day-plan `_med_series`
partial, but every other kind renders through an ad-hoc loop
(`templates/animals/show.plush.html:823-915`): each occurrence is its own
`d-flex … border-top` row carrying a `badge-secondary` kind pill, free-text
detail/source spans, a `dueLabel`, and a **status pill** (`badge-success`
done / `badge-light` skipped/deferred / sr-only late) **plus** a separate
apply/undo control.

Deviations from the contract:

1. **Status by pill, not by toggle colour (§2)**: state is expressed by a
   textual badge next to a neutrally-styled button; there are no
   tier-coloured toggle buttons (red/yellow/white/green), so the §2 colour
   language learned on the day plan does not transfer.
2. **One row per occurrence (§3)**: repeated occurrences of the same item are
   not merged into one line with toggles in due-time order; each repeat gets
   its own row (no `ℹ`-led fixed leading column either — §3.3).
3. **Format divergence**: the day plan renders row kinds with
   `_plan_item_line`-style lines; the animal page hand-rolls a different
   markup block for the same data — three formats (med series, day-plan row
   line, animal-page row) for one contract.

(What the block does right and must keep: the single missing-count pill on the
last row of the day, and the done-badge → fulfillment-record link with
`back=` return — R4-7.2 / R4-7.11b.)

## Expected

- Non-medication rows on `#nav-plan` render through the shared
  `_plan_item_line` partial (guideline §7) with `showAnimal=false` — the
  animal cell is omitted because identity is the page context (§4.3); the
  kind badge stays.
- Tier-coloured toggles per §2 (immediate in-place flip), one merged line per
  item per §3.
- Missing-count pill and fulfillment links preserved.

## Reproduction steps

1. Open an animal with a repeating observation/care/feeding protocol
   (`docs/care-plan-test-data.md` record #6) → `/animals/{id}#nav-plan`.
2. Observe one row per occurrence with status pills + neutral apply controls;
   compare with `/care_plan?kind=observation` (after D4 fix: merged line,
   tier-coloured toggles) — same work, different presentation.
3. Click apply: no tier-colour flip; state shown only via the badge swap.

## Root cause

`templates/animals/show.plush.html:823-915` — the Plan tab's `day.Items` loop
is hand-rolled markup duplicating (divergently) what the shared
`_plan_item_line` partial renders on the day plan; only medication days route
through the shared `_med_series` partial.

## Mapped test IDs

TM-6 (shared `_plan_item_line`, no animal cell, tier-coloured toggles,
missing-count pill + fulfillment links kept) — all four locales.

## Status

**Open.** Fix owner: Phase 5 (animal page `#nav-plan` format parity).
