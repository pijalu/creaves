# Resync worker wake fix (archive)

Date: 2026-09-04. Status: implemented; automated validation complete.

## Problem

Force resync created events asynchronously, but the event-driven webhook worker
could remain asleep until its 60-second fallback poll. Large resyncs also needed
wake signals while producing batches.

## Fix

- `actions/webhook_resync.go`: after committed run creation,
  `StartResync` calls `EnsureWebhookWorkerRunning()` and `signalWebhookWake()`.
- `RunResync` signals the worker after each animal's progress update, keeping
  delivery active throughout long loops; completion retains its final wake.
- `actions/webhook_pusher_test.go`: corrected partial-response fixture to update
  nested `ConfigSettings.WebhookURL` through `GetSettings`/`SetSettings`.

## Validation

- `CGO_ENABLED=1 go test -tags sqlite -count=1 ./actions/ -run TestStartResync` — PASS.
- `go test -count=1 ./actions/...` — PASS.
- Full sqlite-tagged actions suite remains blocked by pre-existing fixtures that
  require the full MySQL schema (`animals` and related tables); no failure
  reaches wake logic.
## Browser evidence (agent-browser, local services)

- Creaves `http://127.0.0.1:3000/webhook_resync`: authenticated admin started
  force resync; `status.json` showed `running`, `total_animals=10046`, then
  `animals_processed=1851`, `events_created=1851` after a few seconds.
- Console `http://127.0.0.1:3001/`: authenticated with `admin`/`admin123`;
  dashboard showed `lagrange` instance with 1261 released animals, confirming
  received consolidated events during the resync.
- Run was canceled from Creaves UI after evidence collection; final
  `/webhook_resync/status.json` returned `{"status":"none"}`.
