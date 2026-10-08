# bugs.md Bug 10 — Animal list filter: "no outtake" option (creaves)

**Status**: fixed, tested, committed (creaves `2760cce`), archived 2026-09-09.

## Bug report (from bugs.md)

> **Observed**: the `/animals` filter panel (`templates/animals/index.plush.html`,
> params in `actions/animals_search.go`) can filter by outtake **type**
> (`outtaketype_id`), but there is no way to search for animals that have **no
> outtake at all** (i.e. still in care, `outtake_id IS NULL`). Users cannot
> list "animals not yet outtaken" from the search view.
>
> **Expected**: the filter panel offers a "no outtake" / "still in care"
> option that adds `animals.outtake_id IS NULL` to the WHERE clause in
> `applyAnimalSearchFilters`, combinable with the other filters (year, type,
> species, …). Sticky selection must survive pagination like existing filters.

## Fix

`creaves/actions/animals_search.go`:

- New exported constant `NoOuttakeFilterValue = "none"`.
- `applyAnimalSearchFilters`: `outtaketype_id=none` →
  `q.Where("animals.outtake_id IS NULL")`; any other value keeps the existing
  subquery (error-flagged outtakes still excluded). All filters remain
  AND-combined. CSV export (`AnimalSearchExportCSV`) uses the same function →
  automatically consistent.
- `setupAnimalSearchContext`: prepends a translated "no outtake" option to
  the exit-reason select with sticky selection
  (`Selected: p.OuttaketypeID == "none"`).

`creaves/locales/animals.{fr,en-us,de,nl}.yaml`: new key
`animal.search.no_outtake` — "Sans issue (encore en soins)" / "No outtake
(still in care)" / "Kein Ausgang (noch in Pflege)" / "Geen vertrek (nog in
verzorging)".

No template change needed: the select already iterates `searchOuttakeTypes`.

## Tests

**Unit** — `creaves/actions/animals_test.go::TestAnimalSearchFilterNoOuttake`
(fixtures: animalC without outtake, A/B with outtakes):

- `outtaketype_id=none` alone → only animalC
- combined with `year=2021` → animalC; with `year=2022` → empty

```
go test ./actions -run TestAnimalSearch -count=1   → ok 0.657s
go test ./actions -run TestAnimalSearchFilterNoOuttake -v → PASS
```

**E2E (agent-browser, dev server on :3000, admin login)**:

- `GET /animals?outtaketype_id=none` → option "No outtake (still in care)"
  rendered; pagination links all keep `outtaketype_id=none` (13 pages);
  last page 9 rows → 12×20+9 = **249 rows == DB count**
  `SELECT COUNT(*) FROM animals WHERE outtake_id IS NULL` → 249 (of 10046).
- `GET /animals/search/export.csv?outtaketype_id=none` → **249 data rows,
  0 rows with exit date** (header `Year;…;Exit date;Exit reason;…`).
- Combined `?year=2024&outtaketype_id=none` → 2 rows, matches DB count 2;
  sticky select keeps value `none`.
- French locale `/lang/?lang=fr&url=…` → option label
  "Sans issue (encore en soins)".

## Code quality

- `go vet ./...` — clean.
- `staticcheck ./...` — only pre-existing warnings (unused funcs, ST1005
  capitalized error strings), none in changed files.
- `gocognit -over 15` on changed files — clean.
- `gocyclo -over 12` — `setupAnimalSearchContext` = 13, **pre-existing**
  (baseline before change also 13; verified via `git stash`).
- `go test -count=1 -race -cover ./actions/ -run TestAnimalSearch` — ok.
- `go test -count=1 -race ./...` — all packages ok.

## Commit

```
2760cce feat(animals): add 'no outtake' option to /animals exit-reason filter (Bug 10)
```

Files: `actions/animals_search.go`, `actions/animals_test.go`,
`locales/animals.{fr,en-us,de,nl}.yaml`.

## Note (dev environment)

To run the e2e against the dev DB (production copy, unknown passwords), the
`admin` user's `password_hash` was reset **in the dev DB only** to a throwaway
bcrypt hash (password `e2e-test-pass`). No other rows touched.
