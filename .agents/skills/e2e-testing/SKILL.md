# e2e-testing (Creaves + Console)

How to bring up the seeded two-app e2e environment and run the agent-browser
validation suite (plan §7.2). Use the `agent-browser` skill for the browser
CLI mechanics; this skill documents the environment and fixture data.

## Fixture data (fixed, known — assert exact numbers)

- `buffalo task db:seed:e2e` (this repo) seeds 9 animals on a fresh DB:
  2025 ×7 (one with an `E2E_ERR` error-outtake, excluded from all report
  tables), 2024 ×2. Species with taxonomy row (`E2E_Hedgehog`, `E2E_Sparrow`,
  `E2E_Newt`), one without (`E2E_NOSPEC` → "Unknown" buckets), NULL ring /
  gender / city on animal 250006, and `E2E City; "Nord"` for CSV escaping.
  It also sets config: instance `e2e-instance-a`, webhook enabled →
  `http://127.0.0.1:3001/webhook/events` with key `e2e-console-key-0123456789`.
- `buffalo task db:seed:e2e` (console repo) seeds the matching bcrypt key, the
  `e2e-instance-b` instance and 6 consolidated rows (2025 ×5 incl. one fully
  NULL-category row, 2024 ×1).
- Full per-table expected counts/% live in `e2e/EXPECTATIONS.md` (this repo).

## Bring-up (from scratch)

```sh
# Creaves (port 3000) — fresh DB, base seed, e2e fixtures
cd creaves
buffalo pop drop -e development && buffalo pop create -e development \
  && buffalo pop migrate -e development
buffalo task db:seed && buffalo task db:seed:e2e
nohup buffalo dev > /tmp/creaves-dev.log 2>&1 &

# Console (port 3001)
cd ../creaves-console
buffalo pop drop -e development && buffalo pop create -e development \
  && buffalo pop migrate -e development
buffalo task db:seed && buffalo task db:seed:e2e   # admin login: admin / admin123
nohup buffalo dev > /tmp/console-dev.log 2>&1 &

curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:3000/   # 302
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:3001/   # 302
```

## Feed the console through the real webhook path

Instance A rows must arrive via resync (exercises the full contract path):

```sh
agent-browser open http://127.0.0.1:3000/auth/new
# login admin / admin, then:
agent-browser open http://127.0.0.1:3000/webhook_resync
agent-browser snapshot -i            # find "Start resync" button
agent-browser click @<ref>
sleep 20                             # pusher ticks every 5s
# verify:
mysql -ucreaves -pcreaves consolidation -e \
  "SELECT instance_id, COUNT(*) c FROM consolidated_animals GROUP BY instance_id"
# expect: e2e-instance-a = 9, e2e-instance-b = 6
```

Key columns that must be populated on instance A rows (contract v2):
`animal_age`, `animal_type`, `species_class`, `entry_cause`,
`entry_cause_detail`, `entry_cause_nature`, `outtake_type`,
`outtake_rating`, `outtake_dead`, `translations`.

## Suite

```sh
cd creaves && ./e2e/run.sh        # asserts all scenarios, both apps, 4 languages
```

Scenarios (plan §7.2): Creaves 1–6 (filters, CSV export content, annual report
exact numbers 2024/2025, annual CSV, 4-language spot-checks, nav links);
Console 1–5 (filters, export, scope all-centers vs A vs B exact numbers,
annual CSV per scope, 4-language + navbar); contract e2e (create/edit animal
in Creaves → console row updated; resync backfills new columns).

## Gotchas

- Creaves login is `admin` / `admin`; console login is `admin` / `admin123`.
- `pkill -f creaves` kills the console too (path match). Use
  `pkill -f 'buffalo dev'` plus exact binary paths.
- `db:seed:e2e` refuses to run twice — reset the DB first.
- Resync uses an explicit `Eager(...)` list in
  `actions/webhook_resync.go`; adding payload fields requires adding the
  association there or events go out with NULLs (regression test:
  `TestRunResyncEmitsNestedOuttakeAndEntryCause`).
- `agent-browser` refs are invalid after navigation — snapshot again.
- Language switch: `open 'http://127.0.0.1:3000/lang/?lang=de&url=/'`
  (fr, en-US, de, nl).

## Verify unit suites (both must stay green)

```sh
cd creaves && GO_ENV=test go test ./...
cd ../creaves-console && CGO_ENABLED=1 go test -tags sqlite ./...
```

**CRITICAL: `GO_ENV=test` is mandatory for raw `go test` in creaves.**
Without it, `models.init()` does `pop.Connect("development")` and the test
binary opens a pool against the **dev** database; the search tests then hang
for 10 minutes (pool starvation — 20 leaked-looking `awaitDone` txs) and the
suite dies with `panic: test timed out`. `buffalo test` sets GO_ENV for you;
plain `go test` does not. Symptom: `TestAnimalSearchFilterYear` stuck on the
first fixture insert with zero statements logged.

If `creaves_test` schema drifts (e.g. `Table 'creaves_test.resync_runs'
doesn't exist`): run `buffalo pop migrate up -e test`. If collations/types
drift beyond repair, rebuild: `DROP DATABASE creaves_test; CREATE DATABASE
creaves_test CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;` then
`mysqldump -ucreaves -pcreaves --no-data --skip-triggers creaves | mysql
-ucreaves -pcreaves creaves_test`.
