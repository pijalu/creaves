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

**Quick Reference**: When asked to modify code, first check the relevant documentation file to understand the existing structure without parsing all source files.

---

## Stack & Architecture
- **Go 1.18** webapp using [Buffalo v0.18.x](https://gobuffalo.io/) framework
- **Entrypoint**: `cmd/app/main.go` → `actions.App()` (`actions/app.go`)
- **Database**: MySQL/MariaDB via [Pop v6](https://github.com/gobuffalo/pop) (ORM/migrations)
- **Frontend assets**: Webpack 5 + Sass + Babel + Bootstrap 4.6 + jQuery + Select2 + Flatpickr
- **Templating**: Plush (`.plush.html`)
- **I18n**: `locales/*.yaml` (fr + en-US)

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
```

**Note**: `db:seed` is **not optional** — it creates admin user, animal types, species, drugs, care types, zones, etc.

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

## Performance Notes

Critical for multi-user deployment on limited hardware.

### Known Bottlenecks
- **Reference data fetched from DB on every request**: `actions/typehelper.go` loads animal types, care types, zones, etc. without caching
- **User re-fetched from DB every request**: `actions/users.go:SetCurrentUser` does `tx.Find(u, uid)` on every authenticated request
- **Landing page loads ALL animals**: `actions/landing.go` loads every in-care animal without pagination
- **N+1 queries**: Some handlers still use `EnrichAnimals` (older) instead of `EnrichAnimalsOptimized`
- **No connection pool tuning**: `models/models.go` uses default Pop connection settings
- **Eager() overuse**: Many handlers use `tx.Eager()` which can generate unexpected queries

### Performance Todos

```
TODO-PERF-001: Cache reference data (animaltypes, caretypes, zones, etc.)
  ├── Blocks: TODO-PERF-002 (user caching can use same cache infrastructure)
  ├── Location: actions/typehelper.go
  └── Impact: HIGH - Every form render hits DB 5-6 times

TODO-PERF-002: Cache current user in session/middleware
  ├── Depends: TODO-PERF-001 (same cache pattern)
  ├── Location: actions/users.go:SetCurrentUser
  └── Impact: HIGH - DB hit on every authenticated request

TODO-PERF-003: Add pagination to LandingIndex
  ├── Location: actions/landing.go
  └── Impact: HIGH - Loads ALL in-care animals unconditionally

TODO-PERF-004: Migrate remaining EnrichAnimals calls to EnrichAnimalsOptimized
  ├── Location: actions/registertable.go, actions/registersnapshot.go
  └── Impact: MEDIUM - N+1 queries on register pages

TODO-PERF-005: Add connection pool configuration
  ├── Location: models/models.go
  └── Impact: MEDIUM - Default pool may not suit multi-user load

TODO-PERF-006: Add HTTP cache headers for static assets
  ├── Location: actions/app.go (ServeFiles)
  └── Impact: LOW - Reduces asset re-download

TODO-PERF-007: Review tx.Eager() usage for N+1 elimination
  ├── Location: Multiple resource files (animals.go, discoveries.go, etc.)
  └── Impact: MEDIUM - Pop Eager can generate unexpected queries

TODO-PERF-008: Add request timing/logging middleware
  ├── Depends: None (new feature)
  └── Impact: LOW - Helps identify slow endpoints in production

TODO-FUNC-001: Create event stream table for multi-instance consolidation
  ├── Depends: None (new feature)
  ├── Location: New migration + models/event_stream.go
  └── Impact: HIGH - Foundation for all consolidation features

TODO-FUNC-002: Implement event producer for animal lifecycle changes
  ├── Depends: TODO-FUNC-001
  ├── Location: actions/animals.go, actions/discoveries.go, actions/outtakes.go
  └── Impact: HIGH - Generates events on discovery, status change, outtake

TODO-FUNC-003: Build event processor for consolidated DB view
  ├── Depends: TODO-FUNC-001, TODO-FUNC-002
  ├── Location: New grift/task: grifts/consolidation.go
  └── Impact: HIGH - Creates unified view across all instances

TODO-FUNC-004: Design self-contained event document schema
  ├── Depends: TODO-FUNC-001
  ├── Location: models/event_stream.go (schema design)
  └── Impact: MEDIUM - JSON document with: discovery location, initial status, current status, timestamps

TODO-FUNC-005: Add instance identifier to all events
  ├── Depends: TODO-FUNC-001
  ├── Location: models/event_stream.go + config
  └── Impact: MEDIUM - Distinguishes events from different centers
```

## Implementation Plan

### Phase 1: Foundation (Weeks 1-2)
**Goal**: Establish event infrastructure, deployable and testable in isolation.

1. **TODO-FUNC-004**: Design event document schema
   - JSON structure: `{instance_id, animal_id, event_type, timestamp, payload: {discovery_location, initial_status, current_status, ...}}`
   - Versioning strategy for schema evolution
   - Validation rules

2. **TODO-FUNC-001**: Create event stream table
   - Migration: `event_streams` table (id, instance_id, animal_id, event_type, payload JSON, created_at, processed_at)
   - Model: `models/event_stream.go`
   - Indexes: `(instance_id, animal_id, created_at)` for querying

3. **TODO-FUNC-005**: Add instance identifier
   - Config: `INSTANCE_ID` env var
   - Default to hostname or UUID if not set
   - Add to all event payloads

**Deliverable**: Events can be written to DB, schema validated, instance IDs tracked.
**Testing**: Unit tests for model validation, migration rollback.

### Phase 2: Event Production (Weeks 3-4)
**Goal**: Generate events on all animal lifecycle changes.

4. **TODO-FUNC-002**: Implement event producer
   - Hook into `actions/discoveries.go`: Create `animal_discovered` event on new discovery
   - Hook into `actions/animals.go`: Create `animal_status_changed` event on status update
   - Hook into `actions/outtakes.go`: Create `animal_released`/`animal_died` event on outtake
   - Producer function: `PublishEvent(tx, eventType, animal, payload)` — writes to `event_streams` within same DB transaction

**Deliverable**: All animal lifecycle changes emit events atomically with the business transaction.
**Testing**: Integration tests verifying events created on discovery/status change/outtake.

### Phase 3: Consolidation (Weeks 5-6)
**Goal**: Process events into consolidated view.

5. **TODO-FUNC-003**: Build event processor
   - Grift/task: `buffalo task consolidation:process`
   - Reads unprocessed events (`processed_at IS NULL`) ordered by `created_at`
   - Idempotent upsert into `consolidated_animals` table: `(instance_id, animal_id, discovery_location, current_status, last_updated)`
   - Marks events as processed
   - Supports reprocessing (clear `processed_at` to rebuild)

**Deliverable**: Runnable task produces consolidated DB view from event stream.
**Testing**: Test with multiple instances' event dumps; verify deduplication and status merging.

### Phase 4: Performance Hardening (Weeks 7-8)
**Goal**: Optimize for limited hardware, multi-user load.

6. **TODO-PERF-001**: Cache reference data
   - In-memory cache with 5-min TTL for `animaltypes`, `caretypes`, `zones`
   - Invalidate on admin changes

7. **TODO-PERF-002**: Cache current user
   - Store serialized user in session cookie (signed)
   - Fallback to DB if cache miss or role changed

8. **TODO-PERF-003**: Add pagination to LandingIndex
   - Default 50 animals per page
   - Preserve existing "all" view for admins via query param

9. **TODO-PERF-005**: Connection pool tuning
   - Max open: 25, max idle: 10, max lifetime: 5m
   - Configurable via env vars

**Deliverable**: App runs efficiently on resource-constrained hardware.
**Testing**: Load test with 50 concurrent users; verify DB connection count < 25.

### Phase 5: Polish & Monitoring (Week 9)
**Goal**: Production readiness.

10. **TODO-PERF-008**: Request timing middleware
    - Log slow requests (>500ms) with endpoint and DB query count

11. **TODO-PERF-006**: HTTP cache headers for static assets
    - `Cache-Control: public, max-age=31536000` for hashed assets

12. **TODO-PERF-004**: Migrate EnrichAnimals to EnrichAnimalsOptimized
    - Update `registertable.go`, `registersnapshot.go`

13. **TODO-PERF-007**: Review Eager() usage
    - Replace with explicit preloading where N+1 detected

**Deliverable**: Production deployment with monitoring and optimized queries.
**Testing**: Full regression test suite passes; performance benchmarks meet targets.

### Rollback Strategy
- Each phase is independent and can be deployed separately
- Event table is additive — no changes to existing tables
- Feature flags: `ENABLE_EVENT_STREAM`, `ENABLE_CONSOLIDATION` env vars
- Database migrations are backward-compatible (new tables only until Phase 4)

## Common Gotchas
- `buffalo dev` handles both Go rebuilds and asset compilation — don't run `npm run dev` separately unless debugging webpack
- `public/assets/` is gitignored and regenerated on build
- Database must exist before migrations; Pop does not auto-create the DB
- The `db:seed` task is idempotent-ish but will fail if run before migrations
- Test docker-compose mounts `test/database.yml` (points to `db` host, not `localhost`)
