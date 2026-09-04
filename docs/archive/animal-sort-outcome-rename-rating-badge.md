# Animal view column sorting + "Outcome" rename with positive/neutral/negative rating badge

Status: **Implemented** (2026-09-05)
Scope: this repo (sorting + rename + badge) — the console-side counterpart
(register sorting, outcome badges, multilingual display) lives in creaves-console.

## Goal

1. The animals list (`/animals`) can be sorted by clicking any column header.
2. UI wording renames "Outtake" to "Outcome" in all four languages
   (en: Outcome, de: Ausgang, fr: Issue, nl: Afloop) without touching model,
   route helper or form-field names that plush/Go depend on.
3. The animal show page displays the outcome type together with a rating badge
   (negative / neutral / positive) derived from the outcome type rating.

## Design

### Column sorting (`actions/animal_sort.go`)

- `animalSortColumns` is a fixed whitelist mapping the public `sort` parameter
  to an ORDER BY SQL fragment. User input is only ever used as a map lookup —
  never interpolated into SQL — so the parameter cannot inject SQL. Scalar
  subqueries are used for joined tables (intake date, type name, age name) to
  avoid row duplication from JOINs.
- Unknown `sort` values and missing/invalid `dir` fall back to the previous
  default ordering (`animals.id desc`), extended by stable tie-breakers so
  pagination does not shuffle rows.
- `animalSortClauses(sortKey, dir)` is a pure function (no DB) returning the
  full clause list; `applyAnimalSort` wires it into the pop query of the List
  handler.
- `sortLink(field, help)` builds the header URL server-side: it toggles
  `dir` when the column is already the active sort key, preserves all other
  query parameters (filters) and drops `page` so a re-sort always restarts at
  page 1. `sortIcon` renders ▲/▼ for the active column. Both are registered as
  plush helpers in `actions/render.go` and used by all four localized index
  templates.

### Outcome rename

Only user-visible strings were renamed:

- renamed: static text in `templates/**` (en/de/fr/nl), flash messages in
  `locales/outtakes.*.yaml`, `outtaketypes.*.yaml`, column headers and report
  titles in `locales/reports.*.yaml`, `animals.de/fr.yaml`, error messages in
  `cares.de/treatments.de/veterinaryvisits.de/cares.nl.yaml`.
- deliberately kept: Go identifiers, route helpers (`newOuttakesPath`,
  `outtaketypesPath`, …), plush context/model references (`animal.Outtake`,
  `animal.OuttakeID`), form field names (`name="Outtake.Date"`),
  `selectOuttaketype`, `guest.OuttakeDate/OuttakeNews` — renaming those would
  break plush binding and routing.

### Rating badge on the show page

The outcome "Type" row in the show tab pane renders a badge next to the type
name: `rating < 0` → `badge-danger` + Negative/Negativ/Négative/Negatief,
`rating > 0` → `badge-success` + Positive/Positiv/Positive/Positief,
otherwise `badge-secondary` + Neutral/Neutral/Neutre/Neutraal.

## Testing

- `actions/animal_sort_test.go`
  - `TestAnimalSortClausesPinsDefaultOrdering` — no/unknown/invalid params
    fall back to the default order (including a `"species; DROP TABLE animals"`
    injection attempt, which must land in the fallback branch, never in SQL).
  - `TestAnimalSortClausesPinsWhitelistAndDirection` — every whitelisted key
    maps to its fixed fragment with asc/desc and the tie-breaker tail.
  - `TestAnimalSortLinkPinsToggleAndPreservation` — toggling, filter
    preservation, page reset.
  - `TestAnimalSortIconPinsIndicators` — ▲ for asc, ▼ for desc, empty
    otherwise.
- Full suite: `go test ./actions ./models` (MySQL-backed) — green.

## Validation (dev server, MySQL `creaves_dev`)

- `/animals` renders 8 sortable headers per language; active column shows the
  direction icon; `sort=species&dir=asc`, `dir=desc`, `sort=intake_date` and a
  bogus `sort` all return 200.
- Localized headers verified for de (Aufnahmedatum, Käfig, Typ, Art, …),
  fr (Date d'admission, Cage, Espèce, …), nl (Opnamedatum, Kooi, Soort, …).
- `/animals/{id}` (animal with outcome, rating > 0): 200 in en/de/fr/nl with
  badge `Positive/Positiv/Positive/Positief`; animals with `dead`/negative
  types show `badge-danger` + Negative…; a rating-0 animal shows
  `badge-secondary` + Neutral.
- `/outtakes`, `/outtaketypes` render 200 in de/fr/nl; the outcome-type table
  header reads Outcome/Ausgang/Issue/Afloop respectively.
- `/reports/annual` shows the renamed report sections (Outcome by type /
  Ausgänge nach Typ / Issues par type / Aflopen per type).
- All 59 changed plush templates parse cleanly (plush Parse-only sweep);
  the earlier malformed else-if chain in the de/fr/nl show badge (leftover of
  a scripted rename) was found by this sweep and fixed.
