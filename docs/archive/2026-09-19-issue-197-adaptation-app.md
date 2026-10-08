## #197 — Gros ticket — adaptation APP — SPEC RECEIVED, CLEAR (9 sub-items) — ALL DONE ✅
**Source:** https://github.com/pijalu/creaves/issues/197 | label: enhancement | milestone: 2026 | reporter: crealan
**Spec (verbatim sub-items, translated structure):**
1. `/animals` search: display the **number of animals found**; extend search with **blessé (`has_wounds`)** and **parasites (`has_parasites`)** selectors (discovery flags).
2. `/animals` table: **remove Traitements + Gavage columns**; replace with **entry-cause number (entry_causes id)** and **outtake status (`outtaketypes.rating`)** columns.
3. **BUG** `/export/view?query=`: downloaded export used the **base query data, not the filtered data**.
4. Outtakes whose outtaketype `location_mode=free`: **"Adresse, lieu précis"** free-text field.
5. `/registertable` columns: Type → **Classe**, add **discovery postal code + city**, **entry-cause id + detail**, outtake **"Adresse, lieu précis"**.
6. `/` (dashboard) and `/animals` tables + animal detail: **quick outtake-creation button**.
7. `/feeding` table sort: **Prochain nourrissage → Espèces → Instruction d'alimentation → Numéro**.
8. Animal general tab: **"A relâcher"** checkbox + badge in the home tables.
9. `/guest`: **CREAVES identity data** (= #150), **two center-customizable free texts**, **"votre animal est blessé" / "votre animal a des parasites"** from `has_wounds`/`has_parasites`.

**DONE ✅ — one commit per sub-item (branch `feature/open-issues-2026-10`):**

| Sub | Commit | Content |
|---|---|---|
| 1 | `6f9c5b0` | animals search — found-count display + wounded/parasites filters (×4 locales) |
| 2 | `dec75e8` | animals table — drop Treatments/Force-feed columns, add entry-cause + exit-status |
| 3 | `cb050bf` | export CSV download honors the view's applied filters (bug fix, regression test) |
| 4 | `7a9fb86` | outtakes — precise location for free-mode types (additive migration) |
| 5 | `d1c0189` | registertable — class, discovery postal/city, entry cause, precise location |
| 6 | `161e4cc` | quick-outtake buttons (dashboard, animals list, animal detail) |
| 7 | `3b39689` | feeding list sort — next feeding, species, instruction, number |
| 8 | `cbe2df4` | animal 'ready for release' flag — general tab + home table badges (additive migration `animals.ready_for_release`; also fixed quickOuttakeFixture cleanup placeholder bug that leaked test rows) |
| 9 | `aa1b93e` | guest page — center guest texts (ConfigSettings.GuestText1/2 + config form ×4) + intake findings info (has_wounds/has_parasites lines, en/fr/de/nl) |

**Test/Validation per sub-item:** plan + handler/unit tests, e2e via agent-browser (×4 locales where templates touched), gates (`go vet`, `staticcheck`, `gocognit -over 15`, `gocyclo -over 12` — no new baseline entries, `GO_ENV=test go test -count=1 -race -cover ./...`). Only known baseline failure remains: grifts `TestMigrationsReplayOnEmptyDatabase` ('Relacher'/'Relaché' unique-name collision under accent-insensitive collation, pre-existing from the outtaketype-portability work, unrelated to #197).

**Notes discovered during the session:**
- bare `go test` without `GO_ENV=test` points at the dev DB → deterministic suite failures; always `GO_ENV=test`.
- pop's column cache is process-wide and poisons on bad `Exists(table-string)` calls (fixed in f93557c).
- Guest direct-link `lang` param works for fresh visitors; for repeat visitors the `lang` **cookie** wins over the URL param (locale switch app-wide works through redirect+cookie). Potential follow-up if QR links must override an existing visitor's language.
- `TestLoadConfigMultipleConfigsCoexist` (pre-existing failing test entry, found during #100 on c98c8cf) passes again on this branch — fixed en passant by the config/selection work; entry removed from bugs.md.
