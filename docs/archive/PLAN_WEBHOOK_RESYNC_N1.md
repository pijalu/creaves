# Plan: Webhook full resync — eliminate per-record SELECT N+1

## Problem

`RunResync` (full resync worker) and `ComputeSyncStatus` (the `/webhook_resync`
sync-status panel, recomputed on every page load) both call
`buildEventPayloadWithTranslations` **per animal**. With `pop.Debug` (dev)
logging, that floods the log with per-record SELECTs:

| Query site (per animal) | Count |
|---|---|
| pop `Eager()` association loading (per record!) | 8 |
| `loadPayloadTranslations`: 4 locales × 12 translation fields, 1 SELECT each | 48 |
| species taxonomy lookup (`species.creaves_species = ?`) | 1 |
| `enqueueResyncStateEvent` existence check | 1 |
| `resyncAbortCheck` re-read of the run row | 1 |
| `tx.Update(run)` progress write | 1 |

≈ 59 SELECTs + 2 writes per animal → 10 000 animals ≈ 590 000 log lines per
resync, plus avoidable DB round-trips on limited hardware.

## Approach

Batch what is shared (reference data), keep what is genuinely per-record
(payload writes).

### 0. `EagerPreload` instead of `Eager` (measured first)

Counting SQL during `RunResync` (60 animals) showed the **dominant** N+1 was
pop's `Eager()` mode itself: it loads associations **per record** — 8 SELECTs
per animal (480 per 60 animals). `tx.EagerPreload(...)` uses pop's batched
preload path (one `id IN (...)` query per association). Applied to `RunResync`
and `ComputeSyncStatus`. Result: 535 → 63 SELECTs for 60 animals.

### 1. `translationPreloader` (new, `actions/event_translations.go`)

- Built once per run from the already-loaded `models.Animals` slice.
- Collects the **distinct** ids per `(table, dbField)` group:
  species (5 field groups), animaltypes, animalages, zones, outtaketypes,
  entry_causes (cause/detail/nature).
- One `LoadTranslations` call per group × 4 locales (≤ 48 queries **per run**,
  not per animal) + one `species` table scan into a `map[creaves_species]Species`.
- `buildEventPayloadInto(tx, pre, animal)` becomes the core builder;
  `buildEventPayloadWithTranslations(tx, animal)` = `buildEventPayloadInto(tx, nil, animal)`
  so single-event paths (publish hooks, snapshot grift) are byte-identical.
- Resync and sync-status loops pass a shared preloader.

### 2. `resyncStateIndex` (`actions/webhook_resync.go`)

- One `SELECT animal_id, content_hash FROM event_streams WHERE instance_id = ?
  AND event_type = 'animal_state'` replaces the per-animal EXISTS check.
- The index is updated in memory on create/requeue within the run.
- Concurrency note: only one resync can run per instance (`StartResync` guard);
  `ComputeSyncStatus` and the webhook worker never *create* `animal_state`
  events, so there is no competing writer for the same (animal, hash) keys.
  The force-path requeue stays a targeted `UPDATE ... delivered_at = NULL`.

### 3. Throttled progress persistence (`resyncCheckpoint`)

- `resyncAbortCheck` (per-animal run re-read) and the per-animal
  `tx.Update(run)` are replaced by a checkpoint every `resyncPersistEvery = 25`
  animals: persist progress, then check for user cancellation.
- Context cancellation is still honoured on every iteration.
- Progress UI (`status.json`) granularity becomes 25 animals — imperceptible at
  resync scale.

### Out of scope

- `grifts/event_snapshot.go` publishes per animal but is a one-off CLI task.
- Webhook pusher delivery queries (already batched).

## Tests

SQLite harness (`-tags sqlite`, `TestMain` in `webhook_pusher_test.go`):

1. **Equivalence**: for a fully associated animal (species taxonomy, entry
   cause, outtake type, translations in en-US/de/nl), payload built via the
   preloader path must be `reflect.DeepEqual` to the per-animal path
   (modulo `Timestamp`).
2. **Query-count regression**: `RunResync` over 60 seeded animals with a
   counting `pop.SetLogger`; SELECT count must stay far below one-per-animal
   (previous behaviour ≈ 51 × 60). Also asserts events are still created.
3. Existing `webhook_resync_test.go` regressions (nested associations, skip vs
   force requeue, disabled webhook) must keep passing.

## Validation

- `CGO_ENABLED=1 go test -tags sqlite -count=1 ./actions/ -run 'Resync|SyncStatus|TranslationPreloader|Payload'`
- `go test ./actions/... ./models/...` (default build, MySQL test DB)
- `gofmt -l` clean; `go vet ./actions/`

## Archive + commit

- Archive summary to `docs/archive/webhook-resync-n1.md` after validation.
- Single commit: plan, implementation, tests, archive.
