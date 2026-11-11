# Setup & Manual Testing Guide

This guide covers both projects in the webhook event-forwarding architecture:

| Project      | Role              | Module path      |
|--------------|-------------------|------------------|
| `creaves`    | **Pusher** (source) — emits events to the console via webhook | `creaves` |
| `creaves-console` | **Receiver** (sink) — ingests + consolidates events | `creaves-console` |

```
creaves (pusher)  ──HTTP POST /webhook/events──▶  creaves-console (receiver)
   event_streams.delivered_at                         event_streams.imported_at
                                                     consolidated_animals
```

---

## 1. Prerequisites

- **Go** ≥ 1.21 (tested with Go 1.26)
- **MySQL** (for running the apps in dev/prod)
- **Buffalo CLI** (optional, for `buffalo` task/migration commands): `go install github.com/gobuffalo/buffalo/buffalo@latest`
- A C toolchain (CGo) — **required** because the SQLite driver (`mattn/go-sqlite3`) is CGo-based. On macOS this is provided by Xcode Command Line Tools.

> ⚠️ The unit tests use **SQLite**, and pop/v6 compiles its SQLite driver **only** when the `sqlite` build tag is set. See [§3. Testing](#3-testing) — forgetting the tag is the #1 gotcha.

---

## 2. Setup

Both projects are independent Go modules and are set up the same way.

### 2.1 Database

**Creaves** (`creaves/database.yml`) uses **MySQL** for `development` and `test`:

```bash
# Create the dev + test databases (adjust user/password to match database.yml)
mysql -uroot -e "CREATE DATABASE IF NOT EXISTS creaves      CHARACTER SET utf8mb4;"
mysql -uroot -e "CREATE DATABASE IF NOT EXISTS creaves_test CHARACTER SET utf8mb4;"
mysql -uroot -e "CREATE USER IF NOT EXISTS 'creaves'@'localhost' IDENTIFIED BY 'creaves';"
mysql -uroot -e "GRANT ALL ON creaves.*      TO 'creaves'@'localhost';"
mysql -uroot -e "GRANT ALL ON creaves_test.* TO 'creaves'@'localhost';"
```

> Note: the **Go unit tests do not use MySQL**. The test harness builds its own
> in-memory SQLite schema (see each project's `*_test.go` `TestMain`). MySQL is
> only needed to actually *run* the Buffalo applications.

**Console** (`creaves-console/database.yml`) uses SQLite for `test` and MySQL for `development`/`production`. No DB setup is needed to run its unit tests.

### 2.2 Install dependencies & migrate

From each project directory:

```bash
cd creaves           # or: cd creaves-console
go mod download

# Apply migrations to the dev database (MySQL). Use GO_ENV=development (default).
buffalo db migrate
# or, without the buffalo CLI:
go run github.com/gobuffalo/pop/v6/cmd/pop migrate up
```

---

## 3. Testing

### 3.1 The build tag (read this first)

pop/v6 gates SQLite behind a **build tag**. The SQLite-dependent test files carry:

```go
//go:build sqlite
// +build sqlite
```

Therefore:

| Command                                  | Behavior                                                    |
|------------------------------------------|-------------------------------------------------------------|
| `go test ./...`                          | SQLite tests are **skipped** (no failures, clean output).   |
| `go test -tags sqlite ./...`             | SQLite tests are **compiled and run**. ✅ use this          |

**If you forget `-tags sqlite`, the tests are silently skipped — you will see `[no test files]` or a low test count, not a failure. Always run tests with `-tags sqlite`.**

### 3.2 Run the full unit test suites

```bash
# ---- creaves (pusher) ----
cd creaves
go build ./...                      # sanity: everything compiles
go test -tags sqlite ./...          # all packages
go test -tags sqlite -race ./...    # with the race detector (recommended)

# ---- creaves-console (receiver) ----
cd ../creaves-console
go build ./...
go test -tags sqlite ./...
go test -tags sqlite -race ./...
```

Expected: every package prints `ok`.

### 3.3 Run a specific test (with verbose output)

```bash
cd creaves
go test -tags sqlite ./actions/ -run TestDeliverBatch_PartialFailureMarksOnlyAccepted -v

cd ../creaves-console
go test -tags sqlite ./actions/ -run TestE2E_EventDelivery -v
```

### 3.4 Key tests by concern

| Concern                          | Project      | Test name                                                    |
|----------------------------------|--------------|--------------------------------------------------------------|
| Boot: worker starts when enabled | creaves      | `TestEnsureWebhookWorkerRunning_StartsWhenEnabled`           |
| Boot: worker stays off if disabled | creaves    | `TestEnsureWebhookWorkerRunning_NoopWhenDisabled`            |
| Partial failure (pusher side)    | creaves      | `TestDeliverBatch_PartialFailureMarksOnlyAccepted`           |
| Full delivery                    | creaves      | `TestDeliverBatch_Success`                                   |
| Circuit breaker                  | creaves      | `TestCircuitBreaker_*`                                       |
| Partial failure (receiver side)  | console      | `TestWebhookEventsHandler_PartialFailure`                    |
| Self-healing redelivery          | console      | `TestWebhookEventsHandler_RedeliveryReprocessesUnprocessed`  |
| Idempotency                      | console      | `TestWebhookEventsHandler_Idempotent`                        |
| **End-to-end forwarding**        | console      | `TestE2E_EventDelivery`                                      |

---

### 3.5 Test coverage

The webhook / event-forwarding logic lives in each project's `actions/` package. Measure how much of it the tests exercise:

```bash
# From either project directory:
cd creaves              # or: cd creaves-console
go test -tags sqlite -coverprofile=/tmp/cov.out ./actions/
go tool cover -func=/tmp/cov.out
```

`-coverprofile` writes per-function coverage to `/tmp/cov.out`; `go tool cover -func` prints a per-function percentage table.

**Focus on the webhook-critical code** (this is what matters):

```bash
go tool cover -func=/tmp/cov.out | grep -E '(webhook|event_processor|consolidation|event_producer)'
```

**Targets & expectations:**

- **≥ 80%** line coverage on all webhook / event-forwarding functions (`webhook_*`, `event_processor*`, `consolidation*`, `event_producer*`).
- The `actions/` **package-level aggregate will be much lower (≈ 5–37%)** — and that is expected: the package also bundles a lot of unrelated CRUD scaffolding the webhook tests never touch. Judge coverage by the per-function numbers above, not the package total.
- Optional HTML report: `go tool cover -html=/tmp/cov.out` opens a browser view of covered/uncovered lines.

---

## 4. Admin Screen Setup

This section documents how to wire up webhook forwarding **through the admin UIs** of both projects. The two screens are complementary: you create a key in the **Console (receiver)** and configure a **sync target** in **Creaves (pusher)**.

```
┌──────────────────────────┐                 ┌──────────────────────────────┐
│ creaves-console          │                 │ creaves                      │
│ (receiver / sink)        │                 │ (pusher / source)            │
│                          │  copy raw key   │                              │
│ Admin → Webhook API Keys │ ──────────────▶ │ Sync Configuration           │
│   → New (instance ID     │                 │   (/sync_configuration):     │
│     REQUIRED; raw key    │                 │   instance ID + event stream │
│     stays retrievable)   │                 │   + Add sync target (paste   │
│                          │                 │   key + URL)                 │
└──────────────────────────┘                 └──────────────────────────────┘
            ▲                                          │
            │   HTTP POST /webhook/events (Bearer key) │
            └──────────────────────────────────────────┘
```

### 4.1 Console side — create a Webhook API Key

> Requires an **admin** account. Non-admin users get HTTP 403 on every webhook-key action (each handler checks `GetCurrentUser(c).Admin`).

1. Start the console (`cd creaves-console && GO_ENV=development buffalo dev`, port 3001) and log in as an admin.
2. Open the **Admin** menu → **Webhook API Keys** → **New** (route `GET /webhook_api_keys/new`). The form carries a built-in *Next steps* hint explaining the pusher→receiver wiring.
3. Fill in the form (`templates/webhook_api_keys/new.plush.html`):
   - **Key Name** — a friendly label, e.g. `Center Brussels`.
   - **Instance ID (required)** — the creaves instance this key belongs to. The pusher must send events whose `instance_id` matches, or they are rejected as an instance mismatch. A previously unknown instance ID is **auto-registered** on first receipt.
4. Click **Generate API Key**. The handler (`actions/webhook_api_keys.go` → `Create`) generates a key (`creaves_<uuid>`), stores a **bcrypt hash** + prefix **and the raw key itself** (`key_value` column, migration `20260501092000`), then redirects to a dedicated one-time page `GET /webhook_api_keys/{id}/created` with the raw key in a readonly input + **Copy** button.
   The raw key **stays retrievable**: the keys list and detail pages display the stored value to admins (`creaves_<prefix>…` only when the value was never stored, i.e. keys created before that migration).

### 4.2 Creaves side — sync configuration and sync targets

> Requires an **admin** account (admin guard on the sync configuration/targets handlers).

1. Start creaves (`cd creaves && GO_ENV=development buffalo dev`) and log in as an admin.
2. Open **Administration → Synchronization** (`GET /sync_configuration`, `templates/config/sync_edit.plush.html`). Set:
   - **Instance ID** — this instance's unique identifier (top-level `InstanceID` on the active config, not under `Settings`). Must match the API key's Instance ID from §4.1.
   - **Enable Event Stream** — `Settings.EnableEventStream`, master switch for event production.
3. In the **Sync Targets** table on the same page, click **Add sync target** (`GET /sync_targets/new`, `templates/sync_targets/_form.plush.html`) and set:

   | Field                 | Form name                     | Value                                             |
   |-----------------------|-------------------------------|---------------------------------------------------|
   | Name                  | `Name`                        | e.g. `Local console`                              |
   | Enabled               | `Enabled`                     | ✅ on (events are pushed to every enabled target)  |
   | Webhook URL           | `WebhookURL`                  | `http://<console-host>:3001/webhook/events`       |
   | API Key               | `WebhookAPIKey`               | the raw key copied in §4.1 (admins/maintainers see the stored value; leave blank on edit to keep the current key) |
   | Batch Size            | `WebhookBatchSize`            | e.g. `100` (events per request; default 1, max 100) |
   | Max Events Per Minute | `WebhookMaxPerMin`            | e.g. `6000` for large backfills (default 60)      |

   Creaves supports **multiple** targets (multi-hub fan-out): one per console. Each target has its own delivery tracking (Pending / Delivered / Undeliverable / Unconfirmed columns on `/sync_configuration`), plus `POST /sync_targets/{id}/retry_undeliverable`.
4. **Save.** The delivery worker starts on target save **and** at application boot via `InitWebhookAtBoot()` (`cmd/app/main.go`).

`IsWebhookEnabled()` (`actions/configs.go`) returns true when **at least one enabled sync target has a webhook URL** — there is no separate per-config `Settings.WebhookEnabled` flag anymore.

### 4.3 Wiring checklist

- The **raw key** created in the Console (§4.1) must be pasted into the **API Key** field of a creaves sync target (§4.2).
- The sync target's **Webhook URL** must point at the Console's `/webhook/events` endpoint.
- The pusher sends the key as `Authorization: Bearer <key>` (`webhook_pusher.go`).
- The creaves **Instance ID** must equal the key's Instance ID (case-insensitive), or deliveries are rejected with 403 (runbook §7.1).

---

## 5. Manual end-to-end webhook test (running both apps)

This exercises the real pusher → receiver flow over HTTP. It doubles as the **end-to-end validation walkthrough** for the admin setup in [§4](#4-admin-screen-setup): start console → log in as admin → create key → start creaves → set instance ID + add sync target → trigger an event → verify delivery.

### Step 0 — Start the receiver (creaves-console)

```bash
cd creaves-console
GO_ENV=development buffalo db migrate          # ensure schema is current
GO_ENV=development buffalo dev                 # serves on http://localhost:3001 (PORT=3001 in .buffalo.dev.yml)
```

### Step 1 — Create a webhook API key (receiver side)

The pusher authenticates with a bearer key created in the console.

Via the console UI: log in as an admin → **Webhook API Keys** → **New** → enter a **Key Name** and the **Instance ID** (required) → **Generate API Key**.
The **raw key** (e.g. `creaves_<uuid>`) is shown on the confirmation page and remains retrievable from the keys list.

A key row looks like:
```
webhook_api_keys: { name, key_hash (bcrypt), key_prefix, key_value (raw, retrievable), instance_id (required), active }
```

> The pusher must send events whose `instance_id` matches the key's
> `instance_id`, or they will be rejected as an instance mismatch (403).

### Step 2 — Configure the pusher (creaves)

```bash
cd creaves
GO_ENV=development buffalo db migrate
GO_ENV=development buffalo dev                 # serves the creaves app on :3000
```

In the creaves UI → **Administration → Synchronization** (`/sync_configuration`):

| Where                    | Field                   | Value                                             |
|--------------------------|-------------------------|---------------------------------------------------|
| Instance Identity        | Instance ID             | must match the key's Instance ID from Step 1      |
| Event Stream             | Enable Event Stream     | ✅ on                                              |
| Sync Targets (Add)       | Name / Enabled          | `Local console` / ✅                               |
| Sync Targets (Add)       | Webhook URL             | `http://localhost:3001/webhook/events`            |
| Sync Targets (Add)       | API Key                 | the raw key copied in Step 1                      |
| Sync Targets (Add)       | Batch Size / Max per min| e.g. `100` / `6000`                               |

Save. The worker starts on target save **and** at application boot via `InitWebhookAtBoot()`.

### Step 3 — Trigger an event (pusher side)

Perform any action in creaves that publishes an event (e.g. register/discover a new animal). The producer (`PublishEvent` in `event_producer.go`) writes a row to `event_streams` with `delivered_at IS NULL`. The background worker (5s ticker) then POSTs the batch to the console.

### Step 4 — Verify the flow

**Pusher side (creaves DB):**
```sql
SELECT id, animal_id, event_type, delivered_at
FROM event_streams
ORDER BY created_at DESC LIMIT 10;
-- delivered_at becomes non-NULL once the console acknowledged the event.
```

**Receiver side (console DB):**
```sql
SELECT id, instance_id, animal_id, event_type, imported_at, processed_at
FROM event_streams
ORDER BY created_at DESC LIMIT 10;

SELECT instance_id, animal_id, current_status, event_count, species
FROM consolidated_animals
ORDER BY last_event_at DESC LIMIT 10;
```

You should see the event arrive in `event_streams` (with `processed_at` set) and a corresponding/upserted row in `consolidated_animals`.

### Step 5 — Inspect delivery logs

The pusher logs to stdout:
```
Webhook worker started
Delivered 3 events to webhook
Delivered 2/3 events to webhook; 1 will be retried   ← partial failure
Webhook delivery failed: webhook returned status 500   ← receiver error
```

---

## 6. How partial-failure handling works

When the console accepts **some but not all** events in a batch:

1. The receiver processes each event independently; failures (bad UUID, instance mismatch, processing error) are collected but do not abort the batch.
2. The receiver responds `200 OK` with:
   ```json
   { "processed": 2, "total": 3, "processed_ids": ["uuid-a", "uuid-b"], "errors": ["..."] }
   ```
3. The pusher parses `processed_ids` and marks **only those** events `delivered_at = now`. Events not in the list keep `delivered_at IS NULL` and are **retried on the next tick**.
4. **Backward compatibility:** if a receiver returns `200 OK` with no `processed_ids`, the pusher assumes all events were accepted (so older receivers still work).
5. **Self-healing:** if an event row exists at the receiver but was never `processed_at`, a redelivery reprocesses it instead of skipping it.

The circuit breaker opens after 5 consecutive receiver failures (5xx / connection errors) and half-opens after 60s.

### 6.1 Resync page polling behavior (`/webhook_resync`)

The resync page fetches `/webhook_resync/status.json` exactly **once** on load. The
2-second polling timer is re-armed **only while the latest run reports
`status: "running"`**; for `none` / `completed` / `failed` / `cancelled` the script
stops after the first response. The page therefore reaches browser network-idle when
no run is active (verified: request count stays at 1 over repeated samples) —
automation tools that wait for network-idle will not time out on it. A "failed" run
keeps the delivery diagnostics visible without resuming the timer.

---

## 7. Troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| `sqlite3 support was not compiled into the binary` | You ran tests **without** `-tags sqlite`. Re-run with `-tags sqlite`. |
| Tests silently show `[no test files]` / low count | Same as above — the `sqlite` tag is not set, so tagged tests are excluded. |
| `could not create new connection` at test startup (MySQL error) | The test DB should be SQLite, not MySQL. This only happens if a test file is missing its `//go:build sqlite` tag and the default connection resolves to MySQL. Ensure all `*_test.go` that use SQLite carry the tag. |
| Pusher logs `webhook returned status 401` | Wrong/missing API key, or the key is inactive. Re-create the key in the console and update the pusher config. |
| Webhook returns **403 `instance_id mismatch`** | Runbook in §7.1 below — key is instance-restricted and `key.instance_id` ≠ envelope `instance.id`. |
| Pusher logs `webhook returned status ...` repeatedly | Circuit breaker will open after 5 failures; check the console is running and reachable at the configured URL. |
| Events never delivered (`delivered_at` stays NULL) | Event stream or sync target disabled. `/sync_configuration`: `Enable Event Stream` checked **and** at least one **Enabled** sync target with a `WebhookURL`; the worker only delivers when `IsWebhookEnabled()` is true (any enabled target with a URL). |
| `buffalo: command not found` | Install the CLI: `go install github.com/gobuffalo/buffalo/buffalo@latest`, or use `pop` directly for migrations. |

### 7.1 Runbook — webhook 403 `instance_id mismatch`

**Failure signature**

```
POST /webhook/events → HTTP 403 {"error": "instance_id mismatch"}
```

**Root cause** — `creaves-console/actions/webhook.go` (`registerEnvelopeInstance`, the
403 is rendered at ~line 107): each webhook API key may be **restricted to one
instance** (`webhook_api_keys.instance_id`). If restricted, the envelope's
`instance.id` (and every event's `instance_id`) must match it, case-insensitively.
Auth failures are 401 (`webhook.go` lines 74/79/87); a 403 is always an
instance/key alignment problem, **not** a credential problem.

**Diagnosis**

1. Console DB: `SELECT instance_id, active FROM webhook_api_keys WHERE
   key_hash` … — note the key's restriction (NULL = unrestricted).
2. Producer config: `SELECT instance_id FROM configs;` in the creaves DB (what
   the producer stamps on events and the envelope).
3. Compare: any difference (e.g. key restricted to `test`, envelope says
   `test-2`) → 403.

**Remediation** — pick one, keep both sides aligned:

- **Align the producer** (preferred): in creaves → Configuration, set
  `Instance ID` = the key's `instance_id` exactly (e.g. `test`). Config is
  cached (`CurrentConfig`) — restart `buffalo dev` or re-save config to reload.
- **Align the key**: in console → Webhook API Keys, issue a key restricted to
  the producer's actual instance ID (or unrestricted), then update
  `WebhookAPIKey` on the producer.

**Verify** (see §2 E2E push example):

```bash
# Auth probe (empty batch must return 200, not 403):
curl -s -w "%{http_code}\n" -X POST http://127.0.0.1:3001/webhook/events \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer creaves_<raw-key>" \
  -d '{"contract_version":2,"instance":{"id":"test"},"events":[]}'
# then push a real/producer-identical event and confirm processed:1,
# event_streams.processed_at set, consolidated_animals row updated.
```

---

## 8. Quick reference

```bash
# Build both
(cd creaves          && go build ./...)
(cd creaves-console  && go build ./...)

# Test both (ALWAYS with the sqlite tag)
(cd creaves          && go test -tags sqlite ./...)
(cd creaves-console  && go test -tags sqlite ./...)

# Race-clean check
(cd creaves          && go test -tags sqlite -race ./...)
(cd creaves-console  && go test -tags sqlite -race ./...)

# Run the apps
(cd creaves-console  && GO_ENV=development buffalo dev)   # receiver, :3001 (PORT=3001 in .buffalo.dev.yml)
(cd creaves          && GO_ENV=development buffalo dev)   # pusher, :3000
```
