# Fix plan — Bug 9 — creaves-console: reports dead count (outcome-based grouping)

> **Status: IMPLEMENTED.** Both repos' test suites green; quality gates pass
> (pre-existing complexity warnings only, see Validation). UpdateFromPayload
> complexity went DOWN (gocognit 45→38, gocyclo 43→37) after extracting
> `applyOuttake`.

## Bug (from bugs.md)
"The number of dead (0 Décédés) is incorrect — group by
positive/neutral/negative outtake. This should be the general approach for
*all* deceased: outcome with positive/neutral/negative."

## Root-cause analysis
1. **Producer event-type choice** (`creaves/actions/outtakes.go:235`): on
   outtake creation the animal_died vs animal_released event is chosen via
   `outtake.Type.Error` (the "duplicate/erroneous entry" flag), NOT
   `outtake.Type.Dead`. A euthanasia outtake (Dead=true, Rating=−1,
   Error=false) publishes `animal_released` → console stores
   `current_status='released'`. Maintenance resync (`maintenance.go:125`) and
   snapshot grift (`event_snapshot.go:92`) use `Type.Dead` instead — the two
   paths disagree.
2. **Payload drops explicit zero values** (`creaves/models/event_stream.go`):
   `OuttakePayload.Rating` and `.Dead` carry `json:",omitempty"`, so a neutral
   rating (0) or dead=false never crosses the wire.
3. **Consumer guards drop zero values again**
   (`creaves-console/models/consolidated_animal.go:281-285`):
   `if payload.Outtake.Rating != 0` / `if payload.Outtake.Dead` — even when
   present, neutral/dead=false are not stored.
4. **Console reports count died via `current_status='died'`**
   (`dashboard.go` ReportsIndex + by_species/by_location/by_type) instead of
   the outtake outcome (`outtake_rating`), so all animals killed by a
   non-"Error" outtake type are invisible in the died column.
5. Console annual report renders SQL literals 'Dead'/'Neutral'/'Alive'/
   'Released'/'Unknown' untranslated (Creaves side localizes them via
   `localizeAnnualSections`).

## Fix plan
### Console (this repo)
1. `models/consolidated_animal.go` — DONE. Outtake application extracted to
   `applyOuttake`: stores `OuttakeRating`/`OuttakeDead` unconditionally when
   an outtake block is present (any of Date/Type/Location set), and derives
   `current_status` from the outcome (negative rating or dead=true → 'died',
   even on `animal_released` events; non-negative outtake → 'released' unless
   already 'died').
2. `actions/dashboard.go` — DONE. `sqlOutcomeDied`/`sqlOutcomeReleased`
   fragments: died = `outtake_rating < 0 OR outtake_dead = 1 OR
   (current_status='died' AND no outtake outcome)`; released = non-negative
   outcome or legacy status fallback. Applied to ReportsIndex (new
   `tallyOutcomes` helper feeding `released`/`died` plus new
   `outcome_positive/neutral/negative` stats) and to the released/died
   columns of by_location/by_type/by_species.
3. `actions/reports_annual.go` — DONE. `localizeAnnualSections` +
   `annualCategoryLabels` map Dead/Neutral/Alive/Released/Unknown into
   fr/de/nl/en-US; wired into `annualReportData` (covers HTML page and CSV).
4. Templates — DONE. reports/index (en/fr/de/nl) gains an outcome card row
   (positive/neutral/negative deceased) under the status cards.

### Creaves (producer, contract-preserving)
5. `models/event_stream.go` — drop `json:",omitempty"` from
   `OuttakePayload.Rating` and `.Dead` only: explicit `rating:0` /
   `dead:false` must cross the wire so the console can store neutral
   outcomes. Backwards compatible (adds fields; old consumers ignore them).
6. `actions/outtakes.go` event-type choice (Error vs Dead) is LEFT UNCHANGED:
   the console now derives outcome/status from the payload's rating/dead
   fields, so the died count no longer depends on which event type was used.

### Console status derivation (replaces producer-side reliance)
7. `models/consolidated_animal.go` UpdateFromPayload: when the outtake block
   is present and carries a negative rating or dead=true, force
   `current_status='died'` even for `animal_released` events (covers the
   producer's Error-flag quirk); non-negative outtake → 'released'.

### Tests
- Console model test: payload with Rating 0 / Dead false stores explicit
  neutral/false; snapshot reset still clears them.
- Console reports index sqlite test: fixtures with rating −1/0/1 and
  status died-without-outtake; assert died/released counts and new outcome
  numbers.
- by_species/by_location/by_type sqlite tests: died column counts negative
  outcomes, not just current_status='died'.
- Annual report: existing fixture already covers rating −1/0/1; add
  assertions for localized category labels in FR.
- Creaves: producer test asserting outtake payload includes explicit
  `rating:0, dead:false` for neutral non-dead types; run creaves suite.

### Validation
- Quality gates run separately: `go vet`, `staticcheck`, `gocognit -over 15`,
  `gocyclo -over 12`, `go test -count=1 -race -cover` (sqlite tags for
  console).
- Browser check (dev): reports index shows correct deceased count grouped by
  outcome in EN/FR/DE/NL.

---

## Completion note (2026-09-03)

Status: **DONE**. All stages implemented and verified.

- Console fix commit: `66c1e97 fix(console): group report outcomes by outtake rating and dead flag`.
- Creaves producer commit: `1f327c8 fix(webhook): always serialize outtake rating/dead in event payloads`
  (removes `omitempty` so neutral (0) / dead=false values cross the wire).
- Console archive commit: `6626c38 docs: archive fix plan for reports outcome grouping bug`.
- Verification:
  - `CGO_ENABLED=1 go test -tags sqlite -count=1 ./...` — all green (console, 4 iterations during dev).
  - `CGO_ENABLED=1 go test -tags sqlite -count=1 -race -cover ./...` — green; coverage actions 55.8%, models 61.9%.
  - New tests: `TestApplyOuttake_NeutralOuttakeStored`, `TestApplyOuttake_NegativeOuttakeOverridesReleasedEvent`,
    `TestApplyOuttake_NoOuttakeLeavesRatingNull`, `TestReports_IndexStats_OutcomeGrouping`,
    `TestReports_IndexStats_EmptyRegister`, `TestReports_ByType_OutcomeGrouping`,
    FR CSV assertions in `TestReports_Annual*`; creaves `TestOmitemptyBackwardsCompat` updated.
  - Quality gates (console): `go vet` clean; staticcheck only pre-existing findings
    (webhook_api_keys ST1005 ×3, models.go ST1019, consolidated_animal nullableString U1000);
    gocognit/gocyclo only pre-existing entries — `UpdateFromPayload` complexity *reduced* below baseline
    (gocognit 45→38, gocyclo 43→37) by extracting `applyOuttake`; `ReportsIndex` kept off the gocyclo list
    by extracting `tallyOutcomes`.
  - Quality gates (creaves): `go vet` clean; staticcheck/gocognit/gocyclo only pre-existing findings
    (grifts dot-imports, models.go ST1019, long-standing complex functions); full `go test -count=1 ./...`
    green on the intended dev-DB environment (actions 379s).
  - Creaves race gate (`go test -count=1 -race -cover`): green for all packages except one
    **pre-existing, environment-sensitive** failure unrelated to this change:
    `TestStartResyncRunCommittedBeforeReturn` hard-codes a 30s progress poll and its own comment states it
    expects the dev DB's 10k+ animal dataset; on an empty `creaves_test` the resync completes before the
    first poll ("run left 'running' without progress: status=completed"), and under `-race` on the 10k-row
    dev DB the worker cannot show progress within 30s (suite also exceeds any practical `-timeout`, >30m).
    Not caused by this fix (resync code paths untouched); flagged as residual test-design issue
    (candidate follow-up: make the poll tolerant of an already-`completed` run with progress > 0).
  - Test-infrastructure note: plain `go test` leaves `GO_ENV` unset, so creaves tests connect to the
    **development** MySQL DB (`models.go: envy.Get("GO_ENV", "development")`). Killed test runs leave
    fixture rows whose fixed ID prefixes collide across suites (RAFT teardown `LIKE '77777777%'` vs RSYNC
    fixture `77777777-...-f1`) and block later runs with FK/duplicate-key errors; cleaned manually.
    Use `GO_ENV=test go test ...` (empty `creaves_test` DB) for fast isolated runs.
  - Browser multi-language validation NOT performed (no browser tool available in this session); covered
    instead by sqlite render tests asserting FR strings (stat labels, FR annual CSV categories) — flagged
    as residual/unverified.
- Deliberately out of scope (documented in plan §"Non-but"): the producer's event-type quirk
  (`outtakes.go:235` uses `Type.Error` instead of `Type.Dead`) was left unchanged; the console derives
  outcome/status from payload content, keeping the webhook contract stable.
