# Fix archive — 2026-09-12 — Species index search box

## Original report (bugs.md)
> ## Species list has no search box
> **Observed**: `/species` (maintainer view) renders the full paginated species table with no way to filter.
> **Expected**: A search box on the species index filters the table server-side.

## Fix

- `actions/species.go` (`SpeciesResource.List`): reads `q` (trimmed) and applies a single
  `LIKE %q%` filter over the canonical columns (`species`, `class`, `` `order` ``, `family`,
  `creaves_species`, `agw_group`, `subside_group`, `native_status`) **or** a subquery over
  `translations WHERE table_name = 'species'`, so localized names match too. `speciesSearch`
  is always set for sticky form value. Pagination (`page`/`per_page`) composes with the
  filter; the buffalo `paginator` helper rebuilds page links from `req.URL.String()`, so
  `q` survives pagination.
- `templates/species/index.plush{,.de,.fr,.nl}.html`: GET search form (search input
  `name="q"` + submit) above the table, localized placeholders
  (en/de/fr/nl).

## Tests
- `actions/reference_guard_test.go::TestSpeciesListSearchFilter` (MySQL test DB):
  self-contained fixtures (3 species + animaltype, uuid-scoped, cleaned up) prove
  family match returns exactly the two family members, canonical name match returns one,
  a translation-only token finds the species whose canonical columns lack it, and a
  no-match token returns an empty list. PASS.
- `go vet ./...` clean; `staticcheck ./...` — no findings in touched files.
- E2E (agent-browser, authenticated admin): `/species` renders the search form;
  `q=Herri` → exactly 1 row (`Larus argentatus`, matched via en-US translation
  "European **Herri**ng Gull" — canonical columns do not contain the token);
  FR locale `/lang/?lang=fr&url=/species` renders "Rechercher espèce, classe, ordre,
  famille…" + "Rechercher".

## Verification
```
go test ./actions -run 'TestSpeciesListSearchFilter' -count=1   # ok
# browser: /species?q=Herri → 1 row; fr locale form localized
```
