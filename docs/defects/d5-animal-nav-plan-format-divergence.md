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

**Verified — 2026-10-05** (fix owner: Phase 5, animal page `#nav-plan` format parity).

### Fix

- `templates/animals/show.plush.html` (x4): the ad-hoc 93-line occurrence
  block replaced by the shared `_plan_item_line` partial, called with
  `{card: item, showAnimal: false, showKind: true, feedingEntry: true}` —
  no animal cell (§4.3), kind badge kept, feeding rows keep the prefilled
  ration modal (R4-2.2). Missing-count pill preserved on the day's last row.
- `templates/care_plan/_plan_item_line.plush.html` (x4, byte-identical):
  new `feedingEntry` param (feeding toggles route to the apply modal);
  terminal single rows render — applied → green done badge doubling as the
  fulfillment-record link (R4-7.11b) + ⏱ late mark, skipped/deferred →
  status badge, out-of-window open → 🔒 lock (§10-A1).
- `actions/care_plan_animal_page.go`: `animalDayCardFor` fills the full
  CardView contract (AnimalYear/AnimalLink/NeedsInput/LateAllowed/Undoable/
  RecordedLate); `mergeDayItems` groups open occurrences per (source ×
  animal) into ONE line with due-time-ordered `Slots`, rep re-tiered to the
  most urgent slot; `OpenCount` counts occurrences per §1.4.

### Verification (2026-10-05)

- `go test ./actions -count=1` → ok (17.4s). Six new pins
  (`care_plan_phase5_animal_plan_parity_test.go`): shared-line delegation
  without animal cell x4 locales, medication block unchanged, shared apply
  implementation, `mergeDayItems` repeat merging + re-tiering, out-of-window
  lock + missing pill kept, R8-2 `?src=` deep-link JS intact. Six round-4/7
  pins re-pointed to the component's new home (fulfillment link, status
  keys, todo glyph, apply toggle) — not weakened.
- agent-browser on dev server, `/animals/10350#nav-plan`: 20 `.plan-item-line`
  rows, none with `.plan-med-animal`, kind badges present; feeding rule
  renders ONE line with `○ 09:00` + `○ 18:00`; cleanup toggle
  `btn-danger ○ 09:00` → click → `btn-success ✓ 09:00` in place, no reload,
  no modal; undo restores `btn-danger ○`; feeding toggle opens
  `#planApplyModal` (prefilled path); `?src=<id>` opens `#planDetails` and
  highlights the trace row; console clean. Evidence:
  `tmp/browser_evidence/phase5/summary.json` + screenshot. Temp data
  created during the flip test was removed; dev DB restored.
