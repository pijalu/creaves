# Archived: creaves-console — Docker build parity with creaves

**Date:** 2026-09-06
**Project:** creaves-console
**Commit:** `c756c0a` docker: build.sh + Dockerfile parity with creaves, DATABASE_URL production config
**Branch:** main

## Original report (from bugs.md)

creaves-console had no `build.sh`; its Dockerfile was stale (built the old
`cmd/consolidation` + `cmd/cli` binaries via plain `go build`, no
migrations/seed on start, no quickstart script). Production DB config used
`DB_NAME`/`DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASSWORD` instead of the
`DATABASE_URL` env pattern used by creaves, so it could not be pointed at a
remote db/port the same way as creaves (`export DATABASE_URL=...` before
starting the container).

Expected: creaves-console ships a `build.sh` equivalent to
`creaves/build.sh` (buildx multi-platform push to
`muaddib/creaves-console`), a Dockerfile that builds the buffalo app binary,
runs migrations + seed at startup (`dockerscript/quickstart`), and reads the
production DB connection from `DATABASE_URL` (same as
`creaves/database.yml` production), with `PORT`/`ADDR` honored like creaves.

## Fix

- **`build.sh`** (new): mirrors `creaves/build.sh` — `docker buildx build
  --platform linux/amd64,linux/arm64 --push -t muaddib/creaves-console .`
- **`Dockerfile`** (rewritten): multi-stage golang:1.21 builder →
  `buffalo build --environment production --static -o /bin/app`; alpine runtime
  with tzdata; `COPY dockerscript/* /bin/`; `ENV GO_ENV=production`,
  `ADDR=0.0.0.0`, `EXPOSE 3001`, `CMD /bin/quickstart.prod.sh`. No npm/yarn
  asset stage (console embeds templates/public via `go:embed`).
- **`dockerscript/quickstart.prod.sh` / `quickstart.dev.sh`** (new, exec bit
  100755): `export GO_ENV=production`, then `/bin/app migrate && /bin/app task
  db:seed && exec /bin/app` — same pattern as creaves.
- **`dockerscript/database.yml`** (new): copied into the image so pop finds its
  config in WORKDIR `/bin`. Production uses
  `url: {{envOr "DATABASE_URL" "mysql://consolidation:consolidation@(localhost:3306)/consolidation?..."}}`.
- **`database.yml`** production block switched from discrete
  DB_HOST/DB_PORT/... keys to the `envOr "DATABASE_URL"` form (parity with
  `creaves/database.yml`).
- **`actions/app.go`**: `ADDR` now read from env (default `127.0.0.1` for local
  dev; the image sets `ADDR=0.0.0.0`), `PORT` already env-driven (default
  3001).
- **`docker-compose.yml`**: web service wires `DATABASE_URL`,
  `ADDR=0.0.0.0`, `PORT=3001` through to the container.

### Bugs discovered during e2e (and fixed)

1. **`/bin/sh: /bin/quickstart.prod.sh: Permission denied` (exit 126)** — the
   new dockerscript files were created without the git exec bit. `chmod +x` +
   re-add → mode `100755`, matching creaves.
2. **`unable to find pop config file` (exit 1)** — pop needs `database.yml` in
   WORKDIR. creaves ships `dockerscript/database.yml`; console's dockerscript
   had none. Added `dockerscript/database.yml` with the `envOr "DATABASE_URL"`
   production block.

## Quality gates (creaves-console)

- `go vet ./...` — PASS
- `staticcheck ./...` — PASS
- `gocognit -over 15 .` — PASS (pre-existing only, files untouched by this
  change: UpdateFromPayload 38, installSafePopTxLogger 25,
  TestOutcomeHelpersAcceptValueAndPointer 23, SyncManagementIndex 16)
- `gocyclo -over 12 .` — PASS (pre-existing only, untouched files:
  UpdateFromPayload 37, LocalizedField 21, SyncManagementIndex 14,
  applyConsolidatedAnimalFilters 14, …)
- `CGO_ENABLED=1 go test -count=1 -race -cover -tags sqlite ./...` — PASS
  (actions 56.9%, excel 56.0%, models 58.4% coverage)

Only Go file changed is `actions/app.go` (trivial ADDR `envy.Get`); all
complexity findings are in files this change did not touch.

## Docker + browser e2e verification

- `docker build -t muaddib/creaves-console .` — PASS (only legacy-ENV /
  JSON-CMD warnings, identical in form to `creaves/Dockerfile`).
- Ran against a separate `mariadb:latest` container on a dedicated network,
  started exactly like the creaves remote shell:
  `export DATABASE_URL='mysql://consolidation:consolidation@(console-e2e-db:3306)/consolidation?...'`
  then `docker run -d --rm -p 3001:3001 --env DATABASE_URL=$DATABASE_URL --env
  TZ="Europe/Brussels" --name creaves-console muaddib/creaves-console`.
- Logs: quickstart.prod.sh ran, **16 migrations applied**, `Admin user created:
  login=admin`, server `starting simple server on 0.0.0.0:3001`, status **Up**.
- `curl http://localhost:3001/` → 302 → `/auth/new`; `/auth/new` → 200, title
  "Consolidation - Animal Care Authority".
- **agent-browser**: logged in as admin/admin123 → redirected to `/`, dashboard
  rendered with "Administrator", nav (Dashboard/Animals/Reports/Admin),
  "Animals by Status" table. ✓

The same shell pattern used for creaves (`muaddib/creaves-console` image,
`DATABASE_URL`, port) now works unchanged for creaves-console.
