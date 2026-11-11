# Archived: creaves-console — consolidated_animals collapsible filter UI

**Date:** 2026-09-05
**Project:** creaves-console
**Commit:** `a02c848` feat(consolidated_animals): collapsible filter bar
**Branch:** main

## Original report (from bugs.md)

The filter bar on http://localhost:3001/consolidated_animals showed all filters
inline (view mode, instance, year, status, species, type, city, entry cause, age,
ring, outtake type) — it took too much vertical space. Request: make filtering
collapsible; keep only year / instance visible by default; collapsed section
expands automatically when a non-default filter is active. Apply to all 4 locale
template variants.

## Fix

Templates `templates/consolidated_animals/index.plush.{html,fr,de,nl}.html`:

- Instance + Year selects stay inline next to the Filter / Reset / Export CSV
  buttons (DE/NL keep their pre-existing `viewMode == "instance"` conditional
  around the Instance select).
- A "More filters" / "Plus de filtres" / "Mehr Filter" / "Meer filters" button
  (Bootstrap 4.6 `data-toggle="collapse"`, JS bundle already loaded by the app
  layout) toggles a `div.collapse#moreFilters` wrapping view mode, status,
  species, type, city, entry cause, age, ring and outcome type.
- Auto-expand: the div gets the `show` class and the button
  `aria-expanded="true"` server-side when any non-default filter is active:
  `viewMode == "instance" || params["status"] || params["species"] || ...`.

### Plush pitfall discovered during verification

The first version used `params["status"] != ""`. In Plush, a missing map key
evaluates to `nil`, and `nil != ""` is **true** — so the section auto-expanded
on every page load. Buffalo exposes `params` as `map[string]string`
(`default_context.go`), so absent filter keys yield `nil` in templates. Bare
truthiness (`params["status"]`) is false for both missing and empty values and
true only for a set non-empty value — verified with a standalone plush/v4
harness before applying. Commit amended with the correction.

## Quality gates (creaves-console)

- `go vet ./...` — PASS
- `staticcheck ./...` — PASS
- `gocognit -over 15 .` — PASS (pre-existing only: UpdateFromPayload 38, installSafePopTxLogger 25, DashboardIndex 17, SyncManagementIndex 16)
- `gocyclo -over 12 .` — PASS (pre-existing only: UpdateFromPayload 37, LocalizedField 21, DashboardIndex 17, applyConsolidatedAnimalFilters 14, …)
- `CGO_ENABLED=1 go test -tags sqlite -count=1 ./...` — PASS
- `CGO_ENABLED=1 go test -tags sqlite -count=1 -race -cover ./actions/ ./models/` — PASS (actions 59.1%, models 58.4% coverage)

## Browser verification (agent-browser)

Logged in as admin on http://127.0.0.1:3001:

- **Default /consolidated_animals**: `#moreFilters` class `collapse w-100 mt-2`
  (no `show`), toggle `aria-expanded="false"`; visible: Instance, Year,
  "More filters", Filter, Reset, Export CSV. ✓
- **Click "More filters"**: section expands; View, Status, Species, Type, City,
  Entry cause, Age, Ring, Outcome type selects render with localized options. ✓
- **`?status=in_care`**: class `collapse w-100 mt-2 show`, `aria-expanded="true"`,
  Status select pre-selected "In care". ✓
- **FR** (`/lang/?lang=fr`): collapsed by default; button "Plus de filtres",
  Instance / Année visible. ✓
- **DE** (`/lang/?lang=de`): collapsed; "Mehr Filter", Jahr visible (Instanz
  hidden in global view per pre-existing DE template conditional). ✓
- **NL** (`/lang/?lang=nl`): collapsed; "Meer filters", Jaar visible. ✓
