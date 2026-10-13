# Bugs — E2E test run 2026-10-07

Tester log for the browser E2E run documented in [E2E-TEST.md](./E2E-TEST.md).
Each bug is passed to a dedicated fix agent; the fix is **automatically retested**
(retest steps + result recorded here).

---

## BUG-1 (blocker) — Migration `20261007120000_rename_ot2_to_decedee` fails on production-restore DB: duplicate OT2-coded rows

**Status:** CLOSED 2026-10-08 — fix dispatched + retest PASS (see Retest result below)
**Severity:** Blocker (blocks `buffalo db migrate` entirely; all later migrations unapplied; app cannot run)
**Found by:** E2E-TEST.md §2 step 2.4 (restore `creaves-db-2026-10-07.gz`, then `GO_ENV=development buffalo db migrate up`)

### Environment

- Restored `creaves-db-2026-10-07.gz` (production snapshot, taken 2026-10-07 08:00,
  schema migrated up to `20261007090000`) into MySQL database `creaves`.
- Current code contains migrations created **after** the snapshot, first pending one:
  `migrations/20261007120000_rename_ot2_to_decedee.up.sql`.

### Steps to reproduce

```bash
mysql -ucreaves -pcreaves -e "DROP DATABASE IF EXISTS creaves; CREATE DATABASE creaves CHARACTER SET utf8mb4;"
gunzip -c creaves-db-2026-10-07.gz | mysql -ucreaves -pcreaves creaves
cd creaves && GO_ENV=development buffalo db migrate up
```

### Observed

Migration aborts with:

```
UPDATE outtaketypes SET name = 'Décédé', updated_at = NOW()
  WHERE name = 'DCD';
: Error 1062 (23000): Duplicate entry 'Décédé' for key 'outtaketypes.outtaketypes_name_idx'
```

`schema_migration` stops at `20261007090000`; 90 migrations applied; every later
migration (care-plan tables, treatment time entries, attachment comments, …)
remains unapplied → the app cannot start against this DB.

### Root cause

1. `20261005160100_seed_canonical_outtaketypes.up.fizz` coded the row **named**
   `Décédé` as OT2, and because no row was *named* `Décédé` yet on this center
   (its canonical dead row was still named `DCD`), the `INSERT … WHERE NOT
   EXISTS (name='Décédé')` created a **second** OT2-coded row
   (`0a7e0000-0000-4000-8000-000000000002`).
2. `20261007120000_rename_ot2_to_decedee.up.sql` only folds an **uncoded**
   orphan (`code IS NULL OR ''`). Both OT2 rows are coded here, so step 1 no-ops
   and step 2's `UPDATE … SET name='Décédé'` collides with the unique
   `outtaketypes_name_idx` because the seeded duplicate already owns that name.

State at failure (both rows coded OT2):

| id | name | code | outtakes referencing it | created_at |
|---|---|---|---|---|
| `83f58302-8fd5-4bce-ac11-8407540d4869` | `DCD` | OT2 | **4,707** | 2021-04-05 |
| `0a7e0000-0000-4000-8000-000000000002` | `Décédé` | OT2 | 0 | 2026-10-07 (seed) |

Baseline totals for retest validation: `outtakes_total=10149`,
`animals_total=10344`, `translations WHERE table_name='outtaketypes' = 0`.

### Expected

`buffalo db migrate up` completes on a production-restore DB; afterwards there is
**exactly one** OT2-coded outtaketype, named `Décédé`, and all historical outtake
rows still resolve to it.

### Acceptance criteria (retest)

1. `GO_ENV=development buffalo db migrate up` exits 0 and applies all pending
   migrations (nothing stops before the end).
2. `SELECT id,name,code FROM outtaketypes WHERE code='OT2'` → exactly 1 row,
   `name='Décédé'`; no row named `DCD` remains.
3. `SELECT COUNT(*) FROM outtakes` → still 10,149; every `outtakes.outtaketype_id`
   resolves to an existing `outtaketypes.id` (FK intact); the 4,707 death records
   now point at the surviving `Décédé` row.
4. No orphan `translations.record_id` rows (table `outtaketypes`).
5. The same migration file must still be correct on the other two documented
   topologies (verify by executing the new up.sql manually against a scratch
   database with constructed fixture state, NOT against the `creaves` DB):
   a. fresh install: single coded OT2 row named `Décédé` → idempotent no-op /
      still ends with exactly one `Décédé` OT2 row;
   b. legacy center: coded `DCD` row + **uncoded** `Décédé` orphan (the case the
      current migration already handles) → still folds orphan correctly.
6. Do **not** drop/recreate the `creaves` database; do **not** modify data by hand
   on the `creaves` database — the fix must be expressed in the migration file
   (`migrations/20261007120000_rename_ot2_to_decedee.up.sql`, down.sql may be
   adjusted if needed) so an unmodified restore + migrate passes.

### Assignee notes

- Pop runs each SQL migration file inside a transaction on MySQL (verified: the
  failed run left no partial effects — `DCD` row untouched). Keep the fix DML-only.
- Deterministic survivor rule suggested: keep the OT2 row that is actually
  referenced by `outtakes` (fall back to oldest `created_at`) so production
  history keeps a stable FK target; fold duplicates into it, delete the losers,
  then rename the survivor to exactly `Décédé` (canonical fields already match:
  `dead=1, rating=-1, error=0, location_mode='none'`).

### Retest result

🔁 **PASS (retested by tester, independent of fix agent).**
`buffalo db migrate up` re-run → exit 0, no pending migrations left
(`schema_migration` = 112 rows = all files on disk, last `20261028100000`).
`outtaketypes WHERE code='OT2'` → exactly 1 row `83f58302…` named `Décédé`
(dead=1, rating=−1, error=0, location_mode='none'); rows named `DCD` → 0.
`outtakes_total` = 10,149 (baseline preserved); 4,707 death outtakes resolve to
the survivor; `translations WHERE table_name='outtaketypes'` = 0;
`animals_total` = 10,344. Fix agent additionally verified 4 topologies on
scratch DBs (fresh install, legacy uncoded orphan, BUG-1 reproduction,
uncoded-DCD+coded-Décédé) plus idempotent re-application. E2E run continued
past this blocker successfully (T1.6–T3 all pass).

---


## BUG-2 (medium) — Landing "cleaned cage" green marker ignores night-shift cares (00:00–03:00)

**Status:** CLOSED 2026-10-08 — fix dispatched + retest PASS (see Retest result below)
**Severity:** Medium (correctness of a daily-care UX signal; night-shift work invisible)
**Found by:** E2E-TEST.md §T7.3

### Steps to reproduce

1. Log in as admin (creaves, en-US or any locale) — current time between 00:00 and 03:00.
2. Record a care for any in-care animal with **Cage cleaned** checked and today's date
   00:00–03:00 (e.g. 2026/10/08 00:12, animal 2066/26 — row exists in `cares` with `clean=1`).
3. Open the landing page `/` and inspect the animal's row.

### Observed

The animal's row does **not** get the green `table-success` marker
(`tr.table-success` count on `/` = 0 despite a clean care recorded minutes
before). Verified twice (00:12 care; page reloads).

### Root cause

`actions/landing.go`:

```sql
SELECT DISTINCT c.animal_id as 'ID' FROM cares c
WHERE c.clean = 1 AND c.date >= DATE_ADD(CURDATE(), INTERVAL 3 HOUR)
```

Comment says "clean cage **within the last 24h**", but the predicate is
"`date >= today 03:00`". A care dated 00:00–03:00 belongs to the current
care-day cycle (previous day 03:00 → today 03:00) yet is excluded the moment
the wall clock passes midnight — the green marker disappears from rows cleaned
1–3 hours earlier. Same flaw hides marker for evening cares after midnight.

### Expected

The marker reflects cares from the current care-day (rolling last 24 h, or a
03:00-anchored window that includes 00:00–03:00 of the current calendar day),
per the function comment.

### Acceptance criteria (retest)

1. With a `cares` row `clean=1, date = today 00:12` (animal 10375 / 2066/26 —
   already present), the landing page marks that animal's row
   `table-success` (browser check: `tr.table-success` count ≥ 1 and the row
   for 2066/26 is green).
2. A care dated yesterday late-evening (e.g. 22:00) still counts during the
   following morning (until at least 03:00).
3. `go build ./...` and `go test -tags sqlite ./actions/ -run Clean` (or a new
   unit test covering the boundary) pass.

### Retest result

**CLOSED — works as designed (tester re-verified premise; change reverted).**
Investigation for the retest uncovered `actions/round10_localization_pins_test.go`
(`TestAppliedTimestampsUseBrowserLocalizationInAllLocales`, round-10 design
pins): rendering applied-at times in the **viewer's browser locale** is a
deliberate, test-pinned product decision spanning animals/show,
treatments/show AND care_plan templates ("each caregiver sees their own
preferred format"). The initially dispatched fix (app-locale via
`<html lang>`) contradicted that design and broke the pin test; it was
**reverted** by a dedicated agent — `toLocaleTimeString(undefined, …)` restored
in all four `templates/care_plan/index.plush*.html`, pin test PASS again.
Documenting here so the design intent is on record; if product wants
app-language times instead, that is a design decision to take across ALL three
template groups together, not a care_plan-only patch.

---

## BUG-3 (high) — Resync/state events mark dead animals `released` on the console: `current_status` inconsistent with outtake

**Status:** CLOSED 2026-10-08 — fix dispatched + retest PASS (see Retest result below)
**Severity:** High (consolidated data wrong for every dead animal delivered by a resync; console "By status" report buckets 5,774 dead animals as `Relâché`)
**Found by:** E2E-TEST.md §T8.7 (console `/reports` "Par statut" after death outtake)

### Steps to reproduce

1. Restore production DB, connect to console, run a full resync (E2E T3).
2. Record a death outtake in creaves (T8.2: animal 927/22 → `Décédé`).
3. Console `/reports` (fr) → "Par statut" table.

### Observed

- `current_status` on the console: **died = 1** (only the freshly-outtaken
  animal, whose live `animal_died` event carried `current_status:"died"`),
  **released = 10,101** — including the **5,774 historical dead animals**
  (4,707 `Décédé` + 1,067 `Euthanasier`/`Mort à l'arrivée`) whose last event
  was a resync `animal_state` snapshot.
- The dashboard cards (outcome-based classification) show the truth
  (Décédés 5,775), but any grouping by the raw `current_status` column
  (console "Par statut" report) is wrong.

### Root cause

`creaves/actions/webhook_resync.go` `applyCurrentStatus()` — used by **every**
full-state event (resync, update path, sync-status hashes):

```go
payload.CurrentStatus = "in_care"
if animal.Outtake != nil { payload.CurrentStatus = "released" }
```

It maps *any* outtake to `released`, ignoring whether the outtake is a death.
Only the live transition helpers set the truth
(`PublishAnimalDiedEvent` → `"died"`, `PublishAnimalReleasedEvent` →
`"released"`). So resync-delivered dead animals arrive as `released`, and the
live-outtake vs resync paths disagree (E2E evidence: the fresh death shows
`died` while the 5,774 resync-delivered dead show `released`).

### Expected

`applyCurrentStatus` must classify dead outtakes as `"died"` (outtake type
`dead` flag), so resync `animal_state`, update-path state events and live
transition events agree. Console `current_status` then reflects death for all
5,775 dead animals.

### Acceptance criteria (retest)

1. `go build ./...` and `go test -tags sqlite ./actions/... ./models/...` pass
   in **creaves**; console tests still pass in **creaves-console**
   (`CGO_ENABLED=1 go test -tags sqlite ./...`).
2. In the running system: start a full resync on creaves; when completed, the
   console shows `SELECT current_status, COUNT(*) FROM consolidated_animals
   WHERE instance_id='creaves-restore' GROUP BY current_status` with
   `died = 5775`, `released = 4327+…` (released-outtake animals), `in_care =
   211` (numbers as of 2026-10-08 ~00:30: total 10,313).
3. Console `/reports` "Par statut" (any locale) now shows Décédé ≈ 5,775 —
   consistent with the dashboard outcome card "Résultat : négatif (décédés)".
4. `/sync_management` shows checksum "matches producer" after the resync
   completes (hash change must not break the ack path).
5. In-care and released animals must be unaffected (their status values stay
   `in_care`/`released`).

### Assignee notes

- The dead classification source in creaves is the outtake type row
  (`outtaketypes.dead`); the animal's outtake is preloaded on the full model —
  check `reloadAnimalForEvent` eager loading includes the outtake's type, and
  that the console payload comparison/hashing path uses the same helper so
  hashes stay consistent.
- Changing the payload changes every dead animal's content hash — expected:
  next resync re-sends them once. Confirm the console re-applies (idempotent
  redelivery path) and acks.
- Console side needs no change if the producer payload is fixed; verify the
  console test fixtures still match the contract.

### Retest result

🔁 **PASS (retested by tester through the real resync pipeline).** Full resync
started from the creaves UI after the fix: `events_created=5775` (exactly the
dead animals whose content hashes changed), `events_skipped_unchanged=4538`
(in-care/released — hashes stable), `events_failed=0`, completed in <20 s.
Console after resync: `current_status` groups **died=5775 / released=4327 /
in_care=211** (total 10,313); console `/reports` "By Status" now shows
`Died 5775 / In care 211 / Released 4327`; `/sync_management` shows
Stored=Expected=Received=Confirmed=10,313, Unconfirmed 0, badges
**"checksum match" + "matches producer"**; creaves `event_streams` undelivered=0
(ack path intact after the hash change). Console tests pass unchanged.

---

## BUG-4 (low) — creaves navbar tooltips hardcoded English in fr/de/nl templates

**Status:** CLOSED 2026-10-08 — fix dispatched + retest PASS (see Retest result below)
**Severity:** Low (visible only as hover tooltip / accessible name; labels themselves are localized)
**Found by:** E2E-TEST.md §T10.1 (missing-translation sweep)

### Observed

`templates/application.plush.html` and **all** localized variants
(`application.plush.fr.html`, `.de.html`, `.nl.html`) hardcode
`title="Day plan"` on the `/care_plan` nav link and `title="Search animal"` on
the animal search box. In the fr/de/nl UIs the visible nav label is translated
(e.g. `Plan du jour`, verified in T4.1) but the tooltip/accessible name stays
English — violating the workspace all-language UI rule.

### Expected

Tooltips localized per template variant (e.g. fr `title="Plan du jour"` /
`title="Rechercher un animal"`; de `title="Tagesplan"` / `title="Tier suchen"`;
nl `title="Dagplan"` / `title="Dier zoeken"`). Sane French reference strings
are already in `locales/care_plan.*.yaml` (`nav.care_plan_dayplan`) — but since
these templates are per-locale static variants, inline the localized strings
directly (matching the existing pattern in those files).

### Acceptance criteria (retest)

1. `grep -c 'title="Day plan"\|title="Search animal"'` → 0 in the fr/de/nl
   application templates (the plain en-US base template may keep English).
2. The French UI's care_plan nav link accessible name/tooltip is French
   (browser: snapshot no longer shows accessible name "Day plan" when lang=fr).
3. Pages still render in all 4 locales (no template syntax errors).

### Retest result

🔁 **PASS (retested by tester through the browser).** fr UI nav accessible
names now localized: `link "Plan du jour"`, `searchbox "Rechercher un animal"`,
`button "Rechercher un animal"` (previously `Day plan` / `Search animal`).
grep count of the hardcoded English titles in fr/de/nl templates = 0; all 4
locales render 200. Fix agent also localized the hamburger toggler
(`aria-label`) in the same pass.

---

## BUG-5 (low) — Care-plan history "applied at" time follows the browser locale, not the app UI language

**Status:** CLOSED — works as designed (change reverted)
**Severity:** Low (time rendering only; values correct)
**Found by:** E2E-TEST.md §T7.1 (care-plan Historique in fr showed `09:00 · 12:11 AM`)

### Observed

Day-plan history rows render the applied-at time with client-side JS:
`templates/care_plan/index.plush.*.html`:
`appliedAt.toLocaleTimeString(undefined, {hour:'2-digit', minute:'2-digit'})`.
`undefined` = the **browser** locale (agent Chrome = en-US → `12:11 AM`), so a
user running the app in fr/de/nl with an English browser sees English 12-hour
times ("12:11 AM") inside an otherwise French page (the due-time right next to
it is server-rendered `09:00` — inconsistent within the same row).

### Expected

Use the app's selected UI language (the `lang` cookie / document locale) for
`toLocaleTimeString`, e.g. pass the locale string derived from the same source
the server-side i18n uses, so the fr UI always renders 24-h French format
consistently.

### Acceptance criteria (retest)

1. With the app in fr (cookie `lang=fr`) and an en-US browser, the applied-at
   time in the day-plan history renders in French format (24 h, e.g. `00:11`),
   matching the due-time format in the same row.
2. Same check in de and nl locales.
3. No template/JS errors in all 4 language variants of `templates/care_plan/index.plush.html`.

### Retest result

**CLOSED — works as designed (tester re-verified premise; initial fix
reverted).** Investigation for the retest uncovered
`actions/round10_localization_pins_test.go`
(`TestAppliedTimestampsUseBrowserLocalizationInAllLocales`, round-10 design
pins): rendering applied-at times in the **viewer's browser locale** is a
deliberate, test-pinned product decision spanning animals/show,
treatments/show AND care_plan templates ("each caregiver sees their own
preferred format"). The initially dispatched fix (app-locale via
`<html lang>`) contradicted that design and broke the pin test; it was
**reverted** by a dedicated agent — `toLocaleTimeString(undefined, …)`
restored in all four `templates/care_plan/index.plush*.html`, pin test PASS
again. Documented so the design intent is on record; if product wants
app-language times instead, decide it across ALL three template groups
together, not as a care_plan-only patch.

---

## BUG-6 (low) — Console webhook-key pages carry stale on-screen instructions (old creaves UI route + "shown only once" claim)

**Status:** CLOSED 2026-10-08 — fix dispatched + retest PASS (see Retest result below)
**Severity:** Low (misleading admin guidance; no functional impact)
**Found by:** E2E-TEST.md §T11.7 (key creation in fr)

### Observed

`creaves-console/templates/webhook_api_keys/new.plush.html` (+ `.fr/.de/.nl`)
instruct, after key creation:

1. "In the source Creaves instance, go to **Config → Webhook Configuration** …"
   (fr: "allez dans Config → Configuration webhook", de/nl equivalent) — that
   creaves UI **no longer exists**: webhook configuration moved to
   `/sync_configuration` + **sync targets**.
2. "The key is shown only once" (fr: "elle n'est affichée qu'une seule fois",
   de "einmal", nl "één keer") — contradicts the current behavior (raw key
   stored in `key_value` and retrievable from the list/detail pages, as the
   `/created` page itself correctly states).

### Expected

The Next-steps hint should point to the current creaves flow
(`/sync_configuration` → **Add sync target**, paste key + URL) and drop the
"shown only once" claim in all four locales, matching the already-correct
wording on the `/created` page.

### Acceptance criteria (retest)

1. `grep -riE "shown only once|qu'une seule fois|nur einmal|één keer" creaves-console/templates/webhook_api_keys/` → 0 matches.
2. `grep -rE "Config → Webhook Configuration|Configuration webhook" creaves-console/templates/webhook_api_keys/` → 0 matches.
3. All 4 locales of `/webhook_api_keys/new` render 200 and mention the
   sync-target flow (spot-check fr + en in the browser).

### Retest result

🔁 **PASS (retested by tester through the browser, fr locale).** Key-creation
page: 0 stale strings (`Configuration webhook` / `une seule fois` absent);
corrected instructions present ("Administration → Synchronisation
(/sync_configuration), puis ajoutez une cible de synchronisation…"; "elle reste
récupérable — la valeur complète est affichée sur la page Clés API…"). Created
page (fr) now shows "Astuce : cette clé reste récupérable…" (fix follow-up
agent corrected the fr/de/nl `created.plush.*` variants too). de/nl render
checked by the fix agent (200 + new strings, old strings 0). Full key lifecycle
retested in fr: create → edit name → delete, all successful.

---

## BUG-7 (high) — Force full rebuild cannot redeliver: re-queued events keep their "delivered" per-target delivery rows, so recovery after console-side cleanup stalls and silently drops events

**Status:** CLOSED 2026-10-08 — fix dispatched + retest PASS (see Retest result below)
**Severity:** High (the advertised disaster-recovery path — "use after a console-side cleanup deleted this instance's data" — does not deliver; ~61% of events silently skipped on first run, 100% on retry)
**Found by:** E2E-TEST.md §T11.10 (instance cleanup + force-resync recovery)

### Repro (browser, production-scale data)

1. Creaves connected to console (instance `creaves-restore`, 16,103 events in
   `event_streams`, sync target enabled with a valid key).
2. Console: **Instances → Delete instance data** (type instance ID to confirm)
   → console purges all animals/events/keys for the instance. ✔ works.
3. Create a new console API key, paste it into the creaves sync target. ✔
4. Creaves `/webhook_resync`: check **Force full rebuild** → Start resync.

### Observed

- Run 1: `failed` — `delivery incomplete: 4001 of 10314 events accepted; 6313
  events not delivered after 5 stalled attempts`.
- Immediate retry (run 2): `failed` — `0 of 10314 events accepted`.
- Left-over state: 6,313 `event_streams` rows with `delivered_at IS NULL`
  while **every** `event_deliveries` row for the target is marked delivered
  (`attempts=0`); console stuck at 7,478/10,314 animals. The pusher makes no
  further progress (queue query matches nothing), and **Retry undeliverable**
  cannot help (it only resets `attempts` on rows where `delivered_at IS NULL`,
  of which there are none).

### Root cause

`actions/webhook_resync.go` → `enqueueResyncStateEvent` (force branch) re-queues
an existing event with

```sql
UPDATE event_streams SET delivered_at = NULL, acknowledged_at = NULL, …
```

but does **not** touch the per-target `event_deliveries` rows. The pusher's
queue selection is

```sql
… LEFT JOIN event_deliveries d …
WHERE (d.id IS NULL) OR (d.delivered_at IS NULL AND d.attempts < max)
```

so any re-queued event whose delivery row still says "delivered" (i.e. almost
all of them in a long-lived target) is invisible to the deliverer. Run 1 only
delivered the events that happened to have no delivery row yet (4,001); run 2
had none left. The resync monitor then declares failure after 5 stalled polls
(~3 s of exponential backoff) — correct per its contract, but the run was
unrecoverable by design.

### Expected

A force re-queue must make events deliverable again — e.g. delete (or reset
`delivered_at=NULL, attempts=0` of) the `event_deliveries` rows of the re-queued
events for every enabled target, atomically with the `event_streams` update.
After a completed force resync the console must be fully rebuilt.

### Acceptance criteria (retest)

1. Unit/integration test in creaves: create target + delivered delivery rows,
   run force resync (test-tightened pump vars) → all events delivered again
   (`event_streams.delivered_at` set, console-equivalent accepts all).
2. Browser retest of the full cycle: console cleanup → new key → force resync →
   run `completed`, `events_failed=0`, console animals back to 10,314 with
   correct status groups, `/sync_management` "matches producer".
3. No regression of the non-force resync (T9 behavior: unchanged events skipped).

### Assignee notes

- Careful with the multi-target case: reset rows for ALL enabled targets, not
  just the first.
- Keep the resync monitor's stall guard (it correctly surfaces dead ends);
  the fix is to make the queue non-empty, not to lengthen the wait.
- The single 401 in the log (08:52:28) predates the key rotation and is
  unrelated (operator recreated the target with the pre-cleanup key).

### Retest result

🔁 **PASS (retested by tester through the full browser cycle).** After the fix:
console cleanup → new key → sync-target update → **Force full rebuild** → run
`completed` with **10,314/10,314 delivered, 0 failed** (pre-fix: 4,001 + 6,313
failed, then 0/10,314). Note: the first retest attempt was blocked by a defect
in the fix delivery itself — the new console migration used SQL-style `--`
comments, which the fizz lexer cannot parse; repaired by a follow-up agent
(comments removed, `int` → `integer`), migration applied cleanly. Delivery-row
reset on force re-queue verified live: all events re-delivered to the target.

---

## BUG-8 (high) — Console rebuild from full redelivery applies historical events in delivery order, scrambling consolidated states

**Status:** CLOSED 2026-10-08 — fix dispatched + retest PASS (see Retest result below)
**Severity:** High (after any full redelivery — target recreate, delivery-row purge — an animal's final consolidated state depends on network delivery order; death records get erased: died 5,775 → 2,152)
**Found by:** E2E-TEST.md §T11.10 recovery (target delete/recreate → all 16,103 events redelivered)

### Repro

Recover per the product's own path after a console-side cleanup: delete + recreate
the creaves sync target (this purges `event_deliveries`, so the worker redelivers
the **entire** event history — current-state snapshots *and* years-old
discovered/released events) in arbitrary batch order.

### Observed

Console rebuilt with 10,314 animals but wrong final states, e.g.:

- animal **2058** — died in care (T8.2, `Décédé` outtake) → console now shows
  **`in_care`, outtake NULL**, `last_event_at` = 2026-10-07 23:49:39 (an old
  state snapshot applied *after* the death event).
- Aggregates: `died=2152 / released=7948 / in_care=214` (correct: 5,775 /
  4,327 / ~212). Years-old `animal_released`/state events overwrite newer
  states purely by arriving later.

(Positive: the `animal_deleted` semantics survive the rebuild — animal 10376's
three events re-processed and removed its row + history again; console event
count settles at 16,100 = 16,103 − 3.)

### Root cause

`creaves-console` `actions/event_processor.go` applies every received event
synchronously in receipt order with last-write-wins semantics. There is no
per-animal ordering guard, so a stale historical event delivered late
overwrites a newer state. Delivery order is arbitrary, therefore the rebuilt
view is non-deterministic.

### Expected

Per-animal application must be order-insensitive for final state: ignore (or
merge without clobbering) events older than the newest state already applied —
compare payload `timestamp` (falling back to event `created_at`) against the
consolidated row's last applied timestamp. `animal_deleted` must keep working
regardless of position (an old event arriving after a delete must not
re-create the animal; a delete arriving after old events must still delete).

### Acceptance criteria (retest)

1. Console unit test: replay a discovered → state(died) → released → state(died)
   sequence in shuffled orders; final consolidated row must always be the
   latest-by-timestamp state (died with outtake), and an old event delivered
   after `animal_deleted` must not resurrect the row.
2. Browser retest of the full cycle (same as BUG-7 acceptance 2): console
   cleanup → target delete/recreate (full redelivery) → console animals 10,314
   with status groups `died≈5775 / released≈4327 / in_care≈212` and animal
   2058 showing `died` + `Décédé`.

### Assignee notes

- Producer-side (BUG-7, `actions/webhook_resync.go` `enqueueResyncStateEvent`):
  force re-queue must also reset the per-target `event_deliveries` rows of the
  re-queued events (delivered_at=NULL, attempts=0 — or delete them) for **every
  enabled target**, otherwise the pusher never re-sends them (queue query joins
  on those rows).
- Console-side: the ordering guard. Mind live-event ordering too (the update
  path emits events with current timestamps — the guard must not drop live
  events on equal timestamps; use <= semantics carefully).
- Keep the resync stall monitor unchanged.

### Retest result

🔁 **PASS (retested by tester through the full browser cycle).** After the
tombstones migration (one follow-up fix: fizz syntax) and force resync: console
`died=5775 / released=4327 / in_care=212` — exact expected values (was
2,152/7,948/214 scrambled); animal 2058 shows `died` + `Décédé` (was erased to
`in_care`); destroyed animal 10376 stays deleted (tombstone semantics; console
event count settles at its events minus the purge trio); 0 pending events;
`/sync_management` Stored=Expected=Received=Confirmed=10,314, Unconfirmed 0,
badges "checksum match" + "matches producer". Console unit suite: shuffled-order
replay, no-resurrection, stale-delete and equal-timestamp tests all pass.

---

## BUG-9 (low) — An all-skipped resync sends no announcement, so a freshly-cleaned console never gets the producer's expected set

**Status:** CLOSED 2026-10-08 — fix dispatched + retest PASS (see Retest result below)
**Severity:** Low (completeness badge unavailable in a legitimate state; data itself is fully synced)
**Found by:** E2E-TEST.md §T12 (from-scratch redo)

### Repro

1. Console: cleanup instance (or fresh console). Creaves already delivers its
   animals via live events (e.g. created after linking).
2. Creaves `/webhook_resync` → **Start resync** (non-force). Both animals are
   already in sync → `events_skipped_unchanged = all`, `events_delivered = 0`,
   run `completed`.
3. Console `/sync_management`: Stored=Received=Confirmed=N, Unconfirmed=0 — but
   **Expected (producer) = "–"** and no "matches producer" badge, because the
   announcement only travels on delivered batches and zero batches were sent.

### Observed vs expected

The run computes the announcement (visible on `/webhook_resync/status.json`:
`announced_expected_total=2`, `announced_at` set) but the console-side copy
(`creaves_instances.announced_*`) is only written from delivered batches. A
complete-but-fully-skipped sync therefore can never prove completeness against
the producer. Force resync works around it (re-sends everything and announces)
but should not be required just to publish the expected set.

### Expected

On resync completion, when `events_delivered == 0` and an announcement exists,
send one announcement-only webhook batch (`{"sync": {...}, "events": []}` —
the console already accepts empty batches) so the console stores the expected
set and can render "matches producer".

### Acceptance criteria (retest)

1. Unit/integration test: completed resync with all events skipped → console
   receives the announcement (creaves_instances.announced_expected_total set).
2. Browser: repeat the scratch scenario — after a fully-skipped resync,
   `/sync_management` shows Expected(producer)=N and the "matches producer"
   badge without a force resync.

### Retest result

🔁 **PASS (retested by tester through the browser).** After the fix: cleanup
instance → new key → **non-force resync** (created 0, skipped 2, delivered 0)
→ console `creaves_instances.announced_expected_total = 2` — the
announcement-only batch was sent and stored with **zero delivered events**
(impossible before the fix). Follow-up force resync re-delivered 2/2 →
`/sync_management` Stored=Expected=Received=Confirmed=2, Unconfirmed 0,
badge **"matches producer"**. Fix-agent unit test
`TestAllSkippedResyncSendsAnnouncementOnlyBatch` asserts the empty batch + sync
header wire format; full actions suite shows no new failures.

---

---

## BUG-10 (critical) — Legacy feeding rows with NULL start/end (or period-only/text-only) are never converted to care plans: feeding silently missing from the Protocol tab

**Status:** CLOSED 2026-10-09 — fix dispatched + retest PASS (see Retest result below)
**Severity:** Critical (data loss in the care-plan world: animal shows a legacy feeding schedule/diet but NO feeding protocol exists — the animal would not be fed per plan)
**Found by:** User report 2026-10-14 — animal `/animals/8842#nav-care`

### Repro

1. Animal 8842 (in care): `feeding = "15 poussins et 5 souris // EAU X8 = 120 poussins et 40 souris"`,
   `feeding_start = NULL`, `feeding_end = NULL`, `feeding_period = 480`.
2. Show page `#nav-care` displays "Feeding from 07:00 - 22:00, every 08:00h (2 meal(s))"
   (phantom times: `FeedingStartFmt/FeedingEndFmt` fall back to DEF_FEEDING_START/END when NULL).
3. Protocol tab (`#nav-plan`): **no feeding plan at all** — `care_animal_plans` has zero rows for the animal.

### Root cause

The startup converter (`convertFeedingSchedules`) only selects
`outtake_id IS NULL AND feeding_period > 0 AND feeding_start IS NOT NULL AND feeding_end IS NOT NULL`.
Animals with `feeding_period > 0` but NULL start/end — or with only a diet text — are skipped
silently, violating the §8.1 no-loss mandate ("all replaced actions must be converted").

### Expected / fix

- Convert EVERY in-care animal carrying any legacy feeding info (period > 0 OR start/end set OR
  non-empty diet text) into a per-animal feeding plan; rows without a derivable schedule get the
  no-loss fallback slot (09:00 or legacy start) flagged "à vérifier" — same pattern as the existing
  fallback path.
  - Existing DBs (marker `startup_v2` already written) are covered by a new one-shot `startup_v4`
    completion pass at boot; fresh installs get the widened query directly in the v2 step.

  ### Retest result

  ✅ PASS 2026-10-09 — full production-path simulation: restored `creaves-db-2026-10-08.gz` (pre-care-plan
  schema, 10 352 animals), applied the 63 pending migrations, booted the app.
  - v2 converter: "feeding 0 rules + 215 plans, treatments 70 plans" — every in-care animal with any
    legacy feeding trace converted, incl. NULL-times/diet-only rows the old code skipped.
  - v4 completion pass: "215 animals considered, 0 feeding plans created, uncovered 0" — idempotent no-op.
  - Markers startup_v2/v3/v4 all written.
  - Animal 8842: plan "Alimentation — 15 poussins et 5 souris // eau x8 = 120 poussins et 40… (conversion)
    (à vérifier)", active, slot 09:00 fallback. Protocol tab (browser, FR): "Spécifique · Alimentation —
    15 poussins et 5 souris… · Nourrissage · Actif · 2026-05-10 → ∞".
  - `go test ./actions` green (converter fixture extended with NULL-times/diet-only/no-trace animals).

---

## BUG-11 (low) — Edit-form frozen-schedule hint points to "l'onglet Plan", but the tab is now named "Protocole"/"Protocol"

**Status:** CLOSED 2026-10-09 — fix dispatched + retest PASS (see Retest result below)
**Severity:** Low (dead-end instruction: user looks for a "Plan" tab that no longer exists)
**Found by:** User report 2026-10-14 — `/animals/8842/edit/#nav-care`

### Observed

`animals.feeding.legacy_frozen` = "Horaire historique (lecture seule) — le plan se gère dans l'onglet Plan de l'animal."
The animal tab is now `t("care_plan.animal_plans.tab")` = "Protocole" (fr) / "Protocol" (en) /
"Protokoll" (de) / "Protocol" (nl).

### Fix

Update `animals.feeding.legacy_frozen` in all 4 locales to name the current tab. Link target
`#nav-plan` is unchanged (still the correct anchor).

### Retest result

✅ PASS 2026-10-09 — browser, `/animals/8842/edit` FR: the Protocol-tab link reads "Gérer dans l'onglet
Protocole" (was "Plan"). Strings updated + verified in all 4 locales (en/fr/de/nl). `TestAnimalForm
LegacyFeedingFrozenAllLocales` updated to the new wording and green in all 4 locales.

---

## BUG-12 (medium) — "Régime alimentaire" stage buttons always answer "Pas de guide…": feeding_guides table is empty and never seeded

**Status:** CLOSED 2026-10-09 — fix dispatched + retest PASS (see Retest result below)
**Severity:** Medium (feature appears dead: every stage click on any species yields the empty state)
**Found by:** User report 2026-10-14 — `/animals/8842/edit/#nav-care`

### Repro

1. Edit any animal → Suivis tab → click Bébé/Juvénile/Adulte/Malade.
2. GET `/suggestions/feeding_guide?species=…&stage=…` → `{"text":""}` (table `feeding_guides` = 0 rows;
   the table is NOT among the 7 startup-seeded reference tables, so fresh installs have no guides at all).
3. Modal opens with "Pas de guide pour cette espèce à ce stade" — perceived as "does not do anything".

### Fix

Make the empty state actionable: explain that no guide is recorded for this species/stage and point
admins to the guide management page (`/feeding_guides`, with link). JS wiring verified working in
all 4 locales (modal opens, canonical stage values match).

### Retest result

✅ PASS 2026-10-09 — browser, `/animals/8842/edit` FR, clicked "Juvénile": modal now shows "Pas de guide
pour cette espèce à ce stade. La table des régimes est vide — les guides sont de simples suggestions ;
saisissez le régime manuellement. [Gérer les régimes]" with an admin link to `/feeding_guides`. Same
change applied to en/de/nl templates.

---

## BUG-13 (medium) — Read-only legacy feeding schedule display (with phantom 07:00–22:00 defaults) instead of the actual feeding protocol

**Status:** CLOSED 2026-10-09 — fix dispatched + retest PASS (see Retest result below)
**Severity:** Medium (misleading info: legacy schedule is frozen/uneditable and shows phantom default
times when start/end are NULL; the real, actionable schedule lives in the animal's feeding plans)
**Found by:** User report 2026-10-14 — "If there are no way to change the feeding time … remove the
information … show a summary of the actual protocol feeding instead"

### Fix

- Show page `#nav-care` (all 4 locales): remove the legacy "Nourrissage/Feeding from…" banner +
  `calculatePeriodRepetitions` JS; render instead a summary of the animal's ACTIVE feeding plans
  (diet + time slots) with a link to the Protocol tab.
- Edit form `#nav-care` (all 4 locales): remove the frozen legacy `07:00 – 22:00 · 08:00` line;
  show the same feeding-plan summary; keep the (corrected) link to the Protocol tab.

### Retest result

✅ PASS 2026-10-09 — browser, `/animals/8842` `#nav-care`: phantom "Feeding from 07:00 – 22:00, every
08:00h" banner GONE (no `bg-info` block, no `calculatePeriodRepetitions`). Replaced by a real summary:
- FR: "Protocole d'alimentation — 15 poussins et 5 souris // eau x8 = 120 poussins et 40 souris — 09:00
  [à vérifier] — Gérer dans l'onglet Protocole"
- EN: "Feeding protocol — … — 09:00 [to check] — Manage in the Protocol tab"
- DE: "Fütterungsprotokoll — … — 09:00 [zu prüfen]", NL: "Voederingsprotocol — … — 09:00 [te controleren]"
Same summary on the edit form (all 4 locales). New shared partial
`templates/animals/_feeding_summary.plush.html` fed by `feedingPlanSummaries()` (Show + Edit).
`go test ./actions` green.

---

# Code review — feature/care-expert protocol changes (2026-10-09)

Static review of the care-plan engine (`models/careplan`) and the actions layer
(service, dayplan, fulfillment, viewmodel, animal page, converter/migration,
HTTP handlers, routes/auth). Findings below; each carries file:line evidence.

## BUG-14 (medium) — Intake-anchored rule + weekday filter: the day-1 slot filter leaks onto the first ALLOWED day

**Status:** ✅ FIXED 2026-10-09
**Severity:** Medium (wrong occurrences: the first matching day silently loses its early slots)
**Found by:** Code review 2026-10-09 — `models/careplan/occurrences.go:64,80-83`

### Repro (engine trace)

Rule: `anchor=intake`, `weekdays=[1..5]` (Mon–Fri), `times=[07:00, 13:00, 19:00]`.
Animal intake: **Sunday 10:00**.

1. `resolveDay1` → day1=Sunday, `firstSlotStart=SlotsAfter(600)=1` (13:00 stays, 07:00 dropped).
2. Main loop: Sunday fails `AllowsWeekday` → **skipped, `firstGeneratedDone` never set**.
3. Monday: `firstGeneratedDone` still false → `times = Times[1:]` → **Monday gets 13:00/19:00 only; the 07:00 slot is never generated**, although day-1 anchoring was meant for Sunday (the intake calendar day) only.

### Root cause

`firstGeneratedDone` applies the intake-day slot filter to the first *generated*
(weekday-passing) day, not to the intake calendar day. With a weekday filter that
excludes the intake day, the filter shifts to the wrong day.

### Expected / fix

Apply the day-1 filter only when the loop day IS the intake calendar day:
replace the flag with `firstSlotStart > 0 && sameLocalDay(loc, day, a.IntakeDate)`
(guard `AnchorIntake` via firstSlotStart > 0, which resolveDay1 already
guarantees for fixed anchors and offset-shifted day-1s).

### Fix applied 2026-10-09

`models/careplan/occurrences.go`: `firstGeneratedDone` flag removed; the day-1
slot filter is gated on `firstSlotStart > 0 && sameLocalDay(loc, day, a.IntakeDate)`.
Regression: `TestGenerateIntakeDay1WeekdayFilter` (models/careplan) — Sunday
intake + Mon–Fri rule keeps Monday 07:00; Monday 10:00 intake still drops
Monday 07:00.
**Retest:** `go test ./models/careplan -run TestGenerate` ✅ PASS 2026-10-09.

---

## BUG-15 (medium) — Day-window end computed as `start.Add(24h - 1ns)`: wrong on DST transition days

**Status:** ✅ FIXED 2026-10-09
**Severity:** Medium (occurrences missed or double-windowed twice a year, per location)
**Found by:** Code review 2026-10-09 — 5 sites:
`models/careplan/next.go:154` (WorkWindow), `actions/care_plan_service.go:43`
(TodayPlanWindow), `actions/care_plan_service.go:60` (DefaultPlanWindow),
`actions/care_plan.go:199` (planWindowParams `to=` parse),
`actions/care_plan_animal_page.go:214` (TreatmentPlanWindow)

### Repro

Europe/Paris, 2026-03-29 (spring forward, 23 h day):
`TodayPlanWindow(now)` end = midnight + 24h−1ns = **2026-03-30 00:59:59+02:00**
→ the "today" window leaks ~1 h into the next day; a 00:30 slot of Monday 30th
appears in Sunday's day plan. On 2026-10-25 (fall back, 25 h day) the window
ends at 23:59:59 of the *wall clock's* 24h — the last real hour (the repeated
hour) is cut off: a slot at 23:30 wall time can fall outside the window.

### Root cause

`Add(24*time.Hour)` is an INSTANT shift, not a calendar-day shift; on DST
transition days the local day is 23 h or 25 h long.

### Expected / fix

Compute the end as `AddDate(0, 0, 1).Add(-time.Nanosecond)` from the local
midnight base (calendar arithmetic honours DST), in all 5 sites. Engine-side
occurrence generation is calendar-based already (`time.Date(...)`), so only the
window bounds are affected.

### Fix applied 2026-10-09

All 5 sites switched to `AddDate(0, 0, N+1).Add(-time.Nanosecond)` (multi-day
windows add past+future+1 calendar days before the 1 ns step back).
Regression: `TestPlanWindowsDST` (actions/care_plan_animal_page_r36_test.go) —
spring-forward end stays on the 29th 23:59, fall-back on the 25th 23:59,
multi-day windows and `careplan.WorkWindow` checked.
**Retest:** `go test ./actions -run TestPlanWindowsDST` ✅ PASS 2026-10-09.

---

## BUG-16 (medium) — Latest-weight / latest-diagnostic picks are nondeterministic on same-date ties → wrong table dosage possible

**Status:** ✅ FIXED 2026-10-09
**Severity:** Medium (a tie silently feeds the WRONG weight into §10-B6 dosage
computation → wrong medication dose displayed/applied)
**Found by:** Code review 2026-10-09 — `actions/care_plan_service.go:399-417`
(fillLastWeights), `:433-446` (fillVetDiagnostics)

### Repro

Two `cares` rows for one animal with the same `date` (datetime granularity is
1 s in the schema — two weighings logged within the same second, or a backdated
entry + a same-day correction both stamped 00:00): the
`JOIN (SELECT animal_id, MAX(date) …)` returns BOTH rows; the `for _, r := range rows`
loop then sets `ctx.LastWeightG` to whichever row arrives last (no ORDER BY →
InnoDB order, effectively random per query plan). `LastWeightG` feeds
`resolvePlanDosage` (dosage = per-grams × weight) → the caretaker can be shown /
charged a dose computed from the superseded weighing.

### Expected / fix

Deterministic tie-break: `ORDER BY c.date DESC, c.id DESC` + pick the first row
per animal (or use `MAX(date)` + `MAX(id)` window function / correlated
subquery). Same for fillVetDiagnostics.

### Fix applied 2026-10-09

Both queries (`fillLastWeights`, `fillVetDiagnostics`) got
`ORDER BY <alias>.animal_id, <alias>.created_at DESC, <alias>.id DESC` plus a
Go-side first-row-per-animal guard (`seen` map). NOTE: the tie-break key is
`created_at` — `cares.id` is a random UUID, not recency — and `created_at` is
datetime (1 s precision), so ties WITHIN the same second still fall to `id`,
but that order is now deterministic per snapshot. Caveat discovered while
testing: pop `Create` stores `created_at` as LOCAL wall clock while time.Time
RawQuery args are converted to UTC by go-sql-driver — rows written via raw SQL
(converters, repairs) get UTC stamps; the tie-break is only reliable across
same-path writes (the common case).
Regression: `TestFillLastWeightsSameDateTie` (actions/care_plan_weight_tie_test.go)
— same-date tie resolves to the latest inserted row repeatably (5×), a later
date still wins regardless of insertion order.
**Retest:** `go test ./actions -run TestFillLastWeightsSameDateTie` ✅ PASS 2026-10-09.

---

## BUG-17 (low) — Protocol-tab plan window text ignores every_days/weekdays: wrong end date, premature "expired"

**Status:** ✅ FIXED 2026-10-09
**Severity:** Low (display-only: the Protocol tab's "2026-05-10 → 2026-05-14"
window line and the planExpired badge are wrong for stepped/filtered schedules)
**Found by:** Code review 2026-10-09 — `actions/care_plan_animal_page.go:887-893`
(planWindowBounds)

### Repro

Schedule `every_days=2, duration_days=5`, start 2026-05-10 → the engine generates
days 10/12/14/16/18 (last day = start + 8 d, §10-B3 counts GENERATED days).
`planWindowBounds` computes `end = start.AddDate(0,0,duration_days-1)` = 05-14 —
4 days short; with a weekday filter the error grows up to 7×. `planExpired`
therefore flags a still-running plan as expired once today passes the displayed
(bogus) end.

### Expected / fix

Reuse the §10-B3 generated-days math (mirror of `courseHorizonDays` in
care_plan_dayplan.go but exact, not worst-case): walk every_days steps counting
weekday-passing days until duration_days generated days are reached.

### Fix applied 2026-10-09

`planWindowBounds` now walks candidate days exactly as
`careplan.GenerateOccurrences` does (step every_days, count weekday-passing
days, same 100000-step defensive cap) and returns the last generated day.
Regression: `TestPlanWindowBoundsGeneratedDays` (actions/care_plan_animal_page_r36_test.go)
— every_days=2 and Mon–Fri-filtered and combined every_days=3+weekdays cases
match the engine's last occurrence day; open-ended stays nil.
**Retest:** `go test ./actions -run TestPlanWindowBoundsGeneratedDays` ✅ PASS 2026-10-09.

---

## BUG-18 (low) — Defer clamp gives up after 30 days: sparse schedules can defer past the next occurrence

**Status:** ✅ FIXED 2026-10-09
**Severity:** Low (edge case: schedules with `every_days` > ~30 or very sparse
weekday patterns let a defer resurface AFTER the next occurrence, leaving the
item non-applicable → recordable only via the late flag)
**Found by:** Code review 2026-10-09 — `actions/care_plan_support.go:114`
(ClampDeferredUntil, `windowEnd := dueAt.Add(30 * 24 * time.Hour)`)

### Repro

Rule `every_days=45`, occurrence due today. Caretaker defers +40 days.
`GenerateOccurrences(src, ctx, due+1s, due+30d)` finds nothing (next occurrence
is at +45 d) → request passes unclamped → the deferred item resurfaces at +40 d
while the occurrence is already superseded → hors-délai 409 on apply without
`late=true`.

### Expected / fix

Either derive the search window from the schedule (`courseHorizonDays`-style:
at least one full step + slack) or, simpler, walk forward until the first
occurrence is found with a generous cap (e.g. 400 days, mirroring
`maxOccurrenceDaySteps`).

### Fix applied 2026-10-09

`ClampDeferredUntil` sizes the search window from the schedule:
`miss_after_hours + 3 × 7 × every_days` days (covers the next generated day
even under a full-week weekday filter), floor 30 days; and when NO successor
exists (course finished), the defer is clamped to the occurrence's own miss
deadline (§4.3) instead of passing through unbounded.
Regression: `TestClampDeferredUntilSparseSchedule` (45-day schedule, +50d defer
clamps to successor − 1 min) and `TestClampDeferredUntilNoSuccessor`
(1-day course: clamps to miss deadline) in actions/care_plan_clamp_test.go.
**Retest:** `go test ./actions -run TestClampDeferredUntil` ✅ PASS 2026-10-09.

---

## BUG-19 (low) — Due-label day word mislabels occurrences on DST transition days

**Status:** ✅ FIXED 2026-10-09
**Severity:** Low (cosmetic: "today/yesterday/tomorrow" label wrong by one day
for occurrences next to a 23 h / 25 h day boundary)
**Found by:** Code review 2026-10-09 — `actions/care_plan_display.go:83`
(DueLabelPartsOf: `days := int(day.Sub(today).Hours() / 24)`)

### Repro

Europe/Paris, now = 2026-03-29 (23 h day), occurrence due 2026-03-30 08:00:
`day.Sub(today)` = 23 h → `int(23/24)` = 0 → rendered as **today** instead of
tomorrow.

### Expected / fix

Compare calendar ordinals instead of hour deltas, e.g.
`days := int(math.Round(day.Sub(today).Hours() / 24))` or, cleaner,
`time.Date`-based `DaysIn`-style difference (Julian day numbers).

### Fix applied 2026-10-09

`DueLabelPartsOf` now counts CALENDAR days by stepping `AddDate(0, 0, ±1)`
between the two local midnights (exact on 23 h / 25 h days); both instants are
normalized into `now`'s location first.
Regression: `TestDueLabelPartsOfDST` (actions/care_plan_display_test.go) —
Mar 29 → Mar 30 and Oct 25 → Oct 26 read "tomorrow", same-day/yesterday stay
exact on transition days.
**Retest:** `go test ./actions -run TestDueLabelPartsOf` ✅ PASS 2026-10-09.

**Full suite 2026-10-09:** `go test ./...` (all packages) ✅ PASS —
models/careplan 0.17 s, actions 20.3 s, everything else green.

---

# Bugs — care-plan toggle/divider review run 2026-10-28

User-reported issues from live review of `http://localhost:3000/care_plan`
(kind=medication / kind=feeding&group=cage) and `/animals/{id}#nav-plan`.
All five (BUG-20..24) FIXED 2026-10-28; browser E2E re-run 2026-10-28
(agent-browser) confirmed each acceptance criterion. A second full browser
run the same day (apply → reload → undo → reload → DB assertions) caught
and fixed one more silent defect the cosmetic-only first pass had missed
(BUG-22 `planUnapplyBatch` — see its "Live-E2E defect found + fixed"
section), and re-confirmed BUG-20/21/23/24 live.

**Error messages de-pointered (user request 2026-10-28):** the 409
`errItemNotReproduced` and the apply-window/validation errors no longer
point to internal spec chapters ("(§6.2)" etc.) — each message is
self-explanatory:
- `actions/care_plan_dayplan.go`: "occurrence not found or no longer
  produced by its source — the plan has changed since this page was
  rendered; reload and try again"
- `actions/care_plan.go`: "reason is mandatory for skip and defer" /
  "occurrence already recorded (idempotent)" / "occurrence is out of its
  apply window (hors délai) — too late to record it from the plan; use a
  manual care entry instead" / "a batch can only contain occurrences from a
  single source"
- `actions/care_plan_fulfillment.go`: "weight is required to record a
  weighing" / "an answer is required to record an observation"

**409 investigation (user request 2026-10-28):** a live click on animal
10320's feeding occurrence (plan `b4cce8fe`, due 2026-10-11T09:00) returned
409 once at 12:41. A probe re-running `ReverifyItem` against the dev DB
with identical inputs SUCCEEDED, and re-clicking the same button at 12:54
returned 201 (care created). Conclusion: transient/non-reproducible — most
likely a stale-DOM click (the page reloads every 60 s) or a transient plan
state; the identical ref applies cleanly. No code change beyond the
self-explanatory message.

---

## BUG-20 (medium) — Medication series buttons split by "matin/soir" bucket captions

**Status:** ✅ FIXED 2026-10-28 (code + unit/render tests + browser E2E 2026-10-28)
**Severity:** Medium (readability: one drug series' time buttons were split
onto several lines by uppercase time-of-day captions — "SOIR" etc.)
**Found by:** User report 2026-10-28 — `/care_plan?kind=medication`

### Repro

An animal with the same drug due morning AND evening (e.g. Citramox 08:00 +
18:00) rendered the series as:

```
Citramox — 0.5 ml        [○ 08:00]
              MATIN      …
              SOIR
                         [○ 18:00]
```

The bucket caption (`.plan-med-bucket`, `t("care_plan.slot." + slot)`) forced
a fresh flex row (`flex: 1 0 100%`) whenever the bucket-fit JS measured a
wrapped group — the buttons of ONE series were visually split by a "soir"-type
splitter.

### Decision (user, 2026-10-28)

> The only moment a splitter may appear is when actions are NOT on the same
> calendar date → a **date group**. A same-day time-of-day change
> (matin/midi/soir) NEVER splits the buttons.

### Fix applied 2026-10-28

- `actions/care_plan_viewmodel.go`
  - `MedSeriesRow.DividerBefore` semantics changed: bucket-change →
    **calendar-date change** (compared on the row's first slot vs the previous
    row's last slot `DueAt.Date()`).
  - New `MedSeriesRow.DividerLabel` — the row's date in `models.DateFormat`
    ("2006/01/02") — rendered as the divider caption.
- `templates/care_plan/_med_series.plush.{html,fr,de,nl}.html` (4 forks kept
  byte-identical)
  - Divider renders `row.DividerLabel` (a date), never a slot name.
  - Per-slot day badges restored for **tomorrow** slots too (the
    `slot.Slot != "tomorrow"` suppression existed only because the bucket
    caption "already said the day" — with date captions hidden on one-row
    groups, every non-today slot now carries its own badge again).
- `assets/css/care-plan.scss` — `.plan-med-bucket` rule kept (it is now the
  date-group caption); comments updated.
- The bucket-fit JS in `_apply_toggle.plush.html` is unchanged: it still
  hides the caption when the whole group fits one row — now operating on
  date captions.

### Regression tests

- `TestMedSeriesChunksThreePerRowByBucket` — same-day bucket change no longer
  divides.
- `TestMedSeriesDividerOnDateChange` (new) — cross-date boundary divides and
  carries `"2026/09/29"`; same-date cross-bucket boundary does not.
- `TestMedSeriesPartialRenders` / `TestMedSeriesCompactSuppressesBucketLabel`
  — fixtures carry `DividerLabel`; render asserts the date caption and the
  dashboard's compact suppression.
- `TestMedSeriesLayoutAndInfoFirst` — asserts
  `class="plan-med-bucket">2026/10/03<` and that NO `>care_plan.slot.*`
  caption splits the buttons.

**Retest:** `go test ./actions -run 'TestMedSeries|TestPhase1BucketCaptionGate|TestMedSlotLateAllowed|TestMedSlotTier'` ✅ PASS 2026-10-28.
**Browser E2E (2026-10-28, agent-browser):** ✅ PASS — `/care_plan?kind=medication`: same-day slots (○ 08:00 ○ 18:00) render on ONE line per series; zero `.plan-med-bucket` captions and no `SOIR`/`MATIN`/`MIDI` splitters in the page text.

---

## BUG-21 (low) — Apply-button tooltip carries no date (HH:MM only + slot name)

**Status:** ✅ FIXED 2026-10-28 (code + browser E2E 2026-10-28)
**Severity:** Low (ambiguous affordance: a multi-day series shows several
identical "○ 18:00" buttons; the tooltip said "Appliquer — soir" without the
date, so the caregiver could not tell WHICH occurrence the click applies)
**Found by:** User request 2026-10-28 — medication apply button

### Fix applied 2026-10-28

- `actions/care_plan_viewmodel.go`: new `MedSlotView.DueAtFull` — the
  complete occurrence time in `models.DateTimeFormat` ("2006/01/02 15:04"),
  populated in `medSlotFor`.
- `templates/care_plan/_plan_slot_toggle.plush.{html,fr,de,nl}.html`: the
  apply button's `title` is now
  `"<apply action> — <slot name> · <DueAtFull>"`
  (e.g. `Appliquer — soir · 2026/10/29 18:00`).

Note: the feeding/care/cleanup **item** toggles (`_plan_item_line`,
`_plan_care_slot`) still have a bare `title="care_plan.apply.action"` — fold
their tooltip into the BUG-22 toggle rewrite (same `.plan-apply-btn` family).

**Browser E2E (2026-10-28, agent-browser):** ✅ PASS — every med slot apply button's tooltip carries the full due datetime (`Apply — Morning · 2026/10/11 08:00`), not just the bucket word.

---

## BUG-22 (high) — Action toggles disappear after click / "done" link replaces the toggle: no in-place green ✓ + undo

**Status:** ✅ FIXED 2026-10-28 (code + regression render tests + browser E2E 2026-10-28)

### Fix applied 2026-10-28

- `templates/care_plan/_plan_item_line.plush.{html,fr,de,nl}.html` — the
  applied single-occurrence branch no longer renders the done badge-link
  INSTEAD of the toggle: it renders a d-none apply button + a visible green
  ✓ `.plan-unapply-btn` (real undo toggle) and a SEPARATE view link
  (`badge badge-success plan-item-view`, external-link icon only) beside it.
- `templates/care_plan/_apply_toggle.plush.{html,fr,de,nl}.html` —
  - New shared `#planUnapplyModal` (title/animal/detail/source + submit);
    the pair-undo click opens it instead of applying instantly.
  - `instantUnapply(btn, done)` performs the confirmed POST
    `/care_plan/unapply {delete_fulfillment:true}` and flips the pair back
    via `markOpen` on 200.
  - `scheduleAnimalConvergeReload` (the forced `window.location.reload()`
    ~1.2 s after every flip on `#nav-plan`) is REMOVED — the in-place flip
    is the source of truth; no page reload anywhere.
  - `window.planApply` exposes `openUnapply` + `pairUndo` for the cage
    batch undo wiring.
- `templates/care_plan/index.plush.{html,fr,de,nl}.html` —
  - `flipBatchRow` no longer hides the group button: new `setBatchState`
    flips it to a green ✓ `.plan-cage-undo` group-undo toggle (tier class
    captured once in `data-tier-class`, ○→✓, count pill hidden).
  - Delegated `.plan-cage-undo` click → `planApply.openUnapply(btn, true)`;
    `window.planUnapplyBatch` loops the row's applied chips through
    `/care_plan/unapply` + `setToggleState(chip, false)`, restoring the
    group button on success.

### Live-E2E defect found + fixed (2026-10-28, second browser run)

The first browser pass above was **premature**: it asserted the cosmetic
flip only. A genuine re-run (apply → reload → undo → reload → DB check)
exposed a silent no-op in the CAGE group undo:

- **Symptom:** on a single-animal cage row (and on the done-tier rows), the
  group undo flipped the button to ○ and closed the modal but made **zero**
  `/care_plan/unapply` calls — after a reload the occurrence was still
  applied (care record still present server-side).
- **Root cause:** `planUnapplyBatch` looked the applied chips up with
  `row.querySelectorAll('.plan-unapply-btn:not(.d-none)')`. Single-animal
  cage rows render NO per-animal pair (R4-7.5: only a `.plan-apply-space`
  spacer) and done-tier rows render link-only chips — so the lookup matched
  zero chips and the loop body never ran. The flip was purely cosmetic.
- **Fix:** `planUnapplyBatch` now iterates the refs carried on the group
  button itself — `JSON.parse(btn.getAttribute('data-items'))` — and POSTs
  `/care_plan/unapply {delete_fulfillment:true}` per ref, flipping the
  matching chip only if one is found in the row. `setBatchState(btn,false)`
  runs after the loop. Verified live: group undo → 200
  `{"deleted_fulfillment":"care","status":"unapplied"}` → reload shows the
  occurrence back to ○ 09:00 and the care record deleted.
- **Also fixed the same run:** `openUnapply` label/detail fallbacks —
  cage/group undo buttons carry no apply sibling, so the confirm modal
  showed a blank Animal/Protocol. The label now falls back to
  `btn.getAttribute('data-cage')` and the detail to
  `btn.getAttribute('data-food')`.
- **Regression pin:** `care_plan_bug22_toggle_test.go`
  `TestBug22BatchButtonFlipsToGroupUndo` now asserts the `planUnapplyBatch`
  body reads `btn.getAttribute('data-items')` and no longer contains
  `querySelectorAll('.plan-unapply-btn:not(.d-none)')`.
- Phase0b DOM baselines re-recorded (`PHASE0B_RECORD=1`) — the recorded
  delta is exactly the `planUnapplyBatch` data-items rewrite, the
  `openUnapply` fallbacks, and the `scheduleAnimalConvergeReload` removal.

### Browser E2E (2026-10-28, second run — full apply/reload/undo/reload/DB loop) — ✅ PASS

- Feeding cage B2 (single-animal row): apply → 201 care created → reload
  shows ✓ → group undo → 200 → reload shows ○ 09:00 → care deleted.
- Weighing (modal): click → weight modal → 142g → ✓ in place → care
  created → confirm-undo → ○ → care deleted.
- Observation (modal): click → answer modal → ✓ in place → record created
  → confirm-undo → ○ → record deleted.
- Medication: apply → 201 treatment entry → ✓ in place → instant undo →
  200, own entry reverted to `pending`, row kept (sibling 18:00 entry).
- `/animals/{id}#nav-plan`: apply/undo run in place; `__e2eMarker` stayed
  alive and `beforeunload` never fired (no `window.location.reload`).
- `templates/care_plan/_plan_slot_toggle.plush.{html,fr,de,nl}.html` — a
  non-undoable applied med slot (`!slot.CanUndo`, fulfillment not
  treatment-backed) stays disabled but its title now EXPLAINS via
  `care_plan.status.undo_locked` instead of being silent.
- `locales/care_plan.{en-us,fr,de,nl}.yaml` — new keys
  `care_plan.unapply.confirm_title/.confirm_expect/.confirm_submit` and
  `care_plan.status.undo_locked` (submit labels deliberately distinct from
  the dismiss "Cancel/Annuler/…" — no cancel/cancel collision).

### Regression tests (actions/)

- `care_plan_bug22_toggle_test.go`: applied branch renders the undo toggle
  + separate view link (not a done-link replacement); every partial with
  `.plan-apply-btn`/`.plan-feeding-one` has a d-none `.plan-unapply-btn`
  sibling in the same parent; the toggle JS carries the confirm modal and
  no `scheduleAnimalConvergeReload`; unapply modal labels exist in all 4
  locales; batch button flips to group undo; locked med slot carries the
  explanation.

### Browser E2E (2026-10-28, agent-browser) — ✅ PASS

- `/care_plan?kind=feeding&group=cage`: cage batch apply → group button
  stays visible, flips to `btn-success plan-cage-undo` ✓, per-animal chips
  hide ○ and reveal green ✓ undos; group undo → `#planUnapplyModal`
  confirm → every chip back to ○ and the group button back to its tier ○.
- `/care_plan?kind=care`: single apply flips in place to a green ✓ undo;
  undo → confirm modal → back to ○ (no reload).
- `/care_plan?kind=medication`: ○ → ✓ in place; ✓ → instant undo back to
  ○ (accepted no-confirm behaviour).
- `/animals/{id}#nav-plan`: apply/undo run in place; no
  `window.location.reload` fires (beforeunload listener stayed at 0).

### Follow-up 2026-10-28 (user request) — per-kind DONE tier ("Create a DONE view that follow exact table and keep current historique")

Each ACTIVE kind tab on `/care_plan` now renders its terminal (applied)
occurrences as a collapsed `badge-success` `.plan-tier` in the kind's OWN
table format — the generic Historique panel (`id="plan-history"`) is
untouched:

- **feeding** → `feed-tier-done`: cage×diet groups via `_plan_tier_feed_table`
  (DoneMode), group-level ✓ undo-all (`.plan-cage-undo`, `data-items` refs) +
  per-animal chips rendered as the BUG-22 pair PRE-FLIPPED (d-none ○
  `.plan-feeding-one` + visible ✓ `.plan-unapply-btn`). A fallback block
  renders the feeding section when only done work remains.
- **cleanup** → `care-tier-done`: cage rows via `_plan_tier_care_table`/
  `_plan_care_line` + `_plan_care_slot` (applied slot = pre-flipped pair).
- **row kinds** (observation/care/weighing) → `tier-done`: compact
  `_plan_item_line` lines (d-none apply + visible ✓ undo + separate
  `plan-item-view` link), most-recent-first.
- **medication** → unchanged: the pre-existing MedDone block
  (`fillDoneViews` is a deliberate no-op there — no duplicate tier).

ViewModel: `DayPlanView.DoneRows/FeedDone/CareDone/DoneCount/DoneCountCap` +
`fillDoneViews`/`doneFeedGroups`/`doneCareGroups` in
`actions/care_plan_viewmodel.go` (`DoneMode bool` on `FeedingGroupView` and
`CareView`). Single-animal cage rows keep the accepted Bug 2026-10-27 #5
design (no per-animal toggle — the group undo-all covers the row). Done-tier
undo flips the occurrence back to ○ locally; the row leaves the tier on the
next natural render. Phase0b DOM baselines re-recorded with the delta
documented in `care_plan_phase0b_dom_test.go`.

Regression tests: `actions/care_plan_done_tier_test.go` (5 tests —
per-kind tier presence, pre-flipped pair sibling presence + order, group
undo-all without to-do glyph classes, medication MedDone no-leak, tier
scoping across tabs, Historique retention on every tab).

Browser E2E (2026-10-28, agent-browser, en locale): feeding tab —
`feed-tier-done` collapsed badge-success, 5 group undo-all buttons, chips
paired (same parent, ○ d-none / ✓ visible); **live undo click** →
`#planUnapplyModal` (animal + detail filled, submit "Yes, undo it") → chip
flipped ✓→○ in place, no reload (page marker survived). cleanup tab —
`care-tier-done` with group undo-all + applied slot pairs. observation/
medication tabs empty in dev data (fixture-backed Go tests pin those
renders); no cross-tier leaks; Historique present on every tab.


### Template/JS audit result (2026-10-28)

Complete inventory of every `.plan-apply-btn` / `.plan-feeding-one` /
`.plan-med-slot` rendering site vs. its undo affordance:

| Surface | Partial | Pair contract | Verdict |
|---|---|---|---|
| Item merged occurrences (care/obs/weighing) | `_plan_item_line` `for slot in card.Slots` | apply+undo siblings in `.plan-item-slot` | ✅ OK |
| Item single occurrence, open | `_plan_item_line` `card.Applicable` | apply+undo+skip+defer in `.plan-item-slot` | ✅ OK |
| **Item single occurrence, APPLIED** | `_plan_item_line` `card.Status=="applied"` (L219–232) | **green `done` badge-LINK, no undo** | ❌ **FIX: render ✓ undo toggle + view-link sibling** (this is also the animal `#nav-plan` renderer — `animals/show.plush.html:550`) |
| Cleanup slots (cage + animal groups) | `_plan_care_slot` | apply+undo in `.plan-item-slot` | ✅ OK |
| Feeding per-animal chips | `_plan_tier_feed_table` L160–169 | apply+undo in one span | ✅ OK (note: undo chip lacks `data-kind`/`data-tier-class` — harmless, colour hardcoded green) |
| **Feeding/cleanup CAGE-LEVEL batch** | `index.plush.html` `flipBatchRow` | chips flip ✓, but group button `d-none` with **no group undo** | ❌ **FIX: keep visible as green ✓ group-undo** (undo-all loop + flip back) |
| Med slots | `_plan_slot_toggle` + `_plan_med_toggle` JS | one button flips in place ○↔✓ | ✅ OK (CanUndo=false → disabled ✓; keep + title hint) |
| Animal-page convergence | `_apply_toggle` `scheduleAnimalConvergeReload` | **forces `window.location.reload()` ~1.2 s after every flip on `#nav-plan`** | ❌ **FIX: remove** (directive: no refresh; the in-place flip is the source of truth) |

Unapply confirmation: user allows one — use the SHARED `#planMessageModal`-style
modal (new `#planUnapplyModal`), not `window.confirm`; med slots keep their
no-confirm instant undo (existing accepted behaviour), pair family gains the
confirm modal (it deletes the fulfillment record).
**Severity:** High (core workflow: one-click apply must flip to a green
undoable state; today the button vanishes on several surfaces)
**Found by:** User report 2026-10-28 —
`/care_plan/?kind=feeding&group=cage` (cage-level action click → button
disappears) and `/animals/10352#nav-plan` (same on the animal plan tab).
User directive: *"the buttons are toggles and should allow the user to
untoggle — a confirmation may be asked — avoid refresh as much as possible —
the 'done' link must NOT replace the toggle button"*; E2E validation required
(incl. "the associated care must undo the state").

### Root-cause analysis (code review 2026-10-28)

1. **Pair-swap requires a sibling the template does not always render.**
   `_apply_toggle.plush.html` `setToggleState(btn, true)` hides the clicked
   `.plan-apply-btn` (`d-none`) and reveals the `.plan-unapply-btn` found via
   **`btn.parentNode.querySelector('.plan-unapply-btn')`**. If the clicked
   button's parent contains no undo sibling, the apply button is hidden and
   nothing appears → **the button "disappears"** (the reported symptom).
   Suspects to audit: the cage-level (`group=cage`) cleanup/feeding lines in
   `templates/care_plan/_plan_care_line.plush.html` and the group-chip
   partials — any branch that renders an apply control WITHOUT a d-none
   `.plan-unapply-btn` sibling inside the SAME `.plan-item-slot` parent.
2. **Applied single-occurrence cards render a "done" LINK, not a toggle.**
   `templates/care_plan/_plan_item_line.plush.html` (single-occurrence
   branch, `card.Status == "applied"`) renders
   `<a class="badge badge-success plan-item-view" href="<FulfillmentLink>">done</a>`
   — a navigation link with **no undo affordance at all**. This is exactly
   the forbidden "'done' link replaces the toggle button" shape. The same
   pattern must be audited in `_plan_tier_rows`, `_plan_care_line`,
   dashboard and animal-tab partials.
3. **Applied MED slots are undoable only when treatment-backed** —
   `_plan_slot_toggle.plush.html` disables the ✓ when `!slot.CanUndo`
   (`CanUndo` requires a non-deleted treatment fulfillment). Directive says
   every toggle must untoggle (a confirmation is acceptable): the disabled
   ✓ needs a defined behaviour (confirm → server refuses with a message, or
   allow undo when the fulfillment is gone by detaching instead of deleting).
4. **`scheduleAnimalConvergeReload()` forces a full reload ~1.2 s after every
   flip on the animal page** (`window.location.reload()`). Directive: avoid
   refresh — the in-place flip must be the source of truth; the reload can
   only remain as a last-resort convergence for server-rendered siblings, or
   be replaced by targeted re-render.

### Expected

- Every open occurrence renders an apply toggle; clicking it records the
  application and the SAME button position shows a green ✓ toggle
  (med family: same element flips; pair family: the sibling swap is
  guaranteed by construction).
- Clicking the green ✓ asks a confirmation (shared modal, not
  `window.confirm`) and on confirm calls `/care_plan/unapply`
  (`delete_fulfillment: true`) → the toggle flips back to ○ **without a page
  reload**; the associated care/treatment/feeding record is removed
  (fulfillment deleted) and the plan state reflects it.
- A "view record" link may exist NEXT TO the toggle (sibling), never
  INSTEAD of it.
- No full-page reload on the care plan, dashboard, or animal plan tab.

### Acceptance criteria (retest — browser E2E, agent-browser)

1. `/care_plan?kind=feeding&group=cage`: click a cage-level action → button
   becomes green ✓ in place; click again → confirm → back to ○; the care
   record created by the apply is gone (animal page Care tab / DB).
2. `/care_plan?kind=medication`: ○ → ✓ in place; ✓ → confirm → ○; the
   created treatment row is removed; tooltip shows the full date+time
   (BUG-21).
3. `/animals/{id}#nav-plan`: same flip/undo on protocol, treatment, feeding
   entries; NO `window.location.reload` fires (watch the tab: no reload
   flash; `performance.navigation`/`window.planActionAge` sanity check).
4. Repeat for kind=care, kind=cleanup, kind=weighing (modal input),
   kind=observation (modal input): the modal path flips in place too.
5. All four locales render the same behaviour (templates share the JS; spot
   check fr + en).

---

## BUG-23 (high) — Deleting a care does not retrigger schedule recalculation

**Status:** ✅ FIXED 2026-10-28 (code + unit test + browser E2E 2026-10-28)

### Fix applied 2026-10-28

- `actions/cares.go` Destroy: after `InvalidateWeightLossCache()` it now
  also runs `queuePostCommitInvalidation(c, InvalidateDayPlanBadgeCache)`
  — the landing day-plan open/late badge refreshes after a care deletion,
  the same invalidation the apply/unapply paths already queued.
- Weight-derived dosage has no stale-weight cache: `lastWeight()` queries
  the DB per plan build, so the next `/care_plan?kind=medication` render
  after deleting a weighing care picks the previous weight automatically.

### Regression tests (actions/care_plan_bug23_destroy_test.go)

- `TestBug23CareDestroyInvalidatesDayPlanBadgeCache`: primes
  `CountOpenItemsCached`, DELETEs a care via HTTP, asserts
  `dayPlanBadgeCache.valid` is false (the badge recomputes on next hit).
- `TestBug23WeightedCareDestroyInvalidatesWeightLossCache`: a weighted care
  DELETE zeroes `WeightLossCachedAt()`.

### Browser E2E (2026-10-28, agent-browser) — ✅ PASS

- `/cares` delete (data-method=DELETE) → flash "détruit avec succès", row
  gone; landing `a.badge[href="/care_plan"]` re-renders live ("Plan du
  jour: 846"). Deleting a PAST fulfillment marks the application
  `fulfillment_deleted` (§10-CP1), so the open count is unchanged — the
  cache-invalidation regression is pinned by the unit tests above.

### Browser E2E (2026-10-28, second run — delete-then-rerender) — ✅ PASS

- Applied a weighing (care 155 g created, application `applied`) → landing
  badge 850 → deleted the care from the animal Care tab ("Are you sure?" →
  accept) → care row `COUNT=0` in DB, application now
  `status=applied, fulfillment_deleted=1` → the VERY NEXT landing render
  shows the badge consistent with the live application state (no frozen
  stale count — the post-commit `InvalidateDayPlanBadgeCache` fired).
- The day-plan row itself keeps the occurrence applied-with-deleted-
  fulfillment by design (`fulfillment_deleted` flag): the application
  record, not the care row, owns the applied state; the badge counts plan
  occurrences, so 850 both before and after the delete is CORRECT, not a
  stale cache. The regression the bug targeted (badge not recomputed after
  Destroy) is pinned by `TestBug23CareDestroyInvalidatesDayPlanBadgeCache`
  and `TestBug23WeightedCareDestroyInvalidatesWeightLossCache` (both PASS).

**Severity:** High (stale plan: after removing a care, occurrences that
depended on it — e.g. weight-derived dosages, feeding schedules — stay as
computed with the deleted record until some other event rebuilds them)
**Found by:** User report 2026-10-28 — "removing/deleting a care *must*
retrigger the schedule but currently does not!"

### Investigation pointers

- `actions/cares.go` Destroy handler: check which invalidations run
  post-commit — the landing badge cache (`CountOpenItemsCached`) and the
  weight-loss cache are invalidated on apply paths
  (`docs/performance-assessment-2026-10-04`); the Destroy path likely skips
  them.
- Weight-dependent dosage: `medSlotFor`/dosage resolution reads the LATEST
  weight; deleting a weighing care must make the next plan build pick the
  previous weight (the build itself is per-request, so the bug is most
  likely a cache: latest-weight lookup index `(animal_id, weight, date)`,
  weight-loss cache, or the plan-level assembly reusing a cached weight).
- Also check: care Destroy must re-publish nothing to the console (cares are
  excluded from the state hash by design) — only LOCAL caches/schedules are
  concerned.

### Expected

Deleting a care (any kind) invalidates every derived/cached artifact the
same way the apply/unapply paths do; the very next plan render (and landing
badge) reflects the removal without manual cache flushes.

### Acceptance criteria (retest)

1. Create a weighing care (weight W2) on an animal whose medication dosage
   is weight-derived → plan shows dosage from W2; delete the care → the next
   `/care_plan?kind=medication` render computes the dosage from the previous
   weight W1 (no stale W2).
2. Landing "open items" badge updates immediately after the delete.
3. Unit test: cares Destroy invalidates the same caches as the unapply path
   (regression test next to the weight-loss cache tests).

---

## BUG-24 (medium) — Cleanup multi-animal cage row: expanded collapsible packs several animals on ONE line

**Status:** ✅ FIXED 2026-10-28 (code + browser E2E 2026-10-28)

### Fix applied 2026-10-28

- `templates/care_plan/_plan_care_line.plush.{html,fr,de,nl}.html`: the cage
  multi-animal container is marked `plan-time-group plan-time-group-animals`
  (only when `group != "animal"`).
- `assets/css/care-plan.scss` AND the inline `<style>` of
  `index.plush.{html,fr,de,nl}.html`:
  `.plan-care-slots .plan-time-group.plan-time-group-animals { display: block; }`
  — overrides the `display:flex` meant for the group=animal label+slots
  layout, so each `.plan-animal-row` stacks on its own line.

### Browser E2E (2026-10-28, agent-browser) — ✅ PASS

- `/care_plan?kind=cleanup`: 20 multi-animal `.plan-time-group-animals`
  containers, 111 `.plan-animal-row` lines; every expanded group stacks its
  animals one per line (strictly increasing row tops) at 1600px AND 900px
  viewport. group=animal rows unchanged (single animal + its toggles).

### Browser E2E (2026-10-28, second run) — ✅ PASS

- `/care_plan?kind=cleanup&group=cage`: expanded the VE27 cage (29 animals)
  — the `.plan-time-group-animals` container computes `display: block` and
  the visible `[data-animal-id]` chips sit at strictly increasing vertical
  positions (~39 px apart: 333, 372, 411, 450, 489…), i.e. exactly ONE
  animal per line. No two animal numbers share a row.

**Severity:** Medium (readability/scanability: the expanded cage row is an
unparseable clump of animal numbers + toggles on a single wrapped line)
**Found by:** User report 2026-10-28 —
`/care_plan/?kind=cleanup` (multi-animal cage rows):
*"the collapsible should contain 1 animal+button per line — avoid grouping
on a single line"*.

### Repro

`/care_plan/?kind=cleanup`, a cage with >1 animal owing the same cleanup:
click the row's collapsible header (count badge) → the expanded list shows
`472/26 ○ 09:00  513/26 ○ 09:00  601/26 ○ 09:00 …` flowing side by side on
one wrapping line instead of one `animal — toggles` line per animal.

### Root cause (code review 2026-10-28)

`templates/care_plan/_plan_care_line.plush.html` (bug 2026-10-27 #5 branch)
renders the per-animal lines as a loop of
`<div class="plan-animal-row d-flex flex-wrap …">` **directly inside**
`<div class="plan-time-group">`.

The stylesheet (`care-plan.scss`, also inlined in the DOM baseline) sets

```
.plan-care-slots .plan-time-group { display: flex; align-items: center;
  justify-content: space-between; gap: 1rem; }
```

— a rule meant for the group=animal layout (one `plan-time-label` + one
`plan-care-slot-row` side by side). In the multi-animal CAGE branch the
`.plan-animal-row` divs become **flex items** of that container → they lay
out horizontally on one (wrapping) line. The per-animal divs are already
correct ("1 animal + its toggles"); the parent flex is what destroys the
one-per-line structure.

### Expected

Inside the expanded collapsible: ONE line per animal —
`472/26  [○ 09:00]` / `513/26  [○ 09:00]` — each `.plan-animal-row` on its
own block line, right-aligned toggles allowed but animals stacked.

### Fix sketch (next session)

Template: mark the cage multi-animal container, e.g.
`<div class="plan-time-group plan-time-group-animals">` (AnimalLines
branch only).
CSS: `.plan-care-slots .plan-time-group.plan-time-group-animals { display: block; }`
(the inner `.plan-animal-row`s keep their own flex). Keep the base flex
rule for the group=animal label+slots layout it was written for.
Apply to all four locale forks of `_plan_care_line` (verify parity — the
forks share this structure).

### Acceptance criteria (retest — browser E2E)

1. `/care_plan/?kind=cleanup` (fr + en): expand a multi-animal cage row →
   every animal occupies its own line with its toggle(s); no two animal
   numbers share a line at any viewport width (check 1600 px and 900 px).
2. group=animal rows are unchanged (label + slots on one line there is by
   design).
3. The per-animal toggle still flips ✓/○ in place (BUG-22 contract).

---

## BUG-25 (low) — `/care_matchers` does not list "Colombidés bébé" (derived matchers hidden by design, but invisible and undiscoverable)

**Status:** ✅ FIXED 2026-10-28 (user ruling: candidate fix 1, widened —
derived matchers are full library members; verified by browser E2E
2026-10-28 — "/care_matchers" lists "Colombidés bébé" with usage count
1, also present in the rule-editor matcher select; 4 locales render.
NOTE: the grey "Derived" badge introduced here was removed again by
BUG-27 — singling out derived matchers adds no value; the used-by count
is now the single, uniform affordance for every matcher)
**Severity:** Low (no functional break — the matcher works and feeds rule
SR4; the admin simply cannot SEE or manage it from the library page)
**Found by:** User report 2026-10-28 —
`http://localhost:3000/care_matchers`: *"does not list 'Colombidés bébé'
but it seems to exist? why?"*

### Answer (root cause, code review 2026-10-28)

The matcher **exists and works** — it is one of the four seed-internal
composite matchers flagged `derived = 1`:

- `actions/care_plan_seeds.go` SeedMatchers: `{Key: "SR4*", Name: "Colombidés bébé", Derived: true,
  Expression: animal_type = "Colombidés" AND animal_age = "bébé"}`
  (siblings: `SR1*` Hérisson bébé < 300 g, `SR6*` Canidés bébé,
  `SR12*` Hérisson bébé ou juvénile).
- `actions/care_matchers.go` `CareMatchersResource.List`:
  `tx.Where("derived = 0").All(matchers)` — the library page **deliberately
  hides derived matchers** (item 9, 2026-10-07, `models/care_matcher.go`:
  *"It is a building block, not library content — derived matchers are
  hidden from /care_matchers and the matcher dropdowns while the rules
  referencing them keep evaluating."*).

Why they exist at all: the `care_rules` schema references ONE matcher, so
the composite seed rules (SR1/SR4/SR6/SR12) inline their two-part condition
into a dedicated matcher row instead of extending the schema. Rule
**SR4 "Colombidés bébé — NB gavage"** (feeding, NB 1/2 50 ml, force-feed,
08:00/11:00/14:00/17:00) evaluates through `SR4*` — so the schedule using
"Colombidés bébé" is fully operational even though the library page does
not show the matcher.

### The actual bug (UX gap)

The hiding is by design, but the page gives **no hint** that hidden
matchers exist, why, or where they are used — an admin looking for the
matcher behind rule SR4 concludes the data is broken. Additionally a
derived matcher can only be edited by knowing its URL/ID (JSON Show/Edit
routes exist), never from the UI.

### Candidate fixes (pick one)

1. **Show derived matchers, badged**: drop the `derived = 0` filter and
   render derived rows with a muted "internal — used by rule SR4" badge
   (and read-only or guarded edit), keeping them out of the rule-editor
   dropdowns only.
2. **Keep hidden + explain**: keep the filter but add a page note
   ("N internal matchers hidden — they back seed rules SR1/SR4/SR6/SR12")
   with a toggle to reveal them.
3. **Status quo + doc**: document the behaviour in the admin guide only
   (weakest — the confusion repeats).

### Acceptance criteria (browser E2E)

- `/care_matchers` either lists "Colombidés bébé" with a clear internal
  badge, or explains on-page how to reveal it; the admin can reach its
  detail/expression without guessing an ID.
- Rule SR4 still evaluates (a baby pigeon intake still generates the
  4×/day gavage plan) — regression check.
- All 4 locales render the chosen UI.

### Fix (2026-10-28, user ruling)

User ruling: *"Hiding a matcher does not bring actual value — if it does,
explicit. Fully visible; add the number of usages on the matcher list;
only admins may update, so that is enough protection."*

Implemented (widened candidate fix 1 — derived matchers are full library
members, also in the rule-editor select):

- `actions/care_matchers.go` `List`: `derived = 0` filter dropped; each
  row now carries `UsedBy` (rules referencing it, via new
  `matcherUsageCounts`); list ordered by name.
- `models/care_matcher.go`: new non-db field `UsedBy int \`db:"-"\``.
- `actions/care_rules.go`: `derived = 0` filter dropped from the rules-list
  matcher dropdown AND from `setRuleContext` (the "except own matcher"
  special case is gone — every matcher is listed).
- `templates/care_matchers/index.plush.{html,fr,de,nl}.html`: new
  "Used by rules" column + grey **Derived** badge (tooltip explains the
  edit coupling) in the name cell.
- `locales/care_plan.{en-us,fr,de,nl}.yaml`: `care_plan.field.used_by`,
  `care_plan.matchers.derived_badge`, `care_plan.matchers.derived_hint`.

**Retest:** ✅ browser E2E 2026-10-28 — `/care_matchers` lists
"Colombidés bébé" (usage count 1, see BUG-27 for the badge removal); rule
SR4 edit select unchanged in behaviour; 4 locales render.

---

## BUG-26 (medium) — `/care_plan`: urgency bubbles sit OUTSIDE the tabs, are missing for feeding/cleanup, and no tab has a "Done" bubble

**Status:** ✅ FIXED 2026-10-28 (code + phase0b baselines re-recorded with
verified diff + browser E2E 2026-10-28: all 6 kind tabs — strip inside
`#planSections` (1 instance, after `.nav-tabs`, 0 above), bubble order
late·now·later·done, bubble counts == tier header counts, click expands
& scrolls to the target tier (hash jump verified), feeding has no private
duplicate; observation/weighing render no tables → no strip, correct per
R4-7.21; locales en/fr/de/nl verified — e.g. fr "En retard 29 / Plus
tard 5", de "Überfällig 29 / Später 5", nl "Te laat 29 / Later 5")
**Severity:** Medium (wrong affordance on the day's main work screen: the
summary reads as page metadata instead of the active tab's index, and the
information set changes per tab without reason)
**Found by:** User report 2026-10-28 —
`http://localhost:3000/care_plan?kind=medication&group=cage` (and other
kinds): *"the bubble 'En retard 29' / 'Plus tard 5' should be within the
tab; ALL tabs should have all bubbles matching all tables (late, now, to
do, done), same logic."*

### Observed

1. **Placement** — the urgency bubbles (`planSummaryStrip`,
   `view.TierLinks`) render at the TOP of the page ABOVE the kind tab bar
   (`#planKindTabs`) for medication/care/weighing/observation. They index
   the ACTIVE tab's sections, but their position makes them read as
   page-level metadata. Feeding renders a second strip INSIDE its tab
   content (`#feedingTabContent`) — two different placements for the same
   component.
2. **Missing bubbles per tab** — the strip is gated
   `if (view.Kind != "feeding")`, and feeding only shows its strip when
   open feeding work exists (`len(view.Feedings) > 0`); cleanup renders
   no strip at all. So depending on the tab the bubbles are above the
   tabs, inside the tab, or absent.
3. **No "Done" bubble anywhere** — `tierLinks()` only emits late/now/later
   (3 keys). Yet every tab renders a fourth "Done" table when applied work
   exists (medication `MedDone`, feeding `FeedDone`, cleanup `CareDone`,
   row kinds `DoneRows`) — not represented in the strip, and not
   expandable by the strip's click handler.

### Root cause

- `templates/care_plan/index.plush.html`: summary strip placed before
  `#planKindTabs` and gated on kind; feeding keeps its own copy.
- `actions/care_plan_viewmodel.go` `tierLinks()`: iterates only the 3
  open tiers; no done entry.

### Fix (guideline §1.4/R4-7.21: strip = index of the rendered tables,
same unit, same order as the tables on the page)

1. ONE strip, rendered INSIDE `#planSections` (i.e. within the active
   tab's content), for EVERY kind, before the tier panels; feeding's
   private strip removed.
2. All FOUR bubbles whenever their table exists — late · now · later ·
   done, always in this order — with the same per-kind unit as the tier
   headers (open applicable occurrences; done count for the done table).
   Bubbles whose table does not exist are omitted (strip never
   contradicts the page, R4-7.21).
3. The strip click handler scopes to `#planSections` for every kind and
   also expands the (default-collapsed) Done tier when its bubble is
   clicked.

### Acceptance criteria (browser E2E)

- Every kind tab (feeding, medication, care, cleanup, weighing,
  observation) shows the bubbles INSIDE the tab content, order
  late → now → later → done, counts equal to the tier headers.
- Clicking the Done bubble expands/scrolls to the Done table on every tab
  that has one.
- No strip above the tab bar on any tab; no duplicate strip on feeding.
- All 4 locales render identically (templates share the same markup).

## BUG-27 (medium) — `/care_matchers`: the "Dérivé" badge singles out derived matchers for a coupling that applies to ALL matchers; the "used by" count is not actionable

**Status:** ✅ FIXED 2026-10-28 (badge removed; used-by count is now a
clickable dropdown listing every referencing rule with show/edit links;
verified by browser E2E 2026-10-28 — no "Dérivé" badge, dropdown opens
listing rules with translated names/kind/inactive marker, show
("/care_rules/{id}") and edit ("/care_rules/{id}/edit") links both
reachable, 0 not clickable, identical in en/fr/de/nl; JSON API still
returns `used_by` + `used_by_rules`)
**Severity:** Medium (misleading affordance on an admin authoring screen:
the badge implies only derived matchers propagate edits to rules, which is
false — editing ANY matcher re-evaluates every rule referencing it)
**Found by:** User report 2026-10-28 —
`http://localhost:3000/care_matchers`: *"Derivé still does not make much
sense — any matcher modification should change all rules that are using
the matcher — it still does not make sense to 'only' explain this for
'derived' — still unclear of the actual meaning of it. The 'Utilisé par
des règles' should show a number — on click — it should open a tooltip
with the list and details of the rules + option to jump to the edit/view
of the rules."*

### Observed

1. The name cell of derived matchers carries a "Dérivé" badge whose hint
   ("editing it affects every rule using it") describes a property of
   EVERY matcher, not just derived ones. Singling out derived rows makes
   the badge's meaning unreadable (what does "derived" itself even mean
   for an admin?).
2. The "Utilisé par des règles" column renders a bare number. The admin
   cannot see WHICH rules reference the matcher without opening each rule
   one by one — yet that list is exactly what you need before editing or
   deleting a matcher (BUG-28 shows the delete guard needs it too).

### Root cause

- BUG-25 fix added the derived badge as a warning; the warning text
  describes generic matcher→rule coupling, not a derived-specific
  property. The `derived` column is seed provenance (composite seed
  rules SR1*/SR4*/SR6*/SR12* inlined into matcher rows) — internal
  bookkeeping, not user-facing information.
- The usage query (`matcherUsageCounts`) returns only counts; the
  template never received the referencing rules.

### Fix (guideline: visibility + explicit information, not concealment;
admin-only authoring is the protection)

1. **Badge removed** from the index name cell in all 4 locales. The
   `derived` DB column stays (seed provenance, internal). The generic
   coupling is now visible for EVERY matcher through the same mechanism:
   the used-by count.
2. **Used-by count is a dropdown** (Bootstrap 4 `dropdown`, no JS init
   needed): the number is the toggle; the menu lists every referencing
   rule — translated name, type, inactive marker — each row linking to
   the rule's show page, with an edit (✎) link jumping straight to
   `/care_rules/{id}/edit`. Count 0 renders as a plain number (no empty
   menu).
3. Handler: `matcherUsageCounts` → `matcherUsage` returns
   matcher_id → []CareRuleRef (id, name, outtake_type, active) in one
   query ordered by rule name; `CareMatcher.UsedByRules` carries the
   list, `UsedBy` stays the count.

### Acceptance criteria (browser E2E)

- `/care_matchers`: no "Dérivé" badge anywhere; every matcher shows its
  used-by number; clicking a non-zero number opens the rule list with
  working show + edit links; 0 is not clickable.
- All 4 locales render identically (templates share the same markup);
  rule names display translated via `tname`.
- JSON API (`Accept: application/json`) still returns the matchers array
  with `used_by` counts and the new `used_by_rules` list.

## BUG-28 (medium) — Deleting a used matcher (and every other planError) returns raw JSON to the browser: no message page, no way back

**Status:** ✅ FIXED 2026-10-28 (`planError` content-negotiated — HTML gets
a translated flash + redirect back, JSON keeps the {code,error} body;
verified by browser E2E 2026-10-28 — HTML DELETE of a used matcher →
303 → back on `/care_matchers` with a red localized alert naming the
count, in en/fr/de/nl; `Accept: application/json` still gets
{"code":409,"error":...}; admin-only 403 gate still hard for HTML)
**Severity:** Medium (dead-end error screen on the main admin flow: the
browser shows a bare JSON blob with no navigation; the user must use the
browser back button to recover)
**Found by:** User report 2026-10-28 — DELETE
`http://localhost:3000/care_matchers/3c38ded6-3015-4c27-bfc3-06bb234a3fab/`
returned `{"code":409,"error":"matcher used by 1 rule(s)"}`: *"This
should be a clear error message and allow the user to go back."*

### Observed

Deleting a matcher that is still referenced by rules (the §7.1-5 usage
guard, correctly rejecting with 409) renders the error as a raw JSON
document in the browser — no flash, no redirect, no link back. The same
helper (`planError`, `actions/care_plan_support.go`) backs ~40 error
paths across care_plan/care_rules/care_matchers/care_animal_plans/
care_rule_exclusions/treatment_time_entries, so EVERY rejected plan
action (validation, not-found, conflict) dead-ends the same way when
hit from an HTML page.

### Root cause

`planError` unconditionally renders `renderJSON({code, error})` — it
never content-negotiates, and the matcher-in-use message itself is a
hard-coded English string, not a translated locale key.

### Fix (guideline: user-facing errors are UI — localized, recoverable)

1. `planError` now content-negotiates: `Accept: application/json` keeps
   the `{code, error}` JSON body at the original status (API contract
   unchanged); HTML requests get `c.Flash().Add("danger", msg)` +
   `303 See Other` redirect to the Referer (fallback `/`), so the user
   lands back on the page they came from with the error displayed as a
   dismissible alert (`_flash.plush.html` already renders `danger`).
2. The matcher-in-use guard message is a locale key
   (`care_plan.matchers.error_in_use`, 4 languages) with the rule count
   interpolated, instead of the hard-coded English string.

### Acceptance criteria (browser E2E)

- Deleting a used matcher from `/care_matchers` (HTML) returns to
  `/care_matchers` with a red alert naming the count of referencing
  rules, in the active locale; no JSON blob, no dead end.
- The same delete with `Accept: application/json` still returns
  `{"code":409,"error":...}` (API contract preserved).
- Other planError paths (e.g. deleting a used rule, invalid plan POST)
  show the same flash-and-back behavior from HTML pages.

---

## BUG-29 (high) — `/care_plan?kind=feeding`: a stale-folded late feeding shows in "En retard" with NO apply button, and its next occurrence never reaches "Plus tard"

**Status:** FIXED 2026-10-12 — feeding promotion + cleanup re-tier
implemented, tests green; awaiting user retest
**Severity:** High (a missed feeding on the day plan is neither recordable nor
does its successor surface as upcoming work — the caregiver sees a red
"En retard" row they cannot act on and the rest of the day's schedule for
that animal is invisible)
**Found by:** User report 2026-10-12 — `/care_plan?kind=feeding`:
*"B1 ACCUEIL gavage panade 200 ml + eau" appears in "En retard" — no
action possible — and does not appear in "Plus tard", even though the
next occurrence is due at 09:00
(cf `/animals/10352?back=%2Fcare_plan%3Fkind%3Dfeeding#nav-plan`).*

**Scope extension (user ruling 2026-10-12, same conversation):** the same
promotion applies to **cleanup** — a late cleanup whose next occurrence is
near (successor nearer than the late is old — the item-8 staleness
distance, e.g. next cleanup < 2 h away) must NOT sit in "En retard": the
row shows in **"Plus tard"** with the next occurrence's toggle, carrying a
**"⏱ N late" badge** for the missed occurrences (record-late / snooze
behind the badge, as today).

### Observed

1. A feeding row (cage B1, zone ACCUEIL, diet "gavage panade 200 ml +
   eau") sits in the **Late / "En retard"** tier.
2. The row carries **no apply affordance**: no per-animal `○ HH:MM`
   toggle, no cage-level group-Apply button — nothing clickable to record
   the missed feeding (the only hint is the `⏱ N` stale badge, which is
   not discoverable as an action and only offers snooze on click).
3. The animal's **next scheduled occurrence (09:00) does not appear in
   the "Plus tard" / Later tier at all** — from the day plan the schedule
   looks like it ends with the missed one, although the animal's Protocol
   tab shows the cadence continuing.

### Root cause (two cooperating mechanisms)

**A. Item-8 stale fold suppresses every apply control on the row.**
When a late occurrence's age exceeds the distance to its next open
sibling (`lateIsStale`, `actions/care_plan_stale_late.go`), the chip is
flagged `StaleLate`:

- `feedingViewOf` (`actions/care_plan_viewmodel.go`) then **excludes it
  from `ApplicableCount`** (`if chip.Applicable && !chip.StaleLate`), so
  the cage-level group-Apply button
  (`templates/care_plan/_plan_tier_feed_table.plush.html`,
  `fcard.ApplicableCount > 0`) is **not rendered**.
- The chip's own toggle is rendered but forced `d-none` inside
  `.plan-stale-slot`, revealed only by clicking the `⏱` badge — and for a
  **single-animal cage row** the per-animal toggle is never rendered at
  all (`len(fcard.Chips) > 1 || group == "animal"` gate →
  `plan-apply-space` spacer), leaving the row with zero visible actions.
- The row still lands in the Late tier (`feedingGroupTier` does not skip
  stale chips), so the net effect is exactly the report: red "En retard"
  row, no button.

**B. Chip dedupe never promotes the successor, so nothing reaches Later.**
`feedingCardChip` (`actions/care_plan_dayplan.go`) keeps **ONE chip per
(source × animal) — the earliest CURRENT occurrence** — and only swaps in
a later occurrence when the existing chip is *superseded*. A stale late
chip is still applicable (its successor is not yet due, otherwise it
wouldn't be stale), hence **not superseded**, so the next occurrence —
already generated in the plan window (visible on the animal's Protocol
tab) — is never promoted to a chip and the card stays anchored in the
Late tier. The Later occurrence survives only as the "+N" `Remaining`
hint, which the feeding template does not even render per chip.

The item-8 design (`care_plan_stale_late.go` header) states the intent
explicitly: *"the stale toggles collapse into one '⏱ N' badge … **and the
next upcoming occurrence renders as the actionable toggle beside it**"*.
Medication series implement that co-display (`hasPendingLateSlot`);
cleanup rows show every occurrence as its own toggle. **Feeding is the
one kind where the "next upcoming actionable toggle beside it" was never
implemented** — chip dedupe makes it structurally impossible today.

**C. Cleanup: the stale fold hides the toggle but keeps the tier.**
Cleanup rows (`CareView`) render every open occurrence as its own toggle
and fold the stale ones behind the ⏱ badge correctly — but
`fillCareTiers` still tiers the row by its most urgent open occurrence
**including the folded stale ones** (tier 0), so a cleanup whose next
occurrence is minutes away still sits in "En retard" with its actionable
toggle hidden. The row must be tiered by its most urgent NON-stale open
occurrence (→ "Plus tard") while the "⏱ N" badge keeps the missed ones
visible on the row.

### Expected

**B1 shows in "Plus tard"** (user ruling 2026-10-12): when the displayed
occurrence is a stale-folded late, the chip is **promoted to the next
upcoming occurrence** (09:00) — the card lands in the Later tier with a
normal applicable `○ HH:MM` toggle, and the missed occurrence(s) live
only behind the `⏱ N` badge (snooze / record-late one by one), exactly
the item-8 design (*"the next upcoming occurrence renders as the
actionable toggle beside it"*). Concretely:

1. The card's chip for (source × animal) is the next open occurrence,
   not the stale late one — tier, sort and group-Apply all follow that
   occurrence (Late tier keeps the row only via the `⏱` badge's stale
   anchor if product wants the late visible too; the actionable chip
   itself is Later work).
2. No row ever renders in a tier with zero apply controls while open
   work exists (single-animal cage rows included).
3. "En retard, no button, nothing later" is impossible: either the late
   is still actionable (toggle visible) or it is folded and the
   successor is shown.
4. **Cleanup** (user ruling): a cleanup row whose late occurrences are
   all stale-folded (successor nearer than the late is old) renders in
   **"Plus tard"**, tiered/sorted by the next occurrence, with the
   "⏱ N" badge on the row showing how many were missed.

### Acceptance criteria (retest)

1. Reproduce with a feeding schedule whose last occurrence is stale-late
   (age > distance to next due, successor not yet due — e.g. cadence 4 h,
   lookahead 30 min, check between the two): the B1 row renders in
   **"Plus tard"** with a clickable `○ 09:00` apply toggle (per-animal or
   group); clicking it records the next occurrence (feeding care row
   created). The missed occurrence remains reachable via the `⏱ N` badge
   (snooze / individual record-late).
2. The animal's next occurrence (09:00 in the report) is the one shown;
   the "En retard" tier no longer displays an inert row for it.
3. Multi-animal cage rows behave the same (the per-animal toggle path
   and the group Apply stay consistent — group Apply never silently
   records the folded stale occurrences, item-8 exclusion preserved).
4. All 4 locales render (the `_plan_tier_feed_table.plush.*.html` forks
   are byte-identical by policy — apply the fix to all four) and
   `go test -tags sqlite ./actions/...` passes (incl. the item-8 stale
   fold pins in `care_plan_stale_late` coverage and the round-7 feeding
   order/group tests).

### Assignee notes

- Do not "fix" this by making stale lates applicable again in the batch
  refs — item 8's exclusion of folded occurrences from group apply is a
  deliberate guardrail (a group apply must never silently record
  overtaken occurrences).
- The minimal design-faithful fix is in the feeding card/chip layer:
  when the group's displayed chip is `StaleLate`, promote/render the
  group's next open occurrence as the actionable chip (mirroring
  `feedingStaleNext`, which already computes that successor for the
  badge's muted "→ next" label) — the data is already on the card, only
  the toggle is missing.
- Check the single-animal cage case specifically: it has no per-animal
  toggle at all today, so the group-Apply suppression alone leaves the
  row completely inert.
- Verify the animal Protocol tab (`#nav-plan`) and the day plan agree
  after the change (the protocol tab was the only place the 09:00
  occurrence was visible).

## BUG-30 (high) — `/care_plan?kind=care`: items render far beyond the configured preference windows (8 h future / 8 h late) — missing per-kind preference rows silently mean "no cap"

**Status**: ✅ FIXED + retested 2026-10-12 — G5 full suite green: preference-cap tests (`TestCarePlanLateRecordBounds`, phase0b goldens re-recorded under the seeded 8h caps) validate the per-kind caps now apply.
**Severity**: high — the day plan is the work screen; an unfiltered kind
re-advertises up to 2 days of future work and a full extra day of late
work, burying what is actually due.
**Found by**: user report 2026-10-12 (care tab shows items *much* further
than the configured 8 h future preference; requested a review of all
tabs).

### Observed

With the preferences configured to 8 h future / 8 h late (and now = 60
min), `/care_plan?kind=care` lists rows whose occurrences are due well
beyond now + 8 h (and can list late work older than 8 h). Other tabs may
be capped correctly, depending on which kinds have a row in the
`preferences` table.

### Root cause

`applyPreferenceCaps` + the handler-side filter in `CarePlanIndex`
(actions/care_plan.go) are correct, and round-10 tests pin them — **but
they only apply to kinds that have a row in the `preferences` table**:
`preferencesByKind` returns "missing row = no cap" by design
("Missing rows behave as 'no cap' — the map simply lacks the kind",
actions/preferences.go:61-62). And `PreferencesEnsureSeeded` — the only
code that creates the six per-kind rows with the R9 defaults
(late 8 h / future 8 h / now 60 min) — runs ONLY when an admin opens
`/preferences` (PreferencesResource.List). The day-plan handler never
seeds.

So on any DB that predates the R9 preferences feature, or that was
seeded before a kind was added to `actionKinds`, the un-seeded kind(s)
render with NO cap: every occurrence inside the raw plan window
([today−24 h, today+2 days], §6.1) reaches the view model — feeding,
care/cleanup, medication, weighing, observation alike. The admin may
even have saved 8 h values for the kinds that HAVE rows, which makes the
un-capped kind stand out ("much further than the configured 8 h").

Secondary effect: an uncapped kind also defeats the item-8 stale fold —
with 2 days of successors in scope a missed occurrence is virtually
always "stale" (a nearer sibling always exists), so the Late tier fills
with folded rows instead of honest capped work.

### Expected

1. Every day-plan request applies the configured windows to EVERY kind:
   kinds without an explicit preference row use the R9 defaults
   (late 8 h, future 8 h, now 60 min) — "no row" must mean "defaults",
   never "no cap". A deliberate "no cap" is only possible via an
   explicit NULL row saved from /preferences.
2. All tabs (feeding, medication, care, cleanup, weighing, observation)
   show only occurrences within [now − late_show_hours, now +
   future_show_hours] for open work; terminal (applied/skipped) items
   stay for History regardless.
3. The care tab of the report shows only rows with at least one
   occurrence inside the 8 h windows.

### Acceptance criteria (retest)

1. On a DB with NO preferences rows at all, `/care_plan?kind=care`
   (and every other kind tab) renders only open occurrences within the
   default 8 h / 8 h windows.
2. Deleting one kind's row (simulating the pre-feature DB) and reloading
   the day plan caps that kind with the defaults — no admin visit to
   /preferences required.
3. Saving an explicit empty (NULL) value from /preferences still means
   "no cap" for that kind (seeded behavior preserved).
4. `go test -tags sqlite ./actions/...` passes, incl. the round-10 caps
   tests and the BUG-29 stale-fold pins (the fold must keep working with
   capped item lists).

### Assignee notes

- Fix at the source: call `PreferencesEnsureSeeded(tx)` in
  `CarePlanIndex` before `preferencesByKind(tx)` (idempotent, cheap —
  one EXISTS query per kind only while rows are missing). Alternatively
  make `preferencesByKind` synthesize default rows for missing kinds;
  either way the day plan must never see "no row".
- Review ALL kind tabs on retest, not just care — any kind missing its
  row was uncapped.
- Do not change `applyPreferenceCaps` boundary semantics (at-or-beyond
  the cap leaves; round-10 pins in care_plan_round10_caps_test.go).

---

# Bugs — care-protocol / animal-page run 2026-10-09 (session 2)

Found during the feeding-protocol feature work (converter "à vérifier" confirm,
range+period slot generator, legacy feeding-field removal, tab-link fix) on
branch `feature/care-expert`, dev DB (animal 10372).

## BUG-31 (medium) — Blinking "?"/"!" FontAwesome placeholders on animal page: v4 icon names used but `v4-shims.js` is not loaded

**Status:** OPEN
**Severity:** Medium (visual: animated blinking "?"/"!" SVG replaces the icon; looks broken and distracts on the animal page + feeding banner)
**Found by:** User report on `/animals/10372#nav-care` (pasted the animated SVG); root-caused statically.

### Observed

FontAwesome SVG+JS renders its "missing icon" placeholder (blinking
question/exclamation mark, `ANIMATION_BASE dur 2s`) for every icon whose name it
cannot resolve. The app bundles `@fortawesome/fontawesome-free@5.15.4` and loads
**only** `js/all.js` (`assets/js/application.js:6`) — the v4 compatibility layer
`js/v4-shims.js` is **never** required. Any template still using a FontAwesome
**4** icon name therefore renders the blinking placeholder.

Confirmed broken icon names in templates (FA5 name in parentheses):

| v4 class used | FA5 name | Where |
|---|---|---|
| `fa fa-clock-o` | `fa-clock` (far) | `templates/animals/_feeding_summary.plush.html:21` (feeding banner slot times) |
| `fa fa-cutlery` | `fa-utensils` (fas) | `templates/animals/_feeding_summary.plush.html:16` (feeding banner title) |
| `fa fa-pencil` | `fa-pencil-alt` (fas) | `templates/animals/show.plush.{html,fr,de,nl}.html:1030-1034` (attachment comment save button) |
| `fa fa-repeat` | `fa-redo` (fas) | `templates/config/sync_edit.plush.{html,fr,de,nl}.html:59` (retry-undeliverable button) |

(`clock-o` is not even in the FA5.15 shim table; `cutlery`/`pencil`/`repeat` are
shimmable but the shim file is not loaded. Verified against
`node_modules/@fortawesome/fontawesome-free/metadata/{icons,shims}.yml` and
`grep -rn "v4-shims" assets/ templates/` → no match.)

Note the CSS (`all.min.css`) is *also* loaded, so some of these may show as
webfont glyphs in browsers where the SVG+JS replacement runs after; the
placeholder appears wherever SVG+JS processes the `<i>` first. Fix the names,
do not rely on the CSS fallback.

### Expected

No blinking placeholders anywhere. Replace the v4 names with the FA5-free names
above (keep the same `fa`/add `fas`/`far` prefix per FA5 style rules — `clock`
is regular-style `far`, the others solid `fas`) in **all four locale variants**
of each file, then reload the animal page and the sync-edit page and confirm the
real icons render (no console "Could not find icon" warnings).

### Acceptance criteria (retest)

1. `/animals/<id>` feeding banner shows the utensils + clock icons (no blink).
2. Attachment comment save button shows the pencil icon on all 4 locales.
3. `/sync_targets/...` edit page retry button shows the redo icon on all 4 locales.
4. `grep -rn "fa-clock-o\|fa-cutlery\|fa fa-pencil\b\|fa fa-repeat\b" templates/` → no matches.
5. Browser console shows zero FontAwesome "Could not find icon" warnings on those pages.

---

## BUG-32 (low) — `TestAnimalFormLegacyFeedingFrozenAllLocales` fails: `/animals/new` body contains `#nav-plan`

**Status:** ✅ FIXED 2026-10-12 — `go test ./actions -run TestAnimalFormLegacyFeedingFrozenAllLocales` PASS; full suite green.
**Severity:** Low (test failure on `feature/care-expert` after the legacy feeding-field removal; behavior itself needs checking — a "new" animal has no Protocol tab)
**Found by:** `go test ./actions -run TestAnimalFormLegacyFeedingFrozenAllLocales` (fails at `actions/animal_feeding_freeze_test.go:137`, `should not contain "#nav-plan"`).

### Observed

After removing the legacy feeding edit block from
`templates/animals/_form.plush.{html,fr,de,nl}.html` (feeding is now managed
only via care plans), the new-animal form (`/animals/new`) still emits the
string `#nav-plan` somewhere — most likely the `_feeding_summary.plush.html`
hint link (`/animals/<id>#nav-plan`) or another template partial rendered in the
care tab. On `/animals/new` the animal has **no id and no Protocol tab**, so
the link/hint must not render at all.

### Expected

Decide the correct behavior: on `/animals/new` the feeding summary partial
should either render nothing, or render its `animal.ID == 0` text-only branch
with no `#nav-plan` reference. Then update either the template (guard the hint
link on persisted animals only) or the test assertion, and rerun.

### Acceptance criteria (retest)

1. `go test ./actions -run TestAnimalFormLegacyFeedingFrozenAllLocales` passes.
2. `/animals/new` renders no dead `#nav-plan` link; `/animals/<id>/edit` keeps
   the working hint link to the Protocol tab.

---

## BUG-33 (enhancement) — "Nouveau protocole" editor: multi-tab input + clear, translated, field-level error indication

**Status:** ✅ FIXED 2026-10-12 (committed on `feature/care-expert`) — translated, field-level error contract implemented in the shared editor (`templates/care_plan/_editor.plush.html`): offending field flagged `is-invalid`, messages fully translated in fr/en-us/de/nl, no raw engine text (BUG-39 fix), no `§` references. E2E-qualified: E2E-PROTOCOL-EDITOR.md P1–P7 + 9.x rows ✅ (G5, 2026-10-12).
**Severity:** Enhancement (usability of `templates/care_plan/_editor.plush.html` + create/update error paths in `actions/care_animal_plans.go`)
**Found by:** User UX review of the protocol editor.

### Requested

1. **Multi-tab layout** in the plan editor (e.g. *Général* — name/kind/active;
   *Horaires* — slots + the new range+period repeat generator; any other field
   groups) so each section is easy to find and fill.
2. **Field-level errors**: on validation failure (e.g. missing name), the
   offending field must be visibly highlighted and the editor must switch to
   the tab that contains it.
3. **Translated, self-explanatory messages**: every validation error must come
   from `locales/care_plan.{fr,en-us,de,nl}.yaml` and read like
   *"Le nom est obligatoire"* — never a raw Go/JSON error, never references
   like "§8.1" / "BUG-13" / schedule-field indexes that only make sense with
   the spec open.

### Acceptance criteria (retest)

1. Saving a protocol with an empty name redisplays the editor, opens the tab
   containing the name field, marks the field (e.g. `is-invalid`) and shows the
   translated message next to it, in all 4 locales.
2. Errors on schedule fields (bad time, empty slots) behave the same on the
   Horaires tab.
3. No error string rendered to the user contains `§`, `BUG-`, a field path, or
   English when the UI locale is fr/de/nl.
4. `go test ./actions` (care-plan suite) passes with new/updated error-contract
   tests.

---

## BUG-34 (medium) — In-page `#nav-plan` links dead after manual tab switch (FIXED)

**Status:** FIXED 2026-10-09 (committed on `feature/care-expert`)
**Severity:** Medium (navigation: the feeding-banner hint link
`http://localhost:3000/animals/<id>#nav-plan` did nothing once the user had
clicked another tab)
**Found by:** User report: *"Links do not always work … link in #nav-care only works if the user didn't use tab to switch."*

### Root cause

Bootstrap 4 tab activation from `location.hash` ran **once at page load**;
clicking an in-page `href="#nav-plan"` link afterwards only changed the hash —
no tab activation — because no `hashchange` handler existed.

### Fix

Added a guarded `hashchange` handler to `templates/animals/show.plush.{html,fr,de,nl}.html`
and `templates/animals/_form.plush.{html,fr,de,nl}.html`:

```js
$(window).on('hashchange', function () {
  var h = location.hash; if (!h) { return; }
  var tab = $('.nav-tabs a[href="' + h + '"]');
  if (tab.length && !tab.hasClass('active')) { tab.tab('show'); }
});
```

(The `.active` guard prevents re-entry with the click handler that already sets
`location.hash`.)

### Acceptance criteria (retest)

1. On `/animals/10372#nav-care`, click another tab manually, then click the
   `#nav-plan` hint link in the feeding banner → Protocol tab activates.
2. Repeat on the edit form (`/animals/<id>/edit`) and in fr/de/nl locales.

---

## BUG-35 (high) — Timeslot creation in the protocol editor is fragile: free-text time entry silently drops values; repeat generator has no format helpers

**Status:** ✅ FIXED 2026-10-12 (committed on `feature/care-expert`) — slot typing hardened: `normalizeHM` pads `9:00`→`09:00` on change and unparseable values are flagged `is-invalid` (no more silent drops; EN 12 h alt-input path covered by the BUG-38 fix). The free-text repeat generator was replaced by the BUG-36 repeat mode (from/to/every pickers + live occurrence count), which supersedes the generator UX. E2E-qualified: E2E-PROTOCOL-EDITOR.md P4/P9.x ✅ (G5, 2026-10-12).
**Severity:** High (core data entry for the protocol feature; values are lost silently → user believes creation "does not work")
**Found by:** User report on `/animals/10372` "New protocol" modal. Static + in-browser analysis 2026-10-09 (agent-browser, dev DB).

### Investigation facts (browser-verified 2026-10-09)

The editor JS itself is wired: on `/animals/10372` → *New protocol* modal,
"add slot" adds a row and the repeat generator expands 09:00→19:00 every
05:00 into `["09:00","14:00","19:00"]` and the schedule textarea syncs to
`{"times":["09:00","14:00","19:00"]}`. No console errors. So the failure is
**not** wiring — it is silent value rejection and missing input guidance:

1. **Silent drop on format mismatch.** `readSchedule()`
   (`templates/care_plan/_editor.plush.html`) only keeps slot values matching
   `^\d{2}:\d{2}$`. Typing `9:00`, `9h00`, `09h`, `09:0` or a 12 h value
   (`2:00 PM` from the EN flatpickr alt-input) **filters the slot out of the
   saved JSON** with no message — the row stays visible, the schedule silently
   loses it. To the user this reads as "timeslot creation doesn't work".
2. **No format helper on the generator.** `repeatFrom`/`repeatTo`/`repeatEvery`
   are plain free-text inputs (`placeholder="05:00"`) with no flatpickr, no
   mask, no hint text. `every` accepts "H:MM" *or* bare minutes: typing `5`
   means 5 **minutes** → the 24-slot cap fires and the user gets 24 slots
   instead of "every 5 hours". Invalid input silently yields an empty count
   (button does nothing — no error).
3. **Flatpickr vs manual typing conflict.** Slot inputs get flatpickr with
   `allowInput:true`; typed text that flatpickr can't parse leaves the raw
   string, which rule (1) then drops.
4. The generator **replaces** all existing slots without confirmation —
   one misclick destroys manually entered times.

### Expected (per user directive)

Timeslot generation must be easy and guide the user to correct values:

1. Slot entry via a proper time control (flatpickr or `<input type="time">`)
   that cannot produce an invalid format; manual text is normalized
   (`9:00`→`09:00`) or rejected with a visible per-field message — never
   silently dropped.
2. The repeat generator uses helpers: time pickers for from/to and a
   **select of preset periods** (30 min, 1 h, 2 h, 3 h, 4 h, 6 h, 8 h, 12 h)
   or a stepper — not free text. The occurrence count label doubles as live
   validation ("invalid range" in red when empty/inverted).
3. The generate button asks before overwriting existing non-empty slots
   (or merges), and invalid input shows a translated inline error instead of
   doing nothing.
4. All labels/errors translated in `locales/care_plan.{fr,en-us,de,nl}.yaml`
   (see BUG-33 — no opaque references).

### Acceptance criteria (retest)

1. Create a feeding protocol from the animal page modal: add a slot by typing
   `9:00` → saved schedule contains `09:00` (normalized) or the field shows a
   clear error; nothing is silently dropped.
2. Generator with from 09:00 / to 19:00 / every "5 h" (selected from presets,
   not typed) → exactly 3 slots 09:00/14:00/19:00 in the saved plan.
3. Generator with empty/inverted input → visible translated error, no slot
   changes.
4. Repeat the full flow on `/care_rules/new` and `/care_rules/<id>/edit` and
   in all 4 UI locales.
5. Covered by the E2E plan: [E2E-PROTOCOL-EDITOR.md](./E2E-PROTOCOL-EDITOR.md).

---

## BUG-36 (feature) — Schedule repeat mode: "start/end + every X" as a first-class, persisted schedule form

**Status:** ✅ IMPLEMENTED 2026-10-12 — G5 E2E qualified (E2E-PROTOCOL-EDITOR.md 9.1–9.17 all ✅; full `go test ./...` green 23:17 CEST)
**Severity:** Feature request (changes the §4.3 schedule document; converter must adopt it)
**Found by:** User directive: *"user should be able to enter a serie of slots OR select a start/end + 'every X' — this should be saved as it and extended as needed to create timeslot (done at save/update). If start/end + every X is set — show to user, do not show timeslot; if unset — timeslot. User MUST select an approach: actual slots or repeat (radio option). App can save slotted time for both to allow an easier execution approach. Convert MUST correctly translate original feeding using the repeat."*

### Requirement

1. **Two mutually exclusive authoring modes**, chosen by a mandatory radio
   option in the protocol editor (`templates/care_plan/_editor.plush.html`):
   - **Slots** — the explicit time list (today's behavior).
   - **Repeat** — `from` (start), `to` (end), `every` (period). When a
     schedule carries a repeat definition, the editor shows ONLY the repeat
     fields (from/to/every + live occurrence count), never the slot rows.
2. **Persisted document** (both `care_rules.schedule` and
   `care_animal_plans.schedule`, §4.7 parity) gains an optional block:
   `"repeat": {"from":"09:00","to":"19:00","every_minutes":300}`.
   `times[]` remains stored and authoritative for execution (engine contract
   unchanged) — the server **expands repeat → times[] at save/update**
   (start + n×period ≤ end, overnight window to<from +24h, cap 24 slots;
   same semantics as `DeriveFeedingTimes`).
3. **Re-open round-trip:** schedule with `repeat` → editor radio = Repeat,
   repeat fields shown, slot rows hidden; schedule without `repeat` → radio =
   Slots, slot rows shown.
4. **Parser support:** `models/careplan/schedule.go` is strict
   (`DisallowUnknownFields`) — `repeat` must be added to `scheduleJSON` +
   `Schedule`, validated (from/to HH:MM, every_minutes > 0, expanded times
   consistent with the block), and serialized back on save.
5. **Converter** (`actions/care_plan_convert_data.go`): legacy feeding rows
   with non-NULL `feeding_start`/`feeding_end`/`feeding_period` must convert
   to the **repeat form** (block + expanded times), with NO " (à vérifier)"
   suffix — the translation is exact. The "à vérifier" flag stays ONLY for
   the NULL start/end fallback (single 09:00 slot, no repeat block).
6. **Qualification DB:** restore the pre-care-expert snapshot
   `/Users/muaddib/dev/creaves.project/creaves-db-2026-10-12.gz` into a scratch
   MySQL DB, run migrations + the startup converter, and verify converted
   feeding plans carry the repeat block matching the legacy columns.
7. **Legacy generator UX** (BUG-35) is replaced by this mode split: the
   repeat mode IS the "generate from range" flow, now with pickers/presets
   and translated validation.

### Acceptance criteria (retest)

1. Create protocol with radio=Repeat (from 09:00, to 19:00, every 5 h) →
   stored schedule has `repeat` block AND `times:["09:00","14:00","19:00"]`;
   day plan shows the 3 occurrences.
2. Re-open that plan → Repeat radio selected, only from/to/every shown.
3. Radio=Slots path unchanged (explicit times, no repeat key stored).
4. Invalid repeat (empty, inverted without overnight intent, every=0) →
   translated inline error, nothing saved (BUG-33 error contract).
5. Restored 2026-10-12 DB: converted feeding plans with legacy start/end/
   period have exact repeat blocks and no "à vérifier"; NULL-legacy rows keep
   the 09:00 fallback + flag.
6. All 4 locales; `go test ./...` green incl. new schedule/converter tests;
   E2E per [E2E-PROTOCOL-EDITOR.md](./E2E-PROTOCOL-EDITOR.md) §P9.

### Execution plan & schedule

Tracked as goals (see session goal list) and the detailed plan:
[../docs/plans/2026-10-09-schedule-repeat-mode.md](../docs/plans/2026-10-09-schedule-repeat-mode.md).

### Decisions (user, 2026-10-09)

1. **Overnight windows allowed** — `to` < `from` wraps past midnight
   (20:00→08:00 every 4 h = 20:00/00:00/04:00/08:00).
2. **"Every" input = hours:minutes duration picker** (flatpickr time-style,
   no calendar) — no free-text minutes ambiguity.
3. **Repeat is source of truth** — the server always re-derives `times[]`
   from the repeat block at save/update; user-edited times are ignored in
   repeat mode.
4. **Repeat mode applies to ALL kinds** — feeding, medication, care,
   observation, weighing.
5. **Qualification** — restore `creaves-db-2026-10-12.gz` into scratch DB
   `creaves_qualif`, migrate, boot app (startup converter runs), verify.

### Progress (2026-10-12)

- **G1 careplan model** — done: `Repeat` block, `ExpandRepeat`, strict
  parse/validate (times == expansion, 24-slot cap), `NormalizeScheduleJSON`,
  `Repeat.FitsCap` (exported for the converter).
- **G2 server save/update normalization** — done: repeat → times expansion
  at save/update for rules and per-animal plans; radio-mode contract.
- **G3 converter + qualification** — done (this goal): `DeriveFeedingTimes`
  now delegates to `careplan.ExpandRepeat` (fixes the unsorted-overnight
  latent crash); `legacyFeedingSchedule` emits `(times, *Repeat, fallback)`;
  exact triples convert unflagged with the repeat block; degenerate/over-cap
  rows stay flagged without a repeat block (honest lossy conversion).
  Converter + MySQL round-trip tests green.
  **Qualification on `creaves_qualif`** (E2E-PROTOCOL-EDITOR §P9b, counts
  recorded there): restore + 63 migrations + boot converter OK; 207 in-stock
  animals → 204 fallback-flagged + 3 fixture exact repeat-form plans,
  **0 mismatches** on the automated join check.
  **Finding:** the real snapshot has no in-stock exact triples — all 6629
  legacy windows are `0001-01-01` zero-dates, nulled by migration
  `20261007140000_animal_feeding_zero_dates_null` (shipped before BUG-36)
  at migrate time, i.e. before the converter runs. Repeat-form conversion
  was therefore verified via a controlled zero-date fixture; on production
  DBs upgraded through that migration chain it will only trigger for rows
  written after the migration.
- **G4 editor UI** — done (2026-10-12): mandatory Slots|Repeat radio
  (`data-pe-mode`, shared `name=peMode` group — one editor instance per
  page), no default on an empty plan; repeat mode = flatpickr from/to clock
  pickers + H:MM duration picker + live "(N occurrences)" count + inline
  `repeat_invalid`/`repeat_cap` errors, slot rows hidden; schedule JSON
  written as `{repeat:{from,to,every_minutes}, times:expanded-sorted}`
  (server re-derives anyway); slots mode normalizes typed `9:00`→`09:00`
  and flags unparseable values (BUG-35 silent-drop fixed); mode switch
  with content in the abandoned mode confirms before discarding; radio
  hidden while raw-JSON mode is authoritative; obsolete `repeat_generate`
  button/JS/locale keys removed. New locale keys in all 4 locales:
  `editor.mode`, `mode_slots`, `mode_repeat`, `repeat_invalid`,
  `repeat_cap`, `repeat_every_hint`, `mode_switch_confirm`.
  `go build ./...` clean; `go test ./actions -run 'Repeat|Convert'` green;
  the 6 failing actions tests are pre-existing (identical failures with
  the G4 files stashed — G5 owns the full suite).
- **G5 full suite + E2E** — ✅ DONE 2026-10-12: `go test ./...` fully green
  (zero failures, 23:17 CEST — inside the 22:00–24:00 historical failure
  window); E2E-PROTOCOL-EDITOR.md rows 9.1–9.17 all ✅ (incl. 9.17
  restored-DB converter qualification). Acceptance 1–4, 6 validated.

---

## BUG-37 (high) — Protocol editor: EN (12 h) slot rows invisible — flatpickr altInput initialized on a detached input

**Status:** FIXED 2026-10-12 (committed on `feature/care-expert`)
**Severity:** High — in `en-US` every time-slot row in the protocol editor is invisible (the real input becomes `type=hidden` and the 12 h alt display input is never inserted into the DOM), so slot entry is impossible in English. fr/de/nl unaffected (no altInput there).
**Found by:** G5 E2E run (E2E-PROTOCOL-EDITOR §2.1–2.2), 2026-10-12.

### Repro

1. Switch UI to English (`/lang/?lang=en-US&url=/animals/10372#nav-plan`).
2. *New protocol* → 1 slot row should be visible; *Add a time* → more rows.
3. Rows render as an empty 12 rem-wide strip: the input has
   `type="hidden"` (`flatpickr-input`) and no visible sibling.

### Root cause

`addSlotRow` in `templates/care_plan/_editor.plush.html` called
`flatpickr(input, {altInput: true, …})` **before** `row.appendChild(input)`.
With `altInput`, flatpickr hides the original input and inserts the visible
alt input as `input.nextSibling` — but on a **detached** input there is no
parent, so the alt input is created and immediately orphaned. The row ends
up with a hidden input + the delete button only. (The repeat from/to
pickers are not affected: `attachRepeatPickers` always runs on inputs
already in the DOM.)

### Fix

Append `input` to `row` before initializing flatpickr in `addSlotRow`
(order: create row/children → append → `flatpickr(...)` → append row to
wrap). Verified in-browser (en-US): after the fix
`fp.altInput.parentElement === row` and the alt input is visible.

### Retest

E2E-PROTOCOL-EDITOR §2.x on en-US — slot rows visible and editable with the
12 h alt display; saved values remain 24 h `H:i` (§2.6).

---

## BUG-38 (high) — Protocol editor: garbage typed in the EN 12 h slot alt-input silently vanishes (no inline error)

**Status:** FIXED 2026-10-12 (committed on `feature/care-expert`)
**Severity:** High — same silent-drop class as BUG-35: typing an unparseable value (`abc`, `25:99`) into the English 12 h flatpickr alt input clears the text and saves nothing, with no field flagged. fr/de/nl unaffected (change listener sits on the visible 24 h input there).
**Found by:** G5 E2E run (E2E-PROTOCOL-EDITOR §2.5), 2026-10-12.

### Root cause

The BUG-35 change listener (`normalizeHM` + `is-invalid`) is attached to the
**real** slot input. With `altInput: true` (en-US), the user types into the
alt input; on commit flatpickr tries to parse the alt text and, on failure,
**clears both the alt text and the real input's value** — so the real input's
change handler sees `''` (not invalid by the `!!value.trim()` rule) and the
garbage disappears without a trace.

### Fix

In `addSlotRow` (`templates/care_plan/_editor.plush.html`), when the slot has
an alt display, attach a second change listener to `input._flatpickr.altInput`:
a non-empty alt value that produced no parseable 24 h value is flagged
`is-invalid` and kept visible; a parseable one normalizes the stored value and
clears the flag. The schedule textarea never receives the invalid value.

### Retest

E2E-PROTOCOL-EDITOR §2.5/§2.6 on en-US — `abc` flagged red and not saved;
`2:00 PM` saves as `14:00`.

---

## BUG-39 (medium) — Protocol editor: schedule save errors leak raw engine text (`careplan: … (§4.3)`) to the user

**Status:** FIXED 2026-10-12 (committed on `feature/care-expert`)
**Severity:** Medium — violates the BUG-33 error contract ("no `§`, no JSON field path, no raw Go text"): a schedule rejected at save (e.g. all slots invalid) showed
`Échec de l'enregistrement — … — careplan: schedule.times needs at least one HH:MM slot (§4.3)` in the modal.
**Found by:** G5 E2E run (E2E-PROTOCOL-EDITOR §2.5), 2026-10-12.

### Fix

`scheduleFieldError` (`actions/care_plan_support.go`) now responds with the
translated `care_plan.editor.schedule_invalid` message (fr/en-us/de/nl added)
instead of forwarding the engine's error string.

### Retest

E2E-PROTOCOL-EDITOR §2.5/§4.x — invalid schedule save shows the translated
message in all 4 locales.

---

## BUG-40 (enhancement) — Protocol editor: grouped cards instead of the requested multi-tab layout

**Status:** DEFERRED (2026-10-12, G5) — the editor groups fields into titled
cards (*Schedule* / *Action content*) rather than a tab strip. The BUG-33
acceptance goal — "in case of error the error field should be clearly
indicated + fully translated" — is delivered without tabs: server-side
localization of validator messages (`localizePlanVerrs`), translated schedule
error (BUG-39), and client-side `is-invalid` marking of the offending field
(`CarePlan`+key, lowercase verrs keys mapped). A tab strip would also require
"auto-switch to the tab containing the error" logic across 12 host templates;
deferred until a user confirms cards are insufficient.
**Severity:** Enhancement.
**Found by:** G5 E2E run (E2E-PROTOCOL-EDITOR §4.1), 2026-10-12.

---

## BUG-41 (high) — Day plan: evening medication applies (22:00–24:00 local) duplicate the treatments row

**Status:** FIXED 2026-10-12 (committed on `feature/care-expert`)
**Severity:** High — at UTC+2 any second same-day medication apply clicked
after 22:00 local opened a NEW treatments row instead of completing the
existing one (duplicate rows, split per-time entries, broken §10-M1 sharing).
At UTC+1 (winter) the window shrinks to 23:00–24:00; UTC servers unaffected.
**Found by:** G5 suite run 2026-10-12 22:49 (`TestCareMedicationSameSlotKeepsDistinctEntries`
+ `TestCarePlanUnapplyMedicationSiblingGuard` failing late-evening only).

### Root cause

`writeMedicationFulfillment` (`actions/care_plan_fulfillment.go`) keyed the
§10-M1 dedup lookup with TZ-aware local `dayStart`/`dayEnd` instants. The
DSN carries no `loc`, so the driver renders bound `time.Time` values as UTC
digits — while `treatments.date` is written by `models.FormWallClockNow()`
(local digits in a UTC instant). The lookup window thus sat 2 h early vs the
stored rows: a row created at 22:49 wall clock fell outside
`[due-day 00:00−2h, next-day 00:00−2h)` and the second apply missed it.

### Fix

The lookup bounds are now expressed in the same wall-clock frame as the
stored column (`wallDay()` strips the zone, keeps the digits); entry dating
(`entryDueAt`) keeps the due-day local frame. The dual-day widening
(click day ∪ due day, §10-M1 midnight rule) is unchanged in scope, only
re-framed.

### Pin

`TestCareMedicationEveningClickSharesRow` seeds a 22:49 wall-clock row by
construction (no clock dependence) and applies a second same-day slot at
23:39: pre-fix it yields 2 rows, post-fix 1 row with 2 done entries —
deterministic at any run time/zone. Verified: fails with the fix stashed,
passes with it applied; the two original failures pass at 23:20.

### Retest

`go test ./actions -run 'Medication|UnapplyMedication'` at any hour;
watch the suite at 22:00–24:00 local.

---

## BUG-42 (enhancement) — Animal page: a multi-slot item line (feeding) wraps its ○/✓ time buttons ragged

**Status:** FIXED + VERIFIED 2026-11-13 (browser, animal 9969 "Pomme Blah", 12 slots; final wrap check after cascade fix)
**Severity:** Enhancement — readability of dense feeding schedules (e.g. 6–8
slots/day on one diet line).
**Found by:** User report 2026-11-13 — `/animals/9969#nav-plan`, Nourrissage
"Pomme Blah": the occurrence buttons wrap onto several lines with no column
alignment (cells are `min-width` only; a cell carrying a day badge is wider).
**Request:** group the time slots — e.g. 5 per line, multiple lines, all
buttons aligned.

### Fix

`_plan_item_line`'s merged-slots cells (`.plan-item-line .plan-med-cell`)
become an equal-width grid cell: fixed `flex-basis` so every cell has the
same width, the day-qualifier badge stacks ABOVE its button inside the cell
(a badge no longer widens its cell), and the ○/✓ button right-aligns under
the badge — every line of wrapped slots keeps the same left edge per column.
Scoped to `.plan-item-line` (feeding/care/obs/weighing merged slots); the
medication rows keep their bucket-caption layout (`plan-med-bucket` /
`plan-med-btns--stacked` measuring JS). Width measured live: the ○/✓ HH:MM
tier buttons render 90 px → cell `flex-basis: 5.75rem` (92 px + 4 px margin
= 96 px pitch); the initial 4.75rem cell made adjacent buttons overlap 10 px.
Verified: 10+2 slots over two rows at a uniform 96 px pitch, columns aligned,
no overlap, no console errors (EN + FR).
Exact 5-per-line wrap: the merged-slots container is capped at
`max-width: 30rem` (5 cells × 96 px pitch = 480 px). The cap was first added
as a standalone rule but silently overridden by the LATER Phase-4/D4 rule
`.plan-item-line .plan-med-btns { max-width: 100% }` (same specificity, later
wins) — merged the 30rem into that rule and deleted the duplicate.
Re-verified live on the visible 12-slot "Pomme" line: computed max-width
480 px, rows per line `[5,5,2]`, per-row pitch 96 px.

### Retest

Animal page `#nav-plan` on an animal with a ≥5-slots/day feeding plan:
slots wrap in aligned columns; day badges sit above their button; no
overlap at any viewport width.

---

## BUG-43 (enhancement) — Protocol editor: the slots/repeat authoring mode is a bare radio pair with no default

**Status:** FIXED + VERIFIED 2026-11-13 (browser, animal 9969 protocol modal, EN + FR)
**Severity:** Enhancement — the *Horaires précis* / *Plage répétée* choice
reads as two detached radios above two swappable blocks, and a new plan
opens with NOTHING selected (both sections' state implicit).
**Found by:** User request 2026-11-13: "use a tab view … link form part
together" + "make sure the radio has a selection on load instead of both
empty". Confirms the BUG-40 tab direction for the mode switch (the wider
multi-tab editor layout stays DEFERRED).

### Fix

The mode radios are REPLACED by a Bootstrap `nav-tabs` strip
(`[data-pe-mode-tab="slots|repeat"]`) — no hidden inputs kept: the active
tab IS the mode state (`getMode` reads the `.active` tab, `setMode` sets
it). The two sections sit in ONE bordered box attached under the tab strip
(`border border-top-0`) so the active tab visually owns its form part. The
switch-with-content confirm (BUG-36 discard guard) is preserved on tab
click. A new/empty schedule now defaults to the **slots** tab instead of
no selection; a persisted repeat block still re-opens on the repeat tab.
Verified: tabs render (fr "Horaires précis"/"Plage répétée", en "Explicit
times"/"Repeat range"), zero `input[name=peMode]` in the DOM, slots tab
active on a new plan, tab clicks flip both sections, confirm-cancel keeps
the current tab AND its entries, confirm-accept discards and serializes
`{"times":[]}`; no console errors.

### Retest

E2E-PROTOCOL-EDITOR §9.1 (updated: tabs, slots preselected on a new plan),
§9.4 mode-switch confirm, §9.12 all 4 locales.

---

## BUG-44 (medium) — Protocol editor: action kind "feeding" must pin Care type to feeding, not offer the care-type serie

**Status:** FIXED + VERIFIED 2026-11-13 (unit tests + browser E2E + crafted POST)
**Severity:** Medium — data quality: a feeding protocol saved with a
non-feeding caretype produces occurrences the feeding views/day plan
classify by caretype, so a wrong pick silently files feedings under e.g.
a cleanup type.
**Found by:** User report 2026-11-13 — *New protocol*: with *Type d'action
: feeding* the *Care type* select still lists the whole care types serie;
it must always resolve to the feeding care type. Only the *care* action
kind should list the serie.

### Fix

Two layers, both sharing the converter's `resolveFeedingCaretype` (first
type=1 caretype, preferring "Repas" then "Alimentation"):

1. **Editor (client)**: `setPlanEditorData` now also exposes
   `planFeedingCaretypeID`; the feeding block's `data-pe="caretype"`
   select renders that single pinned option and is `disabled` (read-only).
   `readPayload` reads a disabled select via a `pinned()` fallback (a
   disabled select's `.value` reads null after a foreign set) and
   `writePayload`'s `set()` restores a disabled select to its pinned
   option instead of blanking it. kind=care keeps the full serie
   (`data-pe="caretypeCare"`, 7 options, enabled).
2. **Server**: `normalizeFeedingPayload(tx, kind, payload)` — applied in
   `CareAnimalPlanCreate/Update` and `CareRulesResource.Create/Update` —
   rewrites `caretype_id` to the feeding caretype whenever
   `action_kind == feeding`, so a crafted POST cannot bypass the pin.
   Normalization (not rejection): legacy/converter payloads keep saving,
   they are silently corrected. Other kinds pass through byte-identical;
   empty/unparseable payloads and a missing feeding caretype in the DB
   pass through untouched (model validation reports those).

Verified: unit tests `TestNormalizeFeedingPayload*` (pin wrong id, already
pinned, missing id, other kinds untouched, degenerate inputs) — pass;
browser E2E on animal 9969 protocol modal — feeding block shows a single
disabled "Meal" (Repas) option, kind switch to care shows the enabled
7-option serie, save round-trip persists the feeding caretype; crafted
POST with a "Soin" caretype_id returns/saves the pinned feeding caretype;
no console errors. Locale parity: the pinned option label goes through
the same `tname("caretypes", …)` translation as before (all 4 locales).

### Retest

New protocol → kind=feeding: Care type shows feeding only (pinned);
kind=care: full serie; switching kinds resets/pins accordingly; saved
feeding plan has the feeding caretype in its payload.

---

## BUG-45 (enhancement) — Protocol editor Advanced: per-tier display caps (max late / max now / max future)

**Status:** FIXED 2026-11-13 — VERIFIED (browser E2E + unit tests)
**Severity:** Enhancement — UX density: a plan with many daily occurrences
(e.g. 12-slot feeding) renders 12 buttons per tier section on the day/animal
plan. The caregiver wants a per-plan knob limiting how many occurrence
buttons each tier (Late / Now / Later) DISPLAYS.
**Found by:** User request 2026-11-13: in the protocol editor's *Advanced*
disclosure, add `display: max future / max late / max now` counters — 0 by
default meaning DISABLED (no cap); purely UX, sets the number of buttons
shown in each category.

### Plan

1. **Schedule document**: three optional int fields, e.g.
   `display_max_late`, `display_max_now`, `display_max_future`
   (omit or 0 = no cap). Add to the careplan schedule model
   (`models/careplan`) + JSON round-trip; unknown-field tolerance already
   exists for forward compat — verify.
2. **Editor** (`_editor.plush.html` Advanced block): three number inputs
   (min 0, default 0) wired into readSchedule/writeSchedule.
3. **View model**: when building tier views (Late/Now/Later), cap the
   rendered slot list per tier at the plan's counter; show a "+N more"
   affordance (detail modal already lists every due time — reuse it).
   NO effect on scheduling/apply logic — purely presentational; apply/
   batch-apply still covers ALL occurrences (or define: batch covers
   visible ones only — DECIDE with user; default: all).
4. **Validation**: negative values rejected/clamped to 0.

### Retest

Plan with 12 slots/day + display_max_now=5 → Now section shows 5 buttons
+ "+7"; 0/absent = uncapped (current behaviour); applies still record the
correct occurrence; counters survive edit round-trip in all 4 locales.

### Fix (2026-11-13)

- `models/careplan/schedule.go`: `DisplayMaxLate/Now/Future` ints on
  `Schedule` + `display_max_late/now/future` JSON keys (omitempty);
  absent/0/negative = uncapped (negative clamped, never rejected —
  UX-only, a bad value must not block a save); round-trip + Normalize
  preserved.
- Editor `_editor.plush.html` Advanced: three number inputs (min 0,
  default 0) wired into readSchedule/writeSchedule; keys
  `care_plan.editor.display_max_late/now/future` in all 4 yamls.
- View model: `CardView.DisplayMax*` populated from `src.Schedule()`
  (`cardFor` day view + `animalDayCardFor` animal page);
  `applyDisplayCaps` folds each tier's overflow (EARLIEST kept, stale
  exempt) into `CappedLate/Now/LaterCount`; applied in `tierRows` and
  `mergeDayItems` after the stale fold.
- `_plan_item_line.plush.*` (4 locales): non-interactive "+N" badges
  (`badge-danger` late / `badge-warning` now / `badge-light` later) with
  `care_plan.display_more_title` tooltip. Batch apply / apply logic
  untouched — caps purely presentational.
- Tests: `models/careplan/schedule_displaymax_test.go` (round-trip,
  defaults, clamp, normalize), `actions/care_plan_bug45_display_caps_test.go`
  (per-tier fold, stale exempt, over-cap no-op).

### Verification (2026-11-13, browser + mysql)

- Animal 9969 "Blah" plan (12 slots): set max now=5 + max future=5 via
  editor → persisted `display_max_now/future: 5`; line renders 7 buttons
  (2 now + 5 future, earliest kept) + one "+5" light badge.
- Apply on a visible slot records the correct occurrence (08:00 →
  care_plan_applications row due 06:00 UTC) — unaffected by the caps.
- Reset to 0 via editor → canonical doc drops the keys; line renders all
  12 buttons, no badge.
- Editor labels verified fr / de / nl / en-US (cookie-switched).
- Day plan `/care_plan` renders clean (no JS errors).
- Note: during cleanup a 2026-09-22 care row (fede6e57) was mistakenly
  deleted then re-inserted with the same id/timestamps (type=Repas) —
  dev DB only.

## BUG-46 — Protocol editor: mid-session raw-JSON edits silently discarded

**Status:** FIXED 2026-11-13 — VERIFIED in browser (2026-11-13)
**Severity:** Minor — data-loss-shaped UX: pasting an unparseable schedule
into the Advanced raw textarea mid-session leaves the garbage visible, but
the next structured-field edit silently overwrites it (`sync()` rewrites
the textarea from the fields). The raw text never becomes authoritative,
no invalid flag is shown, and the user believes a hand edit was kept.
**Found by:** E2E-PROTOCOL-EDITOR row 9.11 RETEST (2026-11-13). Attach-time
raw mode (unparseable STORED schedule) still hides the mode tabs and keeps
raw authoritative, but it is unreachable in practice: `schedule` columns
are MySQL `json` type and reject invalid documents, so only mid-session
entry can produce garbage — exactly the broken path.
**Pre-existing:** behavior identical since the editor's introduction
(3ddc805) — not a BUG-43 regression; 9.11's expectation was written for
the attach-time path.

### Plan

On `change` of the schedule/payload textarea with unparseable content:
enter raw mode symmetrically (hide `[data-pe-mode-group]`, show the raw
banner, `st.rawMode = true` so `sync()` stops rewriting) — the same
end-state as attach-time raw mode. Exiting raw on clean parse already
works. Flag the textarea `is-invalid` while unparseable.

### Fix

- `templates/care_plan/_editor.plush.html` — raw mode is now symmetric:
  entering unparseable JSON in a `data-pe-json` textarea mid-session flips
  the editor into raw mode the same way an unparseable value at attach time
  does (visual groups `[data-pe-mode-group]` hidden, warning banner shown
  once, `st.rawMode = true`). While the pasted text stays unparseable the
  textarea gets `is-invalid` (Bootstrap red border); flags clear on clean
  parse. `writeSchedule`/`writePayload` already bail out when `st.rawMode`
  is set, so subsequent visual edits can no longer clobber hand-edited JSON.

### Verification (browser, 2026-11-13)

Animal 9969 protocol editor, schedule textarea: paste `{garbage` → mode
tabs + visual groups hidden, banner *« Ce JSON n'a pas pu être lu par
l'éditeur visuel — modifiez-le ici directement. »* shown, textarea flagged
`is-invalid`. Edit a visual field (grace 90) → editor stays raw, pasted
text untouched (not overwritten). Paste back valid JSON → banner/tabs
restore, visual reloads the hand-edited values (grace=90, slot 10:00
present). Retest expectation met.

### Retest

Paste `{garbage` in the schedule textarea → tabs hidden, banner shown,
field edits do NOT touch the textarea; fix the JSON → structured editor
resumes with the parsed values.

## BUG-47 (minor) — Feeding protocol editor: locked "Care type" select shown but carries no value

**Status:** FIXED 2026-11-13 — VERIFIED in browser (2026-11-13)
**Severity:** Minor — dead UI. Since BUG-44 the feeding kind pins Care type
to the feeding caretype: a disabled single-option select the user cannot
change. It brings no information and adds noise; the value must still be
transported in the payload (pinned fallback) and server-side normalization
stays the safety net.
**Found by:** user review 2026-11-13.

### Fix

`templates/care_plan/_editor.plush.html` — the feeding caretype form-group
carries `d-none` (element kept in the DOM so `readPayload`'s `pinned()`
fallback still serializes `caretype_id`). No locale change (nothing shown).
Server normalization (BUG-44) unchanged.

### Verification (browser, 2026-11-13)

Animal 9969 feeding editor: no Care type row visible
(`caretypeGroupHidden: true`, `caretypeVisible: false`), payload still
carries `caretype_id fe439281-…`. Kind switch to care → 7-option enabled
select visible; back to feeding → hidden again. 4 locales (fr/en-US/de/nl):
caretype hidden everywhere, payload intact everywhere.

### Plan

Hide the feeding caretype form-group in
`templates/care_plan/_editor.plush.html` (`d-none` on the wrapper, element
kept in the DOM so `readPayload`'s `pinned()` fallback still serializes
`caretype_id`). No locale change (nothing shown). Server normalization
(BUG-44) unchanged.

### Retest

Feeding editor → no Care type row; save → payload still carries the
feeding caretype id. Care kind → select still visible and enabled.

## BUG-48 (enhancement of BUG-40) — Protocol editor: tab/wizard layout with guided validation

**Status:** FIXED 2026-11-13 — VERIFIED in browser (2026-11-13)
**Severity:** Enhancement — the grouped-cards layout (BUG-40) does not
guide a user creating a NEW protocol, and server-side validation errors
surface on the hidden transport textareas (BUG-33 marking), not on the
visible structured field, with no indication of WHICH card holds the
problem.
**Found by:** user review 2026-11-13 (confirms the BUG-40 deferral
condition: cards are insufficient).

### Plan

Turn the editor's cards into a wizard-style tab strip inside
`templates/care_plan/_editor.plush.html` (shared by all 12 host
templates, so one change covers every surface):

- Tabs: **Content** (per-kind action fields) → **Schedule** (mode tabs,
  slots/repeat, every-days/anchor/duration) → **Options** (advanced:
  offsets, grace, display caps, weekdays). Default tab on open: Content.
- **Next/Back** buttons inside the pane area (pure client-side tab
  moves; Save stays host-side).
- **Client-side validation** mirroring the server contract
  (`ValidateActionPayload` / `Schedule.Validate`): Content tab requires
  the kind's required field (feeding→caretype pinned, medication→drug +
  one dosage path, care→caretype, observation→prompt); Schedule tab
  requires ≥1 valid slot or a valid repeat, anchor=fixed requires a
  date. Failures mark the visible field `is-invalid` and put a count
  badge on the offending tab.
- `attach` returns `{refresh, sync, validate, revealError}`.
  `validate()` reveals the first invalid tab and returns false; hosts
  call it before submit (animals/show modal save + care_rules form
  submit). `revealError(key)` maps server verrs keys
  (schedule→Schedule tab, action_payload→Content tab) so a server-side
  rejection still lands the user on the right tab.
- New locale keys (4 locales): `care_plan.editor.next`, `.back`,
  `.validation.required`, `.validation.slots_required`,
  `.validation.anchor_date_required`, `.validation.dosage_choice`.
  Tab labels reuse `care_plan.editor.content`, `care_plan.field.schedule`,
  `care_plan.editor.advanced`; the repeat error reuses the existing
  `care_plan.editor.repeat_invalid` key.

### Fix (all browser-verified 2026-11-13)

- `templates/care_plan/_editor.plush.html` — the two cards + Advanced
  disclosure become a wizard tab strip: **Content** (per-kind action
  fields) → **Schedule** (mode tabs + slots/repeat + every/anchor/duration)
  → **Options** (advanced: offsets, grace, display caps, weekdays).
  Back/Next buttons in a footer; Next runs client validation first.
  `validate()` mirrors the server contract (ValidateActionPayload +
  Schedule.Validate): medication drug + exactly-one-dosage-path, care
  caretype, observation prompt, ≥1 valid slot or valid repeat, anchor=fixed
  needs a date. Marks fields `is-invalid`, puts count badges on tabs,
  reveals the first offending pane, returns false. `revealError(key)` maps
  server verrs keys (schedule/action_payload) onto the wizard so a rejected
  save lands the user on the right tab. Raw mode (BUG-46) skips validation.
  Every (re)open resets to the Content tab and clears stale marks.
- Hosts wired: `templates/animals/show.plush.{html,fr,de,nl}.html` —
  `save()` calls `planEd.validate()` before POST and `planEd.revealError(k)`
  on each verrs key. `templates/care_rules/{new,edit}.plush.{html,fr,de,nl}.html`
  — form submit gated via `e.preventDefault()` when invalid.
- New locale keys ×4 (`locales/care_plan.{en-us,fr,de,nl}.yaml`):
  `care_plan.editor.next`, `.back`, `.validation.required`,
  `.validation.slots_required`, `.validation.anchor_date_required`,
  `.validation.dosage_choice`. Tab labels reuse existing keys
  (`care_plan.editor.content`/`field.schedule`/`editor.advanced`); the
  repeat-mode error reuses `care_plan.editor.repeat_invalid`.

### Verification (browser, 2026-11-13)

- Edit plan (animal 9969): opens on Content tab (Back disabled, Next
  visible); Next → Schedule → Options (Next hidden on last); Back works.
- Blank slot + Next → stays on Schedule, slot `is-invalid`, badge "1" on
  the Schedule tab, footer message "Add at least one time slot".
- Medication kind + Save with empty drug/dosage → save blocked, both
  fields flagged, badge "2" on Content, messages in footer, modal stays.
- Server path: duplicate slots (08:00, 08:00) pass the client check but
  the server rejects (strictly ascending) → 422 handled, schedule textarea
  flagged, **Schedule tab revealed** (revealError), banner shows the
  translated message. Fixing to 08:00/09:00 → save succeeds, plan created
  (then deleted; DB clean).
- FR repeat plan: cleared repeat-from + Next → stays on Schedule, field
  flagged, message "Plage invalide : …" (reused repeat_invalid key).
- 4 locales (fr/en-US/de/nl): tab labels + Back/Next translated
  (Contenu/Horaires/Réglages avancés — Action content/Schedule/Advanced
  settings — Aktionsinhalt/Zeitplan/Erweiterte Einstellungen —
  Actie-inhoud/Tijdschema/Geavanceerde instellingen).
- `/care_rules/new`: wizard tabs render, Content active by default.
- Accidental fixture touch: editing plan `7a9b56cc` saved without `food`
  during testing (kind-switch cleared content); restored via SQL — noted,
  dev DB only.

### Retest

New protocol on animal 9969: opens on Content tab; Next with empty
medication drug → stays, field flagged, badge on Content; valid →
Schedule; invalid repeat → flagged on Next; Save with server-rejected
schedule → Schedule tab revealed. 4 locales: tab labels + Next/Back
translated. care_rules/new: same tabs, submit blocked when invalid.
