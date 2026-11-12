# Combined Setup & Testing

This guide covers a local Creaves → Creaves Console webhook environment. Project-specific commands and configuration are in each repository's `README.md`; this page only covers running the two systems together and validating their contract.

## 1. Topology

```text
Creaves center :3000 ── POST /webhook/events ──▶ Creaves Console :3001
       MySQL creaves                                  MySQL consolidation
```

The applications share neither code nor a database. Creaves produces and retries events; Console authenticates, stores, synchronously processes, and acknowledges them.

## 2. Prerequisites

- Go **1.26** for both modules.
- Buffalo CLI: `go install github.com/gobuffalo/cli/cmd/buffalo@latest`.
- MySQL 8 or MariaDB.
- Node.js/Yarn for Creaves assets.
- CGo/C compiler for Console SQLite tests.
- `goyacc` when regenerating the Creaves Care DSL parser.

Use isolated databases. Never point a fixture, test, cleanup, or recovery run at production.

## 3. Start Console first

From `creaves-console/`:

```bash
go mod download
buffalo pop migrate up
buffalo task db:seed
buffalo dev
```

Console listens on `http://127.0.0.1:3001`. Initial development login: `admin` / `admin123`.

In **Admin → Webhook API Keys**, create a key whose instance ID exactly matches the center configured in Creaves. Copy the generated raw key.

## 4. Start Creaves

From `creaves/`:

```bash
go mod download
yarn install --frozen-lockfile
go generate ./models/careplan
buffalo pop migrate up
buffalo task db:seed
buffalo dev
```

Creaves listens on `http://127.0.0.1:3000`. Initial development login: `admin` / `admin`.

In **Administration → Synchronization**:

1. Set the same instance ID used for the Console key.
2. Enable the event stream.
3. Add an enabled target with URL `http://127.0.0.1:3001/webhook/events` and the generated API key.
4. Review batch size and per-minute rate limit, then save.

The delivery worker starts at boot and wakes on publication/target save. A 60-second fallback tick retries undelivered events.

## 5. Validate live delivery

1. In Creaves, create a discovery/intake for a test animal.
2. Confirm an `animal_discovered` event was produced.
3. Confirm the target acknowledges it and the event is marked delivered.
4. In Console, open the instance, consolidated animal, and source-event drill-down.
5. Record an alive outtake and verify `released`; use another test animal for a dead outtake and verify `died`.
6. Repeat the same delivery and verify idempotent processing rather than duplicate animals or applied side effects.

Also test an invalid key and an instance mismatch. Failed events must not be acknowledged.

## 6. Validate full resync

Use the current [resync runbook](../e2e/RESYNC_RUNBOOK.md). In short:

1. Start a resync from Creaves **Administration → Webhook resync**.
2. Poll `/webhook_resync/status.json`; do not start a second run while one is active.
3. Confirm deterministic `animal_state` events, instance metadata, expected totals, and state-hash acknowledgements.
4. Confirm Console's announced and processed totals agree and no unprocessed events remain.
5. A second resync should skip unchanged snapshots.

For cleanup/recovery, use [Console's recovery walkthrough](../../creaves-console/docs/e2e-cleanup-resync-walkthrough.md). Cleanup and rebuild are destructive and require an isolated backup first.

## 7. Automated tests

Run from each repository independently:

```bash
# Creaves: MySQL test database, including creaves_test
cd creaves
GO_ENV=test go test -count=1 ./...

# Console: SQLite, CGo and build tag required
cd ../creaves-console
CGO_ENABLED=1 go test -tags sqlite -count=1 ./...
```

Focused contract examples:

```bash
cd creaves
GO_ENV=test go test -count=1 ./actions -run 'TestDeliverBatch|TestCircuitBreaker' -v

cd ../creaves-console
CGO_ENABLED=1 go test -tags sqlite -count=1 ./actions -run 'TestWebhook|TestE2E|TestProcessEvent' -v
```

Do not use Console's SQLite flags as a substitute for the Creaves MySQL suite. Run `go vet ./...` in both modules when code changes.

## 8. Browser and localization checks

For template, route, action, model, or schema changes, use authenticated browser sessions and verify actual rendered content and network errors. Test all affected locales: `en-US`, `fr`, `de`, and `nl`.

Minimum integration smoke set:

| Application | Routes/workflows |
| --- | --- |
| Creaves | Login, dashboard, animal detail, intake/outtake, day plan, Protocol editor, Synchronization, Webhook resync |
| Console | Login, dashboard, instances, consolidated animal, drill-down, reports, API keys, Sync Management |

Check both success and failure paths, including CSRF/auth failures and partial webhook responses. Record actual commands and results; old archived E2E reports are not current validation.

## 9. Troubleshooting

| Symptom | Check |
| --- | --- |
| No event rows | Event stream enabled and lifecycle action actually committed |
| Event not delivered | Target enabled, worker/log output, URL, credentials, rate limit, circuit breaker |
| HTTP 401 | Wrong/inactive API key |
| HTTP 403 | Key and event/envelope instance IDs differ |
| Partial acceptance | Only IDs in `processed_ids` were acknowledged; leave all others retryable |
| Console has events but stale rows | Inspect processing errors, then run `buffalo task consolidation:process` on Console |
| Resync stalls | Inspect status endpoint and logs before starting another run |
| Checksum mismatch | Compare producer announcement with Console Sync Management; use reviewed resync recovery |
