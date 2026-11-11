# Care-plan UX round-2 review regressions (#1–#7) — fixed 2026-10-02

User review of the round-2 care-plan rework (commits 228b337..caad579)
reported seven UI regressions. All fixed, tested and e2e-validated on the
dev instance (real production data, animal 8635).

| # | Reported | Root cause | Fix (commit) |
|---|----------|-----------|--------------|
| 1 | Menu bar gets cut instead of collapsing to a hamburger | `navbar-expand-lg` keeps 7 entries inline down to 992px — wider than the bar | e97ec4b: `navbar-expand-xl` + media rule forcing `.nav-label` visible below 1200px (×4 locales) |
| 2 | late/now/later without clear separation/order | round-2 replaced tier tables by per-source cards | a2143d1: tier tables restored in fixed order late (open, `table-danger`) → now (open, `table-warning`) → later (closed); summary badges anchor `#tier-*`; done/history collapsed table |
| 3 | Cards put too much on screen; follow the original list UI | card grids per kind | a2143d1: tier work = original table rows; feeding/cleanup = grouped list sections (`plan-feeding-row`/`plan-cage-row`), never cards |
| 4 | `/care_plan?kind=feeding` showed a collapsible medication | cards were kind-agnostic; sections driven by source type | a2143d1: feeding/cleanup are groupedKind → only their own list sections; medication is tier rows only |
| 5 | TOUT kind tab made the screen unstable | TOUT mixed all kinds incl. collapsibles | a2143d1: tabs iterate `view.Kinds` only (no TOUT); HTML handler resolves default work kind (first with work, feeding fallback); unknown-zone redirect keeps a kind |
| 6 | Medication full of duplicated same-hour buttons | one card+button per occurrence | a2143d1: compact view folds per source group (Remaining/RemainingCap), one row/one apply button; detailed view = one row per occurrence |
| 7 | Animal treatment tab: new block on top of the old, incomplete (no observation), repeated animal label | dedicated Today block (`animalTodayBlock` + `_med_series`) stacked above the accordion | 3b4adbd: Today card/atMed modals/script removed; `animalPlanTodayRows` merges today's medication+observation+care occurrences INTO the accordion's current-date card as original-look rows (clock badges, protocol backlink); dedupe vs legacy treatments of the same day (drug/prompt == treatment.Drug); no animal label |

## Validation

- `go build ./...`, `go vet`, `staticcheck` clean (no new offenders;
  gocognit/gocyclo gates hold for all new code).
- `go test -count=1 ./...` green; `-race ./actions/` green.
- Templates byte-identical across en/fr/de/nl (care_plan) / fork-synced
  (animals show, application) and rendered in all four locales by the
  i18n HTML gates.
- E2E (agent-browser, dev instance, 2026-10-02):
  - `/animals/8635#nav-treatment`: no Today block, no plan-med-slot
    buttons, no animal label; current card 2026/10/02 open with exactly
    "Citramox L.A. (48H) (0.06 ml IM)" + "Nettoyage Fistule (Dessus oeil
    droit)", warning clock badges.
  - `/care_plan?kind=feeding&view=compact`: 164 list rows, 0 cards, no
    medication content, kind tabs without TOUT.
  - `/care_plan?kind=medication&view=compact`: late open/danger (2 rows),
    now open, later closed (39 rows); 41/41 rows exactly one apply
    button; day-aware labels.
  - Navbar at 1000px: hamburger visible, labels shown, nothing cut.
