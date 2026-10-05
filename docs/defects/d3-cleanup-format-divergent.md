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

**Open.** Fix owner: Phase 3 (cleanup parity with feeding/medication).
