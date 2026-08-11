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
