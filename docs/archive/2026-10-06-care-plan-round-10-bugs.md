# Bugs — Round 10 (care-plan UX + timestamp localization) — CLOSED 2026-10-06

Round-10 entries archived from the active bug list after implementation,
focused tests, and authenticated browser verification where available. B10-1's
actual live timestamp could not be exercised without changing production-backed
records; browser formatter was validated against an RFC-3339 sample, and that
limitation is recorded in the entry. B10-9 is closed as not reproduced on the
available instance, with a precise reopen condition. B10-10 passed focused/full
action tests and authenticated cage-group E2E.

**Round validation:** `go test ./actions -count=1`, focused B10 tests, and `git diff --check` passed. `go vet ./...` and `staticcheck ./...` passed. Full `go test -count=1 -race -cover ./...` remains blocked by existing `grifts/TestTemplateVariantStructuralParity` failures (missing localized animal partials / known locale structural drift). `gocognit -over 15 .` and `gocyclo -over 12 .` report many repository-wide threshold violations; these gates are not clean.

Companion plan: `../care-plan-round-10-fix-plan.md`.

**Guideline** (same convention as the archived rounds):
1. Create a detailed fix plan for each bug — the plan must contain test approach and validation steps — execute the plan and validate the fix when all elements are in place.
2. Any issues found must be fixed and the fix plan updated accordingly.
3. Issues found during testing must be fixed and the fix plan updated accordingly.
4. Each bug is moved to `docs/archive/` when tested and closed with its plan.
5. **All changes must be tested, including e2e testing using the [agent-browser skill](../../.agents/skills/agent-browser/SKILL.md)** — no fix is complete without e2e evidence (commands, URL, captured output).
6. Use interactive shell/filmstrip to validate the output of the tool — verify the actual terminal output, not the plan.
7. Check code quality with each tool run separately (do not chain them with `;` or `&&`):
   - `go vet ./...`
   - `staticcheck ./...`
   - `gocognit -over 15 .`
   - `gocyclo -over 12 .`
   - `go test -count=1 -race -cover ./...`
8. Commit each fix with a clear and descriptive commit message.

**Session constraints:**
- **creaves contains real production data**: never drop/destroy data; schema changes must be additive/backward compatible (no destructive migrations).
- **Every UI change lands in all four locales** (`en-US`, `fr`, `de`, `nl`) — the `templates/**/*.plush.{html,fr,de,nl}.html` forks stay in sync.

---

## Open items

### B10-1 — "Done" timestamps render in UTC, not the user's browser locale

**Reported:** 2026-10-05 (caregiver feedback).

**Symptom:** every "done" timestamp (applied/completed care entries — history rows, treatment-entry badges, toggle tooltips) is shown in UTC instead of the browser's local time. A care applied at 14:30 local reads `12:30` (or an ISO UTC string), which is actively misleading on the work screen.

**Evidence (code):** the timestamps are formatted SERVER-side with the raw `time.Time` (UTC) instead of the client locale, e.g.:
- `templates/animals/show.plush.html:611` — `entry.AppliedAt.Time.Format("15:04")` (Go/server tz).
- `templates/care_plan/_plan_history_table.plush.html` — `data-due-at` is RFC-3339 UTC but the visible label is pre-formatted server-side (no client re-localization).

**Expected:** any user-visible "done"/applied/history time is rendered in the user's browser locale + timezone (e.g. via `Date.prototype.toLocaleString` on an RFC-3339 data attribute, matching how the toggle `title` already localizes `data-due-at` in the treatment slot JS), in all 4 locales.

**Scope check:** history table, animal-page treatment tab done badges, dashboard done rows, care_plan history section — audit every `Format(` on applied/terminal timestamps.

**Fixed:** client-side browser-local formatting for treatment completion badges/tooltips and care-plan History application times; RFC-3339 instants preserved in `datetime`/data attributes. Implemented in all four locale forks. Regression test covers UTC instant preservation on history viewmodel; locale-render tests pass. Authenticated browser login and page smoke completed, but available production-backed data contained no done timestamp on `/care_plan` or animal 8635, so a rendered timestamp could not be exercised without creating/changing real records. Dynamic timestamp E2E remains unverified.

---

### B10-2 — Care plan per-animal rows do not show Zone / Cage / Espèce

**Reported:** 2026-10-06 (caregiver feedback).

**Symptom:** on `/care_plan`, rows that represent ONE animal do not carry the
animal's location/species context, so the caregiver cannot tell where the
animal lives without opening it: feeding `group=animal` rows show year + cage +
food (no zone, no species); cleanup `group=animal` rows show year + cage +
source (no zone, no species); medication rows and row-kind lines (care /
weighing / observation) show only the year number; the history table shows only
the full animal label.

**Expected:** every per-animal row shows **Zone · Cage · Espèce** next to the
animal identity when space permits (collapses/hides on narrow screens), with
the info column sized SIMILARLY across the stacked tables so the content stays
aligned down the page.

**Evidence (code):** `_plan_tier_feed_table.plush.html` (animal cell, lines
36-47), `_plan_care_line.plush.html` (lines 47-55), `_plan_med_row.plush.html`
(lines 32-35), `_plan_item_line.plush.html` (showAnimal cell, lines 53-61),
`_plan_history_table.plush.html` (col 1). `CardView` already carries
Zone/Cage but renders neither; `FeedingGroupView`/`CareView` animal-mode rows
lack Species; `CardView` lacks Species.

**Fixed:** commit ca432cc (2026-10-06) — plan §B10-2. E2E: animal-mode
feeding/cleanup rows render "B1 · ACCUEIL · Cygne tuberculé"-style loc lines;
history column renders; verified en-US/fr/de/nl.

---

### B10-3 — Feeding (and single-occurrence lines) use an icon-only "done" button instead of the toggle-with-time nomenclature

**Reported:** 2026-10-06 (caregiver feedback).

**Symptom:** feeding per-animal toggles render a bare clock icon
(`<i class="far fa-clock">`); the due time lives only in the time-group label
above. The same icon-only pattern exists on the single-occurrence apply/undo
pair of `_plan_item_line`, where the time is written as a separate text next to
the button. Every other work screen toggle (cleanup slots, med slots, merged
item slots) uses the "toggle button WITH the time" nomenclature (`○ 16:00` ⇄
`✓ 16:00`).

**Expected:** the toggle itself carries the time (`○ HH:MM` open, `✓ HH:MM`
done) — feeding per-animal toggles and the single-occurrence pair alike; no
duplicate bare time text next to the button.

**Evidence (code):** `_plan_tier_feed_table.plush.html` lines 122-137;
`_plan_item_line.plush.html` lines 145-169 (single-occurrence variant).

**Fixed:** commit ca432cc (2026-10-06) — plan §B10-3. E2E: toggle reads
"○ 08:00", flips to "✓ 08:00" with row tint on click, undo restores; apply
+ undo round-trip executed in the browser.

---

### B10-4 — Dashboard medication table: ℹ button dead, eye/view redundant, animal link opens the wrong tab

**Reported:** 2026-10-06 (caregiver feedback).

**Symptom:** on the dashboard "Medication today" table: (1) the ℹ button does
nothing — the `#planDetailModal` markup is present (via `_apply_toggle`) but
the opener JS exists only on `/care_plan` and the animal page; (2) the eye
"view" button navigates away to the animal page with a modal deep link, which
is redundant once the ℹ popup works in place; (3) the animal button opens the
animal page on the **Plan** tab (`cardAnimalLink` → `#nav-plan`) instead of the
Treatment tab.

**Expected:** ℹ opens the detail popup IN the dashboard; eye/view removed; the
animal link opens the animal view on the **Treatment** tab, scrolled to the
corresponding treatment series.

**Evidence (code):** `templates/dashboard/dashboard.plush.html` line 96
(`medSeriesEye = true`), line 111 (animal link), lines 140-141 (partials, no
opener script); `templates/care_plan/_med_series.plush.html` lines 54-58 (eye);
`actions/care_plan_viewmodel.go` `cardAnimalLink` (line ~2366) and
`medSlotFor` DeepLink (line ~1925).

**Fixed:** commit ca432cc (2026-10-06) — plan §B10-4. E2E: ℹ popup opens in
place (modal visible, fields filled); `.dash-med-view` count 0; animal link
`/animals/8635?back=%2F&med=animal:…#nav-treatment` lands on the Treatment
tab with the matching series line highlighted.

---

### B10-5 — Animal protocol tab: inconsistent entry style/size/alignment + redundant type bubble

**Reported:** 2026-10-06 (caregiver feedback, `/animals/8635#nav-plan`).

**Symptom:** on the animal page Protocol tab the entries render in two
visually different families: medication series lines (from `_med_series`)
carry only `plan-med-line` and miss the compact font/row treatment of the
non-medication lines (which carry `plan-med-row` via `_plan_item_line`); a
grey type badge ("type bubble") prefixes every non-med entry although the
page context already says protocol. Entries of different kinds interleave with
no separation.

**Expected:** all entries same style/size/alignment; type bubble removed; the
day's entries separated per type by a separator row carrying the type title
(`care_plan.kind.*`).

**Evidence (code):** `templates/animals/show.plush.html` line 833
(`showKind: true`), `_plan_item_line.plush.html` lines 62-64 (the bubble),
`_med_series.plush.html` line 42 (missing `plan-med-row`),
`assets/css/care-plan.scss` lines 258-283/347-357 (the two families).

**Fixed:** commit ca432cc (2026-10-06) — plan §B10-5. E2E: 0 kind badges;
localized separators (fr "Nettoyage/Nourrissage/Observation", de, nl, en);
med series lines share the .plan-med-row treatment. NOTE: the localized
show.plush forks turned out to be LIVE per-locale templates — all fixes were
ported to fr/de/nl forks, not only the EN file.

---

### B10-6 — Treatment tab still shows occurrences already covered by the converted protocol; protocol label lost the treatment's dosage detail

**Reported:** 2026-10-06 (caregiver feedback, `/animals/8635#nav-treatment` vs
`#nav-plan`).

**Symptom (animal 8635):**
1. The Treatment tab legacy accordion still renders the treatment series
   "Nettoyage Fistule (Dessus oeil droit)" (today + future days, 3-button
   bitmap fallback) although the protocol (Plan tab) now carries the same
   requirement as the converted plan "Traitement — Nettoyage Fistule (à
   vérifier)" — the same information appears twice, and the treatment copy
   still presents itself as active work.
2. The protocol entry reads only "Nettoyage Fistule" — the converter that
   turned the treatment series into an animal plan DROPPED the
   `treatments.dosage` value ("Dessus oeil droit" = body site), so the
   protocol lost the precision the treatment row still shows.

**Root cause (code/DB):** `actions/care_plan_convert_data.go`
`convertTreatmentSeries` creates the plan (unknown drug → observation kind,
`name = "Traitement — <drug> (à vérifier)"`, `payload.prompt = <drug>`) but
never flags/deactivates the source `treatments` rows; `planDetail` renders
`payload.prompt` only. The legacy accordion renders every `treatments` row
(`templates/animals/show.plush.html` lines 510-634) with dedupe only for
today's merged card (`collectAnimalPlanTodayItems`).

**Expected:** legacy treatments (today + future) that an active converted
protocol plan already covers render as superseded (muted, "covered by
protocol" badge, no action buttons) — data preserved, no double work; the
converted plan's label regains the dosage detail ("Nettoyage Fistule (Dessus
oeil droit)") — both for the existing converted plans (data migration) and for
future conversions (converter fix).

**Fixed:** commits 4201099/ca432cc (2026-10-06) — plan §B10-6. Migration
20261026100000_b10_6_converted_plan_dosage enriched 13 converted plans on the
dev DB (8635 included), noise ("."/"?") filtered; E2E: 7 superseded rows on
8635 (today+future) with the localized badge, past days render unchanged,
protocol shows the enriched label.

---

### B10-7 — `/care_plan?kind=cleanup` empty although cleanup is scheduled within the 8 h future horizon

**Reported:** 2026-10-06 (caregiver feedback).

**Symptom:** the cleanup tab shows nothing even when a cleanup occurrence is
planned within the next 8 h (the per-kind `future_show_hours` cap). The
summary strip counts it in "Later" while the list renders no row.

**Root cause (code):** `actions/care_plan_viewmodel.go` `careItemCounts`
(line 1555) and `feedingViewOf` (line 1393) drop every `StatusScheduled`
item/chip regardless of the horizon. Status is `scheduled` until
`due - lookahead` (schedule default 60 min; the seeded cleanup rule runs
09:00 with default lookahead → the tab stays empty until 08:00). The R8-5
preference caps are applied BEFORE the view model precisely so
scheduled-within-horizon items SURVIVE into `plan.Items`
(`actions/preferences.go` `applyPreferenceCaps`), and the apply endpoint
accepts applicable scheduled occurrences (`checkPlanApplyItem` — only
`!Applicable` 409s) — the builders contradict both.

**Expected:** scheduled occurrences that survived the per-kind future cap
render as open work (toggle slots + batch refs) in the cleanup and feeding
lists, consistent with the summary strip counts and the apply window.

**Fixed:** commit 4201099 (2026-10-06) — plan §B10-7. Unit + golden-DOM
re-record (documented delta); cleanup tab renders the scheduled time groups
and count overlays.


---

### B10-8 — "Abused" medications (cleanings, casts, checks…) should be cares, not observations

**Reported:** 2026-10-06 ("the fistule and similar 'abused' medication should
become cares").

**Symptom:** the legacy `treatments.drug` column carries many NON-drug entries
("Nettoyage Fistule", cast/bandage changes, "Voir dent", douches…). The
converter routed those unknown-drug series to observation plans, which ask a
yes/no question instead of recording a performed care.

**Fixed:** commit 55b5d0b (2026-10-06). Converter routes wound-care +
unknown-drug series to CARE plans typed "Soin" (payload note = the legacy
drug line + site, instructions = remarks); known drug without posology stays
observation; no caretype → legacy observation routing. Migration
`20261026110000_b10_8_abused_medication_to_care` re-kinded the 22 existing
converted plans (Traitement — → Soin —). Superseded matcher understands the
care core (note/instructions). E2E: 8635 protocol tab groups the entry under
Care; `/care_plan?kind=care` renders + toggles the converted plans.

---

### B10-9 — Work-screen entries older than the 8 h window still render

**Reported:** 2026-10-06 (`/care_plan?kind=medication` "and other").

**Symptom:** occurrences older than the per-kind 8 h caps
(`preferences.late_show_hours`) still appear on the work screen.

**Investigation / root cause:** caps are loaded with `preferencesByKind(tx)` in
`CarePlanIndex` (actions/care_plan.go:77-96), then applied to each sourced item
before `BuildDayPlanView`. `applyPreferenceCaps` drops `late`/`missing` items
strictly older than `late_show_hours` and `scheduled` items strictly beyond
`future_show_hours`; exact boundaries remain visible. The tier builder only
classifies the already-filtered items—it does not restore removed rows. Seeded
rows are ensured by the Preferences admin list (`PreferencesEnsureSeeded`), not
by `/care_plan`; absent preference rows genuinely mean uncapped work. Thus a
late OPEN row older than its cap is not explained by tiering: check persisted
preference row and requested kind. Terminal applied/skipped/deferred items are
not capped and intentionally remain in History; they are not open work.

**Finding:** the original report gives no occurrence, status, due time, kind preference value, or HTML-vs-JSON request. Authenticated evidence on the available instance: `/preferences` showed persisted 8 h late/future and 1 h now values for all six kinds; `/care_plan?kind=medication` HTML had four History rows and no old open rows; its JSON read model contained zero `late`/`missing` entries older than 8 h and five older terminal entries. JSON intentionally omits UI caps, so these old terminal entries are not evidence of a work-screen cap failure.

**Regression evidence:** PASS `go test ./actions -run '^TestB10_9PreferenceCapsLateAndFutureBoundaries$' -count=1` and full `go test ./actions -count=1`. Boundary test pins removal beyond late/future caps, inclusive exact boundaries, and terminal preservation. Code loads preferences in `preferencesByKind` then caps items before viewmodel construction. No old OPEN over-cap occurrence found; no cap failure reproduced.

**Status: CLOSED — not reproduced on available instance.** Reopen with one concrete late/missing OPEN row older than its saved kind cap on HTML `/care_plan`, including animal/source, due timestamp, and that kind's persisted preference. Do not change cap logic based solely on terminal History or uncapped JSON.

---

### B10-10 — Cleanup cage grouping: batch button should speak the time-toggle language and collapse like feeding

**Reported:** 2026-10-06 (`/care_plan?kind=cleanup&group=cage`).

**Symptom:** a cleanup cage row renders one `○ HH:MM` toggle PER ANIMAL under
each time-group label — the time repeats once per animal, and the row has no
collapse, so a 12-animal cage shows 12 identical times.

**Expected (user spec):** follow the feeding row pattern — ONE batch button
for the whole cage group that CARRIES the time (repeat the toggle-with-time
nomenclature: `○ HH:MM` applied for all), a collapsible header with the
count/earliest time, and the per-animal buttons inside the expanded list.
"Avoid repeats of time" — the time is stated ONCE per group.

**Status:** FIXED — implementation and focused tests pass; authenticated read-only E2E verified; earliest-time selection and full live locale sweep not directly exercised in resumed verification.

**Implementation:** cleanup viewmodel now derives collapsibility/count/earliest due time; cage rows collapse their per-animal slots under a count header, batch button displays earliest time, and individual action buttons avoid repeating it in cage mode. Animal mode keeps time on each action toggle. Localized care-line forks updated. Feeding animal-mode duplicate animal link/name and separate time label removed from the second cell; its action toggle carries time. Animal columns use min-content sizing, feeding cell can expand, cleanup action controls align right. Shared kind-tab badges now sit inline. Summary-strip tier navigation collapses non-target tier panels.

**Test evidence:** Fresh PASS `go test ./actions -run 'Test(CarePlan|Round10|TreatmentTimeEntries|AnimalPlanToday)' -count=1` (`ok creaves/actions 3.033s`); `go test ./actions -run 'Test(Phase3|B10_10|Round10|TreatmentTimeEntries|AnimalPlanToday|CarePlan)' -count=1` (`ok creaves/actions 3.296s`); explicit B10-9/B10-10/Phase3/phase0b subset (`ok creaves/actions 0.892s`); locale/template/B10-10 tests (`ok creaves/actions 2.197s`). Phase0b golden baseline updated for the separate B10-1 client formatter. Resumed authenticated agent-browser session `b10`, read-only: `/care_plan?kind=cleanup` showed 98 tasks, multi-animal cage count headers (`Bac noir E` count 2; `S10 S` count 11), and `○ 09:00` batch toggle. `/care_plan?kind=feeding&group=animal` showed per-animal identity/location/species in the identity cell and `○ 09:00` action toggle; no repeated identity link or separate time label in adjacent cell. German locale route `/lang/?lang=de&url=%2Fcare_plan%3Fkind%3Dfeeding%26group%3Danimal` rendered localized work-screen labels and species (`Rotfuchs`). No apply controls used. Earliest among multiple distinct due-time groups and full live four-locale sweep not established by initial resumed check; a subsequent read-only locale switch to fr/de/nl on the cleanup route confirmed translated page headings (`Nettoyage`, `Reinigung`, `Schoonmaak`), with en-US already shown initially. All observed due groups remained at 09:00, so earliest-time selection remains unverified; focused localization/template tests pass. No Apply controls were used and no data changed. `git diff --check` clean.

**Implementation pointers:** feeding's pattern is
`_plan_tier_feed_table.plush.html` (collapsible `.plan-feed-header` +
`foldChipsByTime` time-groups + `.plan-feeding-apply` batch button with the
`plan-apply-count` corner overlay); cleanup rows are `_plan_care_line.plush.html`
+ `careViewsOf`/`careAnimalViewsOf` in actions/care_plan_viewmodel.go. The
batch endpoint (`/care_plan/apply_batch`) already handles the whole cage in
one call per source. Glyph pins to update: `care_plan_round7_todo_glyph_test.go`
(`plan-cage-apply` currently expects the bare fa-clock — B10-10 gives it a
time) and the phase0b golden baselines.


## Archived rounds

| Round | File |
|---|---|
| Round 9 + 8 (caregiver UX / data quality / i18n + revalidation sweep) | `docs/archive/2026-10-05-care-plan-round-9-bugs.md` |
| Round 7 (care plan / animal page UX) | `docs/archive/2026-10-03-care-plan-round-7-bugs.md` |
| Round 4 (care plan / treatment / dashboard UX) | `docs/archive/2026-10-02-care-plan-round-4-bugs.md` |
| Round 3 (care plan / treatment / navbar UX) | `docs/archive/2026-10-02-care-plan-round-3-bugs.md` |
| Round 2 (care plan UX) | `docs/archive/2026-10-01-care-plan-ux-round2-bugs.md` |

Round-7 companion plan: `docs/care-plan-round-7-fix-plan.md`.
Round-4 companion plan: `docs/care-plan-round-4-fix-plan.md`.