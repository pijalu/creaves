# Care plan / treatment / dashboard UX — Round 2 bugs (archived)

**Date found**: 2026-09 (analysis round for the care-expert UX work)
**Closed**: 2026-10-01 — all three items fixed by
`docs/archive/2026-10-01-care-plan-ux-fix-plan-round2.md` (WP1–WP7 on
`feature/care-expert`), e2e-validated per plan §12.6 (S1–S9, agent-browser),
gates green. Archived from `bugs.md` per guideline rule 4.

---

## Treatment view in animal

1/ Treatment view on animal *must* be based on protocol/actions — currently it
seems to be based on obsolete treatment record
2/ The view still show the 3 time icons but should show the time series
(instead of time icons morning/noon/evening it should be the actual time label
- 3 per line - following a pseudo-group (morning/noon/evening)))

**Resolution**: WP1 (protocol-driven plan assembly, obsolete-record source
removed) + WP3 shared `_med_series` partial + WP6 animal-tab Today block:
real time labels, 3 buttons per line (morning/noon/evening buckets), protocol
sources only. E2E S4/S5/S7.

## Dashboard: medication view

- The table should use striped rows (.table-striped)
- The animal button should follow the same format as the other tables in the
  dashboard (only number)
- No need to have a number badge on the table
- the treatment per animal view should use a dedicated background (white or
  inverse stripped) to underline the group of treatments for that animal
- The treatment does not have to show a time if the time is part of the button
- the treatment does not need to have 2 lines:
  `Citramox L.A. (48H) — 0.06 ml IM` + `Traitement — Citramox L.A. (48H) · 12:00`
  => The subline is not needed
- The treatment view should open the view + popup
- The treatment should group same treatments together (same name/dose) and
  show the serie of time buttons - following the "Treatment view in animal"
  3 items per line todo.

**Resolution**: WP5 — striped table, year-number-only buttons + dove badge
(outtaken-today), count badge removed, white per-animal card, shared series
partial (no subline, time only in buttons), eye → animal deep-link with
Treatment tab + detail popup, outtaken row class. E2E S4/S5, S9 (outtaken).

## Care plan view

The plan should follow general style (dashboard/treatment view) for
medications => A clear list - grouped time buttons list for same treatments.

Filter approach is not working - selecting/unselection of filters do not show
correct details.

The view does not make sense: Some items are in the accordion - some are not
(the accordion should be used for grouping all elements).

Only 1 badge - e.g. currently, following the filter there can be "0 future"
followed by "12:00 (17)" - the second badge is the global count... meaningless
in filtered view.

Compact/detailed show different list of elements - unclear (late vs current
confusion, e.g. "En retard" at 07:54 for an 08:00 occurrence with an upcoming
action).

Feeding cards are unclear (`/care_plan?view=compact&kind=feeding`): shows
`B1 ACCUEIL … / ● 1904/26 / En retard` — no action possible, no clear
indication of what to do next.

**Resolution**: WP2 (single pipeline: engine → display split → ONE zone×kind
filter pass → FilterStats; every badge equals the visible list; unknown zone
redirect) + WP4 (kind tabs, zone dropdown with counts, sections accordion for
ALL groups, feeding cards with per-chip Apply + batch "Apply group (N)",
late-vs-current supersession semantics) + WP3 (same series component as
dashboard/animal tab). Late-record, outtaken-today and free undo covered by
WP1 + e2e S1/S9. Two e2e-found gaps fixed in `f11fb12` (zone toggle badge
parity; late-record reuses the apply modal for observation/weighing inputs).
