# Event stream — DLQ category, reset attempts, attempts visibility (2026-09-22)

Commit: `4ae53e4` (branch `feature/open-issues-2026-10`, pushed). Direct user
request (no GitHub ticket).

## Request

1. Reset button on event stream to reset delivery attempts.
2. Number of attempts visible.
3. Events that hit the max attempts categorized separately, similar to a DLQ.

## Root context

- `MaxDeliveryAttempts = 25` (`models/event_delivery.go`). Delivery state is
  tracked per (event, target) in `event_deliveries`. Events with all pending
  rows at the cap are silently skipped by the pusher (`attempts < cap`
  filter). Only recovery was the per-target "retry undeliverable" on
  `/sync_configuration`.
- Legacy rollup columns `event_streams.delivery_attempts /
  last_delivery_error` have **no writer anymore** (always 0/NULL) — the old
  index badge was dead.

## Changes

### Backend (`creaves/actions/event_streams.go`)
- `eventStreamRow`: decorates `EventStream` with `Attempts` (max over
  delivery rows) + `Blocked` (any pending row at cap), filled by
  `listEventsSelect` (LEFT JOIN aggregate subquery over `event_deliveries`).
- `deliveryFilterClause("undeliverable")` → EXISTS clause (pending row at
  cap) — the DLQ.
- `List`: runs raw SQL with filter/order embedded (see fix below), exposes
  `dlqCount` for the filter-button badge.
- `Show`: loads per-target delivery rows (`eventStreamDeliveryView`: target
  name, attempts, delivered/acked timestamps, last error).
- `ResetAttempts` (POST `/event_streams/{id}/reset_attempts`): resets
  attempts/last_error on the event's pending delivery rows.
- `ResetUndeliverable` (POST `/event_streams/reset_undeliverable`): same for
  every DLQ row.
- Shared `resetPendingDeliveries` helper: collects affected target IDs,
  resets rows, `webhookPusher.forgetTarget` per target (reopens circuit
  breakers), `EnsureWebhookWorkerRunning()` + `signalWebhookWake()`.

### Routes (`creaves/actions/app.go`)
Both POST routes registered **before** `app.Resource("/event_streams", ...)`
so `reset_undeliverable` is not swallowed by the resource's `{id}` segment.

### Templates (all 4 locales, index + show)
- Index: red "Undeliverable (DLQ) (n)" filter button; red alert with
  "Re-queue all" bulk button when the filter is active and count > 0;
  attempts badge now driven by `event.Blocked`/`event.Attempts`; yellow
  reset (fa-redo) button per blocked row.
- Show: "Deliveries per target" table (target, attempts badge, status
  badge Delivered/Acked/DLQ/Pending, error row), reset button when the
  event is undelivered.
- Templates are i18n-key driven; en-US version copied to fr/de/nl after
  verifying the only diffs were the newly added blocks.
- New i18n keys in `locales/event_streams.{en-us,fr,de,nl}.yaml`
  (`filter-undeliverable`, `dlq-hint`, `reset-dlq`, `reset-dlq-confirm`,
  `reset-attempts`, `reset-confirm`, `show.deliveries/target/attempts/
  delivery-status/acked/pending/no-deliveries`).

## Issues found during implementation

1. **pop `RawQuery` ignores `.Where()`/`.Order()`** in raw SQL mode — first
   List version silently returned unfiltered rows (proved via scratch test
   against `creaves_test`: 300 unfiltered rows). Filter + order now embedded
   in the SQL string; `deliveryFilterClause` only emits code constants, so
   no injection surface.
2. **Dead rollup column** — index badge rewired to `event_deliveries`
   aggregates (see root context).
3. **Pusher alive during handler tests** — a seeded DLQ row on an enabled
   target was consumed mid-assertion by the running worker. The list test
   seeds its row against a **disabled** target.

## Validation

- Go tests (new `actions/event_streams_reset_test.go`, MySQL suite):
  clause mapping incl. cap constant; per-event reset (attempts→0, error
  cleared, 404 path); bulk reset leaving below-cap rows untouched; DLQ
  filter lists/hides the seeded event and exposes `Blocked`/`Attempts` in
  JSON. All pass.
- Quality gates (each run separately): `go vet ./...` clean,
  `staticcheck ./...` clean, `gocognit -over 15` / `gocyclo -over 12` no
  hits in `event_streams.go`,
  `GO_ENV=test go test -count=1 -race -cover ./actions/` ok (51.2%).
- e2e (agent-browser, dev DB, seeded DLQ event 980481 / instance dlq-e2e
  with capped row on "Main console", then cleaned up):
  - `/event_streams?delivery=undeliverable`: red DLQ button "(1)" active,
    "Re-queue all" alert, row `25 — Blocked` badge, reset/view/delete
    buttons. Screenshots: /tmp/dlq_index_en.png, /tmp/dlq_empty.png.
  - Show page: "Deliveries per target" with Main console `25 — Blocked` +
    DLQ status + dial-tcp error row; yellow reset button.
    Screenshot: /tmp/dlq_show_en.png.
  - Locales: fr "Non livrables (DLQ) (1)", de "Unzustellbar (DLQ) (1)",
    nl "Onleverbaar (DLQ) (1)", en "Undeliverable (DLQ) (1)".
    Screenshot: /tmp/dlq_index_fr.png.
  - Reset click → flash "1 deliveries re-queued" → show page attempts 0 /
    Pending; DLQ list empty, button "(0)"; deliveries table then showed
    live retry attempts from the pusher (console up, event rejected as
    fake animal — expected).
  - Regression: delivered/not-delivered/all filters still return rows.
  - Browser confirm() dialog had to be stubbed (`window.confirm = () =>
    true`) — agent-browser's auto-dialog dismissal raced the data-confirm.

## Residual risk

- `event_streams.delivery_attempts` / `last_delivery_error` columns remain
  in the schema (unused); dropping them is a destructive migration — out of
  scope per session constraints.
