# Defect log — care presentation contract (round 2026-10)

Normative reference: [`docs/care-presentation-guideline.md`](../care-presentation-guideline.md).
A deviation from that contract is a defect; each entry below records the violated
guideline section, a measured observation on the local dev instance (admin session),
the file:line root cause, reproduction steps, and the test-matrix rows that gate the
fix. Test data prerequisites: [`docs/care-plan-test-data.md`](../care-plan-test-data.md).

Entry format follows the archived round-9 bug log
(`docs/archive/2026-10-05-care-plan-round-9-bugs.md`): ID / date / URL / severity /
observed (measured) / expected (guideline ref) / repro steps / root cause (file:line) /
mapped test IDs / status.

## Entries

| ID | Title | URL | Severity | Guideline § | Status |
|----|-------|-----|----------|-------------|--------|
| [D1](d1-medication-line-responsive-bucket-divider.md) | Medication line: no responsive stacking + bucket divider rendering defect | `/care_plan?kind=medication` | medium | §5, §6 | open |
| [D2](d2-care-rules-show-raw-json.md) | `/care_rules/{id}` returns raw JSON for browser GET | `/care_rules/{id}` | medium | — (UX routing) | open |
| [D3](d3-cleanup-format-divergent.md) | Cleanup format divergent: no tiers, no per-occurrence toggles, untranslated zone | `/care_plan?kind=cleanup` | high | §1, §2, §3 | open |
| [D4](d4-row-kind-toggle-parity.md) | Observation/care/weighing rows: no tier-coloured buttons, no immediate colour flip | `/care_plan?kind=observation` (care, weighing) | high | §2 | open |
| [D5](d5-animal-nav-plan-format-divergence.md) | Animal `#nav-plan` non-med rows use a third format | `/animals/{id}#nav-plan` | medium | §2, §3, §4.3 | open |
| [D6](d6-care-rules-list-no-sort-filter.md) | `/care_rules` list: no sort / filter / search / paging | `/care_rules` | low | — (admin UX) | open |
| [D7](d7-zone-show-missing-fields-i18n.md) | `/zones/{id}`: `RequiresCleanup` missing, hardcoded English labels | `/zones/{id}` | medium | — (i18n rule) | open |

## Test matrix (row IDs)

The regression sweep (Phase 8) executes this matrix. Each defect maps to the rows
that must pass for it to close; each row runs in **all four locales**
(en-US / fr / de / nl) against the seeded data checklist.

| Row | Surface | Check | Gates |
|-----|---------|-------|-------|
| TM-1 | `/care_plan?kind=medication` | Responsive ladder §5: at 1600/1280/1024/767 px — level 1 animal shrink, level 2 buttons under label, level 3 full stack; no horizontal overflow, no clipped button | D1 |
| TM-2 | `/care_plan?kind=medication` | Bucket captions §6: caption band between button rows of a stacked series; omitted when the series fits one row; never inline between two buttons | D1 |
| TM-3 | `/care_rules/{id}` (browser GET, `Accept: text/html`) | HTML detail page (name, kind badge, matcher, priority, active, validity, payload/schedule, edit/delete, back); `Accept: application/json` still returns JSON | D2 |
| TM-4 | `/care_plan?kind=cleanup` | Tier sections §1 (Late/Now/Later/Done/History, panel + count pill + collapse), per-occurrence toggles §2 (colour = state, immediate flip), zone name localized | D3 |
| TM-5 | `/care_plan?kind=observation` (+ care, weighing) | Toggle colour semantics §2: red/yellow/white/green by state, immediate in-place flip on click, undo restores current tier colour, glyph + `title` + `sr-only` retained | D4 |
| TM-6 | `/animals/{id}#nav-plan` | Shared `_plan_item_line` for non-med rows (§3): `ℹ`-led line, no animal cell (§4.3), tier-coloured toggles, missing-count pill + fulfillment links kept | D5 |
| TM-7 | `/care_rules` | Sortable columns (name/kind/priority/active, asc/desc), filters (kind, active, matcher), case-insensitive `q=` search on name+description, pagination; JSON branch unchanged (full set) | D6 |
| TM-8 | `/zones/{id}` | `RequiresCleanup` row rendered (bool2html); Type/Default/RequiresCleanup labels + values via `t()`; zero hardcoded English in fr/de/nl | D7 |
