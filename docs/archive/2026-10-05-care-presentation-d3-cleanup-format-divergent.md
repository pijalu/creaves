# D3 — Cleanup format divergent: no tier sections, no per-occurrence toggles, untranslated zone

**Date**: 2026-10 (review round). **URL**: `/care_plan?kind=cleanup`.
**Severity**: high (core daily workflow — cleanup is a first-class work kind
but renders in a legacy layout with none of the contract's affordances).
**Guideline**: §1 (tier sections), §2 (toggle-button colour semantics), §3
(one line per item — time grouping).

## Observed (measured)

`/care_plan?kind=cleanup` renders a bare bordered table
(`templates/care_plan/index.plush.html:510-539`): one `<tr class="plan-cage-row">`
per cage with cage name, zone, source link, and a single "Apply cage (N)"
outline-secondary clock button carrying a corner count pill.

Deviations from the contract:

1. **No tier sections (§1)**: cleanup occurrences are not grouped into
   Late/Now/Later/Done/History `.plan-tier` panels — there is one flat table
   under an `<h5>` heading, no urgency tint, no count pill per tier, no
   collapse. Feeding and medication on the same page are fully tiered.
2. **No per-occurrence toggles (§2/§3)**: each cage gets ONE apply button
   regardless of occurrence count/times; there are no per-occurrence toggle
   buttons whose colour carries state (red/yellow/white/green), no glyph, no
   per-time grouping — a partially-done cage is indistinguishable from an
   untouched one, and a single click applies every occurrence at once.
3. **Untranslated zone label**: the cage row shows the raw `ccard.Zone` base
   string with no `t()`/`tname` localization, leaking the canonical French
   zone name into non-French UIs (project all-language rule).

## Expected

- §1: cleanup grouped into the same tier panels as feeding/medication
  (Late → Now → Later → Done → History), count pills, collapse behaviour.
- §2/§3: one line per cage × cleanup with per-occurrence toggle buttons in
  due-time order; colour = state; immediate in-place flip on click.
- Zone name resolved through the translation mechanism like every other
  system record (R9-6 precedent).

## Reproduction steps

1. Seed per `docs/care-plan-test-data.md` record #3 (cleanup rule covering a
   multi-animal cage / `requires_cleanup` zone).
2. Open `/care_plan?kind=cleanup`: observe the flat table, single clock button
   per cage, no tiers, no per-occurrence toggles.
3. Compare with `/care_plan?kind=feeding` (tiered panels, per-animal applied
   states) and `/care_plan?kind=medication` (tier-coloured slot toggles).
4. Switch locale to en-US/de/nl: the zone name stays in the base language.

## Root cause

`templates/care_plan/index.plush.html:510-539` — the cleanup block is a
hand-rolled table ("fix 3" legacy layout) rendered from `view.Cares`, outside
the tier pipeline used by feeding/medication; the viewmodel's `CareView`
carries only cage-level aggregate data (no per-occurrence toggle/time-group
payload), and the zone string is emitted raw.

## Mapped test IDs

TM-4 (tier sections + per-occurrence toggles + localized zone) — all four
locales.

## Status

**Verified** — fixed in Phase 3 (cleanup parity with feeding/medication).

### Fix

- `actions/care_plan_viewmodel.go`: `CareView` extended with `TimeGroups
  []CareTimeGroup` (due-time sub-groups, one slot per occurrence, folding
  mirrors `foldChipsByTime`), `Tier`/`TierClass`/`GroupStatus`/
  `GroupStatusClass` (most-urgent-open rule, same as `feedingGroupTier`),
  `AnimalID` on `ItemSlotView` (per-occurrence apply key), and
  `careAnimalViewsOf` (group=animal rows). `fillCareTiers` distributes rows
  into `v.CareTiers` via the shared tier pipeline; `tierLinks` builds the
  cleanup strip pills from the same `CareTierOpenCap` counters (§1.4: strip
  == badge by construction). `Group` threaded through `DayPlanView` /
  `BuildDayPlanView` / `planSelfPath` / `defaultWorkKind`; `group` param
  parsed + whitelisted (`cage`|`animal`) in `actions/care_plan.go`.
- `templates/care_plan/_plan_care_line.plush.html` (new, ×4 locales):
  shared cleanup row — time sub-group label stated once per due time (+
  count pill), one `.plan-item-slot` apply/undo pair per occurrence
  (`○ HH:MM` tier-coloured `plan-apply-btn` / `✓ HH:MM` btn-success
  `plan-unapply-btn`, `data-tier-class` for undo restore), batch
  `plan-cage-apply` group check in cage mode only, zone via `tbase()`.
- `templates/care_plan/_plan_tier_care_table.plush.html` (new, ×4): tier
  loop partial; `index.plush.html` (×4) renders `care-tier-0/1/2`
  `.plan-tier` sections with header badges, plus the `.plan-group-toggle`
  Cage⇄Animal control (feeding + cleanup) and `&group=` persistence on
  every kind tab / zone link. Feeding table renders the animal cell +
  single-chip rows in animal mode (§4.2) with no batch check (§4.3).
- Locales: `care_plan.group.{label,cage,animal}` in en-us/fr/de/nl.

### Verification (2026-10-05, dev server + agent-browser)

- Tiered sections: `care-tier-0` `.plan-tier` with badge 22; summary strip
  pill `Late 22` (same unit, §1.4). Time sub-group `19:30` label once with
  count pill 11 + 11 per-occurrence toggles; second group `21:30` ×11.
- Immediate flip (§2.2): click `○ 19:30` btn-danger → in-place swap to
  `✓ 19:30` btn-success, no modal, no reload, open toggles 22→21; undo
  restores `btn-danger` via `data-tier-class`.
- Batch group check: `plan-cage-apply` → #planBatchModal → Confirm → all
  21 row toggles flip to green undo buttons.
- group=animal (cleanup): 11 `.plan-care-animal` cells (one line per
  animal), 22 per-occurrence toggles, zero batch buttons; Animal pill
  active. group=animal (feeding): 191 `.plan-feed-animal` cells, 191
  toggles, zero batch buttons. Kind tabs carry `&group=animal`; reload
  preserves the active mode.
- Locales: fr "Regrouper par / Cage / Animal", de "Gruppieren nach /
  Käfig / Tier", nl "Groeperen op / Kooi / Dier", en "Group by / Cage /
  Animal"; zone name via `tbase()` (D3 item 3).
- `go test ./actions -count=1` green (10 `TestPhase3*` tests pin 3-T1…
  3-T6: tiered render, time-label-once, toggle pair contract, late red,
  batch flip refs, group=animal lines + persistence, badge==strip unit).
- Console clean; temp verification rule/matcher/applications removed from
  the dev DB after the run.

---

## Phase 8 regression (2026-10-05) — ARCHIVED

Fix commit: `2099f6c` (Phase 3: cleanup parity — tiered sections, per-occurrence toggles, cage⇄animal grouping).
- `go test ./actions ./models` → all green (3-T1…3-T6 included).
- Sweep ×4 locales: `/care_plan?kind=cleanup` → HTTP 200 in all locales; console + page errors empty.
