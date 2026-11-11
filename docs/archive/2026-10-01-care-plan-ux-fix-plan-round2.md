# Care Plan / Treatment / Dashboard — Round 2 UX Fix Plan

**Date:** 2026-09-30
**Source:** [`../bugs.md`](../bugs.md) — sections "Treatment view in animal", "Dashboard: medication view", "Care plan view"
**Baseline:** [care-plan-ux-fix-plan.md](./care-plan-ux-fix-plan.md) (round 1, implemented) — this plan is the follow-up on the issues found after round 1 shipped.
**Scope:** analysis + design + fix plan only. No implementation until this plan is agreed.
**Workspace note:** all work in `creaves/` only. `creaves/AGENTS.md` is the source of truth for conventions.

---

## 1. Constraints (binding)

- **Production data**: schema changes additive only, no destructive migrations. Current design needs **zero migrations**.
- **All-language UI rule**: every template change applies to the 4 template forks
  (`templates/care_plan/index.plush.{html,fr.html,de.html,nl.html}`,
  `templates/dashboard/dashboard.plush.{html,fr.html,de.html,nl.html}`,
  `templates/animals/show.plush.{html,fr.html,de.html,nl.html}`) plus new keys in
  `locales/care_plan.{en-US,fr,de,nl}.yaml`. `TestCarePlanPagesAllLocales` is the gate — extend it to cover new surfaces.
- **bugs.md workflow**: plan → execute → validate → archive each closed bug to `docs/archive/`; e2e via agent-browser skill; quality tools run **separately** (`go vet ./...`, `staticcheck ./...`, `gocognit -over 15 .`, `gocyclo -over 12 .`, `go test -count=1 -race -cover ./...`); one commit per fix.
- **Laws of UX** ([skill](../../.agents/skills/laws-of-ux/SKILL.md)): every design decision below cites the law(s) that dictate it.
- **Additive principle (user, binding):** the new engine must **add** features, not remove any. Every screen must *at least* fill the same functional need, in a similar fashion, as the previous approach (`feature/depupdate` views + current round-1 engine), *with* better features/UX. Any element this plan drops must either (a) have its function preserved by a documented replacement (audit table §4b) or (b) be an explicit user decision. No silent capability loss.

## 2. Confirmed design decisions (from user)

| # | Decision |
|---|----------|
| D1 | Dashboard "view + popup": clicking a treatment navigates to the animal's Treatment tab **and auto-opens the detail modal** there. |
| D2 | "Late" semantics: an item is late **unless there is a soon-upcoming care** of the same source for the same animal. A feeding not done in the evening still shows as late for some time (grace) **before being replaced by the next one**; grace derives from the feeding cadence (number of feedings / time gaps). A more rational approach is allowed if proposed. |
| D3 | Compact vs detailed: **unify** — same grouped sections (medications / feedings / cares) in both modes; detailed only shows more info per card. |
| D4 | Navigation: **kind = primary tabs**, zone = dropdown selector with counts. |
| D5 | Animal Treatment tab: protocol-driven **Today** block (time-label buttons) + manual "Add treatment" stays + past history below. |
| D6 | Counters (badges) always **filtered to the active zone/kind** — counts must match the visible list. |

## 3. Housekeeping found during analysis

- `creaves/bugs.md` still carries the title *"Type/Species Suggestions & Validation — Bug Follow-up"* from an earlier doc. Rename to *"Care plan / treatment / dashboard UX — Round 2 bugs"* in the first commit.
- Round-1 plan predates the current `buildMedGroups`/`BuildDashboardMedView` split; this plan supersedes its open items.

## 4. Critical review vs Laws of UX (requested)

Findings where the current implementation contradicts the skill's rules. Each finding drives a fix in §6–§9.

| # | Finding (current state) | Law violated | Consequence |
|---|---|---|---|
| R1 | Badges computed **before** zone/kind filter (`openCounters` called pre-filter, viewmodel ~line 294); tier counts filtered but hour chips unfiltered → "0 future" next to "12:00 (17)" | **Cognitive Load** (one badge per state), screen checklist "counts truthful" | Users stop trusting every number on the page |
| R2 | Two mental models on one page: 4-tier accordion **and** Meds/Feedings/Cares sections outside it (compact only); detailed mode shows a different list | **Mental Model**, **Prägnanz**, **Jakob's Law** (old `/feeding` was one model: next feeding per animal) | "The view does not make sense" (bugs.md) |
| R3 | Time-only labels (`15:04`) on occurrences that may be **yesterday's** (plan window includes yesterday) → "En retard 08:00" at 07:54 today | **Cognitive Bias / Working Memory** (asks user to recall which day), **Paradox of the Active User** | Late vs current confusion; caretaker distrusts "late" |
| R4 | Non-actionable rows rendered as primary work items: feeding chip keeps the earliest **open** occurrence even after `Applicable=false` (superseded by the next one) → dead-end card "En retard, no action possible" | **Hick's Law** (backlog shown as next action), **Postel's Law** (dead end), checklist "every row actionable" | Wasted attention; unclear what to do next |
| R5 | Kind chip "Tous" mislabeled with key `care_plan.zone.all` | **Law of Similarity** (same key family = same meaning) | Wrong translations surface if keys diverge |
| R6 | Dashboard medication table deviates from sibling tables: no `table-striped`, full `AnimalLabel` on the button instead of year-number, plus a count badge | **Jakob's Law**, **Law of Similarity** | Extra parsing cost on the most-scanned screen |
| R7 | Two-line treatment cells repeating the drug name + a time that already sits in the button | **Cognitive Load** ("labels that say the thing" — once), **Prägnanz** | Vertical noise, fewer rows per viewport |
| R8 | Animal Treatment tab lists record-shaped history first (per-treatment rows with clock icons) — the caretaker's question "what do I give now?" is answered only indirectly | **Pareto Principle** (80% case first), **Tesler's Law** (schedule complexity pushed onto the user) | Today's protocol actions not visible at a glance |

## 4b. Additive-only audit (binding user rule)

Every capability of the previous approach (`feature/depupdate` views **and** the round-1 engine as shipped) that this plan removes, narrows, or replaces — with the mapping that keeps the functional need filled. Verified against `git show feature/depupdate:…` sources and current code (2026-09-30).

| # | Capability in previous approach | Evidence | Plan's disposition | Need still met by |
|---|---|---|---|---|
| A1 | **Mark a past/late slot done after the fact** — any past day, any user, one click (XOR toggle) | depupdate `templates/animals/show.plush.html` ~493–540 (`btn-schedule`, `past="true"` buttons stay clickable, `btn-danger` = missed) + `actions/treatments.go:52` (no window check) | **Restored, in scope** — §6.2-2 `late` flag on `CarePlanApply` (bypasses only the `!Applicable` check; past-due + unapplied + in-window only) | History rows + dimmed series buttons get one-click "record anyway (late)"; skip keeps reason. Default endpoint behavior unchanged (409) — explicit acknowledgment, not a silent rule change |
| A2 | **Outtaken animals with remaining today-work stay visible + toggleable** on the dashboard (row class `outtaken`, ready-for-release dove badge on the button) | depupdate `dashboard.plush.html` ~119–126 + `actions/dashboard.go:67-77` (SQL has **no** outtake filter) | **Restored, in scope** — new bug **Dash-9**: `loadAnimalContexts` gains a dashboard/plan scope including animals outtaken **today** (`outtake.date >= today`, join + preload `Outtake`); viewmodel flag `OuttakenToday`; depupdate row class + dove badge parity | §8.1 Dash-9; bounded to today-outtaken (depupdate's SQL was "remaining today-treatments" semantics — same-day work, not old outtakes) |
| A3 | **Jump-to-hour navigation** — round-1 `HourChip` anchors into the "later" tier | `care_plan_viewmodel.go` ~82 + template ~240 | **Replaced** — deleted (no depupdate equivalent existed), function re-homed | Summary-strip counters become **anchor links** (late / à faire / plus tard → scroll to first card of that state) + urgency-ordered sections + time label on every series button. Jump = one click, same as before, to a better target |
| A4 | **Urgency triage tiers** (late / today / later / history accordion, round-1) | §5-CP3 root cause (compact/detailed fork) | **Replaced** — tier *structure* goes (it was the CP3 bug), tier *function* stays | Summary strip (filtered counts, now anchors — A3) + urgency-first sort inside every section + history section last (Serial Position). Same scan order: red first, history last |
| A5 | **"Not required" placeholder** — disabled minus button per time slot ("Not required on the morning" tooltip) | depupdate `show.plush.html` ~484–489 | **Info preserved by absence** — a time series shows only scheduled times: no 12:00 button = nothing due at 12:00 (self-evident, no click target ever existed on the minus) | §8.1; bucket dividers render only when the bucket has buttons — no empty regions, no ambiguity |
| A6 | **Per-row remaining-count badge** (round-1 dashboard) | bugs.md **Dash-3 explicitly asks for its removal** | **Removed per bug** — not a capability loss | Open buttons are the count (visual); section header keeps one total. Counting was the bug's redundancy complaint |
| A7 | **Full animal label on the dashboard button** (round-1) | bugs.md **Dash-2 explicitly asks** for sibling-parity year-number | **Removed per bug** — information kept | Species + Cage are table columns (depupdate parity) — label content is still one glance away, button text matches every sibling table |
| A8 | **Free un-toggle by any user** — depupdate XOR toggle also *undid* records, any user, any slot | depupdate `actions/treatments.go:67` (`Timedonebitmap ^= key`, no role check, audited) | **Restored (user decision, this round)** — `CarePlanUnapply` drops the round-1 non-admin gate: any authenticated user may undo any record (applied/skipped/deferred, any kind). The 403 branch (care_plan.go:272–288) is deleted wholesale; `DeleteFulfillment` semantics and 404-on-missing idempotency stay; every undo becomes audit-logged (depupdate pattern) | Depupdate parity for the toggle-undo. The audit entry replaces the round-1 gate as the safety net — see §6.3 |
| A9 | Early apply (mark a slot done before its time) | engine `itemApplicable` = `now < nextDue` (status.go:180-189) | **Already allowed** — no change | Unchanged engine rule; applies to both current and late paths |

**Net:** no capability from depupdate or round-1 is lost. Three restorations enter scope (A1 late-record, A2 outtaken-today, A8 undo-by-any-user), two replacements are documented (A3/A4), two removals are bug-mandated with information preserved (A5–A7). The plan is now strictly additive against both previous approaches, minus exactly what bugs.md asked removed.
## 5. Root-cause map (bug → cause → code)

| Bug (bugs.md) | Root cause | Code location |
|---|---|---|
| **T1** Treatment view based on "obsolete treatment record" | View renders `animal.Treatments` rows (record-centric: one row per drug per day, legacy `Timebitmap`) instead of the plan/protocol occurrences for today; entries created by plan applications are surfaced but the *actionable* list is not built from the care plan | `templates/animals/show.plush.html` ~420–593; data from `actions/care_plan_animal_page.go` + `models/treatment.go` |
| **T2** 3 clock icons instead of time series | Badge per entry bucketed to morning/noon/evening (`entryClockSVG`), actual time not shown; no grouping by (drug, dosage), no 3-per-line chunking | `actions/care_plan_animal_page.go` (`entryClockSVG`), template badge loop |
| **Dash-1** no striped rows | Table class missing `table-striped` | `templates/dashboard/dashboard.plush.html` ~86 |
| **Dash-2** animal button format | Button renders `mg.AnimalLabel` (full "472/26 · Hérisson · A12") while sibling tables render year-number only | `templates/dashboard/dashboard.plush.html` ~92; label built in `animalLabel` (`actions/care_plan_service.go` ~373) |
| **Dash-3** count badge | `dash-med-count` badge "N to give" rendered per row (redundant with visible buttons) | `templates/dashboard/dashboard.plush.html` ~90 |
| **Dash-4** group background | Treatments cell is a plain list; no bounded region per animal | `templates/dashboard/dashboard.plush.html` ~100–140 |
| **Dash-5/6** time duplication + 2-line cell | Slot line renders `<strong>Detail</strong><br><small>source · DueAtHM</small>` although the apply button already carries `DueAtHM` | `templates/dashboard/dashboard.plush.html` ~105–125 |
| **Dash-7** view + popup | Eye button navigates to `slot.ViewLink` only; no deep-link to open the detail modal | `MedSlotView.ViewLink`; `actions/care_plan_viewmodel.go` |
| **Dash-8** no series grouping | `buildMedGroups` groups per animal with one line **per occurrence**; no (drug, dosage) series merge | `actions/care_plan_viewmodel.go` `buildMedGroups` |
| **CP1** med list style | Meds section renders per-slot lines; same as Dash-8 (no series grouping); also differs from dashboard build | `templates/care_plan/index.plush.html` ~369–438 |
| **CP2** filter not working | `CarePlanIndex` narrows `kind` **before** `BuildDayPlanView`, but zone filtering happens later on viewmodel sections — tier cards vs sections filter differently; invalid `zone` values silently pass | `actions/care_plan.go` `CarePlanIndex`; `care_plan_viewmodel.go` filter stage |
| **CP3** accordion inconsistency | Meds/Feedings/Cares sections are built **only in the compact branch** and rendered outside `#planTiers`; detailed mode keeps the tier accordion → two different lists | `actions/care_plan_viewmodel.go` ~233+ (compact/detailed fork) |
| **CP4** badges meaningless | `openCounters` called **pre-filter** → hour chips/tier badges show global counts in filtered view | `actions/care_plan_viewmodel.go` ~294–297 |
| **CP5** "late" at 07:54 / compact vs detailed | (a) plan window includes yesterday (`planWindowPastDays=1`) but `DueAtHM` is time-only → yesterday's 08:00 occurrence displayed as "08:00" late. (b) Compact chip picks earliest **open** occurrence regardless of `Applicable`; once the next occurrence is due the chip is a dead end. (c) Same fork as CP3 | `models/careplan/status.go` (`ComputeStatus`, `itemApplicable`); `care_plan_dayplan.go` chip selection; `Format("15:04")` labels |
| **CP6** feeding cards dead-end | `FeedingChip` keeps earliest open occurrence with `Applicable=false` → card shows "En retard" with no possible action and no hint of the next due time | `actions/care_plan_dayplan.go` `GroupCards`/chip dedupe |

> **Plan-discovered (not in bugs.md):** **Dash-9** — outtaken-today animals invisible on dashboard/care plan (`loadAnimalContextsScoped` filters `outtake_id IS NULL`, care_plan_service.go:149) while depupdate kept them visible + toggleable (`outtaken` row class; depupdate `actions/dashboard.go:67-77` SQL has no outtake filter). Root cause + fix: §4b-A2, §6.3, §8.1.
## 6. Design — "late" vs "current" semantics (D2, fixes CP5/CP6 core)

### 6.1 Current behavior (engine)

- `ComputeStatus` (`models/careplan/status.go`): `late` = now > due+grace; `missing` = now > due+miss; `due` = due−now ≤ lookahead. Windows come from the source schedule (`grace_minutes`, `miss_after_hours`) with defaults 60 min / 24 h / 60 min.
- `itemApplicable` (§10-A1): an occurrence stays **applicable** until the next occurrence of the same (source × animal) becomes due (`nextDue`); the last one in the window until `windowEnd`.
- The plan window includes yesterday (`planWindowPastDays=1`) — so at 07:54 **today**, yesterday's 08:00 feeding is still `late` **and** `applicable` (next due at 08:00 today). Correct per engine, unreadable per UI (time-only label).

### 6.2 Target display rule — "supersession"

Keep the engine as is (statuses stay truthful). Add **one derived display layer rule** in the viewmodel (`actions/care_plan_dayplan.go` / `care_plan_viewmodel.go`), no persistence, no migration:

1. **Work-set rule (Hick's Law / Prägnanz):** an open (no application) occurrence is shown as an *actionable work item* only while it is **current**: `applicable == true` **and** its successor is not imminent — successor due-time minus lookahead > now. Once the successor is due (or imminent within `lookahead`), the occurrence leaves the work set.
2. **Superseded state:** an open occurrence that left the work set gets display status `superseded` (badge: subdued gray, tooltip "replaced by <next due label>" / "hors fenêtre"). It renders **only** in the collapsed history section, never in work tiers, chips, or counters. It stays **recordable** there (additive principle, §4b-A1): the history row and the dimmed series button offer a one-click "record anyway (late)" action — depupdate allowed marking any past slot done after the fact (`btn-schedule` on `dateKey.Past` rows, depupdate `actions/treatments.go:52`, no window restriction) and the new engine must keep filling that need. Mechanism: `CarePlanApply` gains an explicit `late` acknowledgment flag that bypasses **only** the `!Applicable` check (care_plan.go:201), and only for items that are past-due, unapplied, and inside the plan window — future occurrences still 409. Skip-with-reason keeps the same late path. Default behavior without the flag is unchanged (409 "hors délai") — Postel's Law: the bypass is an explicit user acknowledgment (confirm step + distinct button label), never a silent rule change.
3. **Chip/group selection:** per (animal × source) the chip is the **earliest current** occurrence — one next action per group, matching the old `/feeding` view (Jakob's Law). The dead-end case disappears by construction: a non-applicable occurrence can never be the chip again.
4. **Grace derivation (user's rule):** there is **one** mechanism only — rule 1. The time an occurrence stays "late but current" is therefore bounded by the cadence, not by a fixed 24 h miss window. Evening case: a 19:00 feeding missed overnight stays `late` and current until the 07:00 feeding becomes imminent (~06:00 with lookahead), then flips to `superseded` in history and the morning chip takes over — exactly "stays late for some time before being replaced by the next one", grace derived from the feeding times. `grace_minutes` keeps its existing role (due → late), `miss_after_hours` keeps its role (history display: late → missing).
5. **Date-aware labels (fixes "En retard 08:00" at 07:54):** `DueLabel` replaces bare `DueAtHM` everywhere an occurrence of another day can surface: `hier 08:00`, `08:00`, `demain 08:00` (localized via new keys `care_plan.time.yesterday` / `.tomorrow`; multi-day gaps show the short date `02/10`). Time-only is kept only when the occurrence is today.
6. **Late semantics overall:** `late` (red) = current + past due+grace → *act now*. `due` (amber) = current + imminent. `superseded`/`missing` (gray, history) = replaced or lapsed — never red, never in the work count (Selective Attention, Von Restorff: red is reserved for act-now).

### 6.3 Where it lands in code

- `models/careplan/status.go`: export `NextDue time.Time` on `PlanItem` (the nextDue map already exists inside `BuildPlanItems`; additive field, JSON-safe). No status changes.
- New `actions/care_plan_display.go` (small, testable): `IsCurrent(item, now, lookahead) bool`, `SupersededReason(item) string`, `DueLabel(item, now) string`.
- `care_plan_dayplan.go`: chip dedupe switches from "earliest open" to "earliest current"; history entries carry `SupersededBy string` (next due label) for their tooltip (chips themselves are always current, so the field belongs to the history view structs, e.g. a `HistoryRowView`).
- `care_plan_viewmodel.go`: tier/section assignment routes `superseded` items to the history collection; work counters count only current items.
- `actions/care_plan.go` (`CarePlanApply`, :164): add `Late bool` to `planApplyRequest`. When `Late` is set, the `!item.Applicable` 409 branch (:201) is replaced by a bounded check — `item.Occurrence.DueAt.After(now)` still 409s (no pre-recording the future), terminal statuses still 409 (idempotency), overridden still 409. The recorded application is a normal `applied`/`skipped` row: `AppliedAt = now`, `DueAt = occurrence due` — "recorded late" is derivable (`applied_at > due+grace`), no schema change.
- `actions/care_plan.go` (`CarePlanUnapply`, :259): delete the `!u.Admin` gate (:272–288) — §4b-A8, user decision to widen to depupdate parity. Any authenticated user may undo any record (any kind, any status); `UnapplyPlanItem` (care_plan_support.go:135), its `DeleteFulfillment` flag and the 404-on-missing path are untouched — with no privileged subset left, the 403 non-leak concern (and its comment) goes with the branch. Every undo writes a best-effort `auditAnimalChange` entry (actions/animal_audit.go:19, projection = the deleted application row) — depupdate audited its toggle the same way (depupdate treatments.go:82); the widened permission keeps that trail. Refresh the stale "§10-CP1 admin-only" doc comments on both functions in the same commit.
- `actions/care_plan_service.go` (`loadAnimalContextsScoped`, :147): add an `includeOuttakenToday bool` variant used by the dashboard and care-plan assemblies — query becomes `outtake_id IS NULL OR (outtake_id IS NOT NULL AND outtake.date >= <today 00:00>)` via left join + preload `Outtake`; viewmodel carries `OuttakenToday` on animal rows (from `pa.rows[id].Outtake`). `ReverifyItem` keeps the strict in-care scope (an outtaken-today animal's occurrence stays recordable only via the late path — same rule as any other past item).

**Laws:** Hick's (next action, not backlog), Zeigarnik (badge counts only remaining *current* work — reaches 0, gives closure), Jakob's (old feeding-view behavior), Postel's (no dead ends), Von Restorff (red = act-now only), Cognitive Bias/Working Memory (date-aware labels).

### 6.4 Edge cases to cover with tests

- Single daily occurrence (no successor in window): stays late until `windowEnd`, then superseded-lapsed — history entry (reason "hors fenêtre"), still late-recordable via the §6.2-2 path while it is inside the plan window.
- Successor imminent (due in ≤ lookahead): current flips to successor even if predecessor unapplied.
- Deferred item whose defer expired (§10-H1) and whose successor is already due → superseded, reason "replaced".
- Skipped/applied items: never superseded (terminal statuses win).
- Animal released / plan ended mid-day: windowEnd handles (existing §10-A1 rule).
- Late record accepted: `Late=true` on a superseded/missing, unapplied, past-due item inside the window → 201, application row written (A1).
- Late record rejected: `Late=true` on a future-due item → 409; `Late=true` on an already-recorded item → 409 (idempotent); **no flag** on an out-of-window item → 409 exactly as today (default unchanged).
- Outtaken-today animal: appears in dashboard/care-plan assemblies with `OuttakenToday` flag; its past occurrences recordable via the late path; an animal outtaken before today never enters the assemblies (A2 bound).
- Undo widening (A8): a non-admin undoes a skipped feeding, a deferred care and an applied medication → all succeed (200, application row removed, audit entry written); unknown ref → 404 for every user — exactly the admin behavior today; the 403 branch no longer exists.

> **Dashboard window note:** the dashboard builds from `TodayPlanWindow` — a yesterday-lapsed item is visible only in the care plan's history, never on the dashboard. Accepted: the dashboard answers "what now", the plan answers "what happened".

## 7. Design — Care plan view IA (D3, D4 — fixes CP1..CP4)

### 7.1 Information architecture

```
/care_plan?kind=<k>&zone=<z>&view=<compact|detailed>
┌──────────────────────────────────────────────────────────────┐
│ Summary strip:  [3 en retard] [5 à faire] [12 plus tard]  · maj 14:02 │   ← counts = FILTERED (D6)
├──────────────────────────────────────────────────────────────┤
│ Kind tabs (primary): Tous | Nourrissage | Médicaments | Soins | Nettoyage | Pesée | Obs │
│ Zone dropdown:      [Toutes les zones ▾ (24)]   ← per-zone counts for the ACTIVE kind │
├──────────────────────────────────────────────────────────────┤
│ ACCORDION — one collapsible card per GROUP, all items inside │   ← CP3: nothing outside the accordion
│  ▸ Médicaments  (7)      → per-animal cards, drug series      │
│  ▸ Nourrissage (12)      → per-cage cards, next-feeding chips │
│  ▸ Soins        (3)      → per-cage cards                     │
│  ▸ Nettoyage    (2)      → per-cage cards, batch apply        │
│  ▸ Pesée / Observation   → generic rows                       │
│  ▾ Historique   (9)      → applied/skipped/deferred/superseded/missing, subdued │
└──────────────────────────────────────────────────────────────┘
```

- **D4:** kind = primary nav-tabs (they answer "what kind of round am I doing?"); zone = dropdown with per-zone counts. Default: kind=Tous, zone=all. The old zone nav-tabs and the kind chips merge into this single header (Occam's Razor: one nav, not two). Dropdown = plain Bootstrap dropdown (no Select2 — Occam's Razor: no new dependency for ~5 zones).
- **Summary strip = anchors (A3/A4):** the three counters are links (`#plan-first-late`, `#plan-first-todo`, `#plan-first-later`) scrolling to the first card in that state — the jump-navigation the deleted hour chips provided, re-homed to urgency targets (one click, better target). Each targeted card gets the matching `id` in the same template pass.
- **CP3:** the 4-tier accordion is **removed as primary structure**. The accordion groups *content* sections (kind groups + history). Every rendered item lives inside an accordion card.
- **Hour chips removed:** `HourChip` (jump anchors into the old "later" tier, viewmodel ~82, template ~240) has no target once tiers are gone — time context lives in the time buttons and in urgency-ordered sections. Delete the struct, the template block, and `hourCount` (Occam's Razor; also removes the unfiltered-counter surface from CP4). Function mapping per §4b-A3: jump-navigation is preserved by the anchor-linked summary strip above.
- **CP1:** medication section inside the accordion = per-animal card, each drug (same name+dosage) on **one line** with a **series of time buttons** (`08:00 12:00 16:00` — one button per occurrence, 3 per row, pseudo-grouped morning/noon/evening with a thin divider), identical component to the dashboard and the animal Treatment tab (Jakob's / Similarity: one visual language across the 3 screens). Button state: outlined = open, filled ✓ = applied, ⏸ = deferred, ⊘ = skipped, dimmed = superseded (not in Today's series unless detailed). **Dimmed buttons stay clickable** — one click = "record anyway (late)" with a confirm (§6.2-2, §4b-A1): the depupdate ability to mark a missed slot done after the fact, kept.
- **CP5/D3 compact vs detailed:** ONE build path. Both render the same sections/groups; the difference is per-card density:
  - *compact*: one line per group (drug series collapsed to the next open time button + "+2" counter; feeding chip = next feeding only); tier colors as a left border on the card.
  - *detailed*: full series (all time buttons incl. superseded, subdued), plus source link, protocol link, window/grace info, remarks — the info currently in the ℹ modal.
- **CP4/D6:** every count (summary strip, kind tab, zone dropdown item, section header, "+N") is computed **after** the zone×kind filter from the same filtered item set (single `FilterStats` pass). Impossible states like "0 future + 12:00 (17)" disappear by construction.

### 7.2 Filtering pipeline (CP2)

Single, ordered, testable pipeline in `BuildDayPlanView`:

1. `BuildDayPlan(window)` → all items (engine, unchanged)
2. display layer → `current` / `superseded` / history split (§6)
3. **filter by active kind and zone** (zone whitelist-validated against DB zones in `CarePlanIndex`; unknown `zone` → redirect to `?zone=` with flash)
4. group by kind → sections → groups (animal/cage) → items sorted urgency-first
5. `FilterStats` from the final set + zone×kind count matrix for the nav badges

Removing `CarePlanIndex`'s pre-narrowing of `kind` (root cause CP2) — the pipeline above narrows once, in one place.

### 7.3 Auto-refresh guard

Keep the 60 s auto-reload (Doherty) but skip it while a modal is open or an apply is in flight (Flow / Doherty: no interruption, no reload storm). Verify current JS, add guard if missing.

**Laws:** Hick's & Choice Overload (kind tabs + zone pre-filter), Common Region (accordion cards), Miller (≤ ~7 actionable items per card before chunking), Serial Position (late group first, history last), Peak-End (history collapsed, "0 restant" closure), Zeigarnik (truthful remaining badge), Similarity (same badge/button language as dashboard + animal tab), Parkinson (default = today's current work only).
## 8. Design — Dashboard medication table (D1 — fixes Dash-1..9)

`buildMedGroups(plan, zone, todayOnly)` already shared between dashboard and care plan — extend it once, both screens improve (DRY, Similarity).

**depupdate parity:** the target is the `feature/depupdate` "Animals with treatment today" table, same shape — `table-hover table-bordered table-striped`, columns `Number | Cage | Species | Treatments`, year-number animal button deep-linking to the treatment tab, one row per drug (name+dosage) with toggle buttons on the right, one-click XHR toggle on the button itself, remarks inline when present. The extension (bugs.md): the 3 fixed clock-icon slots (morning/noon/evening) become the **actual time series** — hour-labeled buttons per occurrence, pseudo-grouped by the same three buckets.

### 8.1 Table structure

```
│ Number │ Cage │ Species        │ Treatments (white card per animal)                      │
│ 472/26 │ A12  │ Hérisson (♀)   │ ┌────────────────────────────────────────────────┐   │
│  [btn] │      │                │ │ Citramox L.A. (48H) — 0.06 ml IM  [08:00]✓ [12:00] [16:00] │   │
│        │      │                │ │ ─────── divider (noon) ───────                  │   │
│        │      │                │ │ Baytril — 0.2 ml PO               [09:00] [⊕] │   │
│        │      │                │ └────────────────────────────────────────────────┘   │
```

The `Number | Cage | Species | Treatments` columns already exist in the current markup (dashboard ~89–93) — they stay exactly as in depupdate and the sibling tables.

- **Dash-1:** `table-striped` added (matches every sibling table + depupdate).
- **Dash-2:** animal button = `YearNumberFormatted()` year-number only, same component as the sibling tables (`#nav-*` deep-link family); species column keeps `tspecies` + gender like depupdate. The full `AnimalLabel` ("472/26 · Hérisson · A12") stops being duplicated inside the button.
- **Dash-3:** per-row `dash-med-count` badge removed. Remaining count is visible as open buttons; the section header keeps one total.
- **Dash-4:** per-animal treatment block = `bg-white border rounded px-2 py-1` card **inside** the striped row → Common Region: white card = "this animal's treatments", stripe = animal boundary (inverse striping effect, as suggested).
- **Dash-5/6:** one line per **drug series** (same name+dosage merged): `Citramox L.A. (48H) — 0.06 ml IM` followed directly by its time buttons. No subline, no duplicated time — time lives only in the button. Remarks (when present) stay visible as a small line/tooltip under the drug label — depupdate parity.
- **Dash-8:** series = hour-labeled toggle buttons (**the clock icons become hour labels** — user's extension), 3 per row, pseudo-grouped morning/noon/evening (thin divider when the bucket changes); bucket boundaries stay `morning <11h, noon <15h, evening` — same as `medSlotOf`/`entryClockSVG` today (one convention everywhere). The button itself remains the one-click apply/undo toggle (depupdate `btn-schedule` XHR pattern, kept via the existing plan apply endpoint).
- Button states = same state language as care plan and depupdate semantics: outlined/warning = open, ✓ success = applied, ⏸ deferred, ⊘ skipped, dimmed = superseded (click = late-record with confirm, §6.2-2 — travels with the shared `_med_series` component, so the dashboard keeps depupdate's "fix a missed slot in one click" too). Absent occurrences render no button — a time series shows only scheduled times, so "no 12:00 button" *is* the "not required at noon" information the depupdate minus placeholder carried (§4b-A5); bucket dividers render only when the bucket has buttons.
- **Dash-9 (additive, §4b-A2):** animals outtaken **today** stay in the table — `<tr class="outtaken">` row class (depupdate parity: subdued style, work still visible) and the ready-for-release dove badge next to the year-number button (`<span class="badge badge-success">fa-dove</span>`, title "Ready for release", depupdate dashboard ~127). Scope comes from the `includeOuttakenToday` assembly (§6.3): today-outtaken only, older outtakes never enter — same-day semantics as depupdate's "remaining today-treatments" SQL.

### 8.2 View + popup (D1)

- Eye button URL: `/animals/{id}?item=<source_type>:<source_id>&due=<RFC3339>#nav-treatment` — hash selects the tab (existing sibling-table `#nav-*` convention, depupdate used `#nav-treatment`), query params identify the occurrence and open the modal. Eye is icon-only → `aria-label` = drug + due label (accessibility, zero-cost).
- `AnimalsResource.Show` (actions/animals.go:549) reads `item`/`due`, **resolves the item server-side** (reuse the detail viewmodel builder behind the ℹ modal) and passes it + `OpenDetail: true` to the template; tab activation comes from the hash (existing JS). The shared partial renders server-side in the page with the modal's `show` class applied on load — no fetch, no extra round-trip, deep-link works from bookmarks and from the dashboard eye.
- The detail modal markup moves to a shared partial `templates/care_plan/_item_detail_modal.plush.html` (+3 locale forks) used by care plan, dashboard link target, and animal tab (Occam's Razor: one modal, not three copies).

### 8.3 Viewmodel changes

- `buildMedGroups`: merge `Slots` into **drug series**: `MedSeriesView{Key, Label(drug—dosage), Slots chunked by bucket}` inside `MedGroupView`. Dashboard and care plan consume the same struct.
- `BuildDashboardMedView`: unchanged signature (already `buildMedGroups(todayOnly=true)`), gains the series + deep-link URLs.

**Laws:** Jakob's + Similarity (table conventions of sibling tables), Common Region (white card per animal on striped row), Proximity (drug + its time buttons on one line), Cognitive Load (no duplicated name/time), Fitts's (time button = the action target, inline), Pareto (dashboard stays a scan surface; detail one click away).

## 9. Design — Feeding cards actionable (CP6)

Fixes the dead-end card `B1 ACCUEIL graines… // ● 1904/26 En retard`:

- Chip = **earliest current** occurrence (§6.2) → by construction actionable; a superseded occurrence can no longer be the displayed chip.
- Every animal line on the card carries its **one-tap Apply** button (Fitts's) next to the chip — the same `plan-med-slot`-style button component as medications (Similarity).
- Card header gets a **batch apply** ("Tout donner") using the existing `ChipRefsJSON` pattern from `CareView` — one big button per cage card instead of N small ones (Fitts's, Goal-Gradient: cage progress visible via remaining count on the card).
- If an animal's next feeding is later today, the chip shows it with its time (`12:30`) instead of a red yesterday — the card answers "what do I do next" for every animal in the cage (Hick's: next action, not backlog).
- Food-text truncation: full diet stays in the ℹ modal (detail), card shows the normalized food label already computed by `GroupCards` (Cognitive Load: labels carry content).

## 10. Design — Animal Treatment tab (D5 — fixes T1/T2)

### 10.1 Structure (top → bottom)

```
┌─ Treatment tab ─────────────────────────────────────────────┐
│ TODAY — protocol-driven (plan items, kind=medication+care)   │
│  472/26 · Hérisson · A12                                      │
│   Citramox L.A. (48H) — 0.06 ml IM   [08:00] [12:00] [16:00]│
│   Soin de plaie                       [09:30]               │
│   ▸ +3 autres animaux… (collapsed)                            │
│ [＋ Ajouter un traitement]  (manual entry — stays, D5)        │
├──────────────────────────────────────────────────────────────┤
│ HISTORY — date accordion (existing) from treatment entries    │
│  ▸ 30/09 — 3 traitements, 7 entrées (source links kept)       │
└──────────────────────────────────────────────────────────────┘
```

- **T1 ("based on protocol/actions, not obsolete record"):** the **Today** block is built from the **care plan** (`TodayPlanWindow` items for this animal), not from `animal.Treatments` rows. The record list below stays as *history*: `TreatmentEntriesMap()` rows with protocol backlinks (R5-3/R5-4 links preserved). Legacy `Timebitmap` remains only as the fallback badge when a record has no entries — unchanged behavior, clearly below the fold.
- **T2 (time series):** same drug-series + time-buttons component as dashboard/care plan (3 per row, pseudo-grouped morning/noon/evening, actual time labels — **no more generic clock icons**; the icon may stay inside the button as decoration if desired, but the label carries the time). One-tap apply reuses the plan apply endpoint; applied state refreshes in place.
- Manual "Add treatment" opens the existing create flow; the created entry appears in history (and, if it matches a plan occurrence via the R5-4 application link, marks it applied).
- Deep-link `?tab=treatment&item=…&due=…` (§8.2) opens this tab with the item's detail modal shown.

### 10.2 Code

- `actions/care_plan_animal_page.go`: build `TodaySeries []MedGroupView`-compatible data via the shared `buildMedGroups`-style series builder (reuse `MedSeriesView`; feed it the animal's `TodayPlanWindow` items). `entryClockSVG` stays for legacy history badges only.
- `templates/animals/show.plush.html` (+3 forks): Treatment tab = Today block + history accordion; shared partials `_med_series.plush.html` (drug series line) reused from dashboard/care plan.

**Laws:** Pareto (today's actions first — the 80% question), Tesler's (protocol complexity stays in the engine; surface = do/don't), Jakob's + Similarity (same series component on all 3 screens), Hick's (one next action per drug), Serial Position (Today top, history bottom), Peak-End (history collapsed — the tab ends on the completed Today state).
## 11. Work packages

Order matters: engine/display semantics first (everything renders on top of them), then viewmodel unification, then the three screens, then i18n gate. Each WP = 1+ commits, each commit leaves the app green.

### WP1 — Display semantics + additive engine affordances: current vs superseded, date-aware labels, late-record, outtaken-today, undo-by-any-user (§6, §4b-A1/A2/A8)

**Fixes:** CP5(a) "late at 07:54", CP6 dead-end chips (root), Dash/care-plan label confusion; **restores** depupdate capabilities A1 (late record), A2 (outtaken-today visibility) and A8 (undo by any user).

| File | Change |
|---|---|
| `models/careplan/status.go` | export `NextDue` on `PlanItem` from the existing nextDue map (additive) |
| `actions/care_plan_display.go` **(new)** | `IsCurrent`, `SupersededReason`, `DueLabel` — pure functions |
| `actions/care_plan.go` | `Late bool` on `planApplyRequest` (:164): bounded bypass of the `!Applicable` 409 (:201) — past-due + unapplied + in-window only; future/terminal/overridden still 409; default without flag byte-for-byte unchanged |
| `actions/care_plan.go` | `CarePlanUnapply` (:259): delete the `!u.Admin` gate (:272–288) — any authenticated user undoes any record (§4b-A8); best-effort `auditAnimalChange` entry per undo; stale §10-CP1 comments refreshed |
| `actions/care_plan_service.go` | `loadAnimalContextsScoped` variant with `includeOuttakenToday` (left join + preload `Outtake`, `outtake.date >= today 00:00`); dashboard + care-plan assemblies use it; `ReverifyItem` keeps strict scope |
| `actions/care_plan_viewmodel.go` | `OuttakenToday` flag on animal rows; superseded items → history collection; work counters exclude them |
| `actions/care_plan_dayplan.go` | chip dedupe: earliest **current** instead of earliest open; `FeedingChip.SupersededBy` |
| locales ×4 | `care_plan.time.yesterday`, `care_plan.time.tomorrow`, `care_plan.status.superseded`, `care_plan.superseded.replaced_by`, `care_plan.treatment.record_late`, `care_plan.treatment.record_late_confirm` |

**Unit tests (new file `actions/care_plan_display_test.go` + apply-endpoint cases):**
- `TestIsCurrentUntilSuccessorImminent` — successor due in >lookahead → current; ≤ lookahead → not current.
- `TestSupersededNotShownAsChip` — superseded occurrence is never the group chip.
- `TestDueLabelPrefixesOtherDays` — yesterday → "hier 08:00", today → "08:00", tomorrow → "demain 08:00", +2d → short date.
- `TestTerminalStatusesNeverSuperseded` — applied/skipped stay terminal.
- `TestEveningFeedingStaysLateUntilMorning` — 19:00 missed feeding current overnight, flips when 07:00 imminent (user's grace rule).
- `TestSingleDailyOccurrenceLapse` — no successor: late until windowEnd, then superseded-lapsed, recordable.
- `TestLateRecordAccepted` / `TestLateRecordRejectedFuture` / `TestLateRecordRejectedRecorded` / `TestApplyOutOfWindowStill409WithoutFlag` — the A1 bypass bounds (§6.4).
- `TestOuttakenTodayIncludedInAssemblies` / `TestOuttakenBeforeTodayExcluded` — the A2 scope bound (§6.4).
- `TestUnapplyAnyRecordByNonAdmin` (skipped feeding, deferred care and applied medication undone by a non-admin → 200 each, rows removed, audit entries written) / `TestUnapplyMissingRefStill404` — the A8 widening bounds (§6.4).

**Commit:** `care plan: current-vs-superseded semantics, date-aware labels, late-record, outtaken-today, free undo (CP5/CP6, depupdate A1/A2/A8)`

### WP2 — Viewmodel unification: one build path + FilterStats (§7.2, D3, D6)

**Fixes:** CP2 filter pipeline, CP3 compact/detailed fork, CP4 badges.

| File | Change |
|---|---|
| `actions/care_plan.go` | remove pre-narrowing of `kind`; zone whitelist validation → redirect w/ flash on unknown zone |
| `actions/care_plan_viewmodel.go` | single pipeline: engine → display split → zone×kind filter → sections/groups → `FilterStats` + zone×kind matrix; compact/detailed become a per-card density flag on the same structs |
| `actions/care_plan_dayplan.go` | `GroupCards` consumes filtered set (feeding/care/cleanup cards consistent with tiers→sections move) |

**Unit tests:**
- `TestFilterStatsMatchVisibleSet` — for a seeded matrix of (zone, kind): every badge count equals the rendered item count under that filter (the CP4 invariant, property-style).
- `TestUnknownZoneRedirects` — `?zone=Nope` → 302 + flash.
- `TestCompactAndDetailedSameGroups` — same sections/groups/counts in both modes; only density fields differ.
- `TestKindZoneCombinationFilters` — feeding+zone=Quarantine shows only feeding items of that zone.

**Commit:** `care plan: unified viewmodel pipeline, filtered counters, zone validation (CP2/CP3/CP4)`

### WP3 — Shared drug-series component (§8.3, CP1/Dash-8/T2 shared part)

**Fixes:** the "series of time buttons" building block used by all three screens.

| File | Change |
|---|---|
| `actions/care_plan_viewmodel.go` | `MedSeriesView{Key, Label, Rows [][]MedSlotView}` (bucket-ordered, chunked 3/row) inside `MedGroupView`; `buildMedGroups` emits series |
| `templates/care_plan/_med_series.plush.html` **(new)** + 3 locale forks | renders one drug line + time buttons with state classes; data-* hooks for existing apply/undo JS |

**Unit tests:**
- `TestMedSeriesGroupsByDrugAndDosage` — (name, dosage) merge; differing dosage → separate series.
- `TestMedSeriesChunksThreePerRowByBucket` — 5 morning times → rows of 3+2; bucket change forces new row.
- `TestMedSeriesSlotOrderMatchesBuckets` — morning/noon/evening ordering with <11h / <15h boundaries.

**Commit:** `care plan: shared medication drug-series component (CP1/Dash-8/T2)`

### WP4 — Care plan page template (§7, §9)

**Fixes:** CP1 layout, CP3 accordion, CP6 feeding cards.

| File | Change |
|---|---|
| `templates/care_plan/index.plush.html` + 3 forks | new header (summary strip **as anchor links** §4b-A3, kind tabs incl. corrected `care_plan.kind.all` key, zone dropdown w/ counts); sections-accordion (kind groups + history, history rows carry the late-record action §6.2-2); Meds section uses `_med_series` (dimmed = late-recordable); feeding cards get one-tap apply + batch apply; auto-reload guard (modal open / apply in flight) |
| `templates/care_plan/_item_detail_modal.plush.html` + 3 forks **(new)** | detail modal extracted, shared with animal tab deep-link |
| `actions/care_plan_viewmodel.go` | **same commit cleanup:** delete `HourChip` + `hourCount`, `TierView`/tier structs and compact-only branches now dead (the template no longer references them) — the commit stays green because template + viewmodel switch together |
| JS in same templates | keep one-toggle apply/undo endpoints; rebind on new markup |

**Unit/e2e covered in §12.** **Commit:** `care plan: kind-tabs + zone-dropdown IA, sections accordion, actionable feeding cards (CP1/CP3/CP6)`

### WP5 — Dashboard medication table (§8, D1)

**Fixes:** Dash-1..9.

| File | Change |
|---|---|
| `templates/dashboard/dashboard.plush.html` + 3 forks | `table-striped`; year-number animal button + dove badge; remove count badge; white per-animal treatment card; single-line drug series via `_med_series` (dimmed superseded = late-recordable); `outtaken` row class for today-outtaken animals; eye button → deep-link URL + `aria-label` |
| `actions/animals.go` (`AnimalsResource.Show`, :549) | read `tab`/`item`/`due`; resolve item server-side; pass `OpenDetail` |
| `templates/animals/show.plush.html` + 3 forks | Treatment tab selected on load + open shared detail modal when `item` present |

**Commit:** `dashboard: medication table striped + drug series + deep-link view&popup + outtaken-today (Dash-1..9)`

### WP6 — Animal Treatment tab (§10, D5/T1/T2)

**Fixes:** T1, T2.

| File | Change |
|---|---|
| `actions/care_plan_animal_page.go` | build Today series from `TodayPlanWindow` items (reuse WP3 builder); keep `entryClockSVG` for legacy history only |
| `templates/animals/show.plush.html` + 3 forks | Treatment tab: Today block (protocol) + manual Add treatment + history accordion below |

**Commit:** `animals: protocol-driven treatment Today block + time-series buttons (T1/T2)`

> Note: late-record behavior on the Today block travels with the shared `_med_series` component (WP3) — dimmed superseded buttons there are one-click late-recordable with confirm, same as care plan and dashboard. depupdate's per-date past-slot toggles are covered by: Today block (same-day) + care-plan history late-record (yesterday, window) + history accordion undo by any user (§4b-A8 — widened to depupdate parity: any authenticated user, any record, audited).

### WP7 — i18n gate + docs close-out

- New/changed keys in `locales/care_plan.{en-US,fr,de,nl}.yaml`: `kind.all` (replaces misused `zone.all` on the Tous chip), `time.yesterday`, `time.tomorrow`, `status.superseded`, `superseded.replaced_by`, `zone.select` (dropdown label), `feeding.apply_all` (batch), `treatment.today.title`, `treatment.today.add`, `treatment.record_late` + `treatment.record_late_confirm` (A1), `animal.ready_for_release` (dove badge title, A2 — depupdate had it hardcoded English).
- **i18n render gate — update all three existing tests** in `actions/care_plan_i18n_html_test.go`: `TestCarePlanPagesAllLocales` (new markup, compact + detailed + zone dropdown), `TestDashboardMedicationSectionAllLocales` (:236 — new table markup), and add an animals-show treatment-tab + deep-link case (new subtest in the same file). Assert no missing-key markers in output.
- Fix `creaves/bugs.md` title; strike fixed items as they validate; archive per §12.5.

**Commit:** `i18n: round-2 care plan keys ×4 + all-locales render gate`

---

## 12. Validation

### 12.1 Unit / integration (per WP, before commit)

Run targeted: `go test ./actions -run 'TestIsCurrent|TestSuperseded|TestDueLabel|TestLateRecord|TestApplyOutOfWindow|TestOuttaken|TestUnapply|TestFilterStats|TestUnknownZone|TestCompactAndDetailed|TestMedSeries|TestEveningFeeding|TestSingleDaily|TestKindZone'` — then the full suite once per WP merge (§12.3).

### 12.2 E2E scenarios (agent-browser skill, `buffalo dev` on :3000, admin/admin, seeded data)

| # | Scenario | Steps | Pass criteria |
|---|---|---|---|
| S1 | Late vs current (CP5) | open `/care_plan` at a time matching the seed schedules; inspect a group whose earlier occurrence is missed while next is imminent | chip = next occurrence; missed one only in collapsed history, gray, tooltip "remplacé par …"; no "En retard <yesterday's time>" without date prefix |
| S2 | Filters + badges (CP2/CP4) | click each kind tab; select each zone in dropdown; toggle compact/detailed | list content changes every time; every badge equals visible count; no "0 … (N)" mismatch; unknown `?zone=x` redirects |
| S3 | One mental model (CP3) | compare compact vs detailed on same filter | same sections/groups/counts; detailed shows more per card only |
| S4 | Med series (CP1/Dash-8/T2) | dashboard `/`, `/care_plan?kind=medication`, animal Treatment tab | same drug merged to one line; 3 buttons per row; bucket dividers; time only in buttons; no 2-line subline; table striped; year-number buttons; no count badge; white per-animal card |
| S5 | View + popup (Dash-7/D1) | click eye on a dashboard med slot | lands on animal page, Treatment tab active, detail modal open, no console errors |
| S6 | Feeding actionable (CP6) | `/care_plan?kind=feeding` | every animal line has an Apply; batch apply per cage clears the card's open count; no dead-end red chip |
| S7 | Apply round-trip | one-tap apply + undo on each screen (plan, dashboard, animal tab) | state updates in place; treatment entry created/removed; no reload storm; auto-refresh paused while modal open |
| S8 | i18n | switch locale via the app's `/lang` route (SwitchLanguage, app.go:108), repeat S4 under fr / en-US / de / nl | all new strings translated; no raw keys |
| S9 | Additive checks (§4b-A1/A2/A8) | on `/care_plan`: open history, click a superseded row's late-record; on `/` (or with a dev-DB seed): have an animal outtaken today with remaining work; then log in as a non-admin (create one via admin if the seed lacks it) and undo an applied record from history | late record: confirm dialog → row becomes applied (with the recorded-late marker), no 409; future items still refuse; outtaken-today animal visible with `outtaken` row class + dove badge, its missed slots late-recordable; animal outtaken before today absent; non-admin undo: 200, row re-opens, audit entry written |

**Time-dependence note (S1/S6):** e2e cannot control the clock. Run against the real clock + seed schedules; assertions are relative ("chip shows the next due", "missed one in history"), not absolute times. If no group is in the desired state at run time, shift a dev-DB seed time (dev/test only — never prod data) to create the state, and record it as evidence.

Evidence per scenario: URL + captured output (agent-browser), stored in the WP's validation notes; issues found → fix + update this plan (bugs.md rules 2–3).

### 12.3 Quality gates (each run separately, per bugs.md rule 7)

```
go vet ./...
staticcheck ./...
gocognit -over 15 .
gocyclo -over 12 .
go test -count=1 -race -cover ./...
```

Watch: `BuildDayPlanView`/`CarePlanIndex` complexity after the pipeline rewrite — extract helpers early (`FilterStats`, series builder, section builder) to stay under gocognit 15 / gocyclo 12.

### 12.4 Risks & rollback

- **No migrations, no data writes** beyond existing application rows → rollback = `git revert` of the WP commit. Template forks (×4) revert together with their Go counterpart in the same commit.
- Highest-risk change: WP2 pipeline rewrite (filter + grouping move). Mitigate: land after WP1 with the new tests green. Old tier/compact structs are deleted **inside the WP4 commit** (template + viewmodel switch together — no separate dead-code window where either side drifts).
- `NextDue` additive on `PlanItem`: JSON API consumers (care plan JSON export, if any) gain a field — backward compatible.
- Supersession is display-only: no application rows, no engine status changes — history stays truthful ("was due, replaced/lapsed"). Exception (A1): the explicit late-record writes a **normal** application row (applied/skipped + timestamps); "lateness" stays derivable from `applied_at > due+grace`, no new column. The `Late` flag bypass is bounded (past-due, unapplied, in-window) and the flagless path is byte-for-byte unchanged — rollback = revert, no data repair.
- A8 removes an authorization restriction (round-1 §10-CP1). Mitigations: every undo writes an audit entry; `DeleteFulfillment` and 404-on-missing semantics unchanged; missing refs answer 404 to everyone, as they already did for admins. Rollback = revert the WP1 commit — the gate returns with it.
- Auto-reload guard: if the guard misfires (stale page), the manual refresh button remains.

### 12.5 Close-out (bugs.md rules 4–5, 8)

1. All scenarios S1–S9 pass with evidence; unit suite + quality gates green.
2. Strike the three bugs.md sections; move each to `docs/archive/2026-XX-XX-<slug>.md` alongside this plan (renamed `docs/archive/2026-XX-XX-care-plan-ux-fix-plan-round2.md`).
3. One commit per fix already done in WP1–WP7; final commit = archive + bugs.md title fix (if not done earlier).
4. If new issues surface during validation → added to bugs.md → new plan entry → same loop (bugs.md tail rule).

### 12.6 Validation results (2026-10-01, agent-browser on :3000, admin + non-admin)

All scenarios S1–S9 PASS; two gaps found → fixed in `f11fb12` (zone badge
parity, late-record modal for input kinds). Gates at close-out: go vet,
staticcheck, no NEW gocognit/gocyclo offenders vs baseline, full
`go test -count=1 -race -cover ./...` green.

| Scenario | Evidence (abridged) |
|---|---|
| S1 | seed shift (obs rule → 08:00+09:20, lookahead 30) → work chip = "09:20 Due", missed 08:00 only in collapsed history: `tr.plan-item.text-muted`, badge title "Replaced by 09:20"; non-today labels date-aware ("yesterday 12:00", de "gestern 12:00") |
| S2 | Medication badge 30 = 30 cards; Observation 18 = 18; zone S → 16 cards + toggle "Zone: S 16" (was "S 30" — fixed); zone options 5+5+16+4 = 30; unknown `?zone=XXQQ` → redirect drops zone; care/cleanup/weighing 0-open → empty screen, honest "All 0" |
| S3 | compact vs detailed on `kind=medication&zone=S`: strip identical (Late 12 / Later 45), 16 med groups both; detailed rows = occurrences per CP3 (§7.1) — per-view badges match each visible list (D6) |
| S4 | dashboard: table-striped, 21 white per-animal cards, 0 count badges, year-number buttons, series "Citramox L.A. (48H) — 0.06 ml IM ○ 12:00"; care_plan: 45 series lines; animal tab: #animalTodayBlock with series buttons |
| S5 | dashboard eye → `/animals/8635?due=…&item=animal%3A…#nav-treatment` → Treatment tab `active show` + detail modal `fade show` (Detail/Due 12:00/Status Scheduled/Kind Medication) |
| S6 | feeding: 347 Apply buttons / 342 chips; individual Apply flips ✓ disabled in place; "Apply group (1)" removes the card from the open list in place; applied card leaves the list (no dead-end chip) |
| S7 | one-tap apply/undo round-trip ×3 screens (animal tab, dashboard, care_plan): in-place ✓/○ flip, URL unchanged, same node re-clickable; also on a past-due (late-window) med slot |
| S8 | `/lang` ×4 → care_plan + dashboard + animal Today block: fr (Médication, Toutes), de (Fütterung/Pflege, gestern), nl (Voeding, Vandaag), en default; 0 raw keys, 0 "translation missing" |
| S9 | late-record: confirm dialog → apply modal for observation (answer required, §10-L1) → 201, row Done + ⏱ marker, no 409; outtaken-today (seed shift): dashboard `plan-med-row outtaken` + dove badge, care_plan 🕊️, missed slot applicable/undoable, outtaken-before-today absent from dashboard+care_plan; non-admin (carekeeper, admin=0): apply + history Undo → application deleted, row re-opens as work, audit `care_plan_application/delete` recorded |
