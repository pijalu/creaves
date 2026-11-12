# Round 8 — Day-plan UX, cleanup rule, Protocols menu, Preferences

**Status:** IN PROGRESS · **Created:** 2026-10-04 · **Source:** user issue list
(5 items). Per instruction, this document logs and plans each item before any
code change. Each item lists ground truth found in code, the fix design, and
acceptance checks.

---

## Item 1 — Grouped feeding rows hide the DATE ("from 18:00" is unclear)

**Ground truth.** `finalizeFeedingChips` (`actions/care_plan_viewmodel.go:1071`)
sets `FirstTimeLabel = fv.TimeGroups[0].Label` — the bare "18:00". The collapsed
group header (`templates/care_plan/index.plush.html:363`) prints
`t("care_plan.feeding.earliest") + FirstTimeLabel`, so a late group renders
"from 18:00" with no day. The per-time sub-group headers already carry the
day-aware label (`tg.DayKey` / `tg.ShortDate` + time) — only the collapsed
header loses it.

**Fix.** Expose `FirstTimeDayKey` + `FirstTimeShortDate` on `FeedingGroupView`
(copy of the first sub-group's parts); the 4 `index.plush.*` templates compose
the header as `earliest [day-word|short-date] HH:MM` — e.g. "from yesterday
18:00", "depuis hier 18:00". Open (single-line) rows are unaffected.

**Accept.** A late collapsed group reads "from yesterday 18:00" (EN) / "dès
hier 18:00" (FR); a far group reads "from 02/10 18:00"; today's groups still
read "from 08:00". Verified in EN+FR.

## Item 2 — Observation tab: style parity, working protocol link, no repeated title

**Ground truth.**
(a) `_plan_row_line.plush.html` renders `Detail` ("Sexage") AND the source name
("Traitement — Sexage") — the title repeats with a kind prefix. The feeding fix
(`SourceNameRedundant` on `CardView`) already handles this shape for feeding on
the animal page, but not here and not for observation/care/weighing.
(b) The row's source link goes to `/animals/{id}?back=…#nav-plan`. The animal
Protocol tab's "Details" trace table (`#planDetails`) is **collapsed by
default**, so the linked protocol is not visible — the link "does not work".
The trace rows carry no row-level id (only the edit button has `data-id`).

**Fix.**
(a) Generalize `sourceNameRedundantWithDetail` to observation/care/weighing
(name core after the "—" kind prefix contained in the detail → redundant) and
honor `SourceNameRedundant` in `_plan_row_line.plush.html` (same conditional as
the animal page) — the row keeps kind badge + title only.
(b) `cardSourceLink` (plan rows) gains `&src=<sourceID>` (rule sources:
`src=rule:<id>`). The animal page gets `data-source-id` on each trace row plus
a load-time script: with a `src` param, open `#planDetails`, highlight the
matching row (`.plan-trace-highlight`, CSS), scroll it into view. No behavior
change without the param.

**Accept.** Observation row shows "Observation — Sexage" (no second copy);
clicking ℹ/row-link opens the animal Protocol tab with Details expanded and the
producing protocol row highlighted. EN+FR verified.

## Item 3 — General rule: cleanup of occupied cages in "requires cleanup" zones

**Ground truth.** `Zone` (`models/zone.go`) has zone/type/default only. The
matcher registry has `zone` (string) and `cage` fields but no zone-attribute
field; seeds (`actions/care_plan_seeds.go`) are idempotent-per-name but only
run inside the startup converter, which no-ops once its marker row exists
(`care_plan_converter.go:146`).

**Fix.**
1. Migration: `zones.requires_cleanup` BOOL NOT NULL DEFAULT false.
2. Zone model + admin form/index/show (×4 locales) gain the checkbox.
3. `AnimalContext.ZoneRequiresCleanup` + a fill step in `assemblePlanAnimals`
   (load zones with `requires_cleanup`, match by zone name) and a registry
   field `zone_requires_cleanup` (bool).
4. Seeds: matcher `SM14 "Zone à nettoyer" = zone_requires_cleanup = true` and
   rule `SR13 "Nettoyage des cages occupées" (kind=cleanup, daily)`.
5. Small idempotent grift `careplan:seed_cleanup` that runs ONLY the seed step
   (the converter is a no-op post-cutover) so the rule lands on existing DBs.
6. Occupied-only is inherent: rules evaluate over in-care animals only (the
   context loader excludes outtaken-before-today animals).

**Accept.** A zone flagged "requires cleanup" makes its occupied cages produce
a daily cleanup occurrence (visible under Day plan ▸ Cleanup); unflagged zones
produce none; flag toggle re-evaluates without further migration.

## Item 4 — Administration ▸ Protocols sub-menu

**Ground truth.** The admin dropdown (`templates/application.plush*.html:58`)
has three `dropdown-submenu` groups (Configuration/System/Synchronization);
the protocol-related entries (Care types, Feeding guides, Care note templates,
Care rules, Care matchers) are scattered inside Configuration.

**Fix.** New "Protocols" sub-menu grouping those five entries, placed before
Configuration; they leave the Configuration menu. One new `nav.*` locale key
per language; the sub-menu CSS pattern already exists.

**Accept.** Admin menu shows Protocols ▸ (Care types, Feeding guides, Care
note templates, Care rules, Care matchers) in all 4 languages; Configuration
no longer lists them.

## Item 5 — System preferences for the day-plan views (per kind)

**Ground truth.** No preferences table exists (`models/` has Config only, a
single-row JSON settings blob for webhook/sync). The day plan shows ALL open
work: every late/missing occurrence regardless of age, and all future
occurrences the window generates (TreatmentPlanWindow caps at +5d on the
animal page; the day plan's later tier is bounded only by the occurrence
window).

**Fix.**
1. Migration: `preferences` table — `kind` (unique, one of
   feeding/medication/care/cleanup/weighing/observation),
   `late_show_hours` INT NULL (hide late/missing older than this),
   `future_show_hours` INT NULL (hide later-tier occurrences beyond this
   horizon), `now_window_minutes` INT NULL (reserved: window considered
   "now"), timestamps.
2. `models.Preference` + `actions/preferences.go` (list + update; admin UI at
   `/preferences`, System sub-menu) with per-kind rows.
3. Seeding = current behavior: `PreferencesEnsureSeeded` creates any missing
   kind row with all caps NULL (= "show everything", exactly today's behavior);
   called from the index handler — idempotent, no data migration.
4. Application point: `CarePlanIndex` filters the built `plan.Items` per kind
   BEFORE the view model/tier counts are derived, so badges, tiers and lists
   stay consistent (hidden late work stays on the animal Plan tab; the day
   plan is the work screen, not the archive).

**Accept.** `/preferences` lists 6 kinds with the three knobs; setting e.g.
Feeding late_show_hours=4 hides feeding late occurrences older than 4 h from
`/care_plan?kind=feeding` (badge counts drop accordingly); NULL keeps today's
behavior; survives restart.

---

## Execution order

1 → 2 → 4 → 3 → 5 (small template fixes first, then the menu, then the two
migration-bearing items). After each: build; at the end: full `buffalo test`,
browser verification ×4 locales, admin password restored.
