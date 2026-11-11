# E2E-TEST.md — Creaves ⇄ Creaves Console end-to-end browser test plan

**Run date:** 2026-10-07 · **Tester:** automated agent (agent-browser CLI, black-box GUI testing)
**Workspace:** `/Users/muaddib/dev/creaves.project`

| App | Role | URL | Login (after seed) | DB |
|---|---|---|---|---|
| creaves | Pusher (source center) | http://127.0.0.1:3000 | `admin` / `admin` | MySQL `creaves` (restored from backup) |
| creaves-console | Receiver (consolidation) | http://127.0.0.1:3001 | `admin` / `admin123` | MySQL `consolidation` (fresh) |

**Supported languages (both apps):** `fr` (canonical base), `en-US`, `de`, `nl`.
Browser tool: `agent-browser` (see `.agents/skills/agent-browser/SKILL.md`). Evidence = command output + URLs, never screenshots alone.

**Result legend:** ⬜ not run · ✅ pass · ❌ fail (bug id in bugs.md) · 🔁 retested pass after fix.

---

## T1 — Environment reset (drop/create/restore/migrate/seed)

| # | Step | Expected | Result |
|---|---|---|---|
| 1.1 | Stop any running app processes on :3000/:3001 | ports free | ✅ old instance killed |
| 1.2 | `DROP DATABASE IF EXISTS creaves; CREATE DATABASE creaves CHARACTER SET utf8mb4;` and same for `consolidation` | both DBs empty | ✅ |
| 1.3 | Restore `creaves-db-2026-10-07.gz` into `creaves`: `gunzip -c creaves-db-2026-10-07.gz \| mysql … creaves` | restore OK; `animals=10344, users=101, cares=378492, species=494` | ✅ (10344/101/378492/494) |
| 1.4 | Delete admin user: `DELETE FROM users WHERE login='admin'` | 100 users remain, no `admin` row | ✅ 100 users |
| 1.5 | `cd creaves && GO_ENV=development buffalo db migrate up` | exits 0, all pending migrations applied | ❌ **BUG-1** (duplicate OT2 rows) → 🔁 retested after fix: exit 0, 112/112 migrations applied, single OT2 'Décédé' row, outtakes 10149/4707 death refs intact |
| 1.6 | `cd creaves && GO_ENV=development buffalo task db:seed` | admin recreated (`admin`/`admin`), reference data intact | ✅ admin recreated; seed also auto-created fresh `config` row (dump predates config table) with instance_id=hostname — corrected in 3.2 |
| 1.7 | `cd creaves-console && GO_ENV=development buffalo db migrate up` + `buffalo task db:seed` | 23 migrations, `admin`/`admin123` created | ✅ 23 migrations, admin created |

## T2 — Start both apps

| # | Step | Expected | Result |
|---|---|---|---|
| 2.1 | Start console: `GO_ENV=development buffalo dev` (background, log file) | http://127.0.0.1:3001 serves login page | ✅ `/tmp/console-dev.log` |
| 2.2 | Start creaves: `GO_ENV=development buffalo dev` (background, log file) | http://127.0.0.1:3000 serves login page | ✅ `/tmp/creaves-dev.log` |
| 2.3 | Login creaves `admin/admin` via browser (`/auth/new`) | dashboard/landing renders, no 5xx in log | ✅ landing shows 1134 animals in care + day-plan badge |
| 2.4 | Login console `admin/admin123` via browser (`/auth/new`) | Dashboard renders | ✅ fresh dashboard all zeros |

## T3 — Link creaves → console (browser only) + resync

| # | Step | Expected | Result |
|---|---|---|---|
| 3.1 | Console: Admin → Webhook API Keys → New; Key Name `E2E Center Restore`, Instance ID `creaves-restore` | key created; raw key shown exactly once on `/webhook_api_keys/{id}/created` | ✅ raw key `creaves_577b4a78-b5d1-41c9-8a73-4911fa71c9d4` (NOTE: this console build keeps the raw key retrievable on the key pages — intentional, differs from older "shown once" docs) |
| 3.2 | Creaves: `/sync_configuration` — set Instance ID `creaves-restore`, check Enable Event Stream, Save | flash success; values persist on reload | ✅ persists (Event Stream already checked by seed) |
| 3.3 | Creaves: `/sync_configuration` → Add sync target: Name `Local console`, Enabled ✔, Webhook URL `http://127.0.0.1:3001/webhook/events`, API key `<raw from 3.1>`, Batch Size `100`, Max Events/Min `6000`; Save | target listed as Enabled with URL | ✅ listed Enabled |
| 3.4 | Creaves: open `/webhook_resync` | sync status card shows Expected animals; no warning banner (webhook enabled) | ✅ Expected **10311** (= 10344 − 33 error-outtake 'doublon' animals, intentionally excluded from the sync set); checksum shown. First load shows no card (background-computed cache) — second load shows it |
| 3.5 | Click **Start resync**; poll `/webhook_resync/status.json` until `status=completed` | `animals_processed=expected`, `events_failed=0`, `errors` empty | ✅ completed in ~70 s: created 10311, delivered 10311, failed 0, skipped 0 |
| 3.6 | Console: `/events` filter State=Pending | 0 pending; all events Processed | ✅ 0 rows pending; unfiltered list shows `animal_state`/`animal_discovered` rows all Processed |
| 3.7 | Console: `/sync_management` | Stored=Expected=Confirmed, Unconfirmed=0, checksum badges "checksum match" + "matches producer" | ✅ 10311/10311/10311/10311, Unconfirmed 0, both badges present |
| 3.8 | Console: Dashboard | Total Animals = synced count, Active Instances ≥ 1, no error pages | ✅ 10311 animals / 10311 events / 1 instance / 1 key; status split In care 212 + Released 4325 + Died 5774 (outcome-based). NOTE: dashboard counters are served from a 2-min TTL cache — immediately after a big sync the numbers can be stale (seen: 1 event / `--` key count right after warmup) |
| 3.9 | Console: `/instances` | instance `creaves-restore` listed with Animals=synced count, Last event recent | ✅ Default Instance / creaves-restore / 10311 animals / 10311 events / 1 key / last event 23:49 |
| 3.10 | Creaves app logs | no `webhook returned status …` errors | ✅ none |

## T4 — Language coverage (all 4 locales, both apps)

For each locale L ∈ {fr, en-US, de, nl}: switch via `/lang/?lang=L&url=/` (creaves) and `/lang/?lang=L&url=/dashboard` (console), then assert localized chrome on key pages.

| # | App / page | fr | en-US | de | nl |
|---|---|---|---|---|---|
| 4.1 | creaves landing `/` (nav, badge labels) | ✅ | ✅ | ✅ | ✅ |
| 4.2 | creaves animal show page (tabs) | ✅ (Care tab = `Suivis`) | ✅ | ✅ | ✅ |
| 4.3 | creaves reception wizard `/reception/new` (localized labels; full FR walkthrough happens in T5) | ✅ Nouvelle réception | ✅ New Reception | ✅ Neue Aufnahme | ✅ Nieuwe opvang |
| 4.4 | creaves day plan `/care_plan` (tier headings) | ✅ En retard/Maintenant | ✅ Late/Now | ✅ Überfällig/Jetzt | ✅ Te laat/Nu |
| 4.5 | creaves outtake form `/outtakes/new` (type labels localized) | ✅ | ✅ | ✅ | ✅ |
| 4.6 | console dashboard + consolidated animals + animal detail (localized status labels: En soins/In care/In Pflege/In verzorging etc.) | ✅ | ✅ | ✅ | ✅ |
| 4.7 | Console payload canonicality: species/outtake type shown from canonical French reference names regardless of UI language (Spot-check: species name stays French `Hérisson` in all locales) | ✅ with finding | ✅ with finding | ✅ with finding | ✅ with finding |
| 4.7a | **FINDING (expected v2 behavior, documented):** the console detail/reports *display* species LOCALIZED via the payload `translations` map (en `Common Buzzard`, de `Mäusebussard`, nl `Buizerd`, fr `Buse variable`); grouping/reports remain canonical (one bucket per canonical species — verified single `Mäusebussard` bucket in de `/reports/by_species`). Canonical values remain in payload/matching per contract. | — | — | — | — |
| 4.8 | Language switch flash message appears in the target language on both apps | ✅ Language changed to en-US | ✅ Langue changée en fr | ✅ Sprache geändert zu de | ✅ Taal gewijzigd naar nl |

## T5 — New arrival (reception wizard) + automatic sync

Primary walkthrough in **fr** (base locale), then one more arrival in **en-US**; other locales verify form rendering (T4.3).

| # | Step | Expected | Result |
|---|---|---|---|
| 5.1 | Creaves (fr): `+ Réception` → 4-step wizard; step 0: 1 animal | wizard advances | ✅ steps 0→1→2→3 |
| 5.2 | Step 1: Type=`Hérissons / Insectivore`, Species=`Hérisson`, Age=`juvénile`, Ring=`E2E-RESTORE-001` | no type/species mismatch alert | ✅ |
| 5.3 | Step 2: discovery date=now, Postal `1000`, City `E2E-Bruxelles`, Location `Rue E2E 1`, Entry cause `1.1 - Indéterminé` (select2), discoverer `E2E`/`Restore`, country Belgique | step advances | ✅ (note: select2 search typing must go into the dropdown input, not the underlying field) |
| 5.4 | Step 3: intake date=now, Zone=default (`Centre`), General state `bon état E2E`; **Terminer!** | redirect to new animal show page `/animals/{id}`; flash success | ✅ animal **10374 = 2065 (2026)**, flash `L'enregistrement de l'animal a été créé avec succès.` |
| 5.5 | Animal show page | species/age/ring correct; animal is *in care* (no outtake); Zone default applied | ✅ Hérisson/juvénile/E2E-RESTORE-001/Zone Centre |
| 5.6 | Creaves `/animals` search `animal_year_number=2065` | single match → redirects to the animal page | ✅ redirected to `/animals/10374` |
| 5.7 | Within ~15 s: creaves `event_streams` | new events delivered; state event acknowledged | ✅ `animal_discovered` + `animal_state` both delivered; `animal_state` acknowledged (console state-hash ack path works) |
| 5.8 | Console: consolidated row for animal 10374 | synced within seconds | ✅ 2065/Hérisson/E2E-RESTORE-001/E2E-Bruxelles/`in_care` (event_count 1) |
| 5.9 | Console: animal detail (fr UI) | all payload fields populated; Event History with Source "Live update" | ✅ full card incl. taxonomy (Mammalia/Erinaceidae/NS1/Micro-mammifère/SG3), discoverer, cause detail+nature; 2 events (Instantané d'état + Découverte) Source `Mise à jour directe` |
| 5.10 | Second arrival (en-US): species `Pigeon ramier`, Ring=`E2E-RESTORE-002`, city `E2E-Liege`; then fix via animal **Edit** (set species/ring) | animal created + edit re-synced | ✅ animal **10375 = 2066 (2026)**; automation artifact (stale form ref) initially typed ring into the species free-text field — fixed via Edit; the edit produced a new `animal_state` event and the console row corrected to canonical `Pigeon ramier` + ring (edit→sync path verified) |
| 5.11 | `/sync_management` after arrivals | Unconfirmed 0; stored = 10311+2; expected(producer) lags until next resync | ✅ Stored 10313 = Received 10313 = Confirmed 10313, Unconfirmed 0; badge correctly flips to **"MISMATCH vs producer expected — run a full resync"** (live additions exceed the announced set) — sync-visibility feature working as designed; cleared in T9 |

## T6 — Protocols: global care rule + animal protocol

### T6a — Global protocol (care rule) in **nl**

| # | Step | Expected | Result |
|---|---|---|---|
| 6a.1 | Creaves (nl): Administration → Protocols → care rules `/care_rules` | list renders (may contain seeded rules) | ✅ 13 seeded rules + matchers render localized (Zorgregel/Voeding/Schoonmaak…) |
| 6a.2 | New rule: Name `E2E dagelijkse voeding alle dieren`, Action Kind=`Voeding`, Matcher = `Geen matcher (alle dieren)`, Schedule 09:00 daily open-ended, Active ✔; Save | rule listed active | ✅ listed Actief |
| 6a.3 | `/care_rules/{id}/preview` | match_count > 0 | ✅ `{"all_animals":true,"match_count":"all"}` |
| 6a.4 | Open `/care_plan?kind=feeding` | plan rows for in-care animals incl. T5 animals | ✅ 2066/26 gets seeded pigeon rule 08:30; 2065/26 gets seeded `Hérisson juvénile — repas 2x/j` 09:00/18:00 AND our `E2E dagelijkse voeding alle dieren` 09:00; tiers `Te laat 99+ / Later 4` render |
| 6a.5 | Day plan page in all 4 locales | localized tier headings/labels | ✅ (T4.4) |

### T6b — Animal protocol in **de** (on the T5 animal)

| # | Step | Expected | Result |
|---|---|---|---|
| 6b.1 | Animal show (de) → Protokoll tab → Neues Protokoll: name `E2E tierarzt de`, kind `Beobachtung`, time 01:30, active | protocol saved & listed | ✅ saved after filling mandatory `Frage bei der Anwendung` prompt (server correctly rejects observation without prompt: `careplan: observation payload needs prompt (§4.2)` — good validation, error shown in modal); row in `care_animal_plans` with payload+schedule |
| 6b.2 | `/care_plan?kind=observation` shows the animal-protocol occurrence | occurrence visible | ✅ 2065/26 `Frisst das Tier selbststaendig? E2E tierarzt de` ○ 01:30 (morgen + future days) |
| 6b.3 | Landing badge `/` shows open-item badge | badge reflects open day-plan items | ✅ badge live (`Tagesplan: 802` in de at 00:30); count is time-window dependent, new protocol item visible on plan page |

## T7 — Cares / cleaning (fulfilment + manual care)

| # | Step | Expected | Result |
|---|---|---|---|
| 7.1 | **(fr)** Day plan `/care_plan`: apply one feeding item for the T5 animal (group `Appliquer le groupe (1)` → confirm modal `Confirmer l'application`) | item moves to done tier | ✅ `care_plan_applications` row (rule acb24325, animal 10374, due clamped into care window 07:00, status `applied`); item gone from open tiers; `Historique` 174→175 showing `Fait` + `Annuler l'application` link (re-apply 409 path not directly reachable from UI once applied — covered by the item leaving the open tiers) |
| 7.2 | Animal Care tab | new `cares` row visible | ✅ `Repas` row 2026/10/07 22:11 on animal 10374 Care tab |
| 7.3 | **(en-US)** Manual care: `/cares/new?animal_year_number=2066/26`: Date=now, Type=`Monitoring`, **Cage cleaned ✔**, Weight 300; Save | care row in Care tab; landing green marker | ❌ care saved (flash `Care was successfully created.`, DB `clean=1 weight=300 Suivi`) but landing green marker MISSING → **BUG-2** (night-shift clean window); fix dispatched |
| 7.4 | Landing `/` badge | open-item count reflects applied item | ✅ direct evidence = day-plan Historique 174→175 + item left open tiers; badge value is time-window dependent (802) so a strict delta assertion is flaky |
| 7.5 | Console impact check | cares emit **no** events | ✅ console `event_count` unchanged (10374:1, 10375:1); no new creaves events after the care+apply |
| 7.6 | Status reflection: creaves animal remains *in care*; console status unchanged | ✅ both `in_care` |
| 7.7 | Batch care via cage (`POST /cares?cage=…` via UI New→Cares cage picker) — optional | one care per animal in cage | ➖ skipped (single-item apply covered in 7.1; group-apply confirm path exercised in the day plan) |

## T8 — Outtakes (different types) + console sync

One animal per outtake type; validate creaves state then console within ~15 s.

| # | Type (route) | Animal | Expected creaves | Expected console | Result |
|---|---|---|---|---|---|
| 8.1 | **Release** — `Freigelassen` (OT1) via `/outtakes/new?animal_year_number=2066/26` in **de**, location `E2E Freilassung Wald` | outtake recorded; redirect `#nav-outtake` | ✅ flash `Ausgang wurde erfolgreich erstellt.`, header `(Abgegeben)`; console `released` / `Relacher` / rating 1 / location synced; `animal_released`+`animal_state` delivered. All 7 outtake types render localized (Adoption/Verstorben/Duplikat/Eingeschläfert/…/Freigelassen/Überführt) |
| 8.2 | **Death** — `Décédé` (OT2) in **fr** on restored in-care animal 927/22 (2058, Buse variable) | status out | ✅ flash `La sortie a été créée avec succès.`, `(Sorti)`; console `died` / `Décédé` / rating −1 (renamed OT2 from BUG-1 fix renders correctly) |
| 8.3 | **Transfer** — `Transferred` (OT4, location list) in **en-US** on in-care 376/24 (4086) | outtake with list location | ✅ flash `Outcome was successfully created.`, location `Centre de Soins Bruxelles - LRBPO` from the option list; console `released` (alive) / `Transferer` / location synced. NOTE: the outtake picker silently re-renders for already-outtaken animals without an explanatory message (minor UX, observed with 931/22) |
| 8.4 | **Error/Doublon** — third arrival `E2E-RESTORE-003` (10376) then admin `Détruire` in **fr** (confirm dialog `Êtes-vous sûr?` accepted) | Doublon outtake created, animal removed from lists | ✅ flash `L'enregistrement de l'animal a été détruit avec succès.`; `Doublon` (error=1) outtake row created, animal row retained; console consolidated row **deleted** (0 rows) and its 2 events **removed** from `/events` — exact `animal_deleted` semantics |
| 8.5 | `/sync_management` after outtakes | Unconfirmed=0; stored count reflects +3 arrivals −1 deletion | ✅ Stored 10313 = Received 10313 = Confirmed 10313, Unconfirmed 0, event-log=consolidated checksum; `DIVERGENCE avec l'attendu du producteur` badge correct (announced set stale until next resync — cleared in T9) |
| 8.6 | Drill-down for released animal 10375 (fr) | field-level timeline | ✅ `Type de sortie: (vide) → Relacher`, `Statut: in_care → released`, `Statut: (vide) → in_care`; all events Source `Mise à jour directe` |
| 8.7 | Console reports consistency: `/reports` (fr, all centres) | counts include 8.1–8.3 | ✅ cards reconcile exactly: total 10313, in care 211 (212 +3 arrivals −release −death −transfer −deletion), Relâchés 4327 (+2), Décédés 5775 (+1), outcomes positive/negative consistent. **BUT** "Par statut" table exposes **BUG-3**: `Décédé 1 / Relâché 10101` — raw `current_status` says `released` for the 5,774 resync-delivered dead animals (only the fresh live-outtake death is `died`). fixed+retested (see T9) |

## T9 — Sync integrity after mutations (resync idempotency)

| # | Step | Expected | Result |
|---|---|---|---|
| 9.1 | Creaves `/webhook_resync`: Start resync again (after BUG-3 fix) | completes; dedup for unchanged animals; `events_failed=0` | ✅ completed: created 5775 (dead animals — hashes changed by the BUG-3 status fix, exactly as predicted), skipped_unchanged 4538, failed 0, ~15 s |
| 9.2 | Console `/sync_management` | checksum match + matches producer; Stored=Expected=Received=Confirmed | ✅ 10313/10313/10313/10313, Unconfirmed 0, both badges |
| 9.3 | Outtaken animals remain outtaken after resync; BUG-3 retest: dead animals now `died` | ✅ 10375 `released`/`Relacher`, 2058 `died`/`Décédé`, 4086 `released`/`Transferer` all stable; console `current_status` groups: died=5775 / released=4327 / in_care=211; `/reports` By Status = Died 5775 / In care 211 / Released 4327 (BUG-3 🔁 verified) |

## T10 — Missing-feature sweep (add anything missing)

Anything the plan requires but the app lacks (missing translations, missing page, broken flow) is recorded in bugs.md as a bug/feature gap, dispatched to a fix agent, and retested.

| # | Check | Result |
|---|---|---|
| 10.1 | All 4 locales render every page touched above without raw translation keys (`translation missing` / raw ids) | ✅ no raw keys anywhere; **BUG-4** (hardcoded nav tooltips) found + fixed; **BUG-5** investigated → by-design (browser-locale times); minor notes: care-plan history `12:11 AM`-style time is browser-locale by design; outtake picker re-renders silently for already-outtaken animals (no explanation flash) — left as-is |
| 10.2 | No console errors / 500s in either app log across the whole run | ✅ 0× `status=500`, 0 panics/fatals in `/tmp/creaves-dev.log` + `/tmp/console-dev.log` |

---


## T11 — Gap closure (follow-up run 2026-10-08)

Closes the items left open by the main run: T7.7 cage-batch care, BUG-2 retest
criterion 2 in the browser, clean en-US reception re-run, role-guarded accounts,
public guest page, multi-hub sync targets, console admin flows (key creation in
fr + key deletion, annual-report counts, event archives/delete page, instance
cleanup + force-resync recovery).

| # | Step | Expected | Result |
|---|---|---|---|
| 11.1 | Clean en-US reception re-run: arrival #4, species `Pigeon ramier`, ring `E2E-RESTORE-004`, city `E2E-Anvers`, discoverer phone `+32 499 88 77 66` | animal created with correct species/ring in one pass; synced to console | ✅ animal **10377 = 2068 (2026)**, all fields verified (species/ring/city/phone via SQL); console `in_care` synced. NOTE: the wizard's species autocomplete fights fast automation — stale refs typed values into adjacent fields twice (agent-side timing, not an app bug; a field-by-field fill with settle delays is clean) |
| 11.2 | **T7.7 cage-batch care**: New → Cares → cage picker `Bac noir` (2 in-care animals); date `2026/10/07 22:00` via flatpickr calendar UI, **Cage cleaned ✔**, Save | one care row per in-care animal of the cage | ✅ flash `Care was successfully created.`; care rows `clean=1, date 2026-10-07 22:00` for BOTH animals 10312+10313; date set via calendar day-cell + Hour/Minute spinbuttons (flatpickr) |
| 11.3 | **BUG-2 criterion 2 (browser)**: landing green marker for those animals (yesterday-evening clean, ~10h old) | rows `table-success` under rolling-24h fix | ✅ `tr.table-success` count = 3; rows 2003+2004 (`Bac noir`) both `table-success` — old `CURDATE()+3h` code would have excluded them. BUG-2 criterion 2 closed |
| 11.4 | Guest page (logged out, fresh cookie-free session): `/guest/` number `2068/26` + phone `0499 88 77 66` | public match + status shown | ✅ `Animal status — 2068/26 · Common Wood Pigeon · In care` without authentication; **negative case**: wrong phone → query form again, no disclosure |
| 11.5 | Role guard: create user `e2e_lecteur` (role `Reader`, approved) via /users; login as it | per whitelist: ALL GETs allowed, ALL writes blocked | ✅ GET `/reception/new`, `/cares/new`, `/animals` all render (view-everywhere); **POST /cares blocked** → redirect to `/`, care count unchanged (378,498 before/after); user deleted afterwards (0 rows) |
| 11.6 | Multi-hub fan-out: add second Enabled sync target (same console); edit animal 10377 (cage → `E2E-C4`) | both targets deliver; console dedups | ✅ Local 16,102→16,103, Secondary 0→6,000 (backfill on creation); console stored the dual-delivered event **once** (`animal_state`=2 total for 10377, not 3). Second target removed after the check (one operator slip deleted the wrong row — restored via target edit; automation-only, not an app defect) |
| 11.7 | Console key lifecycle in fr: create key (instance `creaves-restore`), verify localized form, edit name, then delete | localized form; raw key page; edit + delete work | ✅ create/edit/delete all successful (flash `API Key updated/deleted successfully`); **found BUG-6** (stale hint text: old creaves UI route + "shown only once" claim in all 4 `new.plush.*`) — fixed + 🔁 retested; follow-up fixed the same stale claim in fr/de/nl `created.plush.*` |
| 11.8 | Annual report counts: `/reports/annual` year 2026, instance scope — Species / Outtake-by-type vs SQL | exact match | ✅ **exact**: total 2060 = SQL 2060; top species 580/210/92/73/66 identical; outtake-by-type Décédé 926 / Relacher 660 / Euthanasier 173 / Transferer 43 / Mort à l'arrivée 39 / Adoption 12 = SQL, total 1853 |
| 11.9 | Console events delete page + archives list (render only — a destructive delete would break the verified producer-checksum state) | pages render, archive list reachable | ✅ both render (fr): delete danger-zone with scope/confirmation semantics; archives page (empty, explanatory). Destructive path left to unit coverage by design |
| 11.10 | **Instance cleanup + recovery**: `/instances/creaves-restore` cleanup (type exact ID) → purge verified → new console key → update sync target → **Force full rebuild** resync → verify heal | cleanup empties the instance; force resync restores everything; checksums match producer | ❌→**BUG-7 + BUG-8** (force resync aborted at 4,001/10,314 then 0/10,314; target-recreate recovery scrambled statuses died 2,152 vs 5,775) → both fixed by dedicated agent (one follow-up: fizz migration rejected `--` comments) → 🔁 **full cycle retested PASS**: cleanup ✔ → new key ✔ → force resync **completed 10,314/10,314, 0 failed** → console **10,314 animals, died 5775 / released 4327 / in care 212**, animal 2058 `died`+`Décédé`, 10376 stays deleted, `/sync_management` checksum match + matches producer |


---


## T12 — From-scratch redo of the key feature (clean migration) — 2026-10-08

Full reset: both DBs dropped/recreated, **complete migration chain from zero**
(creaves: 112 migrations + seed → admin/admin + reference data; console:
24 migrations incl. the new `consolidated_animal_tombstones` + seed →
admin/admin123), apps restarted, then the key sync feature exercised end-to-end
on the empty system (en-US).

| # | Step | Expected | Result |
|---|---|---|---|
| 12.1 | Drop/create `creaves` + `consolidation`; clean `buffalo db migrate up` + `db:seed` on both; restart apps | full chain applies from zero; admins recreated; both apps serve | ✅ creaves 112/112 + seed (497 species, admin); console 24/24 + seed; :3000/:3001 200 |
| 12.2 | Console: login, create key `Scratch Center` / instance `creaves-scratch` | raw key shown on /created page | ✅ `creaves_e164281b…` |
| 12.3 | Creaves: `/sync_configuration` Instance ID `creaves-scratch` + Add sync target `Scratch console` (new key, batch 10, max 6000) | target Enabled | ✅ |
| 12.4 | Reception wizard ×2 (en-US): `E2E-SCRATCH-001` (Hérisson juvénile) and `E2E-SCRATCH-002` (Pigeon ramier adult) | animals 1 and 2 created; auto-synced | ✅ animals **1** and **2**; console: both `in_care` with species/ring correct |
| 12.5 | Release animal 1 (`Relacher`, via `/outtakes/new?animal_year_number=1/26`) | creaves outcome; console → released | ✅ flash `Outcome was successfully created.`; console animal 1 `released`/`Relacher` (location empty — operator ref miss, not asserted) |
| 12.6 | Non-force resync | both events skipped-unchanged; run completed; expected set announced | ❌→**BUG-9** (announcement never sent when nothing delivered — Expected(producer) stayed empty on the fresh console) → fixed (announcement-only batch on fully-skipped resyncs) → 🔁 retested: all-skipped resync now stores Expected=2 at the console without any delivered event |
| 12.7 | Force resync (2 events) | re-delivers + announces; "matches producer" | ✅ completed 2/2 delivered 0 failed; `/sync_management` Stored=Expected=Received=Confirmed=2, Unconfirmed 0, **"matches producer"**; final states animal 1 `released`/Relacher, animal 2 `in_care`; 0 tombstones. (Also re-run after the BUG-9 retest cleanup: cleanup → new key → skipped resync (announcement stored) → force resync → 2/2 + badge) |

**Environment note:** the agent-browser daemon discards cookies after ~1h idle
(recurring mid-flow logouts); fixed by restarting the daemon with
`AGENT_BROWSER_IDLE_TIMEOUT_MS=0`.

---


---

## Bugs found → see [bugs.md](./bugs.md)

| Bug id | Summary | Severity | Status | Retest |
|---|---|---|---|---|
| BUG-1 | Migration `20261007120000_rename_ot2_to_decedee` fails on production-restore DB (duplicate OT2 rows 'DCD' + 'Décédé') | Blocker | FIXED (dedicated fix agent) | 🔁 migrate exit 0, 112/112 applied, 1 OT2 'Décédé' row, 10149 outtakes intact, scratch-topology tests a–d pass |
| BUG-2 | Landing green cleaned-cage marker ignores night-shift cares (00:00–03:00) — SQL window `CURDATE()+3h` vs documented "last 24h" | Medium | FIXED (dedicated fix agent, rolling 24h parameterized) | 🔁 clean care at 00:28 → `tr.table-success` = 1 on `/` (was 0) |
| BUG-3 | Resync/state events mark dead animals `released` on the console (`applyCurrentStatus` ignores outtake dead flag) | High | FIXED (dedicated fix agent, producer-side) | 🔁 resync: 5775 re-sent; console died=5775/released=4327/in_care=211; By Status report correct; checksum "matches producer" |
| BUG-4 | creaves navbar tooltips `title="Day plan"`/`"Search animal"` hardcoded English in fr/de/nl templates | Low | FIXED (dedicated fix agent) | 🔁 fr nav accessible names: `Plan du jour` / `Rechercher un animal`; grep count 0 in fr/de/nl |
| BUG-5 | Care-plan history applied-at time follows browser locale, not app language | Low | CLOSED — by design (round-10 test pins browser localization across animals/treatments/care_plan; initial fix reverted by dedicated agent, pin test PASS) | — (documented; needs product decision if app-language times are wanted) |
| BUG-6 | Console webhook-key pages: stale on-screen hints (old creaves "Config → Webhook Configuration" route + "shown only once" claim) in all locales | Low | FIXED (dedicated fix agent; follow-up fixed fr/de/nl `created.plush.*` too) | 🔁 fr new/created pages: 0 stale strings, corrected sync-target instructions; key lifecycle create/edit/delete retested in fr |
| BUG-7 | Force full rebuild cannot redeliver (re-queued events keep "delivered" per-target delivery rows) — advertised post-cleanup recovery broken | High | FIXED (dedicated fix agent: force re-queue resets `event_deliveries` for all enabled targets) | 🔁 full cycle: cleanup → new key → force resync **completed 10,314/10,314, 0 failed** |
| BUG-8 | Console rebuild replays historical events in delivery order — late old events clobber newer states (died 5,775 → 2,152; animal 2058's death erased) | High | FIXED (dedicated fix agent: per-animal ordering guard + `animal_deleted` tombstones + migration) | 🔁 healed console: died 5775 / released 4327 / in care 212; 2058 `died`+`Décédé`; 10376 stays deleted; checksums "matches producer" |
| BUG-9 | All-skipped resync sends no announcement → fresh console's Expected(producer) stays empty, completeness badge unreachable without a force resync | Low | FIXED (dedicated fix agent: announcement-only batch at completion of fully-skipped runs) | 🔁 all-skipped resync stores Expected=2 console-side with 0 delivered events; force resync then restores animals + "matches producer" |

## Execution notes

- Result cells are filled in as the run progresses; failures reference bug ids in bugs.md.
- Fixes are executed by dedicated sub-agents; every fix is **automatically retested** by the tester and marked 🔁.
