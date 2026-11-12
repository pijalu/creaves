# Creaves — Agent Guide

## Purpose and boundaries

Creaves is the per-center wildlife-care application. It owns animal lifecycle, care, feeding, treatments, veterinary history, reports, reference data, and the Care Expert system. It is the **push side** of Console synchronization.

Creaves Console is a separate module and database. Never add cross-module imports or direct Console database access. The only integration is `POST /webhook/events` with Bearer authentication.

## Read first

| Document | Use |
| --- | --- |
| [README.md](README.md) | Current features, architecture, setup, configuration, tasks, deployment, and troubleshooting |
| [docs/care-expert.md](docs/care-expert.md) | Current Care Expert design and operational invariants |
| [docs/care-presentation-guideline.md](docs/care-presentation-guideline.md) | Normative care-plan UI behavior |
| [docs/SETUP_AND_TESTING.md](docs/SETUP_AND_TESTING.md) | Combined Creaves/Console setup and validation |
| [../creaves-console/docs/specs/SYNC_SPEC.md](../creaves-console/docs/specs/SYNC_SPEC.md) | Canonical webhook v2 wire contract |
| [e2e/RESYNC_RUNBOOK.md](e2e/RESYNC_RUNBOOK.md) | Current recovery procedure |
| [docs/README.md](docs/README.md) | Curated documentation index |
| `docs/archive/` | Historical research, plans, fix rounds, and dated results; not current instructions |

Keep this guide short. Do not copy full contracts, route inventories, model catalogs, or historical fix narratives into it.

## Stack and commands

- Go **1.26**, Buffalo 1.1, Pop v6, Plush, MySQL/MariaDB.
- Development port: **3000**; development and tests both use MySQL.
- Install Buffalo with `go install github.com/gobuffalo/cli/cmd/buffalo@latest`.

```bash
go mod download
yarn install --frozen-lockfile
go generate ./models/careplan
buffalo pop migrate up
buffalo task db:seed
buffalo dev
```

The Care DSL parser is generated and ignored. Install `goyacc` and rerun `go generate ./models/careplan` after changing `models/careplan/dsl.y` or when a clean checkout lacks the generated parser.

`db:seed` reconciles the embedded production-derived reference set and translations and creates the initial admin if absent. It is idempotent but not a lightweight test fixture. Development login: `admin` / `admin`; change it outside a disposable environment.

Use `POP_PATH=/absolute/config-directory` to select an alternate `database.yml`. Verify the selected database before every migration, seed, fixture, repair, or rollback command.

## Testing requirements

```bash
GO_ENV=test go test -count=1 ./...
GO_ENV=test go test -count=1 ./actions -run TestName -v
go vet ./...
```

Tests require an isolated migrated `creaves_test` MySQL database. Never point tests at development or production data. Run focused tests before the full suite and report actual failures; never weaken, skip, or hard-code tests to make them pass.

Template, action, model, route, and schema changes require authenticated browser checks. Exercise every affected locale (`en-US`, `fr`, `de`, `nl`) and both success and error paths. Historical E2E logs in `docs/archive/` are evidence of old runs, not substitutes for a new check.

## UI and localization

Every user-facing UI change must update all four localized template variants and translation catalogs. Prefer shared partials only when behavior remains equivalent across locales. Check structural locale parity and run browser validation after changes.

## Care Expert invariants

- `models/careplan` is the pure-Go matcher, DSL, schedule, occurrence, and why-trace engine. Database resolution and application workflows live in `actions`.
- Human-authored plans (`created_by` set) must not be rewritten by converter or repair tasks. Converter-owned rows have explicit marker/source identity.
- Feeding startup repair is marker-guarded; treatment-series refresh is idempotent and runs at every boot so new prescriptions are discovered.
- Converter output is definitive. `needs_confirmation` is a manual review flag and never suppresses scheduling.
- `careplan:validate-feeding` and `careplan:validate-treatments` are read-only gates. Run them before any repair.
- `careplan:reconvert-treatments` deletes converter-owned non-feeding plans before recreation. Use only after backup and staging validation.
- Derived/managed rule rows have edit restrictions; do not bypass them through generic CRUD changes.

See the current design document for commands and data-flow details.

## Synchronization invariants

- Multiple sync targets are supported. Event production requires the event stream to be enabled; delivery requires an enabled target.
- The worker starts at boot, wakes on publication and target save, and has a 60-second fallback tick.
- Mark an event delivered only when Console explicitly acknowledges its event ID. Partial batches leave all other events retryable.
- Repeated failures open a 60-second circuit breaker. Preserve rate limits, batch limits, and per-target state.
- Full resync uses deterministic `animal_state` events, state hashes, instance metadata, and localized reference values. A classification change can intentionally alter hashes and cause one resend.
- Use `/webhook_resync` for backfill/recovery. Legacy `event:snapshot*` tasks are maintenance tools and do not announce the expected sync set.
- Keep the producer and receiver contract changes synchronized in both repositories. Run contract and error-path tests.

Do not duplicate the payload schema here; use the canonical specification linked above.

## Database and attachment safety

- Treat every existing database as potentially containing real animal and personal data.
- No destructive query, reset, cleanup, rollback, fixture restore, or production repair without explicit scope, a current backup, and a rehearsed rollback path.
- Validate migration down/up on a disposable clone, then replay forward. Keep migration filenames unique and update `migrations/schema.sql` when required.
- Back up attachments as well as the database. Never substitute a fabricated attachment record for an existing file.
- Read-only validators are preferred during diagnosis. Capture actual output before and after any write operation.

## Performance constraints

Preserve the existing reference-data and current-user caches, post-commit invalidation, bulk animal enrichment, bounded association loading, and hot-path indexes. Do not reintroduce per-row `Eager()` loading or whole-table plan enrichment on reviewed paths. Preserve complete reports/exports; pagination or truncation is a separate product decision. Measure before optimizing and add regression/query-count coverage where practical.

## Change checklist

1. Identify whether the change is local to Creaves or affects the Console contract.
2. Add or update regression tests, including failure paths.
3. Preserve all four UI locales and Care Expert ownership/marker invariants.
4. Validate migrations and repairs on an isolated database.
5. Run focused tests, the full MySQL suite, and authenticated browser checks as applicable.
6. Update only the relevant current guide; move dated research and completed plans to `docs/archive/`.
