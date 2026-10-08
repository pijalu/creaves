# Fix archive — 2026-09-05 — webhook_resync gigantic SQL queries

## Original report (bugs.md)
> ## creaves: webhook_resync
> Opening and starting a full resync create gigantic SQL queries that take a
> long time to execute - the approach should ensure a sane default
> behavior/avoid excessive resource usage.

## Root cause
- `StartResync` loaded the **entire animals table** just to compute
  `total = len(animals)`.
- `RunResync` ran one `EagerPreload(...)` over the whole table: the animals
  SELECT plus one SELECT per association, each with an `IN (...)` clause of
  every animal id (thousands) — gigantic statements that MySQL parses slowly,
  that risk `max_allowed_packet`, and that buffer the whole table + all
  associations in memory.

## Fix (creaves commit 7f78f8a)
- `StartResync`: `SELECT COUNT(*)` instead of loading the table.
- `RunResync`: keyset-paginated streaming — chunks of 200 animals
  (`WHERE a.id > :cursor ORDER BY a.id LIMIT 200`). Each chunk loads its full
  association graph with **one bounded LEFT JOIN** (`resyncChunkSelect`):
  animalages, animaltypes, discoveries, entry_causes, discoverers, intakes,
  outtakes, outtaketypes — all 1:1 `belongs_to`, so no row multiplication.
  `resyncAnimalRow` + `resyncRowToAnimal` rebuild the same
  `models.Animal` graph the old EagerPreload produced.
- The translation preloader now runs **per chunk** → all its IN() clauses are
  bounded by the chunk size. Payload building, hashing
  (`StateContentHashPayload`), deterministic event UUIDs, enqueue and
  delivery accounting are unchanged; the expected-state-hash announcement
  accumulates per chunk and is persisted once.
- SQLite test schema: kept PRIMARY KEYs on reference-table id columns (prod
  MySQL has them) — without PKs, duplicate fixture rows accumulate and the
  JOIN multiplies rows.

## Validation
- `go vet ./...` — clean; `staticcheck ./...` — pre-existing warnings only
  (unused funcs, capitalized error strings).
- `gocognit -over 15` / `gocyclo -over 12` — new code under thresholds
  (chunk loop extracted into `processResyncChunk`).
- `buffalo test` (MySQL) — all packages pass.
- `CGO_ENABLED=1 go test -tags sqlite ./...` — all resync tests pass;
  `TestRunResyncQueryCountBounded` now measures **63 SELECTs for 60 animals**
  (old N+1 was ~3490; bound is animalCount+80). Three pre-existing failures
  in `configs_bug_test.go` ("no such table: users") are identical on the
  baseline (verified via `git stash`) and unrelated to this change.
- SQL validated against the production MySQL schema (all joined columns
  confirmed present, chunk query executed successfully on real data).
- Browser check: /webhook_resync page renders the sync status panel
  (expected animals, checksum, per-year buckets) unchanged.
- Full resync not triggered against the production-data dev DB (would push
  ~10k events); chunk behavior is covered by the regression tests.

## Follow-up (creaves commit 0d42fd8, same day)

Verification against the live dev DB exposed two leftovers of the original
fix, both closed in `0d42fd8`:

1. **`ComputeSyncStatus` was not converted.** The /webhook_resync page render
   still ran one full-table `EagerPreload` and hashed all ~10k animals in one
   pass (~69s page load — the "opening" half of the original bug report). It
   now streams the same keyset-paginated chunks as `RunResync` with a
   per-chunk preloader, accumulating totals/buckets/checksum lines per chunk
   (`recordHash` extracted to stay under gocognit 15). Output is unchanged:
   same totals, per-year buckets and expected checksum (lines are sorted
   before hashing; the payload builder is the shared one).
2. **Latent NULL scan bug in the chunk loader.** Every LEFT JOIN id column
   scanned into `uuid.UUID`; animals WITHOUT an outtake (the normal in-care
   case, present in the sync-status fixtures) aborted the chunk query with
   `uuid: cannot convert <nil> to UUID`. RunResync tests only seeded
   fully-associated animals and a full resync had never run against real
   data, so this was invisible. All join-side id columns are now
   `nulls.UUID` with `.Valid` guards (pop zero-value belongs_to semantics).

New regression tests: `TestComputeSyncStatusQueryCountBounded` (52 SELECTs
for 60 animals, bound animalCount+80; largest placeholder list 4) and
`TestResyncChunkLoaderEquivalence` (chunk-loaded graphs with AND without
outtake yield payloads + content hashes identical to EagerPreload).

Gates re-run after the follow-up: `go vet` clean; `staticcheck` unchanged
(pre-existing warnings only); `gocognit`/`gocyclo` — no new entries over
threshold (pre-existing offenders as documented above); MySQL suite
`GO_ENV=test go test -count=1 -race -cover -tags development ./...` green;
sqlite suite `CGO_ENABLED=1 go test -tags sqlite -count=1 ./...` green except
the same three documented pre-existing configs failures; sqlite with `-race`
fails the same ~19 webhook/worker tests as the unmodified baseline (global
tx-logger swap + background workers — pre-existing, unchanged by this fix).

Browser re-verification (agent-browser): /webhook_resync renders the sync
panel (expected 10046, per-year buckets, checksum
sha256:9e1a5fa12c1b0768bb668f7bbfd8f66d89555ebed1a5cbbee0eb28e9214f2cbf) in
~6s instead of ~69s, and the checksum matches the console /sync_management
checksum ("matches producer", 10046 confirmed / 0 unconfirmed) — proving
hash identity end-to-end. A full resync was still not re-triggered (would
push ~10k events); the already-delivered 10046/10046 state plus the checksum
match serve as the end-to-end proof.
