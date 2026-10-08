# Archived: creaves-console — Multiple queries on /consolidated_animals

**Date:** 2026-09-05
**Project:** creaves-console
**Commit:** `9ca34ec` fix(dashboard): bound localized filter label queries with GROUP BY
**Branch:** main

## Original report (from bugs.md)

Running a query on consolidated animals (http://localhost:3001/consolidated_animals)
ran an important number of queries — unclear why so many were executed. The POP log
showed, per page load:

- 1 paginated main query + 1 COUNT (expected)
- 8 `SELECT DISTINCT <field>` option queries (bounded, index-friendly)
- 5 unbounded full-table scans:
  `SELECT entry_cause, translations FROM consolidated_animals WHERE entry_cause IS NOT NULL`
  (same for `animal_age`, `outtake_type`, `species`, `animal_type`) — each reading
  **every** consolidated_animal row, translations JSON included.

## Root cause

`localizedGroupLabels(tx, scope, field, lang, baseWhere, baseArgs)` in
`actions/dashboard.go` built the label map for one filter dropdown by scanning all
rows of `consolidated_animals` matching the scope filter. Called 5 times per page
render (species, animal_type, entry_cause, animal_age, outtake_type) ⇒ 5 full-table
scans including the bulky `translations` JSON column.

## Fix

Group in SQL instead of scanning in Go:

```sql
SELECT <field>, MIN(translations) AS translations
FROM consolidated_animals
<where>
GROUP BY <field>
```

Rationale: all animals sharing a canonical value share its translation set
(translations are keyed by the creaves reference record), so any one row's
translations represent the whole group. `MIN()` is the portable way
(MySQL/MariaDB/SQLite) to pick one per group under `ONLY_FULL_GROUP_BY`.

Result: at most one row per distinct field value per query; identical label maps.

The 8 `SELECT DISTINCT` option queries were already bounded and were left as-is.

## Quality gates (creaves-console)

- `go vet ./...` — PASS
- `staticcheck ./...` — PASS
- `gocognit -over 15 .` — PASS (pre-existing only: UpdateFromPayload 38, installSafePopTxLogger 25, DashboardIndex 17)
- `gocyclo -over 12 .` — PASS (pre-existing only: UpdateFromPayload 37, DashboardIndex 17, applyConsolidatedAnimalFilters 14, installSafePopTxLogger 13)
- `CGO_ENABLED=1 go test -tags sqlite -count=1 ./...` — PASS (actions 5.5s)

## Browser verification (agent-browser)

Logged in as admin on http://127.0.0.1:3001, opened /consolidated_animals:

- **EN**: Species dropdown → "Alpine Accentor", "Dunnock", "Eurasian Skylark",
  "Northern Goshawk", …; Entry cause → "Gardening accident", "Hunting, Fishing",
  "Weather conditions", "Stuck - Trapped".
- **FR**: Espèce → "Accenteur alpin", "Accenteur mouchet", "Alouette des champs",
  "Bécasse des bois", …; Cause d'entrée → "Accident de jardinage ⇨ robot tondeuse,
  débroussailleuse", "Chasse, Pêche ⇨ Hameçons, fil de pêche", …

One dropdown option renders as a raw UUID ("ad5dcb5a-…"): that species record has no
translations in the consolidated data — pre-existing data gap, unaffected by this fix.

Localized labels render identically to before in both locales; page renders with no
error banners.
