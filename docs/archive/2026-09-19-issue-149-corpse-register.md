## #149 — Registre des dépouilles — SPEC RECEIVED + EXTENDED, CLEAR
**Source:** https://github.com/pijalu/creaves/issues/149 | labels: CONFORME SPW
**Spec (from reporter, updated):**
- Dedicated report listing **dead animals only** (outtake type with dead flag → generates a corpse).
- Same style as the existing "registre" report.
- Columns: **numéro de l'animal** (year number), **espèce**, **entrée** (numéro d'entrée), **date de décès**.
- **Corpse destination tracking**: in the list, ability to **select corpses** (multi-select) and **mark their destination**:
  - destination = **"organisme"** — free text input **with suggestions** (autocomplete from previously used values);
  - store **date** of the operation and **the user** who performed it;
  - the report must **display** these informations (destination, date, by user).
**Plan:**
1. **Migration (additive)** on `outtakes`: `corpse_destination` VARCHAR(255) NULL, `corpse_destination_at` DATETIME NULL, `corpse_destination_by_id` CHAR(36) NULL (→ users). Corpse exists iff outtake type `Dead=true`; columns only meaningful for those rows. No data backfill needed. Down migration drops the 3 columns (additive, prod-safe).
2. New report action under existing reports infra (`actions/reports*`): "Registre des dépouilles" — animals whose outtake type has `Dead=true`, joined to outtake (death date) and intake (entry). Year filter consistent with sibling registre.
3. Columns: num annu, species name, entry year/number, outtake date **+ destination, destination date, destination by** (empty when unmarked).
4. **Bulk marking**: checkboxes per row + "select all"; form with organisme text input wired to autocomplete `/suggestions/corpse_destination` (mirrors existing `/suggestions/outtake_location` infra — DISTINCT values from `outtakes.corpse_destination`), date picker defaulting to now; recording user = `current_user` server-side (never from form). POST endpoint validates: only outtakes with dead type. Also allow per-row quick mark for single corpse.
5. View under `templates/reports/` matching existing registre layout + CSV export including the new columns.
6. Menu entry in reports section; i18n keys in all 4 locales.
**Test:**
- Unit: query returns only dead-outtake animals with correct columns (species resolved, death date from outtake); live/in-care animals excluded.
- Handler: bulk mark endpoint stores destination + date + current user on selected outtakes; rejects/ignores non-dead outtakes; unauthenticated blocked; suggestions endpoint returns distinct prior values.
- e2e (agent-browser): report renders with seeded deaths, filter works, select 2 corpses → mark destination → values persisted + displayed, export downloads with new columns.
**Validation:** `go test ./...`; agent-browser flows; 4 locales verified.
**DONE ✅ commit c01ab8b + d7c476c** — 3 additive outtakes columns (fizz, drop_column down verified both directions); corpse register report (dead-only, year filter, num annu/species/entry/death + destination columns); bulk mark + per-row quick mark (organisme autocomplete `/suggestions/corpse_destination`, date default now, user from session, non-dead outtakes ignored); CSV export w/ new columns; menu ×4; templates ×4; locales ×4; handler tests (dead-only query, mark semantics, suggestions, CSV); e2e verified (select-all=1023 exact, bulk=2, quick=1, display+suggestions+CSV) and test marks reverted (0 residual). Post-commit fix d7c476c: destination_at ParseInLocation.
