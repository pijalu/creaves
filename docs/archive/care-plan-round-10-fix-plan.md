# Care plan round 10 — fix plan (B10-1 … B10-10)

Companion of `bugs.md` round 10. Each bug: root cause → fix → test approach →
validation. Session constraints apply (bugs.md): real production data — no
destructive migrations; every UI change lands in all four locales
(`en-US`, `fr`, `de`, `nl`).

## B10-1 — browser-local done timestamps

**Fix.** Applied-at timestamps now remain RFC-3339 instants in markup and are formatted with `Date.toLocaleTimeString` in the viewer's browser. Animal Treatment badges/tooltips, legacy-treatment badges and care-plan History rows are covered across `en-US`, `fr`, `de`, and `nl`; no server timezone formatting remains for applied timestamps.

**Tests/validation.** `TestBuildDayPlanViewMedTiers` pins the UTC RFC-3339 value passed to the client formatter. Focused locale render tests pass. Authenticated agent-browser login and `/care_plan` + `/animals/8635` smoke loaded; existing production-backed records on these screens had no completed timestamp. Non-mutating browser E2E then appended a synthetic DOM `<time class="js-local-time" datetime="2026-10-06T13:13:00Z">` on the treatments page and ran the page's exact formatter; authenticated session `b10` reported browser timezone `Europe/Brussels` and rendered `03:13 PM`, matching `toLocaleTimeString`. This confirms formatter behavior without changing any data; production record formatting itself remains unavailable for read-only verification. Repeated synthetic check after app locale switches fr/de/nl produced the same `03:13 PM` each time because formatter intentionally uses browser default locale (`undefined`), not app locale; this validates stable local-time conversion, not locale-specific time notation.

---

## B10-10 — cleanup cage grouping and table-cell efficiency

**Root cause.** Cleanup cage rows rendered an independent time label and one
button per animal per due-time group. The apply-cage control carried only a
clock, so its scope/time were unclear. Feeding animal mode repeated animal link
and time in its second cell after the first identity cell already provided both.
Animal cells had only `min-width`, leaving table layout to allocate inconsistent
widths; action controls were not anchored to the right edge.

**Fix.** Care viewmodel now derives row collapse state, total item count, and
earliest due label. Cage rows collapse their occurrence toggles under a count
header; batch `○ time` control carries earliest day-aware time, with count overlay.
Cage-mode per-animal toggles omit repeated time; animal-mode single actions retain
time. Feeding animal rows suppress redundant animal link and time label in second
cell while preserving time on action toggle. Shared care-table CSS keeps identity
columns stable, allows feeding content column to use remaining width, and right
aligns cleanup actions. Summary badges are inline inside nav tabs; selecting a
tier shortcut collapses other tier panels. Changes applied to all four localized
template forks.

**Tests/validation.** Added cleanup batch/time/collapse pins, viewmodel count/time
assertions, and B10-10 duplicate-cell test. Updated glyph pins and phase0b DOM
goldens. Fresh focused verification in the actual checkout:

- Fresh rerun: `go test ./actions -run 'Test(CarePlan|Round10|TreatmentTimeEntries|AnimalPlanToday)' -count=1` — PASS (`ok creaves/actions 3.033s`).
- Fresh rerun: `go test ./actions -run 'Test(Phase3|B10_10|Round10|TreatmentTimeEntries|AnimalPlanToday|CarePlan)' -count=1` — PASS (`ok creaves/actions 3.296s`). Explicit B10-9/B10-10/Phase3/phase0b subset — PASS (`ok creaves/actions 0.892s`): `go test ./actions -run '^Test(B10_9PreferenceCapsLateAndFutureBoundaries|B10_10FeedingAnimalModeDoesNotRepeatAnimalCell|CarePlanPhase0bDOMEquivalence|Phase3CleanupRendersTieredSections|Phase3CageRowStatesTimeOncePerOccurrenceGroup|Phase3CleanupTogglePairContract|Phase3GroupAnimalOneLinePerAnimal|Phase3GroupChoicePersists|Phase3GroupAnimalRendersHTTP|Phase3TierBadgesSpeakSummaryStripUnit|Phase3CleanupTierTableContract)$' -count=1`. Locale/template/B10-10 tests — PASS (`ok creaves/actions 2.197s`): `go test ./actions -run 'Test.*(Locale|Localization|TemplateVariant|B10_10)' -count=1`.
- Four-locale parity: fresh SHA-1 identical for `_plan_care_line` (`82603adc6f6e017c63bd2b95e28c8f5c8f82b903`), `_plan_history_table` (`211923a023da95cc83a8a98841f818283c44bef5`), and `_plan_tier_feed_table` (`6e2fceacca408bd1bcdb3f8633fffc3e9d7c650f`) variants across en-US/fr/de/nl; localized `index` forks intentionally differ in existing translated copy; localization/template tests pass for structure/convention.
- `git diff --check` — PASS.
- Authenticated read-only agent-browser session `b10` at `http://127.0.0.1:3000`: login at `/auth/new` redirected to `/`; `/care_plan?kind=cleanup` rendered 98 cleanup tasks and cage rows including `Bac noir E` and `E1 - Aqua7 E` with collapsed count `2`, plus batch toggle `○ 09:00`; `S10 S` showed count `11` and `○ 09:00`. This verifies collapse/count and visible batch time; earliest-of-multiple-due-times was not distinguishable because observed groups showed 09:00. `/care_plan?kind=feeding&group=animal` rendered animal identity/location/species on one line (e.g. `R24 · R · West European Hedgehog`), feeding detail in following cell, and `○ 09:00`; no repeated animal link or separate duplicate time appeared there. No apply/toggle controls used; no data changed. Four-locale template parity confirmed by focused localization/template tests. Fresh read-only live locale switching in session `b10` to French, German, and Dutch at `/lang/?lang=<locale>&url=%2Fcare_plan%3Fkind%3Dcleanup` rendered translated page headings (`Nettoyage` / `Reinigung` / `Schoonmaak`); German and Dutch headings localized while existing task labels remain source copy. en-US route is the initial English view. All visible due groups showed 09:00, so earliest-time selection remains unverified; no apply/toggle controls used and no data changed.

**Closure:** B10-10 remains recorded as fixed in `docs/archive/2026-10-06-care-plan-round-10-bugs.md`; round handover archived at `docs/archive/2026-10-06-care-plan-round-10-handover.md`. Focused tests, phase0b DOM test, diff check, and authenticated read-only browser assertions for collapsed counts, 09:00 batch control, and feeding animal-cell deduplication passed. Live locale switching to fr/de/nl verified on cleanup route; earliest-time selection remains unverified because all observed groups showed 09:00. B10-9 remains closed as not reproduced.

---


## B10-9 — investigate work-screen age caps

**Finding / root cause.** `CarePlanIndex` loads saved rows with
`preferencesByKind(tx)` and applies the matching kind's caps to sourced items
before building the view model (`actions/care_plan.go:77-96`). The cap function
removes `late`/`missing` items at or beyond `late_show_hours`, and scheduled
items at or beyond `future_show_hours` (`actions/preferences.go:71-96`). The tier
builder merely sorts/classifies survivors; it cannot restore capped items. This
inclusive cutoff was confirmed after the prior B10-9 investigation; it is a UI
contract change, not evidence that the original incident reproduced.
Preference rows are seeded on the admin preferences page, not the work screen;
a missing row is uncapped. Authenticated browser check of `/preferences` showed
persisted defaults of 8 h late + future per kind and 1 h now window for all six
kinds. Terminal application rows are preserved in History by design and must not
be mistaken for open late work. JSON read-model requests intentionally do not
apply these UI caps.

**Incident result: closed as not reproduced on available instance.** The report
omits occurrence identity/status/due time and request format. Authenticated
`/care_plan?kind=medication` showed four HTML History rows and no stale open rows;
the JSON read model returned zero late/missing items older than 8 h and five
older terminal items. This is consistent with terminal History being retained
and JSON being uncapped, not with a demonstrated HTML cap failure. Reopen only
with an identified late/missing OPEN row older than its persisted kind cap on
HTML `/care_plan`, including animal/source, due timestamp, and saved preference.
Do not reopen the incident or claim it reproduced without that counterexample. The separately confirmed inclusive cap-boundary change is a preference/UI contract, not an incident finding.

**Regression test/evidence.** Original test `TestB10_9PreferenceCapsLateAndFutureBoundaries` treated exact cap as included; Goal 1 replaces that contract with `TestPreferenceCapsExcludeAtAndBeyondLateAndFutureBoundaries`, asserting exact and beyond cutoff exclusion, just-inside retention, and unchanged terminal rows. The live preferences page showed 8 h late/future values and 1 h now window for all six kinds; handler loads persisted values and applies caps before viewmodel construction.


The user also reported variable-width Animal cells on `/care_plan`. Set fixed
11rem width/min/max on shared per-animal columns in all four localized index
forks; medication label overflow ellipsizes. Updated phase0b golden baseline
because rendered style is intentionally different. Verification:
`PHASE0B_RECORD=1 go test ./actions -run '^TestCarePlanPhase0bDOMEquivalence$' -count=1` — PASS.

---

Companion of `bugs.md` round 10. Each bug: root cause → fix → test approach →
Shared verification for every item:

```bash
go vet ./...
staticcheck ./...
gocognit -over 15 .
gocyclo -over 12 .
go test -count=1 -race -cover ./...
```

E2E evidence with the agent-browser skill on `buffalo dev` (admin/admin),
per item below; each page also smoke-loaded in `fr`, `de`, `nl`.

---

## B10-7 — cleanup/feeding show scheduled occurrences within the future horizon

**Root cause.** `careItemCounts` (viewmodel.go:1555) and `feedingViewOf`
(viewmodel.go:1393) drop every `StatusScheduled` item. Status is `scheduled`
until `due − lookahead` (schedule default 60 min), so a 09:00 cleanup rule
renders an empty tab until 08:00 even though the per-kind preference cap
(`future_show_hours = 8`, applied in `CarePlanIndex` before the view model)
explicitly keeps those items in `plan.Items`, the summary strip's "Later"
bucket counts them (`statsOf` → `IsCurrent` passes scheduled items), and the
apply endpoint accepts applicable scheduled occurrences.

**Fix.**
1. `careItemCounts`: drop only `!openStatusAction(it.Status) || !it.Applicable`
   — remove the explicit `StatusScheduled` exclusion. Everything reaching the
   builder already passed the caps (or has no caps). Update doc comments.
2. `feedingViewOf`: same — remove the `chip.Status == StatusScheduled`
   condition; superseded chips stay dimmed info chips, terminal chips stay off.
3. Comment updates: `feedingViewsOf` header ("OPEN work only: superseded and
   scheduled chips stay off") now reads "superseded stay off; scheduled
   within the future horizon render as open work".

**Tests.**
- New/updated unit tests: a scheduled cleanup item within the horizon yields a
  `CareView` row with an applicable slot + batch ref; same for a feeding chip
  (`care_plan_viewmodel_test.go` family). Assert the old exclusion tests are
  re-pointed (scheduled BEYOND a cap is dropped at `applyPreferenceCaps`
  level, not builder level).
- `go test ./actions -run 'Cleanup|Feeding|CareItem' -count=1`.

**Validation (e2e).** `/care_plan?kind=cleanup` before 08:00 (or with a rule
whose time is later today) lists the cage rows with `○ HH:MM` slots; the
toggle applies (201) and the row flips. Same smoke for feeding.

---

## B10-3 — toggle-with-time nomenclature for feeding + single-occurrence pairs

**Root cause.** The feeding per-animal toggle (`_plan_tier_feed_table`
lines 122-137) renders an icon-only clock; the single-occurrence apply/undo
pair of `_plan_item_line` (lines 145-169) renders an icon-only clock next to a
bare time text. The rest of the work screen (cleanup slots, med slots, merged
item slots) uses `○ HH:MM` ⇄ `✓ HH:MM`.

**Fix.**
1. `_plan_tier_feed_table.plush.html`: apply button text `○ <%= chip.DueHM %>`,
   undo button text `✓ <%= chip.DueHM %>` (FeedingChip.DueHM exists). The
   time-group label above stays (it carries the DAY for non-today groups).
2. `_plan_item_line.plush.html` single-occurrence variant: drop the bare
   `<%= card.DueHM %>` text (keep the day badge), apply button `○
   <%= card.DueHM %>`, undo `✓ <%= card.DueHM %>`.
3. `setToggleState` pair-row path swaps visibility only (no text rewrite) —
   verified, no JS change needed.
4. Copy both templates over their `.fr/.de/.nl` forks (byte-identical forks).

**Tests.** `actions/care_plan_i18n_html_test.go` locale-fork convention test
still passes; add an assertion that the feeding toggle carries `○ ` + time if
a fixture exists (feeding table markup test).

**Validation (e2e).** `/care_plan?kind=feeding`: per-animal toggle reads
`○ 16:00`, flips to `✓ 16:00` on click, undo restores. Animal page plan tab
feeding row same. No duplicated time text next to single-occurrence toggles.

---

## B10-2 — Zone · Cage · Espèce on every per-animal row

**Root cause.** The view models carry the data (CardView.Zone/Cage,
MedGroupView.Species/Zone/Cage) but the templates never render it; the
animal-mode builders (`feedingAnimalViewsOf`, `careAnimalViewsOf`) and
`cardFor` never populate Species.

**Fix.**
1. View models: add `Species` to `FeedingGroupView`, `CareView`, `CardView`;
   populate from `plan.AnimalRow(...)` in `feedingAnimalViewsOf`,
   `careAnimalViewsOf`, `careViewsOf` (cage rows too — the cage cell already
   shows the zone; species belongs to the ANIMAL-mode rows only, but the cage
   row keeps its existing shape), and `cardFor`. Species names are already
   request-localized via `localizePlanSpecies` (handler runs before
   `BuildDayPlanView`); templates print them via `tspecies(...)` for the
   fallback path, matching the dashboard's usage.
2. Templates — one shared muted info line class `.plan-loc`:
   - `_plan_tier_feed_table.plush.html` animal cell: caption becomes
     `Cage · Zone · Espèce` (replaces the cage-only caption).
   - `_plan_care_line.plush.html` animal cell: same caption added.
   - `_plan_med_row.plush.html` animal cell: adds the `.plan-loc` line
     (care_plan index only; the dashboard table has its own Cage/Species
     columns and does not use `_plan_med_row`).
   - `_plan_item_line.plush.html` showAnimal cell: adds the `.plan-loc` line.
   - `_plan_history_table.plush.html`: new narrow `<td>` after the animal
     cell with the `.plan-loc` line.
3. CSS (`assets/css/care-plan.scss`):
   `.plan-loc { display:block; font-size: .78rem; }` with a responsive
   collapse `@media (max-width: 991.98px) { .plan-loc { display:none; } }`
   ("if space does not permit, it collapses"); shared first-column width for
   the per-animal tables
   (`.plan-feed-animal, .plan-care-animal { min-width: 11rem; }` + a shared
   `.plan-loc` max-width) so the stacked tables align.
4. Copy all touched templates over the 4 forks.

**Tests.** View-model unit tests assert Species/Zone/Cage on the animal-mode
rows; the existing phase-4/round-4 row-kind tests keep passing with the new
field. i18n HTML test (fork convention) passes.

**Validation (e2e).** `/care_plan?kind=feeding&group=animal`,
`?kind=cleanup&group=animal`, `?kind=medication`, `?kind=observation` and the
history section: each per-animal row shows `Cage · Zone · Espèce`; narrow
viewport (≤ 991px) hides the line; the first column width matches between the
feeding and cleanup tables.

---

## B10-4 — dashboard medication: ℹ popup in place, eye removed, animal link → Treatment tab

**Root cause.** The `#planDetailModal` is on the dashboard (via
`_apply_toggle`) but the `.plan-detail-btn` opener JS is not; `medSeriesEye`
adds the dashboard-only eye; `mg.AnimalLink` = `cardAnimalLink` → `#nav-plan`.

**Fix.**
1. `templates/dashboard/dashboard.plush.html` (+ 3 forks): remove
   `<% let medSeriesEye = true %>`; add a small delegated opener script
   (model: `animals/show.plush.html:1318-1355`) that fills `#planDetailModal`
   from the button's `data-*` attributes — the animal row stays a LINK in this
   context (the dashboard is not the animal's page).
2. `templates/care_plan/_med_series.plush.html` (+ 4 forks): remove the eye
   block and its comment (`medSeriesEye` has no setter left).
3. Dead-code sweep behind the eye: `MedSlotView.DeepLink`,
   `animalItemDeepLink`, `resolvePlanItemDetail` (+ `planItemDetail`,
   `openPlanDetail` in the Show handler and the show-template deep-link
   block) exist ONLY to serve the eye's URL. Remove the whole chain and its
   tests (grep `DeepLink|resolvePlanItemDetail|openPlanDetail|planItemDetail|"?item="`).
4. `buildMedGroups`: after the series assembly, build the animal link with the
   new `cardAnimalTreatmentLink(animalID, back, firstSeriesRef)` →
   `/animals/<id>?back=<back>&med=<source_type>:<source_id>#nav-treatment`.
   (The care_plan index med rows share `buildMedGroups`; medication →
   Treatment tab is the correct target there too.)
5. `templates/care_plan/_med_series.plush.html`: stamp each series line root
   with `data-source-type` / `data-source-id` (from
   `series.Rows[0].Slots[0]`) — the scroll anchor for (4) and B10-5 reuse.
6. `templates/animals/show.plush.html`: on load, `?med=<type>:<id>` activates
   the Treatment tab, finds the first
   `#treatmentPlan .plan-med-line[data-source-id="<id>"]`, opens its day card,
   scrolls it into view and highlights it (pattern of the existing `?src=`
   trace script, lines 1274-1286).

**Tests.** Update `care_plan_i18n_html_test.go` (`.dash-med-view` assertions
→ removed-eye assertions), `care_plan_viewmodel_test.go` (DeepLink test →
AnimalLink treatment-tab test); new test for `cardAnimalTreatmentLink` shape.

**Validation (e2e).** Dashboard: ℹ opens the popup in place (fields filled,
treatment link row when applied); no eye button anywhere; animal button lands
on `#nav-treatment` scrolled+highlighted to the treatment series. `go test
./actions -run Dashboard -count=1`.

---

## B10-5 — protocol tab: uniform entries, no type bubble, per-type separators

**Root cause.** `showKind: true` renders the grey kind badge on every
non-med entry; the med-series lines on the animal page carry only
`plan-med-line` (missing the `plan-med-row` compact font/row treatment the
item lines get); kinds interleave within a day.

**Fix.**
1. `_plan_item_line.plush.html`: remove the `showKind` bubble block and the
   flag; drop `showKind` from every caller (`_plan_tier_rows.plush.html`,
   `templates/animals/show.plush.html:833`) — all 4 forks of the partial.
2. `templates/animals/show.plush.html` day loop: track the previous item's
   kind; when it changes emit
   `<div class="plan-kind-separator"><%= t("care_plan.kind." + kind) %></div>`
   (localized titles already exist: `care_plan.kind.*`). Items render
   kind-grouped: sort the day's items by kind (stable, Go side —
   `animalTreatmentDays`/`animalDayCardFor` already produce the items) so the
   separators are stable.
3. `_med_series.plush.html` line div: add `plan-med-row` when the caller sets
   `medSeriesRow` (new optional flag — set `true` on the animal page only, so
   the care_plan index and dashboard shapes are untouched).
4. CSS: `.plan-kind-separator` (small-caps muted band with top border,
   `care-plan.scss`).

**Tests.** Update the Phase-5/D5 tests that pass `showKind`; new test
asserting the day items are kind-grouped; i18n fork test green.

**Validation (e2e).** `/animals/8635#nav-plan`: no grey badges; each day card
groups its entries under localized type separators; med series and item lines
share font size/alignment (same row family).

---

## B10-6 — legacy treatments covered by the protocol render superseded; protocol label regains the dosage

**Root cause.** The converter created the plan but left the source treatments
live and dropped `treatments.dosage` for unknown-drug (observation) plans; the
legacy accordion renders every treatment row with no protocol cross-check.

**Fix.**
1. **Superseded marks (display only, no data change).**
   `setAnimalShowPlanData` (actions/animals.go) builds
   `treatmentSupersededByPlan map[string]bool` (treatment UUID → true):
   for every ACTIVE converter-made plan of the animal (`created_by IS NULL`),
   core = `payload.prompt` (observation) or `payload.drug` (medication),
   lowercased/trimmed; a treatment with `date >= today` whose lowercased/
   trimmed drug matches the core is marked. Template: today+future accordion
   rows with the mark render muted (`plan-legacy-superseded`), label struck
   through, badge `t("care_plan.animal_plans.superseded_by_protocol")` (new
   key ×4 locales), no bitmap buttons.
   `animalPlanLegacyDrugs` (today-card dedupe) skips superseded treatments so
   the PROTOCOL row is the actionable one on the today card.
2. **Converter fix.** `convertTreatmentSeries` unknown-drug branch: when the
   series dosage is non-empty, prompt = `<drug> (<dosage>)` and name =
   `Traitement — <drug> (<dosage>) (à vérifier)`; medication branch unchanged.
3. **Data migration (additive, idempotent) for existing converted plans.**
   Raw SQL migration: for active, `created_by IS NULL`, `action_kind =
   'observation'`, `name LIKE 'Traitement — %'` plans, join the animal's
   treatments on `LOWER(TRIM(drug)) = LOWER(TRIM(payload.prompt))` where the
   matching dosages are ONE distinct non-empty value, then
   `payload = JSON_SET(payload, '$.prompt', CONCAT(prompt, ' (', dosage, ')'))`
   and `name = REPLACE(name, 'Traitement — <prompt>', 'Traitement — <prompt>
   (<dosage>)')`; guarded against re-application (prompt already containing
   the dosage). Down migration restores the previous prompt/name (reverse
   replace, `JSON_SET` back) for symmetry.
4. **Today-card dedupe robustness.** `animalPlanLegacyDrugs` also collects
   `drug (dosage)` composites so the enriched prompt still dedupes against
   legacy rows that are NOT superseded (e.g. non-converter plans).

**Tests.** New unit tests: superseded-map building (observation + medication
cores, today+future scope, past treatments untouched, caretaker plans
ignored); converter test with dosage (prompt/name carry it); today-card
dedupe with composite labels. Migration tested against the dev DB copy
semantics: run `buffalo pop migrate up` then re-run (idempotent), verify
animal 8635 plan prompt.

**Validation (e2e).** `/animals/8635#nav-treatment`: the Nettoyage Fistule
rows of today+future render muted with the "covered by protocol" badge and no
3-button block; past days unchanged; `/animals/8635#nav-plan` shows
"Nettoyage Fistule (Dessus oeil droit)"; the today card shows the protocol
row (actionable) instead of the dead bitmap row.

---

## Execution order

1. B10-7 (engine semantics; view-model only) — unlocks the e2e evidence for
   the tab the other items render on.
2. B10-3 + B10-2 (template nomenclature + loc line; same files).
3. B10-5 (protocol tab structure, reuses `_med_series` data attributes).
4. B10-4 (dashboard + deep-link chain removal + `?med=` scroll).
5. B10-6 (superseded marks + converter + migration; largest blast radius last).
6. Quality gates + full suite + e2e sweep (all locales) per bugs.md.

## Commit plan

One commit per bug (bugs.md guideline 8), each with its validation evidence;
migration commit includes the idempotency proof.
