# 2026-09-20 — Multi-hub sync: sync_targets table + per-target delivery tracking

Resolved workspace `bugs.md` item #1 ("Sync — permanently rejected events must become
'undeliverable' and be retryable") as part of a larger feature on branch
`feature/open-issues-2026-10` (creaves repo): one Creaves instance can now push its
event stream to **multiple** Creaves Console instances, each with its own delivery
state, circuit breaker and rate limit.

## Design decisions (vs. the literal bug text)

The bug report asked to mark receiver-rejected events undeliverable **immediately**.
The implemented semantics is **retry with a per-target attempt budget** instead:
a partial accept (HTTP 200, event missing from `processed_ids`) is often transient
(receiver busy, one event momentarily invalid), so rejected events stay PENDING and
are retried on the next tick; the per-target attempt counter caps at
`models.MaxDeliveryAttempts = 25`, after which the event is **undeliverable for that
target** and drops out of the selection (poison-message backstop). Only HTTP
401/403 (bad API key — retrying cannot succeed) marks a batch undeliverable
immediately. The admin **"Retry undeliverable"** action resets attempts and clears
the stored error, re-queueing the events — the user-facing outcome the bug asked for.

## What changed

### Schema (additive migrations, prod-safe)
- `sync_targets`: id, name, enabled, webhook_url, webhook_api_key,
  webhook_batch_size, webhook_max_per_min, timestamps. Backfilled from the legacy
  config webhook settings (one "Default" target when a webhook was configured).
- `event_deliveries`: id, event_id, target_id, attempts, delivered_at,
  acknowledged_at, last_error (NULLable), timestamps. Backfilled from
  `event_streams.delivered_at` (one delivered row per already-delivered event and
  the backfilled target).
- Config settings keep only global `InstanceID` + `EnableEventStream`; the legacy
  `webhook_*` keys in the settings blob are ignored on read and no longer written.

### Pusher (`actions/webhook_pusher.go`)
- Fan-out: `deliverPendingBatch` iterates all enabled targets; per-target circuit
  breaker + per-minute rate limiter; one slow/failing target does not block others.
- Per-target selection `pendingEventsForTarget`: events with no delivered delivery
  row for that target and attempts < cap, oldest first. New targets therefore
  replay the full backlog (console onboarding) at their own pace.
- `event_streams.delivered_at` / `acknowledged_at` are **rollups**: set only when
  every enabled target has delivered/acknowledged the event.
- Rejected events (partial accept) → `recordDeliveryFailures` (attempts+1, error
  stored), retried next tick; 401/403 → `markBatchUndeliverable` immediately.
- **E2E-found fix**: a 200 response with an unparseable body (contract violation,
  e.g. `"confirmed": true` where an array is expected) previously returned an
  error **without** recording attempts — the batch retried forever, bypassing the
  poison backstop. Both the body-read-error and unmarshal-error paths now call
  `recordDeliveryFailures`. Regression test:
  `TestDeliverBatch_UnparseableResponseIncrementsAttempts`.

### Admin UI (`/sync_configuration`, 4 locales via per-locale plush templates)
- Global section: Instance ID + Enable Event Stream (unchanged semantics).
- Sync Targets table: Name, URL, Status, Pending / Delivered / Undeliverable /
  Unconfirmed counts, actions Edit / Delete / **Retry undeliverable** (visible
  only when the target has undeliverable events; POST +
  `data-confirm`; JSON `{"requeued": N}` or flash redirect).
- New/Edit forms: name, enabled, URL, API key, batch size, max/min (clamped
  1..100 / 1..9999 server-side). API-key policy preserved: maintainers see the
  stored key (type=text), plain admins write-only (blank submit keeps the stored
  key); `MaskedAPIKey` renders `••••` + last4.
- Target CRUD wakes the delivery worker when a target becomes deliverable.

### Console contract
Unchanged: `contract_version: 2` payload shape and response semantics
(`processed/total/processed_ids/errors/confirmed`). Verified against two mock
receivers implementing the documented response.

## Tests

- **MySQL suite** (`GO_ENV=test go test ./actions/ ./models/`): green.
  New handler tests in `actions/sync_targets_test.go` (create clamping, 422 on
  enabled-without-URL, blank-key preservation, key hidden from plain admin,
  destroy cascades deliveries, retry-undeliverable JSON).
- **Sqlite pusher suite** (`CGO_ENABLED=1 go test -tags sqlite ./actions/`):
  failure set identical to HEAD baseline (24 pre-existing). New fan-out tests:
  `TestFanOut_DeliversToEveryEnabledTarget` (2 enabled + 1 disabled target,
  per-target rows, rollup), `TestFanOut_RollupWaitsForSlowTarget`, poison-queue
  block rewritten against `event_deliveries`, plus the unparseable-response
  regression test.
- **Race**: `go test ./actions/ -race` green. Found and fixed during hardening:
  handler tests that create enabled targets wake the real worker, which shares
  `models.DB` (pop `*Connection` is not goroutine-safe) with later HTTP tests;
  `StopWebhookWorker()` (synchronous) now runs in those tests' cleanup and in
  the resync-commit test.

## E2E evidence (agent-browser, dev instance on :3000, 2 mock consoles :3101/:3102)

1. Login admin/admin → `/sync_configuration` shows Instance Identity + Event
   Stream + Sync Targets ("Default" → console :3001, 2 undeliverable).
   Screenshots `/tmp/e2e_sync/step1_login.png`, `step2_page.png`.
2. Created "Console Alpha" (:3101, key-alpha) and "Console Beta" (:3102,
   key-beta) via the UI — both listed with zeroed stats.
   Screenshots `step3_alpha.png`, `step4_beta.png`.
3. Edit Alpha: maintainer sees stored key `key-alpha` in plain text; blank-key
   submit + max/min 60→30 → DB shows key preserved, 30 stored.
   Screenshot `step5_edit.png`.
4. **Fan-out**: animal 10233 species edit → event `abd53508-…` → delivered to
   all three targets (per-target `event_deliveries` rows, attempts=1), rollup
   `delivered_at` set only after the last target delivered; both mock receivers
   logged the event. (First attempt also demonstrated the unparseable-response
   infinite-retry bug — mock returned `"confirmed": true` — fixed above.)
5. **Retry undeliverable** (bugs.md #1): Default had 2 events at attempts=25
   ("instance block mismatch" from the real console). Click Retry →
   attempts reset to 0, `last_error` cleared, worker retried on the next tick
   (attempts=1, console still rejects — correct). Screenshot `step6_retry.png`.
6. **Locales**: page verified in EN, FR ("Cibles de synchronisation",
   "Non livrables"), DE ("Synchronisationsziele", "Unzustellbar"), NL
   ("Synchronisatiedoelen", "Onleverbaar"); new-target form in NL
   ("Nieuw synchronisatiedoel"). Screenshot `step7_fr.png`.
7. Cleanup: Alpha/Beta targets + their delivery rows removed from dev DB;
   animal species restored; mock receivers stopped.

## Quality gates
- `go vet ./actions/ ./models/` clean; `staticcheck` clean (removed an unused
  const alias); `gofmt` clean on all touched files.
- `gocognit -over 15`: reworked `deliverTargetBatch` 42 (baseline `deliverBatch`
  38 — same class, well below the file's pre-existing 109/79/62 hotspots).
- `gocyclo -over 12`: 38 vs baseline 36.
- `go test -count=1 -race -cover ./actions/ ./models/` green (models 69.8%).

## Notes / open items for the user
1. The 2 stale dev events (animal 10233, old instance `BigMac.local`) remain
   undeliverable for "Default" — the console on :3001 legitimately rejects them
   ("instance block mismatch"). Discard or fix the console's instance registry.
2. New sync targets replay the **entire** event backlog at their configured
   rate (default 60/min ≈ 3h for 10k events). That is the intended console-
   onboarding path; raise `webhook_max_per_min` for faster catch-up.
