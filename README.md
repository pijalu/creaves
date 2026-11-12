# Creaves

Creaves is the per-center wildlife-care application. It manages animal intake, daily care, feeding, treatments, veterinary history, releases and deaths, reference data, reports, and the rule-driven Care Expert system. Each installation owns its own MySQL/MariaDB database.

Creaves can publish animal lifecycle events to one or more Creaves Console instances. Console is a separate application and database; there is no shared code, shared database, or Console-to-Creaves control channel.

## Current features

- **Animal lifecycle**: reception/discovery, intake, in-care history, travel, release, death, and erroneous-record outtakes.
- **Daily care**: cares, feeding schedules, medications and treatments, weighing, observations, cage cleanup, veterinary visits, attachments, audits, and TODOs.
- **Care Expert**: matcher/rule library, protocol editor, per-animal plans and overrides, legacy-plan conversion, day plan, occurrence application, deferral, and validation tools.
- **Registers and reporting**: searchable animal lists, annual statistics, corpse/discoverer reports, configured CSV/Excel exports, and printable views.
- **Administration**: users and roles, species and taxonomy, native statuses, outtake rules, zones, drugs, care types, note templates, and localization.
- **Integration**: multiple webhook targets, delivery tracking and retry, circuit breaking, event snapshots, full resync, and recovery tooling.
- **Localized UI**: English (`en-US`), French (`fr`), German (`de`), and Dutch (`nl`). User-facing changes must preserve all four template/locale variants.

## Architecture

| Area | Responsibility |
| --- | --- |
| `actions/` | Buffalo handlers, authorization, Care Expert orchestration, reports, event production, webhook delivery, and resync |
| `models/` | Pop models, validation, persistence, and the pure-Go `models/careplan` engine |
| `migrations/` | Ordered SQL/Fizz migrations and the generated `schema.sql` snapshot |
| `grifts/` | Seeds, maintenance tasks, conversion validation, snapshots, and translation tooling |
| `templates/`, `locales/` | Plush UI in four locales and translation catalogs |
| `assets/`, `public/` | Webpack sources and compiled static assets |
| `e2e/` | Current fixture expectations and the resync operator runbook; historical results are archived |
| `docs/` | Small set of current guides; `docs/archive/` contains historical research and closed work |

The application uses server-rendered Plush pages with session authentication and CSRF protection. Reference-data caching and batched association loading are used on hot paths; behavior-preserving query constraints are documented in `AGENTS.md`.

### Care Expert design

Matchers select animal contexts, rules attach action payloads and schedules to matchers, and per-animal plans override or instantiate those rules. The engine calculates occurrences and why-traces without requiring database access. Database-backed handlers resolve plans, apply or defer occurrences, and preserve human-authored plans during idempotent legacy conversion. Startup migrations and validators are marker-guarded; repair commands are not routine setup steps.

See [Care Expert current design](docs/care-expert.md) and the normative [Care Presentation Guideline](docs/care-presentation-guideline.md).

### Console synchronization

Creaves is the **push side**. Animal changes produce rows in `event_streams`; enabled sync targets deliver batches to `POST /webhook/events` on Console with a Bearer API key. Only Console-confirmed event IDs are marked delivered. Failed events remain retryable, duplicate event UUIDs are idempotent, and a circuit breaker pauses repeated endpoint failures.

Full resync sends deterministic `animal_state` snapshots, including localized reference values and the fields required by the v2 contract. Normal operation starts automatically with the web application; the legacy `event:snapshot*` commands are maintenance tools, not a replacement for configured webhook delivery.

The canonical wire contract is [Console Sync Specification v2](../creaves-console/docs/specs/SYNC_SPEC.md). Operator steps are in [the resync runbook](e2e/RESYNC_RUNBOOK.md).

## Requirements

- Go **1.26** (see `go.mod`)
- Buffalo CLI: `go install github.com/gobuffalo/cli/cmd/buffalo@latest`
- MySQL 8 or MariaDB for development, tests, and production
- Node.js and Yarn for the Webpack asset pipeline
- `goyacc` only when regenerating the Care Expert parser: `go install golang.org/x/tools/cmd/goyacc@latest`
- Docker with Buildx for optional container builds

## Local setup

> Use a disposable database. Existing databases may contain real animal and personal data; never run seeds, repairs, cleanup, migration rollbacks, or fixtures against production without an explicit backup and approved maintenance plan.

1. Install dependencies:

   ```bash
   go mod download
   yarn install --frozen-lockfile
   go generate ./models/careplan
   ```

2. Create the development and test databases and a local user matching `database.yml`:

   ```sql
   CREATE DATABASE creaves CHARACTER SET utf8mb4;
   CREATE DATABASE creaves_test CHARACTER SET utf8mb4;
   CREATE USER 'creaves'@'localhost' IDENTIFIED BY 'creaves';
   GRANT ALL ON creaves.* TO 'creaves'@'localhost';
   GRANT ALL ON creaves_test.* TO 'creaves'@'localhost';
   ```

   Adjust credentials for the local installation; do not reuse these development defaults in production.

3. Migrate and seed:

   ```bash
   buffalo pop migrate up
   buffalo task db:seed
   buffalo dev
   ```

   The development server listens on **http://127.0.0.1:3000**. The initial development login is `admin` / `admin`; change it before any shared or production deployment.

`db:seed` is an idempotent reference-data and startup-translation reconciliation, not a small fixture. It loads the embedded production-derived reference set, preserves existing values where documented, and creates the default admin if needed. It is not part of the application test harness.

## Configuration

| Setting | Purpose |
| --- | --- |
| `GO_ENV` | `development`, `test`, or `production`; defaults to `development` |
| `DATABASE_URL` | Production MySQL/MariaDB URL; include `parseTime=true` and `multiStatements=true` |
| `POP_PATH` | Directory containing an alternate `database.yml`; useful for isolated non-production runs |
| `ADDR` | Listen address; development defaults to `127.0.0.1`, containers use `0.0.0.0` |
| `PORT` | HTTP port; default `3000` |
| `DEFAULT_TZ` | Container timezone; default `Europe/Brussels` |
| `INSTANCE_ID` | Optional center identifier override; otherwise the persisted configuration is used |
| `TRUSTED_PROXIES` | Comma-separated trusted proxy CIDRs/IPs for guest client-IP handling |

Configure center identity and sync targets in **Administration → Synchronization**. Each target needs the Console URL, an instance-specific API key, enablement, batch size, and rate limit. Keep API keys in runtime configuration, never in documentation or source control.

## Everyday use

1. Configure center identity and reference data.
2. Record discovery/intake, then manage the animal from its detail page.
3. Review the dashboard and `/care_plan` for due work; apply or defer occurrences with the required confirmation data.
4. Maintain protocols through Care Rules and the animal Protocol tab rather than editing generated JSON directly.
5. Record outtakes for release, death, or erroneous records; lifecycle events then flow to enabled Console targets.
6. Use **Administration → Webhook resync** for progress and recovery, not repeated manual snapshot creation.

## Maintenance commands

| Command | Purpose |
| --- | --- |
| `buffalo task db:seed` | Reconcile startup reference data and create the default admin if absent |
| `buffalo task db:seed:startup` | Run only the embedded startup reference-data reconciliation |
| `buffalo task careplan:seed` | Add missing Care Expert library matchers/rules idempotently |
| `buffalo task careplan:validate-feeding` | Read-only comparison of converted feeding plans with legacy fields |
| `buffalo task careplan:validate-treatments` | Read-only check that future treatment series have active converted plans |
| `buffalo task careplan:repair-feeding` | Marker-guarded feeding repair; use after backup and validation |
| `buffalo task careplan:migrate-cage-rules` | Marker-guarded legacy cage-rule conversion |
| `buffalo task careplan:reconvert-treatments` | Delete and recreate converter-owned non-feeding plans; human-authored plans are preserved |
| `buffalo task event:snapshot:stats` | Show event snapshot statistics |
| `buffalo task event:snapshot` | Create missing legacy snapshots; not the normal resync path |
| `buffalo task admin:reset-password` | Development/fixture-only admin reset; never use on production |
| `python3 scripts/update_translation_seeds.py --check` | Check embedded translation SQL for drift |

Repair and reconversion commands can change large data sets. Run read-only validators first, capture their output, back up the database, and rehearse on a clone.

## Testing

Creaves tests use **MySQL**, including `creaves_test`; they do not use Console's SQLite harness.

```bash
GO_ENV=test go test -count=1 ./...
GO_ENV=test go test -count=1 ./actions -run TestName -v
go vet ./...
go generate ./models/careplan
```

Run browser tests with authenticated sessions for template, route, model, or schema changes. Exercise all four locales where UI text or structure changes. The combined local setup and cross-application workflow is in [Setup & Testing](docs/SETUP_AND_TESTING.md); fixed fixture expectations are in [e2e/EXPECTATIONS.md](e2e/EXPECTATIONS.md).

## Build and deployment

```bash
buffalo build --environment production -o bin/creaves
export GO_ENV=production
export DATABASE_URL='mysql://user:password@(host:3306)/creaves?parseTime=true&multiStatements=true'
./bin/creaves migrate
./bin/creaves task db:seed
./bin/creaves
```

The production binary embeds migrations, templates, locales, and static assets. Back up both the database and attachment storage before upgrades. Apply and verify migrations on a production-like clone first, keep the previous binary available, and confirm the schema level before switching traffic.

### Docker

```bash
docker build -t creaves .
docker run --rm -p 3000:3000 \
  -e DATABASE_URL="$DATABASE_URL" \
  -e ADDR=0.0.0.0 -e PORT=3000 creaves
```

The container entrypoint runs migrations and `db:seed`, then starts the server. Mount attachment storage where durable files are required. `test/docker-compose.yml` is an isolated development stack, not a production compose file.

## Troubleshooting

- **Parser missing after a clean checkout**: install `goyacc` and run `go generate ./models/careplan`.
- **Tests cannot connect**: confirm `GO_ENV=test` and that `creaves_test` is migrated and accessible with `database.yml` credentials.
- **Events remain undelivered**: check target enablement, instance ID, API key, Console logs, circuit-breaker state, and event error details.
- **Resync appears stuck**: inspect `/webhook_resync/status.json` and application logs before starting another run.
- **Care conversion mismatch**: run the read-only feeding/treatment validators; do not immediately run destructive repair tasks.
- **Translation drift**: run `python3 scripts/update_translation_seeds.py --check` and review `translation_worklist.csv`.

## Documentation and contributing

Start with [the documentation index](docs/README.md). `AGENTS.md` contains the concise engineering and safety rules. Only current design, operations, and runbooks live outside `docs/archive/`; dated research, fix rounds, plans, and results are historical and must not be used as current setup instructions.

For changes: keep migrations reversible, preserve webhook compatibility, test error paths, regenerate the parser when the DSL grammar changes, update all four UI locales, and never weaken tests to obtain a green run.

No repository-level license file is currently provided; contact the project owner before redistribution.
