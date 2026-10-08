# Testing Coverage Plan

> Goal: add test coverage to **all key packages** of the application — not only
> the event-forwarding subsystem that is already tested.

## 1. Current state (baseline)

Two Go modules share this repo:

| Module          | Go   | Role                |
|-----------------|------|---------------------|
| `creaves`       | 1.18 | main application    |
| `creaves-console` | 1.21 | admin / console app |

### Tested today (event-forwarding focus)
- `creaves/actions`: `event_processor`, `event_producer`, `feeding`, `instance`, `webhook_pusher`
- `creaves/models`: `config`, `consolidated_animal`, `event_stream`
- `creaves-console/actions`: `event_processor`, `webhook`, `webhook_api_keys`
- `creaves-console/models`: `models`

### Untested (the gap this plan closes)
Pure-logic packages and model helpers across the board.

## 2. Testing strategy

1. **Prefer pure unit tests** — construct structs / call functions directly, assert
   on return values. These need **no database**, run fast, and are stable in CI.
2. **Focus on logic-bearing functions** — formatters, calculators, parsers,
   validators-without-DB, and helper functions. Skip trivial getters and
   embed/`main.go` boilerplate.
3. **Each micro-task targets one cohesive package or file-group** so it fits in a
   single clean-context goal (minimal token usage).
4. **Target ≥80% coverage** on the *logic-bearing* functions of each package
   (not blanket file coverage — ORM/CRUD handlers needing a live DB are
   out-of-scope for this pass).
5. Every new test file must `go test` green and `go vet` clean.

## 3. Out of scope (this pass)
- Buffalo CRUD handlers (`List/Show/Create/...`) that require a `buffalo.Context`
  and live DB — covered separately by integration tests.
- `grifts/` DB seed/migration scripts.
- `cmd/*/main.go` entrypoints, `*/embed.go` FS boilerplate.

## 4. Micro-tasks

Each micro-task below is executed as a **dedicated goal with fresh context**.

### creaves module

#### M1 — `models/` helper & formatter unit tests
- **Scope:** all model files' pure methods — `String()`, `DateFormated()`,
  `YearNumberFormatted()`, `ZoneAsString()`, `OrderedKeys()`, `LastWeight()`,
  `FeedingPeriodHourMinute()`, `FeedingStartFmt()`, `FeedingEndFmt()` and similar
  across: `animal`, `discovery`, `intake`, `outtake`, `travel`, `treatment`,
  `care`, `species`, `veterinaryvisit`, `logentry`, `drug`, `zone`, `locality`,
  `entry_cause`, `animalage`, `animaltype`, `caretype`, `discoverer`,
  `native_status`, `outtaketype`, `subside_group`, `traveltype`, `user`.
- **Files to create:** `models/<name>_test.go` adjacent to each source file
  (only for files that contain pure methods worth testing).
- **Verify:** `go test ./models/ -v` green; coverage of logic functions ≥80%.

#### M2 — `actions/` pure helper functions
- **Scope:** the testable helper functions, not the CRUD handlers:
  `containsUUID`, `LoadConfig`/`GetInstanceID`/`IsEventStreamEnabled`/
  `IsWebhookEnabled` (if pure / mockable), `calculateFeeding`,
  `calculateFeedings`, `suggest`, `EnrichAnimals*` helpers, `setupContext`
  where feasible.
- **Files:** extend existing `actions/*_test.go` or add new `_test.go`.
- **Verify:** `go test ./actions/` green; new helpers covered.

#### M3 — `utils/` package
- **Scope:** `TrimStringFields` (reflection-based field trimmer).
- **File:** `utils/utils_test.go`.
- **Verify:** `go test ./utils/` green.

#### M4 — `stuff/feeding/` package
- **Scope:** `calculateNextMealTime` (already partially covered — harden edge
  cases: invalid formats, boundary times, frequency logic).
- **File:** extend `stuff/feeding/feed_test.go`.
- **Verify:** `go test ./stuff/feeding/` green.

#### M5 — `excel/` + `export/` query-config parsers
- **Scope:** `sheetPosition`, `getConfig`, `getQuery` (config parsing logic).
- **Files:** `excel/excel_test.go`, `export/export_test.go`.
- **Verify:** `go test ./excel/ ./export/` green.

### creaves-console module

#### M6 — `models/` helpers
- **Scope:** `webhook_api_key`, `user`, `import_run`, `event_stream`,
  `consolidated_animal` pure methods (String(), formatters, key generation).
- **Files:** `models/<name>_test.go`.
- **Verify:** `go test ./models/` green.

#### M7 — `actions/` helpers (consolidation, dashboard)
- **Scope:** pure helper functions in `consolidation_runner.go`, `dashboard.go`,
  `users.go`, `event_processor.go` that do not require a live `buffalo.Context`.
- **Files:** extend or add `_test.go` in `creaves-console/actions/`.
- **Verify:** `go test ./actions/` green.

## 5. Execution model

- One **queued goal per micro-task (M1–M7)**, each with `freshContext: true`,
  so context stays minimal.
- Each goal's completion criterion = the package's `go test` command exits 0
  and adds meaningful coverage.
- Progress is tracked via the goal queue. Tasks are independent and can run in
  any order.
