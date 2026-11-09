# Creaves query and page performance plan

## Scope and method

This audit covers the main `creaves` application only. `creaves-console` is intentionally excluded. The route inventory in `actions/app.go` was traced through page handlers, CRUD resources, reports, exports, autocomplete endpoints, event production/delivery, resync, maintenance, and the templates/model associations used by those paths.

The work uses these rules:

- Preserve returned rows, ordering, complete animal histories, and complete exports.
- Measure on the production-sized development MySQL database before adding an index or rewriting SQL.
- Replace per-row Pop eager loading only on reviewed read paths; do not change Pop's global eager mode.
- Keep reference/user/report caches and their existing invalidation semantics unchanged.
- Treat pagination or truncation of a currently complete export/history as a separate product change, not an invisible optimization.
- Add reversible migrations and regression coverage for every implemented query/index change where practical.

Baseline: `go test -count=1 ./...` passed in all packages (20.78 s) before edits.

Development database scale at audit time: 382,033 cares, 59,288 treatments, 18,730 translations, 10,296 animals, and 127 animals currently in care. Timing values below are local measurements, not production Service Level Objectives.

## Findings by page/query family

| Page or path | Current access pattern | Assessment and action |
| --- | --- | --- |
| `/` landing | All in-care animals plus clean-cage query | Accept current shape: 127 in-care rows; clean-cage query about 0.018 ms. Revisit only if in-care count grows substantially. |
| `/dashboard` | Warning-care derived aggregation; today's treatments and vet visits; type counts; 24 h logs; due todos; cached weight loss | **High priority.** Warning-care SQL measured about 283 ms, mostly materializing a 9,355-row/3,450-group aggregate. Add and benchmark a covering `(type_id, animal_id, date)` care index. Batch today's vet-visit animal loading. Existing date indexes cover treatment/log filters. Add todo/vet-date indexes only if benchmark/explain supports them. Weight-loss query measured about 36.5 ms and is already cached. |
| `/feeding` | One grouped query for animals with feeding schedules | Measured about 0.657 ms on current data. Keep; no duplicate index for an already fast path. |
| `/animals` and animal-number shortcuts | Paginated list; optimized bulk enrichment | Keep `EnrichAnimalsOptimized` and its deduplicated batch loads. No per-row association conversion needed on the list itself. |
| `/animals/{id}` | Complete care, treatment, and vet histories with default `Eager()`; attachments; paginated audits | **High priority for query count.** Convert reviewed history collections to `EagerPreload`, preserving every row and order. Add `(animal_id, created_at, id)` for audit pagination. Complete histories remain a memory/latency risk for exceptionally old animals and require separate UI design before any cap. |
| `/animals/search` | Parameterized filters; paginated HTML; complete CSV | Keep SQL filters and optimized list enrichment. CSV intentionally remains complete; streaming/chunking is a later memory optimization requiring response/error semantics tests. |
| `/register`, `/register_snapshot` | Paginated register; snapshot capped at 2,000; no-treatment enrichment; complete CSV | Existing `EnrichAnimalsOptimizedNoTreatments` correctly avoids an unused treatment query. Keep row completeness. Do not silently reduce the snapshot cap. |
| `/treatments`, `/veterinaryvisits`, `/travels`, `/discoveries`, `/outtakes`, `/logentries`, `/cares` | Paginated CRUD lists; several use default `Eager()` | **Medium priority.** Use explicit `EagerPreload` for reviewed list associations and dashboard vet visits. Single-record `Find` calls do not have the slice N+1 problem and can remain unchanged. |
| `/todos` | Open and done lists, ordered; dashboard due subset | Add indexes for open/done filtering and ordering if supported by `EXPLAIN`; current table is empty, so production verification is still required. Batch done-by user lookups are already present. Preserve complete sections. |
| `/reports/annual` | Twelve grouped stats and totals | Existing cache retained. Year predicates and grouped result shapes reviewed; no unvalidated rewrite. |
| `/reports/corpses`, `/reports/discoverers` | Year-filtered/full report rows | Keep result completeness and existing filters. Add no index without a measured slow report on production-scale data. |
| `/export*`, `/excel*` | Configured SQL; configured CSV streams rows; HTML and Excel materialize results | Configured CSV retains every row, year/view filters, ordering, UTF-8 BOM, and NULL handling while retaining only the current row. Other CSV handlers and Excel exports remain memory-bound; no arbitrary row limit. |
| `/suggestions/*` | `DISTINCT ... LIKE '%q%' ORDER BY ... LIMIT 10/25` | Leading-wildcard substring search cannot use a normal B-tree index efficiently. Keep bounded results and exact endpoint semantics. Translation joins benefit from the new translation index. Do not add low-selectivity text indexes that cannot support `%...%`. |
| Reference-data CRUD, users, config | Mostly paginated lists and primary-key form/show lookups | No systemic issue. Remove the duplicate care-type translation load on `/caretypes/new`; retain reference caching and batch translation helpers. |
| `/event_streams` | Paginated events joined to an aggregate over all deliveries; DLQ count | Reviewed as a scaling risk, but dev has only 6 events/0 deliveries, so no timing claim. Existing `(event_id,target_id)` and target indexes support lookups. Defer SQL reshaping until realistic delivery data can prove a win. |
| `/sync_configuration` | Targets plus one aggregate per target | Few targets; existing `target_id` index. Accept current design rather than add complexity for a tiny N. |
| `/webhook_resync`, sync status | Keyset chunks; bounded joins; incremental translation preloader | Already optimized and protected by query-count tests. Keep; do not reintroduce whole-table eager loads. |
| Maintenance snapshot/pusher/producer | Keyset/batched paths and delivery indexes | Already substantially optimized. One default eager outtake-type batch in maintenance can use preload without changing event semantics. No contract changes. |
| Templates and translation helpers | Per-request maps and batched `(table,field,locale)` loads | **High priority.** Existing `(table_name,record_id,field,locale)` index is in the wrong column order for locale-wide loads. Add `(table_name,field,locale,record_id)`. The measured species lookup was about 54.1 ms. |
| Animal audits | Admin-only, paginated by animal/date/id | Add a composite index matching filter and deterministic order; retain both `created_at` and `id` in the key. |

## Implementation sequence

### 1. Measured index changes

Create one reversible SQL migration, then update `migrations/schema.sql` consistently.

Candidates:

1. `translations (table_name, field, locale, record_id)` — validated by the measured slow lookup and existing query shapes.
2. `cares (type_id, animal_id, date)` — validate with temporary-index before/after `EXPLAIN ANALYZE`; keep only if it materially improves the warning aggregate without replacing required indexes.
3. `animal_audits (animal_id, created_at, id)` — matches the exact paginated filter/order.
4. `veterinaryvisits (date)` — matches list ordering and dashboard range predicate.
5. `todos (done_at, todo_date)` and a done-list ordering index — inspect optimizer behavior; avoid redundant keys or unsupported claims on an empty dev table.

Rollback: each `down.sql` drops only indexes introduced by its paired `up.sql`. Test migration down/up on the disposable test database and run the full suite with the final schema.

### 2. Bounded association loading

Use `EagerPreload(...)` explicitly on:

- Animal show history collections (cares, treatments, vet visits).
- Paginated treatment, vet-visit, travel, discovery, outtake, log-entry, and care lists where associations are rendered/serialized.
- Today's dashboard vet visits and maintenance's batched outtake types.

Do not switch RawQuery structs without associations, single-record finds, or the custom optimized animal-list enrichment. Add regression tests that both verify loaded associations and bound SELECT counts independently of row count.

### 3. Small redundant-work removal

Remove the exact duplicate `setTranslationValues` call in `CaretypesResource.New`. No UI or locale change is needed.

### 4. Verification

- Focused tests for query count, associations, translation behavior, and existing page responses.
- `gofmt`, package tests, then `go test -count=1 ./...`.
- Real MySQL before/after `EXPLAIN ANALYZE` for accepted indexes; record actual timings and plans.
- Migration rollback/forward validation on the test database.
- Authenticated Chrome DevTools smoke tests for dashboard, animal detail/history, each modified CRUD list, care-type new form, and sync/maintenance paths affected by code changes.
- Final diff review for accidental behavior, UI, contract, or console changes.

## Implemented results and validation (2026-11-09)

### Query counts

The new tests first failed against default `Eager()` loading and missing indexes. After targeted `EagerPreload()` conversion, five-row JSON list requests have fixed query costs. Counts below include only SELECTs against the tables monitored by each test, excluding unrelated middleware queries.

| Path/fixture | Before | After |
| --- | ---: | ---: |
| `/treatments?per_page=5` | 7 | 3 |
| `/veterinaryvisits?per_page=5` | 12 | 4 |
| `/discoveries?per_page=5` | 9 | 4 |
| `/outtakes?per_page=5` | 12 | 4 |
| `/logentries?per_page=5` | Not separately recorded | 3 |
| `/travels?per_page=5` | Not separately recorded | 5 |
| `EnrichAnimal`, four rows in each of care/treatment/vet history | 24 | 9 |

The log-entry and travel tests create five associated records so their budgets cannot pass vacuously on empty tables. Animal history assertions cover `Animal`, `Type`, self-referential `LinkTo`, and veterinary `User` associations as well as row completeness. Cage outtakes now preload only `Intake`, rather than every animal history association; the existing batch-create test still passes. The duplicate new-care-type translation query was removed.

### MySQL measurements

`EXPLAIN ANALYZE` was run on the production-sized development MySQL 8 database. The temporary-index columns record the controlled index experiment; the final run occurred later under different cache/load conditions and must not be presented as a controlled percentage improvement.

| Query | Baseline | Temporary index | Final observed run | Final plan/result |
| --- | ---: | ---: | ---: | --- |
| Dashboard warning-care aggregate | 64.2 ms warm | 5.27 ms | 18.1 ms | Uses covering `cares_type_id_animal_id_date_idx` |
| Locale-wide species translation lookup | 4.60 ms warm | 0.794 ms | 2.99 ms | Uses `translations_table_field_locale_record_id_idx` |
| Treatment first-page date/time/animal ordering | 79.7 ms | 0.0583 ms | 0.265 ms | Baseline scanned/filesorted; final uses `treatments_date_timebitmap_animal_id_idx` |

The veterinary date-range and open-todo queries select their new indexes. The audit index exactly matches animal filtering plus deterministic `created_at, id` ordering. The event-list query still chooses a table scan for only six events, which is a reasonable optimizer choice; no event-list timing improvement is claimed. No delivery aggregate rewrite was made.

Seven indexes were added in `20261110090000_query_performance_indexes`: translation lookup, warning-care aggregation, treatment list ordering, animal-audit pagination, veterinary date filtering, todo filtering/order, and event page ordering. Migration rollback on `creaves_test` left zero of these indexes; forward migration restored all seven with the expected columns. Both development and test databases were left migrated up. `migrations/schema.sql` contains only these intended index additions.

### Configured CSV streaming follow-up

`RunQuery` no longer calls `FetchRows` for CSV downloads. It executes the configured/year-filtered query, discovers columns, then scans, filters, and CSV-encodes one row at a time. `FetchRows` remains for HTML and Excel and now also checks `rows.Err()` instead of silently accepting an iteration failure. Scan/iteration error tests and byte-for-byte comparisons against the materialized implementation cover all rows, year filtering, view filtering, UTF-8 BOM, accents, and NULL values.

Same-run development benchmark (`register`, three iterations, `io.Discard`, MySQL):

| Implementation | Time/op | Cumulative allocated bytes/op | Allocations/op |
| --- | ---: | ---: | ---: |
| Materialize then write | 119.2 ms | 14,914,530 | 647,653 |
| Stream current row | 117.1 ms | 10,252,685 | 637,171 |

Allocated-byte total fell 31.3%; latency was unchanged within this small benchmark. The structural win is retained memory: complete result slices are no longer built by configured CSV downloads. A real authenticated download produced 10,459 CSV records including the header, 21 columns, and a 1.8 MiB body with the expected BOM and first/last years. A UI-driven no-match global filter produced exactly the header row. No page errors occurred.

### Regression and browser checks

- `go test -count=1 ./...` passed in all packages after configured CSV streaming (19.11 s wall time).
- Focused query-count, index, and cage-batch tests passed.
- Read-only authenticated headless Chromium checks used `agent-browser` because Chrome DevTools MCP daemon startup was unavailable. Dashboard, animal 8055 with all three history types, treatments, veterinary visits, travels, discoveries, outtakes, log entries, new care type, cage B1 outtake, event streams, maintenance, webhook resync, and sync configuration all rendered without page errors or application-error markers. No create/update/delete/snapshot/resync action was submitted.
- No templates, user-facing text, locale files, webhook contracts, or console files changed.

Follow-up authenticated HTML/table validation on 2026-11-09 used `admin/admin` in headless Chromium. The native Chrome DevTools MCP daemon again failed during automatic startup (`goa: unknown command "start"`), so the run used the `agent-browser` fallback. HTTP statuses below come from the browser's navigation performance entry; row counts are rendered `tbody tr` counts after network idle.

| Route | Actual rendered result |
| --- | --- |
| `/dashboard` | HTTP 200; four tables and 33 body rows across warning animals, weight loss, treatments, and veterinary visits. |
| `/treatments?per_page=5` | HTTP 200; six expected columns and five rows; first row linked animal 2207/26 and showed Quadrosol 10% (IM), 0.18 Ml. |
| `/veterinaryvisits?per_page=5` | HTTP 200; five expected columns and five rows with date, veterinary, user, animal, and diagnostic values. |
| `/travels?per_page=5` | HTTP 200; animal number, user, type, and distance columns rendered; the development table had zero rows. |
| `/discoveries?per_page=5` | HTTP 200; four expected columns and five rows with populated location/date/discoverer data. |
| `/outtakes?per_page=5` | HTTP 200; four expected columns and five rows with animal/date/type/location data. |
| `/logentries?per_page=5` | HTTP 200; four expected columns and four existing rows with timestamps, user, and description data. |
| `/caretypes/new` | HTTP 200; "New Caretype" heading and the form rendered without an application error. |
| `/animals/10516` | HTTP 200; all detail tabs rendered, including seven care rows, a dated Quadrosol treatment entry, and the veterinary section. |
| `/animals/8055` | HTTP 200; 50 care rows and one veterinary row rendered with animal/date/user/diagnostic values. |
| `/export/view?query=register` | HTTP 200; 21 French columns, 50 rendered rows, and 210 pagination pages. A `no-such-export-value` global search reduced the table to the single "Aucune entrée correspondante trouvée" result row. |

The follow-up run reported no browser page errors or HTTP/application-error markers. Console output contained only Buffalo live-reload informational messages. No mutation, snapshot, or resync action was submitted.

## Explicitly deferred / residual risks

- Complete animal histories, non-configured CSV handlers, online HTML export views, and Excel workbooks can still consume substantial memory. Configured CSV downloads are now streamed; the remaining paths need separate chunked/streaming designs and explicit error handling without data truncation.
- `%substring%` autocomplete remains a table/distinct/sort workload by nature. Full-text or normalized suggestion tables are separate feature work.
- Event-list aggregate reshaping needs production-scale delivery data before changing proven sync SQL.
- Todo indexes cannot be timing-validated on the empty development table; include only indexes justified by predicates/plans, and request production `EXPLAIN ANALYZE` after deployment.
- Index write/storage overhead must be observed on the care and treatment histories, which are large and write-heavy. The migration should run during a maintenance window on large installations.
- Descending treatment keys were benchmarked on MySQL 8. Older MariaDB versions may ignore descending key direction while remaining schema-compatible, so the full filesort improvement must be rechecked on the production database/version.
- The event-list aggregate still scans all matching deliveries; its index only supports page ordering. Reassess with realistic delivery volume.
