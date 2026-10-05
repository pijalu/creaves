# Performance assessment — creaves + creaves-console (round 10)

Date: 2026-10-04. Branch `feature/care-expert` (creaves), console repo same date.

## Method

1. **Static inventory** of the documented hot paths (creaves AGENTS.md
   "Performance Notes") against the current code — several documented
   bottlenecks were ALREADY fixed in earlier rounds (ref-cache for reference
   tables, `cachedUserByID`, `EnrichAnimalsOptimized`, weight-loss cache,
   connection-pool caps in database.yml). Docs were stale; see "Docs" below.
2. **Query-level audit**: per-request SQL captured from the dev log, sizes
   from `information_schema` (creaves dev = production scale: 10.2k animals,
   **373k cares**, 57k treatments, 19.5k translations; console seeded with a
   12k-row consolidated_animals + 40k event_streams synthetic dataset).
   Slow statements measured with `EXPLAIN ANALYZE` and prepared-statement
   wall-clock (warm, best of 2).
3. **End-to-end timings**: best-of-3 `curl` page loads, before and after.
4. **Validation**: unit tests (query-shape + behavior pins), full test
   suites both projects, agent-browser e2e (rendering + a live
   apply→badge→undo round trip).

## Findings and fixes (creaves)

### P1 — `fillLastWeights` scanned each animal's whole care history (50–106ms → ~5ms)
Every day-plan/landing/dashboard request resolves the latest weight per
in-care animal via `GROUP BY animal_id / MAX(date)` over weighted cares. With
no index covering `(animal_id, weight, date)`, the per-animal probe read ~118
cares rows (measured `EXPLAIN ANALYZE`); the full query took 50–106ms warm —
the single largest DB cost on every plan page.
**Fix**: covering index `cares_animal_id_weight_date_idx (animal_id, weight,
date)` — the aggregate becomes an index-only pass (~5ms).

### P2 — Dashboard "Animals in alert" aggregated over all animals ever (41ms → ~7.6ms)
`SQL_CARES_IN_WARNING`'s `MAX(date)` subquery grouped care history for every
animal ever recorded, then discarded the outtaken ones at the join.
**Fix**: restrict the aggregate to in-care animals (the outer WHERE already
demanded it) + index `cares_animal_id_type_id_date_idx (animal_id, type_id,
date)`. The per-animal probe now reads only the animal's warning-type cares.

### P3 — Plan assembly full-scanned all 10k animals per request (~20ms saved)
`loadAnimalContextsIncludingTodayOuttaken` used
`WHERE outtake_id IS NULL OR outtake_id IN (SELECT …)` — the OR form makes
MySQL abandon the `outtake_id` index and scan the whole table on EVERY plan
assembly (care_plan pages, dashboard, care-schedule, landing badge).
**Fix**: two indexed loads (in-care via the covering index; today-outtaken
via their outtake rows) merged in memory.

### P4 — Nested-eager N+1 on listings (2N queries eliminated)
`enrichAnimalsOptimized` loaded today's treatments with `tx.Eager()`. Pop's
level-2 nested eager issues **one query per parent row**: a landing page
with N treatments paid N × `treatment_time_entries WHERE treatment_id = ?`
plus N × `animals WHERE id = ?` (the unused belongs_to) — 28+30 per-row
queries observed on the dev landing page. The only consumer is
`Treatments.TodayStatitics()` which reads `Entries`.
**Fix**: plain treatment query + ONE bulk `treatment_time_entries WHERE
treatment_id IN (…) ORDER BY due_at` attached in memory.

### P5 — Animals index paid the treatments query for nothing
The animals list rendered no Treatments but used `EnrichAnimalsOptimized`.
**Fix**: `EnrichAnimalsOptimizedNoTreatments` (the variant that exists for
exactly this purpose).

### P6 — Landing badge re-ran the full §6.1 plan assembly per request (~55ms saved)
`CountOpenItems` builds memberships, generates occurrences and joins
applications — for two advisory badge numbers on the most-hit page.
**Fix**: `CountOpenItemsCached` (30s TTL) invalidated post-commit by the
apply/unapply/batch handlers via the existing
`queuePostCommitInvalidation` mechanism. Verified live: apply → badge
1219→1218, undo → 1219, both within the TTL (no stale window observed).

### Migration
`20261004210000_add_cares_hot_path_indexes.{up,down}.fizz` — the two P1/P2
indexes; `migrations/schema.sql` regenerated. Note: fizz has no comments —
the rationale lives here.

## Findings and fixes (creaves-console)

### C1 — Report labels parsed the translations JSON of every scanned row (~34ms → ~20ms)
`localizedGroupLabels` aggregated `MIN(translations)` over
`consolidated_animals` — every report page (by_species/by_type/by_location)
paid a full scan that parsed the JSON column per row (measured 33.6ms on
12k rows). **Fix**: two-step — `SELECT field, MIN(id) GROUP BY field`
(cheap string min, no JSON), then fetch one representative row per distinct
value; with the base/fr UI language the JSON is never read at all
(LocalizedField's canonical fallback IS the French label). Relies on the
documented invariant that rows sharing a canonical value share the same
translation set (translations are keyed by the creaves reference record) —
pinned in `actions/report_labels_sqlite_test.go`.
Console indexes were already comprehensive — no schema change needed.

## Audited, already optimal (no action)

- Reference-table cache (`refcache.go`), user cache (`cachedUserByID`),
  weight-loss cache (`cache_utils.go`) — implemented with single-flight
  loads + post-commit invalidation (AGENTS.md "Known Bottlenecks" is stale).
- Plan fill pipeline (`fillIntakeCondition`, `fillLastWeights`,
  `fillVetDiagnostics`) — already bulk (§9 no-N+1).
- Species fallback JOIN (`tnameResolveByBase`) — index-covered, ~0.02ms.
- Landing clean-cage query — uses `cares_date_idx`, ~0.03ms.
- `SQL_ANIMAL_COUNT_IN_CARE_PER_TYPE`, dashboard GROUP BYs — small tables.
- A NOT EXISTS rewrite of the in-care OR query was benchmarked (25–45ms —
  WORSE than the OR at 20ms); the two-query split (P3) is the win.
- Console `event_streams` lookups (webhook auth idempotency, instance
  history) — all index-covered.

## End-to-end results (best-of-3, dev machine, warm)

| Page | Before | After |
|---|---|---|
| creaves `/` (landing) | 133ms | **48ms** (2.8×) |
| creaves `/dashboard` | 127ms | **48ms** (2.6×) |
| creaves `/care_plan?kind=feeding` | 113ms | **70ms** (1.6×) |
| creaves `/care_plan?kind=medication` | 104ms | **63ms** (1.7×) |
| creaves `/care_plan?kind=observation` | 103ms | **58ms** (1.8×) |
| creaves `/reports/care_schedule` | 123ms | **67ms** (1.8×) |
| creaves `/animals` | 26ms | 34ms (page already small; N+1s gone) |
| console `/reports/by_species` | 34ms | **20ms** (1.7×) |
| console `/reports/by_type` | 34ms | **20ms** (1.7×) |
| console `/reports/by_location` | 21ms | 21ms (labels not the bottleneck there) |

The four >100ms creaves pages were all dominated by the P1+P3 queries
(db=75–99ms of the total); both were eliminated.

## Validation evidence

- **UT**: creaves — 4 new pins (`actions/perf_round10_test.go`: badge-cache
  semantics incl. poison+invalidate, bulk Entries attach + TodayStatitics
  read model, NoTreatments listing, no-OR pin); full suite green. Console —
  `actions/report_labels_sqlite_test.go` (two-step labels + rendered page
  in en-US and fr, run 3× for the representative-row randomness); full
  SQLite suite green. Migration replay test
  (`TestMigrationsReplayOnEmptyDatabase`) green with the new fizz.
- **E2E (agent-browser)**: creaves login → landing badge renders; care_plan
  medication apply (18:00 slot) → landing badge 1219→1218 within TTL; undo
  → 1219; applications table row created/deleted; animals list, dashboard,
  care_plan render unchanged. Console: by-species report renders labels
  correctly in en-US and fr.
- Admin password restored from `tmp/admin-orig-hash.txt` after the browser
  pass.

## Docs

creaves `AGENTS.md` "Performance Notes" updated: the fixed bottlenecks are
marked done and this document is referenced; Phase 4 status updated.

## Residual opportunities (not implemented, by cost/benefit)

1. **Day-plan assembly memoization**: care_plan/dashboard/care_schedule each
   still build the plan per request (~35ms each now). A short-TTL plan cache
   keyed by (window, invalidated-on-apply) would cut further, but the badge
   cache pattern shows the invalidation surface — worth it only under
   measured load.
2. **cares table growth**: at 373k rows the P1/P2 indexes hold; revisit if
   cares grows >2–3M (partitioning by date).
3. **Landing page loads all in-care animals** (~230 rows today, fine); at
   multi-thousand in-care counts, group in SQL instead of Go.
4. Console `by_location` remains ~21ms (aggregation-bound, no JSON involved).
