# Bugs — E2E test run 2026-10-07

Tester log for the browser E2E run documented in [E2E-TEST.md](./E2E-TEST.md).
Each bug is passed to a dedicated fix agent; the fix is **automatically retested**
(retest steps + result recorded here).

---

## BUG-1 (blocker) — Migration `20261007120000_rename_ot2_to_decedee` fails on production-restore DB: duplicate OT2-coded rows

**Status:** OPEN → fix dispatched
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

**Status:** OPEN → fix dispatched
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

**Status:** OPEN → fix dispatched
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

**Status:** OPEN → fix dispatched
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

**Status:** OPEN → fix dispatched
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

**Status:** OPEN → fix dispatched
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

**Status:** OPEN → fix dispatched (together with BUG-7: same recovery subsystem)
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

**Status:** OPEN → fix dispatched
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
