# Bugs — open list

Running bug list for the current round. Add every new report here, fix it with
a fix plan, validate, then move the resolved entries to `docs/archive/` when the
round is closed.

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