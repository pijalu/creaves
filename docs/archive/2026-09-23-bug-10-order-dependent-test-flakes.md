# Bug 10 — test suite: order-dependent flakes from shared webhook-worker state

**Date**: 2026-09-23
**Status**: RESOLVED — commit [`c4270f1`](../../creaves) "tests: stop order-dependent flakes from webhook worker leak + readTimeout kills" (pushed to `feature/depupdate`)

## Observed

Full `go test -count=1 -race ./...` runs failed with a **varying** set of tests while
every affected test passed in isolation:

- Run 1: event-stream reset ×3, remap ×2, reference delete, sync-target retry — all 403.
  Root: mw-csrf v1.0.2 no longer reads the token from the query string nor exempts JSON
  bodies. Fixed in commit `4f50806` (tests send `X-CSRF-Token` header).
- Run 2: `TestReportsNavShowsExportsEntry`, `TestTodosIndexOrderingAndDashboard`,
  `TestExportCsvUTF8Encoding`, `TestExportCsvHonorsViewFilters`, `TestExportViewYearFilter`,
  `TestExportExcelYearFilter`.
- Run 3 (post partial fixes): `TestExportExcelRegistrePivotCache` — new member, same family.

Verified pre-existing on the pre-fix baseline commit `2966833` (via `git worktree`).
Evidence signatures in failing runs: `level=error ... "database error on committing or
rolling back transaction: invalid connection"`, `[mysql] packets.go:58 read tcp
127.0.0.1:3306: i/o timeout`, dashboard `db=3.07s ... status=500`.

## Root causes (two, independent)

### 1. Webhook delivery worker outliving its test

`EnsureWebhookWorkerRunning` is called from many handler paths (event publish,
sync-target CRUD, DLQ reset/retry, resync). Test cleanups called `StopWebhookWorker`,
but left `syncTargetsKnown=true` — so any later test whose handlers publish an event
silently **restarted** the worker. The orphaned goroutine then queried the shared
`models.DB` pop connection every wake/60s tick (`purgeExpiredEvents` +
`EnabledSyncTargets`), concurrent with later tests' requests → the
`invalid connection` / i/o-timeout 500s landed on whichever heavyweight test was
running (varying set).

### 2. Test-DSN `readTimeout=3s` under water

`creaves_test` carries production-scale fixture data (~10k animals). The
export/dashboard queries (DISTINCT + ORDER BY + 4-5 joins) legitimately exceed 3s
under `-race` + parallel package load (e.g. `stat_communes` took 13.3s in-suite).
go-sql-driver kills a connection when a single read exceeds the deadline; the killed
connection then surfaces as `invalid connection`/500 in whichever request reuses it.
This also explains the earlier experiment where a 10s bump turned fast 500s into
10s hangs — the worker leak (cause 1) was still active then and pushed queries past
any margin; with cause 1 fixed, the timeout margin itself became the remaining killer.

## Fix (commit `c4270f1`)

1. `actions/webhook_pusher.go` — new `webhookWorkerAutoStartDisabled` guard
   (`atomic.Bool`, default false) checked in `EnsureWebhookWorkerRunning`. Production
   is unaffected (flag never set outside tests); suites that assert worker behaviour
   start it explicitly via `StartWebhookWorker`.
2. `actions/testmain_mysql_test.go` — TestMain sets the guard, purges stale
   `event_streams`/`event_deliveries` rows once at suite start, and stops the worker
   after `m.Run()`.
3. `actions/sync_targets_test.go` — `seedHandlerSyncTarget` cleanup additionally
   resets `SetSyncTargetsKnown(false)`.
4. `database.yml` — test DSN `readTimeout` 3s → 60s (dev/production DSNs unchanged;
   `creaves_test` is disposable by contract).

## Validation

- Full suite **twice consecutively green**: `GO_ENV=test go test -count=1 -race ./...`
  → all packages `ok` (`/tmp/creaves_bug10_runA.log`, `/tmp/creaves_bug10_runB.log`,
  ~215s each, 0 failures).
- Targeted trigger→victim ordering green (sync-target/reset tests followed by
  export/dashboard tests).
- Quality gates (run separately): `go vet ./...` clean, `staticcheck ./...` clean,
  `gocognit -over 15` / `gocyclo -over 12` show only pre-existing hotspots (none in
  changed code), sqlite-tagged suite compiles (`go vet -tags sqlite ./actions/`).

## e2e note

No UI or product behaviour change (production code path is guarded by a default-false
flag; only the disposable test DB's DSN changed), so browser e2e via agent-browser is
not applicable. Suite-level validation above stands in as the fix's acceptance
evidence, per the bug's own expected-behaviour criterion ("passes repeatedly ≥2
consecutive runs regardless of test order").
