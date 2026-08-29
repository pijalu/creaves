# Creaves - Agent Guide

## Documentation Index

This project has comprehensive agentic documentation split across multiple files:

| Document | Purpose | Location |
|----------|---------|----------|
| **AGENTS.md** (this file) | Project overview, setup, conventions, performance notes | `/AGENTS.md` |
| **MODELS_DOCUMENTATION.md** | All database models, fields, relationships, validations | `/MODELS_DOCUMENTATION.md` |
| **ACTIONS_DOCUMENTATION.md** | All HTTP handlers, routes, business logic | `/actions/ACTIONS_DOCUMENTATION.md` |
| **TEMPLATES_DOCUMENTATION.md** | All Plush templates, forms, JavaScript interactions | `/TEMPLATES_DOCUMENTATION.md` |
| **MIGRATIONS_DOCUMENTATION.md** | Database schema, migrations, indexes, triggers | `/MIGRATIONS_DOCUMENTATION.md` |
| **ASSETS_DOCUMENTATION.md** | Frontend assets, libraries, build process | `/ASSETS_DOCUMENTATION.md` |
| **UTILITIES_DOCUMENTATION.md** | Helper functions, utilities, patterns | `/UTILITIES_DOCUMENTATION.md` |

**Event Forwarding**: This project pushes animal lifecycle events to **Creaves Console** via webhooks.
See the [Event Forwarding to Creaves Console](#event-forwarding-to-creaves-console) section below and the
[Creaves Console AGENTS.md](../creaves-console/AGENTS.md) for the consolidation architecture.

**Quick Reference**: When asked to modify code, first check the relevant documentation file to understand the existing structure without parsing all source files.

---

## Stack & Architecture
- **Go 1.18** webapp using [Buffalo v0.18.x](https://gobuffalo.io/) framework
- **Entrypoint**: `cmd/app/main.go` → `actions.App()` (`actions/app.go`)
- **Database**: MySQL/MariaDB via [Pop v6](https://github.com/gobuffalo/pop) (ORM/migrations)
- **Frontend assets**: Webpack 5 + Sass + Babel + Bootstrap 4.6 + jQuery + Select2 + Flatpickr
- **Templating**: Plush (`.plush.html`)
- **I18n**: `locales/*.yaml` (fr, en-US, de, nl) plus startup reference translations in `grifts/translations_*.sql`

## Prerequisites
- Go 1.18+
- Buffalo CLI: `go install github.com/gobuffalo/cli/cmd/buffalo@latest`
- Node.js + npm/yarn
- MySQL/MariaDB running locally (or use Docker test setup)

## Database Setup
Connection config is in `database.yml` (development/test use `localhost:3306`, db `creaves`, user/pass `creaves`).

```bash
# Run migrations
buffalo pop migrate up

# Seed reference data (required for app to function)
buffalo task db:seed

# Seed only the 7 startup reference tables from the embedded production dump
buffalo task db:seed:startup
```

**Note**: `db:seed` is **not optional** — it creates admin user, animal types, species, drugs, care types, zones, etc.

`db:seed` first loads embedded production reference data from `grifts/creaves-startup.sql.gz`
(7 tables: `animalages, animaltypes, caretypes, outtaketypes, drugs, species, dosages`),
skipping any table that already contains rows (idempotent per table). The standalone
`buffalo task db:seed:startup` runs only this step.

**Startup translations**: after base rows are loaded, `db:seed:startup` backfills French keys and applies embedded `grifts/translations_{fr,en-US,de,nl}.sql` artifacts. Inserts are unique-key idempotent and preserve existing values. The artifact inventory covers 4,686 non-empty startup values per locale; see `I18N_STARTUP_TRANSLATION_PLAN.md` for regeneration and review workflow.

**Warning**: the dump INSERTs have no column names — they rely on column order matching
`migrations/schema.sql`. Adding a NOT NULL column without a default to any of these 7
tables requires regenerating the dump.

## Development Commands

```bash
# Install Go + JS deps
buffalo plugins install
npm install
yarn install

# Run dev server with auto-reload (builds Go binary + watches assets)
buffalo dev

# Or build assets manually
npm run dev        # webpack --watch
npm run build      # webpack --mode production

# Build production binary
buffalo build --environment production --static -o bin/app
```

**Buffalo dev config**: `.buffalo.dev.yml` — builds `./cmd/app` into `tmp/creaves-build`, watches `.go` and `.env` files.

## Testing

```bash
# Run all Go tests
buffalo test

# Run a single package
go test ./actions
go test ./models

# Run a single test
go test ./actions -run TestCalculateFeeding
```

**Test DB**: Uses same MySQL connection as dev (`database.yml` test env). No SQLite fallback.

**Docker test stack** (full integration):
```bash
cd test && docker-compose up --build
```
- Builds app in dev mode, runs migrations + seed, exposes on port 80
- MariaDB latest with volume `test/db_data`

## Migrations
- Located in `migrations/`
- Format: Buffalo Pop [Fizz](https://github.com/gobuffalo/fizz) (`.up.fizz`/`.down.fizz`)
- Raw SQL files also accepted (e.g. `migrations/trigger-animal.sql`)
- Schema dump: `migrations/schema.sql`

## Key Directories
| Directory | Purpose |
|-----------|---------|
| `actions/` | HTTP handlers, routing, middleware, business logic |
| `models/` | Pop models, database entities |
| `templates/` | Plush HTML templates |
| `migrations/` | Database migrations (Fizz + SQL) |
| `grifts/` | CLI tasks (seed data, admin creation) |
| `assets/` | JS, CSS, images for Webpack |
| `public/assets/` | **Generated** — do not edit (gitignored) |
| `locales/` | I18n YAML files |
| `fixtures/` | Test fixtures (TOML format for Pop) |
| `dockerscript/` | Production startup scripts |
| `test/` | Docker compose for integration testing |

## Build & Deploy

```bash
# Docker multi-arch build and push
./build.sh   # Uses buildx for linux/amd64 + linux/arm64
```

**Production startup**: `dockerscript/quickstart.prod.sh` — runs migrations, seeds DB, then starts app.

## Code Style
- Go: standard `gofmt` (enforced by CodeClimate config)
- JS: `jshint` available in devDependencies
- Method length threshold: 100 lines (`.codeclimate.yml`)

## Important Conventions
- **Auth**: Routes under `/auth` and `/registration` skip `Authorize` middleware
- **CSRF**: Enabled globally via `mw-csrf`
- **DB transactions**: Every request wrapped in Pop transaction (`popmw.Transaction`)
- **Time formats**: Custom formats registered in `models.DateTimeFormat` and `models.DateFormat`
- **Environment**: Controlled by `GO_ENV` (not `NODE_ENV` — webpack uses `GO_ENV` override)
- **Assets**: Webpack outputs to `public/assets/` with content hashing; `manifest.json` tracks filenames

---

## Event Forwarding to Creaves Console

Creaves produces animal lifecycle events and **pushes them via HTTP webhooks** to
[Creaves Console](../creaves-console) — a separate consolidation app that provides a
unified view across all geographically-distributed Creaves instances.

### Why Webhooks (Not DB Sharing)

Creaves databases are spread across multiple centers (different networks, behind
firewalls/NAT). Direct DB connections are not feasible. The webhook push model lets each
instance forward events over standard HTTPS to the central console.

### Architecture

```
Creaves (this app)                    Creaves Console
┌─────────────────────┐              ┌──────────────────────┐
│ Animal created/     │              │                      │
│   updated/released  │              │  POST /webhook/events│
│       ↓             │              │        ↑             │
│ PublishEvent()      │              │ WebhookEventsHandler │
│   → event_streams   │              │   → event_streams    │
│       ↓             │  HTTP POST   │       ↓              │
│ WebhookPusher       │ ───────────► │  EventProcessor      │
│   (background worker│  Bearer key  │   → consolidated_    │
│    every 5s)        │              │     animals          │
│   → delivered_at    │              │                      │
└─────────────────────┘              └──────────────────────┘
```

### Key Files

| File | Purpose |
|------|---------|
| `actions/event_producer.go` | `PublishEvent()` + typed helpers (`PublishAnimalDiscoveredEvent`, etc.) |
| `actions/webhook_pusher.go` | Background worker: circuit breaker, rate limiting, batch delivery |
| `actions/configs.go` | Config management: `LoadConfig()`, `IsEventStreamEnabled()`, `IsWebhookEnabled()`, `GetInstanceID()` |
| `actions/event_streams.go` | Event stream list/show/destroy handlers (admin UI) |
| `models/event_stream.go` | EventStream model + EventPayload structs |
| `models/config.go` | Config model + ConfigSettings (webhook config) |
| `templates/config/_form.plush.html` | Webhook configuration UI |
| `templates/event_streams/` | Event stream admin views |

### Event Lifecycle

1. **Event produced**: When an animal is discovered, has a status change, is released,
   or dies, `PublishEvent()` creates an `event_streams` record with a UUID, the
   instance ID, animal ID, event type, and full payload.

2. **Worker delivers**: The background `WebhookPusher` (started on first event, ticks
   every 5 seconds) queries undelivered events (`delivered_at IS NULL`), batches them
   (configurable batch size), and POSTs to the console webhook URL.

3. **Mark delivered**: On HTTP 200 response, events are marked with `delivered_at`.

### Event Types

| Type | Trigger | Hook Location |
|------|---------|---------------|
| `animal_discovered` | New animal intake | `actions/discoveries.go` |
| `animal_status_changed` | Status update | `actions/animals.go` |
| `animal_released` | Release outtake | `actions/outtakes.go` |
| `animal_died` | Death outtake | `actions/outtakes.go` |

### Webhook Payload Structure

Each event is sent as JSON with this shape (the `payload` field contains the full structured data):

**Canonical values**: payload display values (`species`, `animal_type`, `animal_age`, outtake `type`, etc.) are always the **canonical base-locale (French) reference names** stored in the base table columns — regardless of the UI language active on the producing instance. The multilingual UI (translations table, `tname` helpers) never alters payload content. A future `translations` map in the payload is possible but out of scope.

```json
{
  "events": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "instance_id": "center-strasbourg",
      "animal_id": 42,
      "event_type": "animal_discovered",
      "payload": {
        "animal":      { "id": 42, "year": 2024, "year_number": 17, "species": "Hérisson", "gender": "M", "cage": "A12", "zone": "Quarantine", "ring": "FR-2024-017", "animal_type": "Mammifère", "animal_age": "Adulte", "species_class": "Mammalia", "species_agw_group": "...", "species_subside_group": "...", "species_native_status": "Indigène" },
        "discovery":   { "id": "...", "location": "...", "postal_code": "67000", "city": "Strasbourg", "date": "2024/01/15 10:30", "entry_cause": "...", "entry_cause_detail": "...", "entry_cause_nature": "...", "reason": "...", "note": "...", "return_habitat": false, "in_garden": true, "discoverer_firstname": "...", "discoverer_lastname": "...", "discoverer_email": "...", "discoverer_phone": "..." },
        "intake":      { "id": "...", "date": "2024/01/15 11:00", "general": "...", "has_wounds": true, "wounds": "...", "has_parasites": false, "parasites": "", "remarks": "..." },
        "outtake":     { "id": "...", "date": "2024/03/01 09:00", "type": "Released to Wild", "location": "...", "note": "...", "rating": 1, "dead": false },
        "initial_status":  "in_care",
        "current_status":  "in_care",
        "previous_status": "",
        "user_id":     "uuid",
        "user_login":  "admin",
        "timestamp":   "2024-01-15T10:30:00Z"
      },
      "created_at": "2024-01-15T10:30:00Z"
    }
  ]
}
```

Sent with headers:
```
Content-Type: application/json
Authorization: Bearer creaves_<api-key>
```

The Console responds with `{"processed": N, "total": M}` on success.
See `../creaves-console/AGENTS.md` for the full contract spec.

### Configuration

Via admin UI: **Configuration** page (`/configs`).

| Setting | Field | Default | Purpose |
|---------|-------|---------|---------|
| `EnableEventStream` | Checkbox | true | Master switch for event production |
| `WebhookEnabled` | Checkbox | false | Master switch for webhook delivery |
| `WebhookURL` | URL | (empty) | Console webhook endpoint, e.g. `https://console.example.com/webhook/events` |
| `WebhookAPIKey` | Password | (empty) | Bearer token from Console's API key management |
| `WebhookBatchSize` | Number | 1 | Events per HTTP request (1-100) |
| `WebhookMaxPerMin` | Number | 60 | Rate limit (events/minute) |

Config is cached in `CurrentConfig` (global). First load auto-creates a default config
with `instance_id` = hostname or `INSTANCE_ID` env var.

### Webhook Delivery Details

- **Circuit breaker**: After 5 consecutive failures, delivery pauses for 60 seconds
  (state: open → half-open → closed on success).
- **Rate limiting**: Respects `WebhookMaxPerMin`.
- **Timeout**: 30-second HTTP client timeout.
- **Retry**: Events remain `delivered_at IS NULL` until successfully delivered. The
  worker retries on every tick (5s). After app restart, the worker resumes delivery.
- **Shutdown**: `RegisterWebhookShutdown()` (called in `cmd/app/main.go`) stops the
  worker on `EvtAppStop`.

### Database Schema (event_streams)

```sql
event_streams:
  id           UUID PRIMARY KEY
  instance_id  VARCHAR     -- source instance identifier
  animal_id    INT         -- source animal ID
  event_type   VARCHAR     -- animal_discovered|animal_status_changed|animal_released|animal_died
  payload      JSON        -- full structured payload (animal, discovery, intake, outtake, status, user)
  processed_at TIMESTAMP NULL  -- (unused on producer side)
  delivered_at TIMESTAMP NULL  -- set when webhook delivery succeeds
  created_at   TIMESTAMP
  updated_at   TIMESTAMP
```

Indexes: `(instance_id, animal_id, created_at)`, `processed_at`, `event_type`, `delivered_at`

### Snapshot Task (Initial Backfill)

For migrating existing animals into the event stream:

```bash
buffalo task event:snapshot         # Create events for animals without any
buffalo task event:snapshot:force   # Force-create (may duplicate)
buffalo task event:snapshot:stats   # Show statistics
```

### Setup Checklist (Connecting to Console)

1. Deploy Creaves Console (see `../creaves-console/AGENTS.md`)
2. In Console: create a Webhook API Key, copy the raw key
3. In Creaves: go to **Configuration**:
   - Set `Instance ID` (unique per center, e.g. `center-strasbourg`)
   - Enable **Event Stream**
   - Enable **Webhook**
   - Set **Webhook URL** to `https://<console>/webhook/events`
   - Paste **API Key**
4. Create or edit an animal → verify event appears in Console dashboard
5. For existing data: run `buffalo task event:snapshot`

### Known Issues / WIP

- **Worker not started at boot**: `StartWebhookWorker()` is only called lazily from
  `PublishEvent()`. If webhook is enabled but no new events arrive, undelivered events
  from a previous session won't be picked up until a new event triggers the worker.
  Fix: call `LoadConfig()` + `StartWebhookWorker()` in `App()` initialization.
- **Partial failure handling**: The console returns HTTP 200 even when some events in
  a batch fail. The pusher marks the entire batch as delivered, potentially losing
  failed events. (Console logs the errors in the response body.)

### Troubleshooting (Event Forwarding)

| Symptom | Check |
|---------|-------|
| Events not delivered to console | 1. `IsWebhookEnabled()`? 2. `WebhookURL` + `WebhookAPIKey` set? 3. Circuit breaker open (5 failures → 60s pause)? 4. Console reachable from Creaves network? |
| `delivered_at` stays NULL | Worker not started. Create any animal event to trigger `StartWebhookWorker()`. (WIP: should start at boot) |
| Console returns 401 | API key mismatch — regenerate key in Console admin UI |
| Console returns 400 | Malformed payload — check `event_producer.go` payload building |
| Events delivered but missing in console view | Run `buffalo task consolidation:process` on the Console side |
| Need initial backfill | `buffalo task event:snapshot` creates events for all existing animals |

### Testing Event Forwarding

```bash
# Unit tests (event producer, config)
go test ./actions/... ./models/...

# Verify event creation without console:
# 1. buffalo dev, login as admin
# 2. Create an animal (discovery flow)
# 3. Check event_streams table: SELECT * FROM event_streams ORDER BY created_at DESC LIMIT 5;

# Verify webhook delivery:
# 1. Set WebhookURL + APIKey in Configuration
# 2. Enable webhook
# 3. Create/edit an animal
# 4. Check delivered_at is set within ~5s
# 5. Check Console dashboard shows the event

# Snapshot backfill test:
buffalo task event:snapshot:stats
```

---

## Performance Notes

Critical for multi-user deployment on limited hardware.

### Known Bottlenecks
- **Reference data fetched from DB on every request**: `actions/typehelper.go` loads animal types, care types, zones, etc. without caching
- **User re-fetched from DB every request**: `actions/users.go:SetCurrentUser` does `tx.Find(u, uid)` on every authenticated request
- **Landing page loads ALL animals**: `actions/landing.go` loads every in-care animal without pagination
- **N+1 queries**: Some handlers still use `EnrichAnimals` (older) instead of `EnrichAnimalsOptimized`
- **No connection pool tuning**: `models/models.go` uses default Pop connection settings
- **Eager() overuse**: Many handlers use `tx.Eager()` which can generate unexpected queries

### Implementation Tracking

See [TODO.md](./TODO.md) for complete list of pending and completed TODOs with implementation status.

**Quick Status:**
- ✅ **Phase 1: Foundation** - Event stream table, schema design, instance identifiers - COMPLETED
- ✅ **Phase 2: Event Production** - Event producer hooks in discoveries/animals/outtakes - COMPLETED
- 🔄 **Phase 3: Consolidation** - Event processor, consolidated view - IN PROGRESS
- ⏳ **Phase 4: Performance** - Caching, pagination, connection pooling - PENDING

## Testing Requirements

**MANDATORY END-TO-END TESTING:** All changes to templates, actions, models, routes, or database schema MUST be tested using Chrome DevTools MCP with authenticated sessions.

### Required Testing Checklist

Before marking any task as complete, verify:

- [ ] **Server starts** without errors (`buffalo dev`)
- [ ] **Database seeded** with admin user (`buffalo task db:seed`)
- [ ] **Authentication works** (login as admin/admin)
- [ ] **All modified pages load** without template errors
- [ ] **Forms submit correctly** with validation working
- [ ] **Navigation works** between related pages
- [ ] **Feature flags/configurations** persist after save
- [ ] **No 500/404 errors** for valid requests
- [ ] **Admin menu links** visible and functional

### Chrome DevTools MCP Testing

Use the Chrome DevTools MCP skill for standardized testing:

```bash
# Load the skill (automatically available)
# Located at: ~/.config/opencode/skills/chrome-devtools-testing.md
```

**Quick Test Flow:**
1. Start server: `buffalo dev > /tmp/buffalo.log 2>&1 &`
2. Navigate to: `http://127.0.0.1:3000/auth/new`
3. Login with: admin/admin
4. Test all modified features
5. Verify no console errors or 500s

**See full testing procedures in:** `~/.config/opencode/skills/chrome-devtools-testing.md`

### Chrome DevTools MCP Guidelines

**Avoid screenshots/pixel validation unless explicitly requested by the user.** Use page snapshots (`take_snapshot`), console output, and HTTP responses for verification instead. Screenshots should only be used when:
- The user explicitly asks for visual verification
- Debugging layout/rendering issues
- Demonstrating UI changes to the user

This preserves context and reduces token usage.

## Common Gotchas
- `buffalo dev` handles both Go rebuilds and asset compilation — don't run `npm run dev` separately unless debugging webpack
- `public/assets/` is gitignored and regenerated on build
- Database must exist before migrations; Pop does not auto-create the DB
- The `db:seed` task is idempotent-ish but will fail if run before migrations
- Test docker-compose mounts `test/database.yml` (points to `db` host, not `localhost`)
- **Always test forms with checkboxes** — they require manual parameter binding to avoid `strconv.ParseUint` errors
