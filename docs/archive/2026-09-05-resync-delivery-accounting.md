# Fix archive — 2026-09-05 — Webhook resync silently loses events (partial delivery)

## Original report (bugs.md #1, CRITICAL)
> ## creaves: Webhook resync silently loses events (partial delivery) — CRITICAL
> Resync run `28b27188` reports `status=completed`, `events_created=10046`,
> `errors=""`, but the console received only **5327** events (2025/2026 never
> delivered, 2024 partial). The old "webhook accepted 97/100 events" delivery
> error was invisible to the run status.

**Expected:** a completed resync means every event was delivered and accepted;
partial delivery must retry and, if impossible, mark the run failed with
per-batch diagnostics.

## Root cause
`RunResync` marked the run `completed` right after **enqueuing** events —
delivery was a fire-and-forget job of the background webhook worker. Partial
batch accepts (receiver answered `processed_ids` shorter than the batch) left
those events pending, the worker's drain loop stopped on the delivery error
and the circuit breaker opened; nothing ever tied the undelivered events back
to the run, so the run stayed green while events were silently lost.

## Fix (creaves)
- **Additive schema change** (`20260906000000_add_resync_delivery_accounting`):
  `resync_runs.events_delivered`, `resync_runs.events_failed` (int, default 0).
  `migrations/schema.sql` updated.
- **`actions/webhook_resync.go`**: after production the run enters
  `completeResyncDelivery` (`resyncDeliveryRunner`): it counts the run's
  events (`resync_run_id` attribution — force-mode re-queues now re-point
  `resync_run_id`), persists live delivered/failed counters, drives
  `deliverBatch()` until every event is accepted or
  `resyncDeliveryMaxStalled` (5) consecutive attempts show no delivery
  progress. Partial accepts (97/100) are retried automatically (rejected
  events stay `delivered_at IS NULL` and are re-picked).
  - all delivered (or nothing created) → `status=completed`
  - stall bound exhausted → `status=failed` with diagnostic
    `"delivery incomplete: X of Y events accepted; Z events not delivered
    after N stalled attempts"` appended to the run's structured error list
    (production errors preserved).
  - cancellation (user or context) still honoured during delivery wait.
- **`actions/webhook_resync_handlers.go`**: `/webhook_resync/status.json`
  now reports the latest run **regardless of status**, so a failed run stays
  visible on the page after the worker exits (it used to vanish).
- **`templates/webhook_resync/index.plush*.html`** (en/fr/de/nl): the poller
  renders `Delivered` / `Failed` counters and, for failed runs, a red alert
  (`#resync-failure`) with the diagnostic; run-provided strings are
  HTML-escaped.
- **`models/resync_run.go`**: `EventsDelivered` / `EventsFailed` fields with
  JSON tags (surfaced through `status.json` automatically).

## Tests
- `actions/webhook_resync_delivery_test.go` (sqlite-tagged, httptest stub
  receiver): partial accept is retried and the run completes
  (`TestResyncDeliveryRetriesPartialAccept`); permanent rejection marks the
  run failed with 3 delivered / 1 failed + diagnostic
  (`TestResyncDeliveryHardFailMarksRunFailed`); no configured receiver marks
  failed, never completed (`TestResyncDeliveryNoReceiverMarksRunFailed`);
  nothing-created run completes (`TestResyncDeliveryNothingCreatedCompletes`).
- `actions/webhook_resync_ui_test.go`: all four locale templates expose the
  counters + failure alert; the `status.json` payload carries
  `events_delivered` / `events_failed`.
- `TestRunResyncQueryCountBounded` extended: a completed run must have
  delivered every created event; SELECT bound widened
  `animalCount+50` → `+80` for the fixed accounting queries.

## Validation
- `go vet ./...` clean.
- `staticcheck ./...`: 49 findings, all pre-existing (ST1005 capitalized
  error strings, U1000 unused funcs) — none in the changed code.
- `gocognit -over 15 .`: only pre-existing offenders; the new delivery code
  was refactored (`resyncDeliveryRunner.iterate`) to stay within limits.
  Pre-existing noted: `WebhookResyncStart` (17),
  `TestRunResyncQueryCountBounded` (21 at HEAD).
- `gocyclo -over 12 .`: no new findings.
- `go test -count=1 -race -cover ./...` (MySQL) and
  `CGO_ENABLED=1 go test -tags sqlite ./...`: green.
- Browser check (agent-browser, NL locale, dev server): failed run renders
  `Status: failed … Geleverd: 5327 Mislukt: 4719` plus the red
  `#resync-failure` alert with the diagnostic; completed run renders
  `Status: completed … Geleverd: 10046 Mislukt: 0` without an alert.
- Full end-to-end resync against the dev console (fix-plan validation step 5:
  console `event_streams` = 10046) depends on bug #2 (consolidation) and is
  covered by the follow-up goal; the delivery contract itself is pinned by
  the stub-receiver tests above.

## Notes
- The delivery loop bypasses the circuit breaker and rate limiter on purpose
  (bounded by the stall limit instead); deliveries stay idempotent
  (deterministic UUIDs + console upsert).
