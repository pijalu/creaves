# Item 3 — Port Creaves export reports to Console

## Goal
All Creaves export reports available in Console, for all/selected instances,
as online HTML view + CSV download.

## Scope decision (user)
- Port only reports whose data exists in `consolidated_animals`.
- Species taxonomy (family/order/game/huntable) and entry-cause ID **must** be
  ported → extend webhook payload + console schema.
- Backfill: wipe + full resync (console DB is disposable).

## Changes

### A. Webhook contract (both AGENTS.md, keep in sync)
Add to `payload.animal`: `species_family`, `species_order`, `species_game`,
`species_huntable`. (`entry_cause_id`, discoverer contact already in
`payload.discovery` — only need console-side storage.)

### B. Creaves (producer)
1. `models/event_stream.go` AnimalPayload: add SpeciesFamily, SpeciesOrder,
   SpeciesGame (bool), SpeciesHuntable (bool).
2. `actions/event_producer.go`: populate from species join (already selects
   family/order/game/huntable).
3. Tests: payload builder includes new fields.

### C. Console (receiver)
1. Migration: add to `consolidated_animals`:
   `species_family`, `species_order`, `species_game` (bool),
   `species_huntable` (bool), `entry_cause_id`,
   discoverer: `discoverer_firstname/lastname/address/city/postal_code/
   country/email/phone/note/donation`.
   Donation: add to creaves payload (Discovery.DiscovererDonation) too — needed
   for donation/discoverer registers.
2. `models/event_stream.go`: add matching payload fields (animal taxonomy +
   discovery discoverer_donation).
3. `models/consolidated_animal.go`: struct fields + UpdateFromPayload mapping.
4. `actions/event_processor_test.go` createTables: extend test schema.
5. `actions/export_reports.go` (new): query registry + handlers
   `ExportReportsIndex`, `ExportReportView`, `ExportReportCSV` with
   `reportScope` instance scoping.
6. `actions/export_queries.go` (new): ported SQL over consolidated_animals.
7. Templates: online view (sortable/filterable) + CSV download links; wire
   into reports/csv page.
8. Tests: query registry + handler scope tests (sqlite).

## Ported vs skipped reports
Feasible (data present after A-C): detail_register, register, dead_register,
descoverer_register, species_registre, entry_age, sortie_reason, sortie_types,
animals_species, AGW_group, animals_types, animals_family, animals_order,
animals_class, animals_game, animals_huntable, native_status,
entry_causes_détail, entry_causes, nature_entry_causes, Nombre, Annexe_2A/2B/
2024 (subside/class→grouped), donation_register (needs donation), etc.

Truly infeasible (no console equivalent): entry_date_year, entry_day_week,
day_to_month (intakes-only aggregates — derivable from intake_date), treatments
_day_year (no treatments), animal_gavage (no feeding/force_feed),
controle_espèce (needs full species list). Documented per user decision.

## Validation
- go vet, staticcheck, gocognit, gocyclo, go test -race -cover (both projects)
- e2e via agent-browser: run console, open export report online view, download
  CSV, verify instance scoping.
