# Defect log — care presentation contract (round 2026-10) — CLOSED

Normative reference: [`docs/care-presentation-guideline.md`](../care-presentation-guideline.md).

**All entries D1–D7 fixed, verified, and archived 2026-10-05 (Phase 8
regression sweep).** The archived entries, with fix commit refs and per-phase +
Phase 8 verification evidence, live in `docs/archive/`:

| ID | Title | Fix commit | Archived entry |
|----|-------|-----------|----------------|
| D1 | Medication line: responsive stacking + bucket divider | `5f4242b` (+`da3701f`) | [2026-10-05-care-presentation-d1-medication-line-responsive-bucket-divider.md](../archive/2026-10-05-care-presentation-d1-medication-line-responsive-bucket-divider.md) |
| D2 | `/care_rules/{id}` raw JSON for browser GET | `2f51d6e` | [2026-10-05-care-presentation-d2-care-rules-show-raw-json.md](../archive/2026-10-05-care-presentation-d2-care-rules-show-raw-json.md) |
| D3 | Cleanup format divergent (tiers, toggles, zone i18n) | `2099f6c` | [2026-10-05-care-presentation-d3-cleanup-format-divergent.md](../archive/2026-10-05-care-presentation-d3-cleanup-format-divergent.md) |
| D4 | Observation/care/weighing toggle parity | `a08e4b2` | [2026-10-05-care-presentation-d4-row-kind-toggle-parity.md](../archive/2026-10-05-care-presentation-d4-row-kind-toggle-parity.md) |
| D5 | Animal `#nav-plan` non-med format divergence | `b65fe35` | [2026-10-05-care-presentation-d5-animal-nav-plan-format-divergence.md](../archive/2026-10-05-care-presentation-d5-animal-nav-plan-format-divergence.md) |
| D6 | `/care_rules` list sort/filter/search/paging | `1da9e7e` | [2026-10-05-care-presentation-d6-care-rules-list-no-sort-filter.md](../archive/2026-10-05-care-presentation-d6-care-rules-list-no-sort-filter.md) |
| D7 | `/zones/{id}` RequiresCleanup + i18n | `aab947c` | [2026-10-05-care-presentation-d7-zone-show-missing-fields-i18n.md](../archive/2026-10-05-care-presentation-d7-zone-show-missing-fields-i18n.md) |

Phase 8 sweep evidence (2026-10-05, admin session on dev):

- `go test ./actions ./models` → `ok creaves/actions 17.157s`, `ok creaves/models 0.581s`.
- `npm run build` → webpack 5.111.1 compiled; digests `application.5e6078ac981991b8c195.js`,
  `application.3923f4c5795d07e387e8.css`, `care-plan.31d6cfe0d16ae931b73c.js`;
  `public/assets/manifest.json` updated.
- Browser sweep (agent-browser, authenticated): every defect URL
  (`/care_plan?kind=medication|cleanup|observation|feeding`, `/animals/2058#nav-plan`,
  `/care_rules`, `/care_rules/02aae3db-…`, `/zones/4eafb532-…`, `/dashboard`)
  × fr/en-US/de/nl → **36/36 HTTP 200, zero console errors, zero page errors**.
- R4-4.1 auto-refresh floor e2e: apply at T+54s → page alive at T+64…85s polls,
  reload at T+85–90s (≥30s after last action). In-place toggle flip confirmed.
- Dashboard medication cell (R9-3 `.dash-med-cell`) unchanged: computed styles
  `flex-wrap:wrap` / label `white-space:normal` / btn group `flex-basis:100%`;
  in-place apply from dashboard works, no reload, no console errors.

The test matrix rows TM-1…TM-8 (formerly below) are covered by the per-phase
test pins (1-T1…7-T2) referenced inside each archived entry.

New defects against the care presentation contract should open fresh entries in
this directory following the format documented in
`docs/archive/2026-10-05-care-plan-round-9-bugs.md`.
