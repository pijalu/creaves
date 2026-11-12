# Plan: webhook delivery partial acceptance ("webhook accepted 97/100 events")

Status: implemented (see `docs/archive/webhook-partial-accept.md`).

## Problem

During full resync bursts the Console receiver can reject a subset of a batch
(validation failures, upstream conflicts). The producer logged
`webhook accepted 97/100 events` while the delivery bookkeeping treated the
response as all-or-nothing: either every event was marked delivered (silent
data loss on the Console) or none was (duplicate resends of accepted events).

## Contract to implement

1. Parse the receiver response fully: `processed`, `total`, `processed_ids`,
   `errors` — not just `processed_ids`.
2. `processed < total` or non-empty `errors` ⇒ partial response: mark ONLY the
   IDs listed in `processed_ids` as delivered; leave the rest pending.
3. Return an explicit error (`webhook accepted %d/%d events`) so the delivery
   loop and logs surface the partial acceptance.
4. A partial response records a circuit-breaker failure (deliberate: a
   receiver degrading to accepting nothing must open the circuit instead of
   being hammered every wake).
5. Compatibility, without reopening the original bug:
   - empty body on 200 ⇒ legacy all-accepted;
   - `processed == total == len(events)` with no errors and no ids ⇒ all
     accepted (the documented Console full-success shape);
   - anything else without ids ⇒ nothing marked delivered (retry, never
     silently drop).
6. Accepted-but-unmarked events keep `delivered_at IS NULL` and are retried on
   the next tick / wake — retries are idempotent on the Console (same event
   id).

## Tests

- `TestDeliverBatch_PartialFailureMarksOnlyAccepted` — subset accepted ⇒ only
  accepted events delivered, rejected stay pending.
- `TestDeliverBatch_ExplicitPartialResponseDoesNotDeliver` — explicit
  `processed < total` with no ids ⇒ error, nothing delivered.
- `TestDeliverBatch_FullCountResponseMarksEventsDelivered` — full-count
  success marks all delivered.
- `TestDeliverBatch_PartialAcceptRecordsBreakerFailure` — partial response
  records a breaker failure; full success resets it.

## Validation

`CGO_ENABLED=1 go test -tags sqlite ./actions ./models -count=1` — pass.
