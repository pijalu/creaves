# Care plan round 10 — fix plan (B10-2 … B10-7)

Companion of `bugs.md` round 10. Each bug: root cause → fix → test approach →
validation. Session constraints apply (bugs.md): real production data — no
destructive migrations; every UI change lands in all four locales
(`en-US`, `fr`, `de`, `nl`).

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
