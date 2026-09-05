# Sync checksum + confirmation feedback fix (archive)

Status: implemented (bugs.md item 4).

## Problem

Sync verification was broken on both sides in complementary ways:

- Console `/sync_management` showed stored & expected checksum
  `sha256:e3b0c442…7b855` — the SHA-256 of the **empty string** — and printed
  **"checksum match"** against itself, while the producer's real expected
  checksum was different. The console's "Expected" count was derived from
  *received* events, so undelivered animals were undetectable.
- Creaves `/webhook_resync` showed `Delivered & current: 0 · Unconfirmed:
  10046 (10046 never synced)` immediately after a completed 10046-animal
  resync: there was no console→creaves acknowledgement at all, and — worse —
  the producer status page hashed payloads *without* `CurrentStatus` while
  the resync/update producers hashed *with* it, so its computed hashes never
  matched any stored `content_hash` and nothing could ever count as current.

## Root causes

1. Console checksum was computed over an empty/degenerate set and compared
   against itself; empty sets must be "no data", never a match.
2. No acknowledgement path: an HTTP 200 delivery proved nothing about what
   the console stored.
3. Producer-side hash divergence: `CurrentStatus` ("in_care"/"released") was
   applied by the resync and update paths before hashing but not by the
   sync-status computation.
4. Legacy events (created before `state_hash` existed in payloads) could not
   be acknowledged by the console — it can only echo the `state_hash` it
   has stored — so re-delivered old events stayed unconfirmed forever.

## Fix

### creaves (producer)

- New `event_streams.acknowledged_at` column; `ComputeSyncStatus` counts
  "Delivered & current" only when the console echoed the event's state hash
  (acknowledgement), not on bare delivery. Never-synced = current hash has
  no pending event either.
- New shared `applyCurrentStatus` used by the resync, the update path and
  the sync-status computation, so all three hash identically.
- `RunResync` announces the expected set (`announced_expected_total`,
  `announced_expected_checksum`, `announced_at` on the run) computed with
  **zero extra SELECTs** from the run's preloaded animals (query-count bound
  test kept green).
- `deliverBatch` attaches the `sync` announcement to batches whose events
  carry a `resync_run_id` and applies the response's `confirmed` acks
  (only when the event was accepted *and* the echoed hash equals the
  event's `content_hash`).
- Force re-queue resets `acknowledged_at` and **backfills the payload with
  its `state_hash`**, making legacy events acknowledgeable.
- Checksum formula shared with the console: `StateSetChecksum` over sorted
  `"<animal_id>|<hash>"` lines, SHA-256, `sha256:` prefix.

### creaves-console (receiver)

- Checksums over stored animals only; empty set → "no data yet" badge,
  never a checksum, never a match.
- New instance columns `announced_expected_total`, `announced_expected_checksum`,
  `announced_at`; the resync envelope's `sync` block is stored and rendered
  ("Expected (producer)" column, announced total, "matches producer" /
  "MISMATCH vs producer expected" badges gated on non-empty equality).
- Webhook response carries `confirmed: [{id, state_hash}]` per processed
  `animal_state` event.
- A redelivered event whose payload differs from the stored one (e.g. the
  producer's legacy `state_hash` backfill) is re-stored and re-applied;
  `processEvent` stays idempotent for unchanged state.

### Contract (v2, documented in both AGENTS.md)

Request envelope gains an optional `sync` block on resync batches; response
gains `confirmed` acknowledgements alongside `processed_ids`.

## Tests

creaves: `TestStateSetChecksumGolden`,
`TestExpectedStateHashesMatchProducerPath` (status-path hash must equal
producer-path hash — pins root cause 3),
`TestComputeSyncStatusCountsPerYearAndChecksum` (delivered-but-unacknowledged
stays unconfirmed), `TestDeliverBatch_ConfirmedAcksMarkAcknowledged`,
`TestDeliverBatch_AttachesResyncAnnouncement`,
`TestDeliverBatch_NoAnnouncementForRegularEvents`,
`TestRunResyncForceRequeuesDeliveredEvents` (asserts the payload backfill),
`TestRunResyncQueryCountBounded`.

creaves-console: `TestInstanceSyncStatus_EmptyInstance` (no-data, empty
checksums), `TestInstanceSyncStatus_AnnouncedStatusLoaded`,
`TestWebhookEventsHandler_ConfirmsStateHashesAndStoresAnnouncement`,
`TestWebhookEventsHandler_RefreshesLegacyPayloadOnRedelivery`,
`TestSyncManagementIndex_ShowsProducerBadges`, locale template assertions.

## Validation

- `go test -count=1 -race -cover ./...` green in both repos (guideline
  command); creaves `CGO_ENABLED=1 go test -tags sqlite ./...` green;
  console `CGO_ENABLED=1 go test -tags sqlite -count=1 -race -cover ./...`
  green.
- `go vet`, `staticcheck`, `gocognit -over 15`, `gocyclo -over 12` clean on
  all touched code; `WebhookEventsHandler` complexity reduced from
  gocyclo 35/gocognit 60 (HEAD baseline) to 13/15.
- Browser E2E (dev servers, 10046 animals): force full resync → producer
  page shows `Delivered & current: 10046 · Unconfirmed: 0 (0 never synced)`,
  expected checksum `sha256:b6833e60…`; console shows announced expected
  total 10046, stored/expected/received/confirmed 10046 · 0 unconfirmed,
  both checksums `sha256:b6833e60…` with "checksum match" and
  "matches producer" badges. Verified via agent-browser.

## Pre-existing warnings (unrelated, noted per guideline)

- creaves `staticcheck`: `ST1005` capitalized error strings (cares,
  logentries, configs), `U1000` unused (animal_audit, app,
  event_processor_test) — untouched files.
- creaves gocognit/gocyclo: `buildEventPayloadInto` 81/43,
  `AnimalsResource.Update`, `EnrichAnimalsOptimized`, `parseValueTuples` …
  — pre-existing; `deliverBatch` remains at its HEAD baseline (33).
- creaves `-tags sqlite -race` full suite: failures pre-existing at HEAD
  `ffcfef1` (verified via a clean worktree); the guideline suite (no sqlite
  tag) passes.
- gofmt drift in `actions/configs.go`, `actions/config_templates_test.go`,
  `actions/event_processor_test.go` (creaves) — untouched.
- creaves-console `SyncManagementIndex` gocyclo 14 / gocognit 16 and
  `UpdateFromPayload`, `DashboardIndex`, `installSafePopTxLogger` —
  pre-existing, untouched.
