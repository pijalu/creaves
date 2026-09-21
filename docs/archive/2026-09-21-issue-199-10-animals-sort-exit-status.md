# Issue #199-10 — Animals list: sortable Entry cause + Exit status columns

Date: 2026-09-21
Commit: `5aeb597` on `feature/open-issues-2026-10`

## Request (from pijalu/creaves#199)

Item 10: on `/animals`, make the *Entry cause* and *Exit status* columns
sortable and display meaningful values (localized cause label, exit-status
badge) instead of raw IDs.

## Implementation

- `actions/animal_sort.go` — whitelisted two new sort keys with explicit
  SQL fragments:
  - `entry_cause` → sub-select joining `discoveries` → `entry_causes`
    through `animals.discovery_id`.
  - `exit_status` → sub-select joining `outtakes` → `outtaketypes.rating`
    through `animals.outtake_id` (NULL when the animal has no outtake yet;
    NULLs sort first on `asc`).
- `actions/animals.go` — new `loadDiscoveriesWithEntryCauses(tx, ids)`
  helper bulk-loads discoveries plus their entry cause and attaches them to
  the listed animals (replaces the inline discoveries block in
  `enrichAnimalsOptimized`, whose gocyclo dropped 31 → 29).
- `templates/animals/index.plush*.html` (4 locales) — both column headers
  are now sort links; the cause cell renders
  `tname("entry_causes", id, label)` (localized label, raw-ID fallback) and
  the exit cell renders the same badge semantics as the animal show page
  (`badge-danger` <0, `badge-success` >0, `badge-secondary` 0) via new
  locale keys.
- `locales/animals.{en-us,fr,de,nl}.yaml` — added
  `animals.exit_status.positive | .negative | .neutral`.

## Tests

- `actions/animal_sort_test.go` — pins the exact SQL fragments for both new
  sort clauses.
- `actions/animals_test.go` — `TestAnimalsIndexEntryCauseAndExitStatus`
  builds fixture animals (positive / negative / no outtake) and asserts:
  localized cause label shown, raw ID hidden, badge text per rating, sort
  links present in headers, entry-cause asc/desc ordering, exit_status
  asc (NULL first) and desc ordering.
- Full suite: `GO_ENV=test go test -count=1 -race -cover ./...` passes with
  only the two pre-existing grifts debts (template parity known-drift list,
  migrations replay accent-collation duplicate).

## e2e verification (agent-browser, dev DB)

- `/animals` headers expose `/animals/?dir=asc&sort=entry_cause` and
  `/animals/?dir=asc&sort=exit_status`.
- EN: cause labels rendered ("Undetermined", "Predation - Fight", …);
  `?sort=exit_status&dir=desc` → first rows all "Positive" badges.
- FR (`/lang/?lang=fr`): headers "Cause d'entrée" / "Statut de sortie ▲";
  cause labels localized ("Trouvé en un lieu…", "Piègeage anti-nuisible",
  "Coincé - Enfermé", "Collision"); "Positive" badge rendered.
