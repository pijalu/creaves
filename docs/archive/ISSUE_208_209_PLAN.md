# Plan — Issues #208 & #209

Source issues:
- **#209 — Maj des droits des différents types de compte users** (permissions per account type)
- **#208 — Correctif et adaptations 08/11** (17 fix/adaptation items)

Conventions applied everywhere:
- All-language UI rule: every template change is mirrored in `.plush.html` (en),
  `.plush.fr.html`, `.plush.de.html`, `.plush.nl.html`; new strings added to all
  `locales/*.yaml`.
- Enforcement layered in **handlers** (server-side) first, **templates** only hide
  UI the user has no right to use.

---

## Issue #209 — Permissions

### 209.1 Maintainer-only surfaces
Elements visible **only** to maintainers (`current_user.Maintainer`):
- System menu pages **Maintenance** (`/maintenance`) and **Configuration** (`/config`)
- Entire **Synchronization** sub-menu (`/sync_configuration`, `/event_streams`,
  `/webhook_resync`, `/sync_targets`)
- In the **Configuration** sub-menu: Translations (`/translations`), Outtake
  locations (`/outtake_location_options`), Native statuses (`/native_statuses`),
  Outcome types (`/outtaketypes`), Localities (`/localities`), Animal types
  (`/animaltypes`), Animal ages (`/animalages`)

Implementation:
- `actions/`: switch existing `requireAdmin`/`cu.Admin` checks to
  `requireMaintainer` in `maintenance.go`, `configs.go` (config resource; keep
  `/config` GET reachable per current scope split — verify), `event_streams.go`,
  `webhook_resync.go`, `sync_targets.go`, `translations.go`, `localities.go`,
  `animalages.go` (already maintainer-checked for animaltypes/outtaketypes/
  native_statuses/outtake_location_options).
- `templates/application.plush.*.html`: wrap Maintenance + Configuration links in
  `if (current_user.Maintainer)`; wrap the whole Synchronization sub-menu likewise;
  add the Configuration-menu links listed above to the existing maintainer block.

### 209.2 Entry causes page for maintainers
- `EntryCausesResource` exists (`/entry_causes`) with no auth check → add
  `requireMaintainer` to all mutating handlers (and list/show, matching other
  reference tables) and add a menu entry in the Configuration sub-menu
  (maintainer block) in all 4 locales.

### 209.3 Species: visible to admins, editable only by maintainers
- `actions/species.go`: `List`/`Show` → `requireAdmin`; `New`/`Create`/`Edit`/
  `Update`/`Destroy` keep `requireMaintainer`.
- `templates/species/index.plush.*.html`: hide New/Edit/Delete buttons for
  non-maintainers (show for `current_user.Maintainer`).

### 209.4 Maintainer accounts
"Un compte admin peut voir/créer/éditer un compte mainteneur alors que seul un
mainteneur peut créer/éditer un compte mainteneur" — i.e. only maintainers may
create or edit maintainer accounts. Largely implemented already
(`actions/users.go`: `if cu == nil || !cu.Maintainer { u.Maintainer = false }`
and update-side restore). Remaining gaps:
- Account-role selector in `templates/users/_form.plush.*.html`: hide the
  "maintainer" option unless `current_user.Maintainer`.
- Update path: when editing an **existing maintainer** account, a non-maintainer
  admin must not be able to modify it at all → block edit/update of a maintainer
  user unless actor is maintainer.
- Tests: extend `users_account_role_test.go`.

### 209.5 New role: Utilisateur sans traitements
New account type **"Utilisateur (sans traitements)"**: same rights as a regular
user **except** create/edit of treatments (`/treatments` New/Create/Edit/Update/
Destroy and the Treatment-tab buttons).
- Model: new `User.Role` value `utilisateur_sans_treatments` + `IsNoTreatments()`
  helper; extend `UserRoleNames`, `applyAccountRoleSelection`, `userAccountRole`,
  `user_sort.go` filter.
- Guard: `RoleGuard` — for this role block non-GET on `/treatments*`.
- Templates: hide Treatment new/edit/delete buttons in
  `templates/animals/show.plush.*.html` (nav-treatment tab) and the "New →
  Treatment" menu entry.

### 209.6 New role: Compte partagé sans traitements
New account type **"Compte partagé (sans traitements)"**: shared account (no
personal menu) without treatment create/edit.
- Same mechanics as 209.5 but combined with `Shared = true` → selector value
  `shared_sans_treatments` setting `Shared=true` + `Role=utilisateur_sans_treatments`.

### 209.7 Scientifique adaptations
Hide for scientist accounts:
- Pages `/dashboard/`, `/feeding`, `/logentries`, `/reception/new`, and the 4
  pages of the "New" menu (`/cares/new`, `/veterinaryvisits/new`,
  `/treatments/new`, `/outtakes/new`).
- RoleGuard (scientist): currently GET-everything + POST /veterinaryvisits.
  Restrict: GET blocked on `/dashboard`, `/feeding`, `/logentries`,
  `/reception/new`; POST /veterinaryvisits stays (their one write).
- Navbar (`application.plush.*.html`): hide Dashboard/Feeding/Log entries/
  Reception/New-menu entries for `current_user.Role == "scientifique"`.
- Remove buttons the role has no right to use (already covered by read-only
  guard; verify show/index templates hide edit/delete for restricted roles).

### 209.8 SPW adaptations
- Same page hides as scientist (`/dashboard`, `/feeding`, `/logentries`,
  `/reception/new`, New menu) — SPW whitelist already excludes these; navbar
  hides added.
- Animal show tabs: remove **Suivis (Care)**, **Vétérinaire**, **Traitement**,
  **Médias** tabs for SPW; **add Audit tab** access (currently admin-only in
  `templates/animals/show.plush.*.html` → allow `current_user.Admin ||
  current_user.IsSPW()`; audit data is read-only JSON, no extra handler change
  needed beyond page access already granted by whitelist on `/animals/`).

---

## Issue #208 — Correctifs et adaptations

| # | Item | Implementation |
|---|------|----------------|
| 1 | Remove "Guides Alimentaires" system | Remove menu link, routes `/feeding_guides*`, `actions/feeding_guides.go`, templates, `SuggestionsFeedingGuide`, `models/feeding_guide.go` + migration dropping the table, animals/_form.plush.* references, locale files `feeding_guides.*.yaml`; adjust tests. |
| 2 | Guest page "Prochain nourrissage : 00:00" | `templates/guest/show.plush.*.html`: don't render the line when no feeding slots configured (`NextFeeding` invalid / "00:00"). Check `guest.go buildGuestView` sets a valid value or empty. |
| 3 | "Détails de l'instance (vue invité)" into System menu | Locate the instance/guest-view page; add a System sub-menu link in all locales. |
| 4 | `/registertable` export button + centering | Fix template layout: restore export button to far right; re-center table. |
| 5 | "Source de chaleur" + O2 on same row in `#nav-care` | `templates/animals/show.plush.*.html`: put both fields on one row. |
| 6 | `/animals` filter "Statut de sortie" between "Motif de sortie" and "Blessé" | Add search param + query in `actions/animals_search.go`, select in `templates/animals/index.plush.*.html` (all locales). |
| 7 | `/export/view` add Excel export option | `/export/excel` already exists → add option/link in `templates/export/view.plush.html`. |
| 8 | `/outtakes/new` suggest known "Adresse, lieu précis" | Reuse autocomplete suggestions (like `SuggestionsOuttakeLocation`) for the precise-location field. |
| 9 | Include `entry_causes.indication` in reception/new cause search | Extend the entry-cause select/search in `templates/reception/new.plush.*.html` (Select2 matcher or appended text) + handler data if needed. |
| 10 | Phone format unify reception/new vs guest | Normalize to local format `0470112233` (strip `+32` prefix) consistently: guest normalize + reception save path. |
| 11 | `/outtakes/new` block death date < entry date | Server-side validation in `OuttakesResource.Create/Update` + client-side date min on the form. |
| 12 | `/reception/new` block future discovery/intake dates | Server-side validation in `ReceptionNew` POST + `max` attr on date inputs. |
| 13 | `/reports/annual` translate "Sorties par évaluation" + "Sorties : morts vs relâchés" (fr) | Add/fix locale strings in `reports/annual.plush.*.html` + `locales/*.yaml`. |
| 14 | `/reports/annual` table including animals still at center | New table in handler `ReportsAnnualIndex` + templates (all locales). |
| 15 | `/reports/annual` show number of animals received in the year at top | Compute count in handler, display at page top (all locales). |
| 16 | `/reports/annual` Top-20 species total shows year total, % on year total | Show the Top-20 subtotal separately (or drop the misleading figure) while keeping percentages over the year total. |
| 17 | Default care note template per care type | `care_templates` gets `caretype_id` + `is_default`; admin UI selector; care form pre-fills note with the default template for the chosen care type (`/suggestions/care_templates` extension + JS in cares form). |

---

## Execution order

1. 208.1 feeding-guides removal (largest blast radius, touches shared form)
2. 209 permissions (209.1 → 209.8) with `reference_guard_test.go`-style tests
3. 208 quick template fixes (2–7)
4. 208 forms/validation (8–12)
5. 208 reports/annual (13–16)
6. 208.17 care template defaults (needs migration)
7. Full verification: `go build`, `go test ./...`, locale parity check, browser
   smoke test via Chrome DevTools MCP (admin + a restricted account).
