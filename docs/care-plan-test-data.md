# Care-plan test data checklist

Seeded-data prerequisites for the care-plan test matrix (guideline §8).
Target: the dev database (`buffalo task db:seed` baseline + the records below,
created through the UI or the care-rule/plan editors so they exercise the
real write paths).

## Required records

| # | Record | Why the matrix needs it |
|---|--------|-------------------------|
| 1 | One cage (e.g. `VI-01`) holding **≥3 in-care animals** | feeding/cleanup cage-level grouping, collapse behaviour, per-animal vs group checks |
| 2 | One **feeding** protocol on that cage, **2×/day** (e.g. 08:00 + 16:00) | time sub-group labels, cage grouping, group=animal toggle |
| 3 | One **cleanup** rule covering that cage (or a `requires_cleanup` zone) | D3 parity: tiers, per-occurrence toggles |
| 4 | One **medication** protocol **3×/day** (e.g. 08:00 / 13:00 / 18:00) on one animal | bucket-boundary crossing (morning/noon/evening captions), multi-slot series stacking |
| 5 | One medication protocol with a **dosage** requirement | dosage dialog on apply (422 dosage_required path) |
| 6 | One **observation** protocol, repeating **3×/day** on one animal | D4: merged line with multiple toggles, answer modal |
| 7 | One **weighing** protocol on one animal | weight-input modal path |
| 8 | One occurrence **in the past, unapplied, still applicable** (late tier) | red button + red row, late-record confirm |
| 9 | One occurrence **already applied today** (done) | green button, Done tier, undo path |
| 10 | A second **zone** with at least one in-care animal | zone filter dropdown, zone counts |

## State checks before a run

- [ ] `SELECT COUNT(*) FROM animals WHERE status='in_care'` ≥ 5
- [ ] Feeding protocol (2) produced occurrences today (`/care_plan?kind=feeding`)
- [ ] Medication protocol (4) shows 3 slots today (`/care_plan?kind=medication`)
- [ ] Cleanup occurrences visible (`/care_plan?kind=cleanup`)
- [ ] Observation occurrences visible (`/care_plan?kind=observation`)
- [ ] At least one red (late) and one green (done) toggle on the plan

## Refreshing state

Late/done states are time-dependent. Before a regression sweep:
1. apply one open occurrence (done data point);
2. leave one past-due occurrence unapplied (late data point);
3. re-run the affected matrix rows.
