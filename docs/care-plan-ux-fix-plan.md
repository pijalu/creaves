# Care Plan UX Fix Plan — detailed implementation plan

**Date**: 2026-09-28
**Scope**: `creaves/` only (no Console impact).
**Source issues**: `../../bugs.md` Part 2 (U1–U10).
**Design framework**: `skills/laws-of-ux/SKILL.md` (Laws of UX checklist).
**Constraint set**: 4 locales (`en-US/fr/de/nl`, template forks + yaml keys) · no cross-cage apply (§10.5-N2) · reuse `ReverifyItem` on apply paths · JSON API (`/care_plan` JSON shape) must stay backward-compatible (fields may be added, not removed).

---

## 1. Legacy UX inventory — the regression baseline

What the replaced screens gave caretakers (evidence from git `b9b6a20~1` and current `templates/`):

| # | Capability | Where (legacy) | Clicks to act |
|---|---|---|---|
| L1 | Zone tabs (nav-tabs), hash-remembered selection | `templates/feeding/index.plush.html` | 1 click to filter |
| L2 | One row per animal showing **next** feeding time only | same | 0 (visible) |
| L3 | Criticality row colors (danger=overdue / warning=soon / success/info=future) | same | 0 (glance) |
| L4 | Big primary "Feed" button → prefilled care form, `?back=` returns to the same zone tab | same | 1 click |
| L5 | "Close" (skip) warning button per row | same | 1 click |
| L6 | AM/PM clock dots for treatment slots (done=green, due=red/orange, none=−) | `templates/treatments/index.plush.html`, `templates/landing/index.plush.html:168-180` | 0 (glance) |
| L7 | Today row highlighted (`table-warning`) | `templates/treatments/index.plush.html` | 0 |
| L8 | Animal ID as primary button linking to animal page | feeding/treatments tables | 1 click |

**Regression rule**: any L1–L8 capability must exist in the new UI with **equal or fewer clicks** and equal or better glanceability. The E2E gate (Phase 8) walks this matrix explicitly, including a side-by-side run of the old version when useful.

## 2. Target information architecture

```
/care_plan                  WORK SCREEN (default) — "what do I do next?"
  ?view=compact|detailed    compact = next open occurrence per (source × animal) [default]
  ?zone=<name>              zone tab filter, default "all", badges = open counts
  ?kind=<kind>              kind chips, badges = open counts
  ?from=&to=                window override (unchanged); HTML default shrinks to
                            [yesterday 00:00, today 23:59] (overdue carry-over + today)
/reports/care_schedule      REPORT SCREEN (new, read-only) — "understand the schedule"
  ?group=zone|cage|animal   grouping switch (default zone)
  ?from=&to=                default [today, today+2d], hard cap unchanged (14 d)
```

Non-goals: animal Plan-tab day glimpse (spec §7.3.3, separate effort); admin rule-editor changes; engine/schedule semantics changes.

## 3. Phase breakdown

### Phase 1 (UX-1, issues U7/U8/U4) — Engine: next-open selector + work-window scoping

**Engine** (`models/careplan/next.go`, new):

```go
// NextOpenPerGroup returns, per (source_type, source_id, animal_id), the
// earliest non-terminal occurrence (status scheduled|due|late|missing),
// plus the count of remaining non-terminal occurrences of that group
// today. Terminal (applied/skipped/deferred) and overridden items are
// excluded from the primary list but counted separately.
type NextOpenItem struct {
    Item        PlanItem
    Remaining   int // other open occurrences of the same group in-window
    DoneToday   int // applied/skipped/deferred occurrences of the group
}
func NextOpenPerGroup(items []PlanItem) []NextOpenItem
```

- Sort order of result: status urgency (missing, late, due, scheduled) then DueAt, then animal id — deterministic, urgency-first (Selective Attention / Serial Position).
- Tests (`models/careplan/next_test.go`): mixed statuses per group pick earliest open; all-terminal group yields no item but counts; overridden excluded; defer-expired resurfaces (status already recomputed by `BuildPlanItems` — selector consumes statuses, no recomputation); determinism (two runs equal).

**Handler** (`actions/care_plan.go`):
- HTML default window when no `?from/to`: `from = today 00:00 − 24h`, `to = today 23:59:59` (today + yesterday's carry-over). JSON default unchanged (spec window) — HTML passes explicit window so behavior is per-request, no global change.
- New context values: `view` (compact|detailed, default compact), `zone`, `kind` (exists), computed view-model (Phase 2 consumes).

**Gate**: `go test ./models/careplan/ -run NextOpen -v` green; `go build ./...` green; existing `go test ./actions/ -run CarePlan` green.

### Phase 2 (UX-2, issue U2) — Work screen: urgency tiers, positioned badges, hour chips, auto-refresh

**View model** (`actions/care_plan_viewmodel.go`, new): `DayPlanView` built from `DayPlan` + params:

```go
type TierView struct { Key string; Open []CardView; Count int }  // late | now | later | done
type DayPlanView struct {
    Tiers      [4]TierView
    Zones      []ZoneTab   // name + open count
    Kinds      []KindChip  // kind + open count
    Hours      []HourChip  // distinct HH:00 of "later" items, with counts
    UpdatedAt  string      // HH:MM of render (auto-refresh indicator)
    View, Zone, Kind string
}
```

- Tier mapping: `missing|late` → **late**; `due` → **now**; `scheduled` → **later**; `applied|skipped|deferred` → **done** (collapsed by default); `overridden` → excluded from work screen entirely (visible on animal Plan tab only).
- Badge rendering: `min(count, 99)` displayed as `99+` when capped — helper `BadgeCap(n int) string` in actions (unit-tested).
- Hour chips: distinct hours of *later* items; clicking scrolls to `#h-<HH>` anchor inside the tier (no reload).

**Templates** (`templates/care_plan/index.plush.html` + 3 forks — forks are byte-identical copies today; keep them identical via copy):
- Bootstrap 4.6 structure: tier headers as `position-relative` buttons with `position-absolute` badges (`badge badge-pill badge-danger/warning/primary/success`), `data-toggle="collapse"` targets; late+now default `show`, later+done collapsed.
- Auto-refresh: `<script>setTimeout(function(){location.reload()},60000)</script>` + "updated at HH:MM" next to title (§10-CP6c).
- No full-page scroll trap: tiers collapse; anchors per hour.

**i18n**: new keys `care_plan.tier.late|now|later|done`, `care_plan.updated_at`, `care_plan.view.compact|detailed`, `care_plan.zone.all`, `care_plan.badge.more` — all 4 yamls.

**Gate**: `go build ./...` + `go test ./actions/ -run 'CarePlan|I18n'` green; grep asserts each fork contains `position-absolute` badge markup; locale key parity check (script or existing i18n test).

### Phase 3 (UX-3, issues U1/U5) — Grouping, per-kind content, meaningful labels

**Grouping** (`models/careplan/grouping.go`): add `GroupingCageDiet Grouping = "cage_diet"`; registry: `KindFeeding: GroupingCageDiet` (cleanup stays `GroupingCage`). Group key = `cage | normalized food text` where food comes from payload (`planPayload.Food` — **add `Food string \`json:"food"\`` and `ForceFeed bool** to `planPayload` in `care_plan_fulfillment.go`, it currently lacks them).

**Cards** (`actions/care_plan_dayplan.go`): extend `GroupCageCards` → generic `GroupCards(items, plan)` dispatching on grouping strategy:
- `GroupingCage` (cleanup): unchanged shape (source × cage).
- `GroupingCageDiet` (feeding): `FeedingCard{Cage, Zone, Food, ForceFeed, Items}`; per-animal chip = `{{yearNumber}}/{{YY}}` + status dot.
- Apply semantics unchanged: batch endpoint stays source×cage-scoped; a feeding card that spans multiple sources (rule + animal plans) applies in one client-side loop grouping item refs per source (JS posts N batch calls sequentially, then one reload). No cross-cage apply (§10.5-N2) preserved.

**Per-kind detail line** (`planItemJSON` + view model): `Detail` string per kind — feeding: food (+ force-feed icon); medication: `drug — dosage` (literal dosage; when `dosage_from_dosages_table`, show `⧗ auto` hint + last-weight tooltip when available — light display, full resolution stays at apply time §10-B6); care/cleanup/weighing: note; observation: prompt. `Detail` additive in JSON (backward compatible).

**Labels**: display name cleanup — strip the ` (conversion)` suffix and ` (à vérifier)` marker from *display* (kept in DB for rollback identification; strip in the view-model layer only, helper `DisplayName(name string) string`, unit-tested). Where a detail line exists (diet/drug), it renders **primary**, the plan name secondary muted.

**Overridden**: filtered out of the work screen (Phase 2 view model); count surfaced only in debug/detailed view.

**i18n**: `care_plan.card.apply_group`, `care_plan.card.animals`, `care_plan.feeding.force_feed`, `care_plan.medication.auto_dosage` ×4.

**Gate**: `go test ./models/careplan/ ./actions/ -run 'Grouping|Group|DisplayName|CarePlan'` green; build green.

### Phase 4 (UX-4, issues U3/U6) — Links, zone tabs, kind chips, fast actions

- **Links** (spec §7.2a): animal label → `/animals/{id}#nav-plan?back=/care_plan`; source name → `/care_rules/{id}?back=...` (rules) or `/animals/{id}#nav-plan` (animal plans); applied items (done tier) → fulfillment `/cares/{fid}` / `/treatments/{fid}` with `?back=`. Requires `FulfillmentType`/`FulfillmentID` on `ApplicationView` (engine struct extension, additive) + `loadApplications` mapping + `planItemJSON.fulfillment_*` fields (additive).
- **Zone tabs**: replicate legacy pattern (nav-tabs + `location.hash` memory, adapted from `b9b6a20~1` feeding JS — hash-prefixed `z-`); badge per tab = open-item count in that zone; "Toutes" tab first.
- **Kind chips**: one click `?kind=`, active state, open counts.
- **Big apply button**: `btn btn-success` (not `btn-sm`, not outline) inline on card — Fitts's Law. Row click target = whole card header for collapse.
- **Skip/Defer modal** (spec §7.3.1 buttons [⏭ Reporter] [🚫 Ignorer]): one shared Bootstrap modal; skip = reason only; defer = reason + hours-number input (default +1h §10-L3, client converts to RFC3339 now+Nh); posts existing `/care_plan/apply` JSON with `status` + `note` + `deferred_until`. Error toast via existing pattern.
- **Cage batch entry**: cage cards get `✅ Appliquer la cage (N)` → `POST /care_plan/apply_batch` with the card's item refs; result handling: reload on all-applied, alert listing failures otherwise.

**Gate**: `go test ./actions/` green; build green; fork parity re-checked.

### Phase 5 (UX-5, issues U9/U10) — `/reports/care_schedule` report view

- Route: `app.GET("/reports/care_schedule", ReportsCareScheduleIndex)` in the reports block of `app.go`; handler in `actions/reports_care_schedule.go` reusing `BuildDayPlan` (window params, cap enforced there); read-only.
- **Group switch** `?group=zone|cage|animal`:
  - `zone`: sections per zone (legacy convention `AnimalByZoneMap`), tables sorted by time; zone headers with day totals.
  - `cage`: one block per cage (round order: zone, cage), animal rows only where the animal's plan **diverges** from the cage-dominant group (same kind+detail at same time = grouped row "cage A12 · 3 animaux").
  - `animal`: full per-animal chronological trace.
- **Legacy visual language** (L1/L3/L6/L7): row classes by urgency (danger=late/missing, warning=due, info=scheduled today, success/light=applied), today column highlight, medication items rendered with the AM/noon/PM clock-dot SVGs (reuse `contentFor` blocks pattern from landing/treatments — copy SVG defs into the new template).
- Nav: add `<a class="dropdown-item" href="/reports/care_schedule">` next to the other reports entries in `templates/application.plush.html` (+ fr/de/nl forks), key `nav.reports_care_schedule` ×4 locales.
- i18n keys `care_plan.report.*` ×4.

**Gate**: build + `go test ./actions/ -run Reports` green; template render smoke via handler test (HTML 200, contains group markers) ×1 locale + i18n key parity.

### Phase 6 — Parity & suites gate

- Fork parity: `index.plush.html` == fr/de/nl forks (byte-compare; they carry no hardcoded strings) and reports template forks identical; application.plush forks each contain the new nav item.
- Locale parity: every new key present in all 4 `locales/care_plan.*.yaml` + nav key in the shared locale files; run existing i18n test harness (`care_plan_i18n_html_test.go` pattern).
- `go build ./...`, `go test ./models/...`, `GO_ENV=test go test ./actions/...` all green.

### Phase 7 — E2E validation (mandatory, agent-browser)

Rig:
1. DB: restore `../creaves-db-2026-09-28.gz` into the dev DB (or scratch DB + `database.yml` override), `buffalo pop migrate up`, boot `buffalo dev` (converter idempotent — already-marked DB stays converted).
2. Old version side-by-side (optional but recommended for the regression matrix): `git worktree add /tmp/creaves-legacy b9b6a20~1`, run on port 3002 against a copy of the dump, compare `/feeding` + `/treatments` flows.
3. **agent-browser** walkthrough (skill `.agents/skills/agent-browser/SKILL.md`):
   - login admin/admin; open `/care_plan`:
     - tiers render collapsed/open as designed; badges capped `99+`;
     - compact default: one card per (source × animal), `+n` badges;
     - zone tabs filter + badge counts; kind chips filter;
     - apply one feeding card → 201 → card flips to done tier after reload; cage card batch-apply → N applied;
     - skip/defer modal: mandatory reason enforced (422 without), defer resurfaces;
     - links: animal label → animal page; applied item → care record; back-param returns;
     - auto-refresh indicator present; no console errors.
   - `/reports/care_schedule`: group=zone|cage|animal render; legacy colors/dots visible; read-only (no apply buttons).
   - Locale sweep: switch language ×4 on both pages (no missing-key `T()` artifacts).
   - **Regression matrix check** (Section 1 L1–L8): each capability verified equal-or-better click count.
4. Update `bugs.md` Part 2: statuses U1–U10 → FIXED with commit refs + validation evidence.

**Gate**: checklist above fully checked; `go test ./...` green; bugs.md updated.

## 4. Risk register

| Risk | Mitigation |
|---|---|
| Template forks drift (fr/de/nl copies) | Byte-identical copies enforced by Phase 6 diff gate |
| Feeding card spanning multiple sources breaks batch constraint | Client loops per-source batches; server constraint untouched (§10.1-5) |
| Compact view hides a needed occurrence | Detailed toggle one click away; `+n` badge signals hidden items |
| Auto-refresh interrupts modal input | Reload suppressed while any modal is open (`$(...).is(':visible')` guard) |
| `(conversion)` strip hides rollback marker | Display-layer only; DB names untouched |
| Actions tests need MySQL | localhost:3306 confirmed up; `GO_ENV=test go test ./actions/...` is the gate |
| `migrations/schema.sql` has unrelated local modification | Do not touch; never commit it accidentally (per-file commits) |

## 5. Effort estimate

| Phase | Content | Size |
|---|---|---|
| 1 | engine selector + window scoping + tests | 0.5–1 d |
| 2 | tiers/badges/refresh templates + i18n | 1 d |
| 3 | grouping + detail + labels | 1 d |
| 4 | links/zones/actions | 1 d |
| 5 | report view + nav | 1 d |
| 6 | parity + suites | 0.5 d |
| 7 | E2E + bugs.md close-out | 0.5–1 d |
