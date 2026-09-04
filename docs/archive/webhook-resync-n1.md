# Webhook resync N+1 fix (archive)

Date: 2026-09-04. Status: implemented; automated validation complete.
Plan: `PLAN_WEBHOOK_RESYNC_N1.md` (this repo).

## Problem

Full resync (`RunResync`) and the `/webhook_resync` sync-status panel
(`ComputeSyncStatus`, recomputed on every page load) flooded the SQL log with
per-record SELECTs — ≈ 59 SELECTs per animal:

- pop `Eager()` mode loads associations **per record** (8 SELECTs/animal);
- payload builder issued 48 translation SELECTs + 1 species SELECT per animal;
- per-animal EXISTS check for the deterministic state event;
- per-animal run re-read + progress UPDATE.

10 000 animals ≈ 590 000 log lines per resync.

## Fix

- `actions/webhook_resync.go`, `actions/sync_status.go`: `Eager()` →
  `EagerPreload()` (pop's batched `id IN (...)` association loading).
- `actions/event_translations.go`: `translationPreloader` — one translation
  query per (reference table, field, locale) group + one species scan **per
  run**, replacing the per-animal lookups; `buildEventPayloadInto` core builder
  (per-animal path preserved verbatim for single-event publishes).
- `actions/webhook_resync.go`: `resyncStateIndex` replaces the per-animal
  EXISTS with one query + in-memory adds (single-writer: concurrent resyncs
  are refused; no other code path creates `animal_state` events);
  `resyncCheckpoint` throttles progress UPDATE + cancel re-check to every 25
  animals (context cancellation still honoured per iteration).
- Force requeue (`UPDATE ... delivered_at = NULL`) and event creates remain
  per-record writes — they are the resync's product.

Measured (SQLite harness, 60 animals): 535 → 63 SELECTs after batching; the
remaining count is a fixed budget independent of animal count (old: ~3540).

## Validation

- `CGO_ENABLED=1 go test -tags sqlite -count=1 ./actions/` — PASS.
  - New `TestRunResyncQueryCountBounded`: SELECT-count regression guard.
  - New `TestTranslationPreloaderEquivalence`: batched payload ≡ per-animal
    payload (translations + taxonomy), `reflect`-level equality.
  - Existing regressions (nested associations in state events, skip vs force
    requeue, disabled webhook, sync-status checksums/years) all PASS.
- `go test -count=1 ./actions/ ./models/` (default build, MySQL) — PASS.
- `go build ./...`, `go vet ./actions/ ./models/`, `gofmt` — clean.
- Not re-run in a browser this round (no template changes; the resync page
  render path is covered by `TestWebhookResyncIndexSyncPanelRenders`).

## Residual notes

- Species taxonomy lookups cannot execute on the SQLite harness (column named
  `order`, unquoted by the sqlite dialect); both old and new paths fail there
  equally — pre-existing harness limitation, production MySQL unaffected.
- `grifts/event_snapshot.go` still publishes per animal (one-off CLI task,
  out of scope).
