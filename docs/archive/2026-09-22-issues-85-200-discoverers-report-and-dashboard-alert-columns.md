# Resolved: GitHub issues #85 and #200 (creaves)

Date: 2026-09-22 — branch `feature/open-issues-2026-10` — both tickets closed and assigned to crealan.

## Issue [pijalu/creaves#200](https://github.com/pijalu/creaves/issues/200) — Dashboard "Animaux en alerte" column adaptation

**Request:** remove columns Poids / cage nettoyée / Type / consultation eye from the dashboard "Animals in alert" frame; add Zone, Cage, Espèce. Final order: Date, Animal, Zone, Cage, Espèce, Note.

**Fix (commit 6f5d700):**
- `actions/dashboard.go` — `SQL_CARES_IN_WARNING` now also selects `a.zone`, `a.cage`, `a.species`.
- `models/care.go` — `CareWithAnimalNumber` gained `Zone`, `Cage` (nulls.String) and `Species` (string).
- `templates/dashboard/dashboard.plush.html` + `.fr` + `.de` + `.nl` — table rebuilt with the requested column order; zone via `tbase("zones", ...)`, species via `tspecies()`; warning row coloring and the animal number button kept.

**Validation:**
- `go build ./...`, `go vet`, `staticcheck`, `go test -count=1 -race ./models` — clean.
- gocognit flags `AnnotateCaresForDisplay` (17) — pre-existing, untouched.
- e2e (agent-browser): headers verified in 4 locales —
  - en-US: `Date, Animal, Zone, Cage, Species, Note`
  - fr: `Date, Animal, Zone, Cage, Espèce, Note`
  - de: `Datum, Tier, Zone, Käfig, Art, Notiz`
  - nl: `Datum, Dier, Zone, Kooi, Soort, Notitie`
- Rows populated (e.g. `2026/09/19 16:06 | 1267/26 | R | R15 | Hérisson | rien mangé, perte de poid`), no console errors.

## Issue [pijalu/creaves#85](https://github.com/pijalu/creaves/issues/85) — Discoverers view in Reports + discoverer data reuse

**Request:** (1) a Reports page listing discoverers with their linked animals + export; (2) reuse an existing discoverer's data when encoding an animal.

**Fix (commit 7cc78da):**
- Report: new `actions/reports_discoverers.go` — `ReportsDiscoverersIndex` (`GET /reports/discoverers`, year filter via `selectAnnualYear`, left-join discoverers→discoveries→animals) and `ReportsDiscoverersExportCSV` (`GET /reports/discoverers/export.csv`, translated header, same `writeCSV` helper as other exports). Routes registered in `actions/app.go`. Single i18n-driven template `templates/reports/discoverers.plush.html`; keys `nav.reports_discoverers`, `reports.discoverers.*` added to `locales/reports.{fr,en-us,de,nl}.yaml`; menu entry added to the Reports dropdown in all 4 `application.plush.*.html` variants.
- Reuse: new `GET /suggestions/discoverer_lookup?q=` returning up to 10 full discoverer records (JSON). A picker was added above the Discoverer block of `reception/new.plush.*.html` and `animals/_form.plush.*.html` (all 4 locales each): selecting a suggestion prefills every `Discovery.Discoverer.*` field and stores the ID in a hidden input; any manual edit of a discoverer field detaches the picked record (delegated `input` handler).
- Server: `reusePickedDiscoverer()` in `AnimalsResource.Create` updates the picked discoverer with the form values and links the new discovery to it instead of creating a duplicate; `relinkPickedDiscoverer()` in `AnimalsResource.Update` re-links the discovery when a different discoverer was picked.

**Validation:**
- `go build`, `go vet`, `staticcheck` clean; `reports_discoverers.go` passes gocognit/gocyclo; pre-existing complexity flags in `animals.go` unchanged in nature (Create 40→34 after extraction).
- `go test ./actions -run 'Discoverer|Discovery|Reception'` — ok. Other actions tests failures reproduced identically on the pre-change baseline (stash) — pre-existing breakage, not a regression.
- e2e (agent-browser):
  - Report page fr: title "Découvreurs et animaux associés", 1942 rows, headers `Nom, Prénom, Adresse, Code postal, Ville, Pays, Email, Téléphone, Animal, Espèce`; en-US/de/nl titles verified.
  - CSV export verified in fr (`Prénom;Nom;…;Espèce`) and en-US (`First name;…;Species`).
  - Picker: suggestions render, click prefills all fields + hidden ID; manual edit clears the ID.
  - Full reception submit with picked discoverer (Stephanie Dumont) created animal 980480 linked to the existing discoverer `cd26e400-…` (verified via SQL: no duplicate discoverer created). Test animal then marked Doublon through the app destroy flow to restore DB state.
