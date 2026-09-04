# Event stream record UI: delivered column, delivery filters, locally-processed clarification (archive)

Status: implemented.

## Problem

- The event stream listing showed only a "Processed" column; webhook delivery state (`delivered_at`) was invisible at a glance and only visible on the record page.
- No way to list only undelivered events (the ones a broken receiver/webhook left behind).
- "Locally Processed" staying "No" looked like a bug: `processed_at` is only set by the local consolidation run (`cmd/consolidation`, `grifts/consolidation.go`), never by the normal web flow, and nothing in the UI explained that.

## Fix

- `EventStreamsResource.List` accepts a `delivery` query param: `all` (default, no filter), `not-delivered` (`delivered_at IS NULL`), `delivered` (`delivered_at IS NOT NULL`). Unknown values fall back to no filter.
- Listing gained a Delivered Yes/No column and a three-button filter group (All / Not delivered / Delivered) with active state.
- Record page: "Locally Processed" row now carries a hint ("Set only by the local consolidation run; independent of webhook delivery."), and the listing column label was renamed from "Processed" to "Locally Processed".
- Locales updated: en-US, de, fr, nl (templates are translation-key driven and shared).

## Validation

- `go test -tags sqlite ./actions -run 'TestDeliveryFilterClause|TestEventStreamsIndexDeliveryFilterRenders|TestEventStreamsShowLocallyProcessedHintRenders' -count=1` — pass (param mapping + button group render/active states + hint render).
- `go test ./actions ./models -count=1` — pass.
- `go test -tags sqlite ./actions -count=1` — pass.
- `go build ./...` — pass.
