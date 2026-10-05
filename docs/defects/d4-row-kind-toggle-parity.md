# D4 — Observation/care/weighing rows: no tier-coloured buttons, no immediate colour flip

**Date**: 2026-10 (review round). **URL**: `/care_plan?kind=observation` (also
`kind=care`, `kind=weighing`). **Severity**: high (state illegibility on three
daily work kinds — the caregiver cannot read late/now/later/done from the
buttons). **Guideline**: §2 (toggle-button colour semantics).

## Observed (measured)

Row-kind occurrences (observation, care, weighing) render apply buttons with a
fixed neutral style instead of the §2 tier→colour mapping: there is no
red/yellow/white-by-tier / green-when-done colouring (§2.1). On click,
`_apply_toggle`'s `markApplied`
(`templates/care_plan/_apply_toggle.plush.html:289-315`) only swaps visibility
— `btn.classList.add('d-none')`, `undo.classList.remove('d-none')`, plus the
R9-1 light-green row tint on `.plan-animal-row` (feeding only). The button
itself never flips to `btn-success` in place; the colour/state change appears
only after the next full auto-refresh re-render (§2.2 violation: "the colour
change MUST be immediate on click — the button flips in place, no page
reload"). Undo has no `data-tier-class` to restore the occurrence's *current*
tier colour (§2.4).

By contrast the medication slot toggle (`_plan_med_toggle`) flips
`○ HH:MM` btn-warning → `✓ HH:MM` btn-success immediately (R4-7.25, R9-2) —
row kinds lack that policy entirely: `slotTierClass` (Go) maps tiers for
medication only, and the row-kind templates hard-code their button classes.

## Expected

- §2.1: every row-kind occurrence toggle coloured by state — late red, due-now
  yellow, later white-with-border, done green.
- §2.2: immediate in-place flip on click; undo restores the colour the
  occurrence has *now*.
- §2.3: glyph + localized `title` + `sr-only` status retained alongside
  colour.
- §2.4: tier→class defined once server-side (`slotTierClass`); templates
  render the provided `TierClass`; original tier class rides on
  `data-tier-class` for client-side undo.

## Reproduction steps

1. Seed per `docs/care-plan-test-data.md` records #6 (observation 3×/day), #7
   (weighing), #8 (one late occurrence), #9 (one done occurrence).
2. Open `/care_plan?kind=observation`: the late occurrence's apply button is
   not red; the done one is not green — colours do not encode state.
3. Click apply on an open occurrence: the button hides and an undo appears,
   but no green in-place flip; state colour only changes after the next
   auto-refresh reload.
4. Compare with `/care_plan?kind=medication` where the slot button flips to
   green immediately.

## Root cause

`templates/care_plan/_apply_toggle.plush.html:289-315` — `markApplied`/
`markOpen` implement the toggle as a `d-none` visibility swap only, with no
colour class transition and no `data-tier-class` restore path; the row-kind
viewmodel (`CardView`) carries no `TierClass` from a shared server-side
policy, so templates pick fixed button classes.

## Mapped test IDs

TM-5 (tier-coloured toggles, immediate flip, undo restore, glyph/title/sr-only
retained) — all four locales, on observation + care + weighing.

## Status

**Verified** (2026-10-05, Phase 4 — row-kind toggle line parity).

### Fix

- `actions/care_plan_viewmodel.go`: new `ItemSlotView` (+ `itemSlotFor`)
  gives every row-kind occurrence the shared §2.4 policy — `Tier` from
  `tierOrder`, `TierClass` from `slotTierClass`, `LateAllowed`, `NeedsInput`.
  `tierRows` merges open current occurrences per (source × animal) into ONE
  card (`§3`): `CardView.Slots` holds every toggle, the line's tier/colour is
  its most urgent slot's. `CardView.TierClass` + `AnimalYear` added.
- `templates/care_plan/_plan_item_line.plush.html` (×4 locales): merged line
  renders one `.plan-item-slot` pair per occurrence — apply
  `btn <%= slot.TierClass %>` with `data-tier-class`, glyph `○ HH:MM`; undo
  `btn-success` with the same `data-tier-class`, glyph `✓ HH:MM`; late-record
  `– HH:MM` when `LateAllowed`. Single-occurrence fallback pair also renders
  `btn <%= card.TierClass %>` + `data-tier-class`. Animal cell shows the
  year number with the full label on `title` (§5 L1).
- `templates/care_plan/_apply_toggle.plush.html` (×4): `setToggleState`
  pair-row path now recolours the visible sibling in place — `btn-success`
  on apply, the `data-tier-class` classes restored on undo (§2.2/§2.4).
  Feeding/cleanup buttons carry no `data-tier-class` → untouched.
- `assets/css/care-plan.scss`: row-kind line walks the Phase-1 ladder —
  nowrap overrides removed, label wraps (no ellipsis), `@media
  (max-width: 767.98px)` full stack for `.plan-item-line`.

### Verification

1. `go test ./actions -run 'TestPhase4' -count=1` — 7/7 PASS
   (`care_plan_phase4_row_kind_parity_test.go`, maps TM-5):
   4-T1 late observation → `btn-danger` toggle on the late tier;
   4-T2 apply → sibling painted `btn-success` in place (static pin ×4 forks);
   4-T3 undo → `data-tier-class` restored (pair block scoped, ×4 forks);
   4-T4 3×/day protocol → ONE line, 3 due-ordered slots, `Remaining=2`,
   compact+detailed identical;
   4-T5 ladder scss pins + year-number animal cell ×4 forks;
   4-T6 weighing/observation keep the input modal (`data-kind`/`data-detail`
   on every slot, `isInputKind` dispatch before `instantApply`).
2. `go test ./actions -count=1` → `ok creaves/actions 12.790s` (full suite;
   Phase-0b DOM baselines re-recorded — observation delta = merged slots,
   tier classes, year-number cell, day badges; other kinds delta = the
   shared `_apply_toggle` JS only).
3. agent-browser e2e on dev server (http://127.0.0.1:3000, admin login):
   - `/care_plan?kind=observation` — late rows render `○ HH:MM` toggles
     `btn btn-danger … plan-item-slot-btn` with `data-tier-class="btn-danger"`;
     due-now `btn-warning`; animal cell `594/26` with
     `title="594/26 · West European Hedgehog · H45"`.
   - Click due-now toggle (`○ 17:00`, btn-warning) → `#planApplyModal`
     opens (input kind); answer submitted → in-place flip:
     apply hidden, undo visible `btn-success` `✓ 17:00`, no reload.
   - Undo → apply restored `btn-warning` `○ 17:00` in place.
   - Late observation (`Contrôle pattes`, btn-danger): apply → modal →
     green `✓`; undo → `btn-danger` `○ 12:00` restored.
   - `/care_plan?kind=weighing` — red late toggle `○ 10:00`; click → modal
     with visible weight field; weight submitted → green flip; undo →
     `btn-danger` restored.
   - Console: no JS errors (Live Reload logs only).
   - Temp fixture rules (`TEST-D4 — contrôle 3x/jour`, reactivated weighing
     rule) removed from the dev DB after the sweep; test care entries
     cleaned.
