# AGENTS.md — Creaves

Bird sanctuary day-to-day management webapp (intakes, animals, cares, treatments,
feeding schedules, travels, vet visits, exports). Actively developed ("very ongoing
dev"). Go backend, MySQL, jQuery/Bootstrap frontend.

## Tech stack

| Layer | Tech |
|---|---|
| Language | Go 1.18 module (`go.mod` says 1.18; local toolchain 1.26) |
| Web framework | [Buffalo](https://gobuffalo.io) v0.18.9 |
| ORM / DB | [Pop](https://github.com/gobuffalo/pop) v6.1.0 + MySQL (MariaDB-compatible), Fizz migrations |
| Middleware | buffalo-pop `popmw.Transaction`, `mw-csrf`, `mw-paramlogger`, `mw-i18n/v2`, custom auth |
| Templating | Plush `.plush.html` (embedded via `go:embed` in `templates/embed.go`) |
| i18n | `locales/*.yaml` — **en-us and fr** (embedded via `locales/embed.go`) |
| Frontend | webpack 5, Bootstrap 4.6, jQuery, select2, bootstrap-table, flatpickr (in `assets/`) |
| Exports | CSV (`export/`) and Excel (`excel/`, excelize v2), both YAML-config-driven queries |
| Tasks | Grifts (`grifts/`) — e.g. `db:seed` |
| Auth | Custom username/password, bcrypt hashes, `admin` / `approved` / `shared` flags |

## Verified commands

Run from repo root. MySQL **must be running locally** for anything importing
`models` (its `init()` calls `pop.Connect`).

Dev server — **`buffalo dev`** (config in `.buffalo.dev.yml`, builds `cmd/app`):
```sh
buffalo dev
```

Build — **`build.sh`** (this is THE build: docker buildx multi-arch, pushes to
Docker Hub `muaddib/creaves`):
```sh
./build.sh
```

Webpack (frontend assets; also run inside the Docker build):
```sh
npm run dev       # watch
npm run build     # production build -> public/assets (gitignored)
```

Compile/test sanity (not the deployment build):
```sh
go build ./...            # WORKS (verified)
go test ./actions/        # WORK (verified; needs local MySQL via database.yml "test" env)
go test ./stuff/...       # WORK (verified; pure unit tests — feeding logic)
go test ./...             # FAILS (pre-existing, not you) — vet failures in
                          # drugs/gen.go (fmt.Print w/ %d) and excel/testdir/bla.go
                          # (int→string conversion). Do NOT "fix" unless asked —
                          # they are generators/tooling, unrelated to app code.
```

DB config: `database.yml` — dev/test both point to
`mysql://creaves:creaves@(localhost:3306)/creaves?parseTime=true&multiStatements=true&readTimeout=3s`.

## Project layout

| Path | Purpose |
|---|---|
| `cmd/app/main.go` | Entry point (`actions.App().Serve()`) |
| `actions/app.go` | **All routes + middleware** (nerve center) |
| `actions/render.go` | Render engine, custom helpers (`bool2html`, `dbgDump`) |
| `actions/*.go` | Handlers; one `XxxResource` per model + custom pages (dashboard, feeding, reception, registertable, registersnapshot, suggestions, export) |
| `models/*.go` | Pop models + validations; `models.go` opens `DB`, `constants.go` sets time func |
| `migrations/` | Fizz up/down pairs (49 migrations; + `schema.sql`, `trigger-animal.sql`) |
| `templates/<plural>/` | Per-resource Plush views; each has `.plush.html` + `.plush.fr.html` |
| `locales/` | i18n YAML per resource + `all.*.yaml`, in `en-us` and `fr` |
| `grifts/` | Task definitions incl. `db:seed` (creates admin + all reference data) |
| `export/`, `excel/` | YAML-configured SQL queries → CSV / Excel reports |
| `drugs/` | `gen.go` code-generates `grifts/create_drugs.go` from `drugs.csv` |
| `assets/` | Frontend sources (scss/js) compiled by webpack to `public/assets` |
| `public/`, `templates/`, `locales/` | Have `embed.go` for `go:embed` (Buffalo FS) |
| `utils/` | Shared helpers (e.g. `TrimStringFields`) |
| `stuff/` | Standalone `main` experiments (feeding scheduling) |
| `test/` | Docker-based test env (`docker-compose.yml`, `Dockerfile`, `start.sh`) — Docker daemon not running locally |
| `db/` | DB backups/dumps (`.sql.gz`) |

## How requests flow

1. `actions.App()` wires middleware top-down — order matters (comment in file):
   paramlogger → CSRF → `popmw.Transaction` (tx available as `c.Value("tx").(*pop.Connection)`) → i18n.
2. Auth: `SetCurrentUser` + `Authorize` applied after `/` route; `/auth`, `/lang`, `/registration` skip it.
3. Every handler gets a **transaction from `c.Value("tx")`** — use it instead of `models.DB`
   for request work. Models accept `tx *pop.Connection` in validators.
4. Routes/resources: `app.Resource("/animals", AnimalsResource{})` and explicit
   `app.GET/POST/...`. `ServeFiles` catch-all must stay last.
5. Views rendered via `r.HTML("...plush.html")`; translations via `T.Translate(c, "key")`.

## Conventions

- **Naming (Buffalo):** model singular (`Animal`) ↔ table plural (`animals`) ↔
  resource plural (`AnimalsResource`) ↔ path plural (`/animals`) ↔ template folder plural.
- **i18n:** two mechanisms — action-side `T.Translate(c, "users.xyz")` for flash
  messages/errors, and **separate template files** for views: every view exists as
  `index.plush.html` (default/en) + `index.plush.fr.html` (French). The i18n middleware
  sets the `languages` context key and Buffalo's render engine auto-picks the `.fr.html`
  variant. Templates do **not** use an inline `t()` helper. When adding UI text, update
  BOTH template files; when adding action strings, update both `en-us` and `fr` YAML
  files in `locales/`.
- **Nullable fields:** use `github.com/gobuffalo/nulls` (`nulls.String`, `nulls.Time`,
  `nulls.UUID`, `nulls.Bool`), not pointers, for nullable DB columns.
- **IDs:** most tables use `uuid.UUID` PKs (`github.com/gofrs/uuid`); `animals` uses int `ID`.
- **Time:** `models.DateTimeFormat = "2006/01/02 15:04"`, `models.DateFormat = "2006/01/02"`.
  `models.NowOffset()` exists because Pop parses timestamps as UTC — always use it
  instead of `time.Now()` for DB timestamps (global `pop.SetNowFunc` already does this).
- **Trim:** call `utils.TrimStringFields(&model)` before save (trailing spaces are a known issue).
- **Validation:** implement `Validate(tx)`, `ValidateCreate`, `ValidateUpdate` per model
  (generated pattern; `validators.StringIsPresent` etc.).
- **Performance:** hand-rolled `Enrich*` helpers (`EnrichAnimalsOptimized`,
  `EnrichCares`) bulk-load associations with `WHERE id IN (?)` instead of N+1 preloads.
  Use them when listing; keep them efficient (map-based).
- **Exports:** add queries to `export/config.yaml` (CSV) / `excel/config/config.yaml` (Excel)
  rather than hard-coding SQL in handlers where possible.

## Testing notes

- Only two test files: `actions/feeding_test.go` and `stuff/feeding/feed_test.go`
  (feeding-schedule calculation — table-driven; add cases here when touching feeding logic).
- `actions` tests require local MySQL up and the `test` env in `database.yml` to resolve.
- No CI workflow in repo. `test/` docker-compose exists but Docker isn't running locally.
- `go vet` will flag `drugs/gen.go` and `excel/testdir/bla.go` — pre-existing, ignore
  unless the task is explicitly about those dirs.

## Gotchas

- **`go test ./...` fails** — vet failures in `drugs/gen.go`, `excel/testdir/bla.go`
  (see Verified commands). Don't chase these during unrelated work.
- **Transaction middleware:** use `c.Value("tx")`, not `models.DB`, inside handlers;
  wrapping things in manual transactions on top can deadlock/timeout.
- **Time zones:** feeding logic is timezone-sensitive; `constants.go` compensates for a
  Pop UTC-parsing bug. Don't "simplify" it casually.
- **Routing order:** middleware/routes declared top-down in `app.go`; adding routes after
  `ServeFiles` means they never get called.
- **`.goa/`** is untracked local agent tool state — don't commit it or rely on it.
- **`bin/`, `tmp/`, `public/assets/`, `node_modules/`, `vendor/`, `db/`, `test/db_data`**
  are gitignored — never commit build output.
- Docker build is the standard build path: `build.sh` runs `docker buildx build`
  (multi-arch, pushes to Docker Hub `muaddib/creaves`); `test/docker-compose.yml` + `test/Dockerfile`
  provide a containerized dev/test env (Docker daemon not running locally).

## Branching

- Default branch `main`; work happens on `feature/*` branches (e.g. `feature/excel`,
  `feature/perfs`). Remote: `git@github.com:pijalu/creaves.git`.
- Create a feature branch for non-trivial changes; keep `main` green (`go build ./...` +
  `go test ./actions/ ./stuff/...`).
