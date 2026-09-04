# Event stream record status fix (archive)

Status: implemented.

## Problem

Event records remained marked `Delivered No` because the Console success response reported `processed`/`total` counts without `processed_ids`; producer accepted no IDs and never persisted `delivered_at`.

## Fix

Full-count successful responses now mark all submitted events delivered; partial responses still require explicit processed IDs.

## Validation

- `go test ./actions -count=1` — pass.
- Regression test added for `processed == total` response.
