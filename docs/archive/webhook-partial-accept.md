# Webhook delivery partial acceptance fix (archive)

Status: implemented.

## Problem

During full resync the Console can accept a subset of a pushed batch (e.g. 97
of 100 events) and report `processed: 97, total: 100, processed_ids: [...]`.
The producer logged `webhook accepted 97/100 events` but the bookkeeping was
all-or-nothing, so the accepted subset was not reliably marked delivered and
the rejected events were not reliably retried.

## Fix

`deliverBatch` now parses the full receiver response (`processed`, `total`,
`processed_ids`, `errors`):

- partial (`processed < total` or `errors` non-empty): mark ONLY listed
  `processed_ids` delivered, leave the rest `delivered_at IS NULL` for retry,
  return `webhook accepted %d/%d events`, record a circuit-breaker failure;
- full-count success (`processed == total == batch size`, no errors, no ids):
  mark all delivered;
- legacy empty body on 200: mark all delivered;
- any other response without ids: deliver nothing (retry, never drop).

Retries are idempotent on the Console (event id dedup), so re-sending the
unaccepted subset is safe.

## Validation

- `CGO_ENABLED=1 go test -tags sqlite ./actions ./models -count=1` — pass.
- Regression tests: `TestDeliverBatch_PartialFailureMarksOnlyAccepted`,
  `TestDeliverBatch_ExplicitPartialResponseDoesNotDeliver`,
  `TestDeliverBatch_FullCountResponseMarksEventsDelivered`,
  `TestDeliverBatch_PartialAcceptRecordsBreakerFailure`.
