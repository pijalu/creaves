# D1 — Medication line: no responsive stacking + bucket divider rendering defect

**Date**: 2026-10 (review round). **URL**: `/care_plan?kind=medication`.
**Severity**: medium (readability + overflow on narrow viewports; stray captions
inline between buttons). **Guideline**: §5 (responsive fall-back ladder), §6
(bucket captions).

## Observed (measured)

1. **No stacking ladder**: the global compact single-line rules
   (`assets/css/care-plan.scss:319-365`) force `flex-wrap: nowrap` on
   `div.plan-med-row.d-flex` / `.plan-med-row.plan-item-line` (l.326),
   `.plan-med-row .plan-med-line` (l.331) and `.plan-med-btns` (l.334), and
   `.plan-med-label` carries `white-space: nowrap; overflow: hidden;
   text-overflow: ellipsis` (l.348-351). On the care-plan page there is no
   dashboard-style `.dash-med-cell` override (R9-3 fixed `/dashboard/` only,
   scoped to `.dash-med-cell`), so at narrow widths the day-plan medication
   rows cannot re-flow: the label ellipsizes (§5 violation — R4-7.24 "never
   cut a description") and the button group pushes past the cell instead of
   dropping under the label (§5 level 2 / level 3 never engage).
2. **Bucket divider rendering defect**: the R4-2.4 labelled bucket divider
   (`.plan-med-bucket`, `t("care_plan.slot.*")`) renders as an inline band
   *between two buttons of the same row* when a multi-bucket series still fits
   one line — §6.2 requires the caption between button rows only, omitted when
   the series fits one row (the slot `title` already carries the bucket name).

## Expected

- Guideline §5 ladder, in order: level 1 shrink the animal cell to
  `YearNumberFormatted` (full label in `title`/`ℹ` modal); level 2 buttons
  wrap under the item label on their own full-width row; level 3 full stack.
  Never overflow horizontally, never clip a button, no `text-overflow:
  ellipsis` on content.
- Guideline §6: captions separate *button rows* of one stacked series; omitted
  when the series fits one row.

## Reproduction steps

1. Seed per `docs/care-plan-test-data.md` record #4 (medication 3×/day
   crossing morning/noon/evening buckets).
2. Open `/care_plan?kind=medication` at 1600 px: a 3-bucket series shows a
   bucket caption band inline between buttons of one row (defect 2).
3. Narrow the viewport to 1024 px then 767 px (or shrink the browser window):
   the drug label ellipsizes and/or the button group overflows horizontally
   instead of stacking under the label (defect 1). Compare with `/dashboard/`
   at the same widths where the R9-3 `.dash-med-cell` override stacks
   correctly.

## Root cause

`assets/css/care-plan.scss:319-365` — the R9 "COMPACT SINGLE-LINE" block
applies global `flex-wrap: nowrap` / label `ellipsis` to every `.plan-med-row`
with no ladder levels and no care-plan-scoped wrap override (R9-3 scoped its
fix to `.dash-med-cell` only). The divider gate in
`templates/care_plan/_med_series.plush.html` (`row.DividerBefore &&
!medSeriesCompact`) has no "series fits one row" condition, so the caption
renders inline.

## Mapped test IDs

TM-1 (responsive ladder at 1600/1280/1024/767 px), TM-2 (bucket captions
between rows only) — all four locales.

## Status

**Verified.** Fixed in Phase 1 (medication line responsive ladder), commit
`5f4242b` (feature/care-expert).

### Fix

1. **Level 1** (`templates/care_plan/_plan_med_row.plush.html` ×4): the animal
   cell renders `mg.AnimalYear` (`YearNumberFormatted`, e.g. `1267/26`) with
   the full label in the cell `title`; the ℹ detail modal keeps
   `data-animal-label`. `MedGroupView.AnimalYear` was already populated by
   `buildMedGroups` (`actions/care_plan_viewmodel.go`).
2. **Level 2** (`assets/css/care-plan.scss`): the R9 global
   `flex-wrap: nowrap` on `.plan-med-row .plan-med-line` and the GLOBAL
   `.plan-med-btns { flex-wrap: nowrap }` (old scss:333) are gone; the R4
   base wrap rules apply again, so a group that no longer fits beside the
   label wraps under it, right-aligned. The row-kind lines keep their
   compact single-line treatment scoped to `.plan-item-line`. The bucket-fit
   JS adds `.plan-med-btns--stacked` (full-width row) to a group whose
   buttons actually wrapped.
3. **Level 3** (`@media (max-width: 767.98px)`): full stack — animal number
   on its own line, label `white-space: normal` (no ellipsis, R4-7.24),
   buttons wrap left-aligned.
4. **Bucket divider (§6.2)**: `.plan-med-bucket` keeps `flex: 1 0 100%` (a
   caption forces its own row — it can NEVER be inline between two buttons);
   the bucket-fit JS in `_apply_toggle.plush.html` (shared component, all
   locales) hides the captions of a group that fits one row and shows them
   once the buttons stack. No-JS fallback: captions visible on their own
   rows (safe default).
5. **Undo tier class**: already Phase-0b — `setToggleState(false)` restores
   the button's own `data-tier-class`; pinned by `TestPhase1UndoRestoresTierClass`.

### Verification (2026-10-05, admin session, dev server)

- `go test ./actions -count=1` → **ok 13.8s**, incl. the new
  `actions/care_plan_phase1_ladder_test.go` (1-T1..1-T7: ladder CSS pins,
  animal-year cell, viewmodel feed, bucket gate + JS wiring, undo tier
  restore, x4 fork parity, dashboard override intact). Phase-0b DOM
  baselines re-recorded (PHASE0B_RECORD=1); pre-record diff inspected — only
  the sanctioned deltas (animal cell year/number + title, fitMedBuckets JS
  block; zero removals on feeding/cleanup/observation).
- `npm run build` → webpack compiled (digest `application.b48ed1d7….css`).
- agent-browser probes on `/care_plan?kind=medication` (evidence:
  `tmp/d1_evidence/`):
  - **1600 / 1024 / 767 px**: `hOverflow=0`, 0 clipped buttons (30 slots),
    level1=true on all 25 rows (cell text `NNNN/26`, full label in title);
    no raw i18n key leaks.
  - **Level 3 at 767 px**: animal cell full-width own line, labels
    `white-space: normal`, `text-overflow: clip`, none clipped; wrapped
    groups left-aligned.
  - **Level 2** (probe-injected 18 buttons + 9 buckets in one group):
    `plan-med-btns--stacked` engaged, group full-width (837 px = line
    width), 9/9 captions visible, each a full-width band (837 px), none
    inline between buttons, group under the label, `hOverflow=0`.
  - **§6.2 at 1600/1024**: 5 bucket captions in DOM, 0 visible when the
    series fits one row (slot titles carry the bucket name).
  - **Click → undo**: `○ 12:00 btn-danger plan-med-apply` → click →
    immediate `✓ 12:00 btn-success plan-med-unapply` (no reload) → undo →
    `○ 12:00 btn-danger plan-med-apply` (original tier class restored from
    `data-tier-class`).
  - **Dashboard regression**: 17 `.dash-med-cell` groups keep
    `flex-wrap: wrap` + `flex-basis: 100%`, 0 captions (compact), 0 clipped
    buttons, `hOverflow=0`.
  - **Locale sweep** fr/de/nl: `hOverflow=0`, level1=true, no key leaks;
    stacked captions localized (`Avond` under nl).
- Residual scope note: row-kind lines (`.plan-item-line`) keep the compact
  single-line treatment — their ladder belongs to D3/D4.
