# Legacy Treatments — Critical Assessment & Migration Plan

**Status:** OPEN · **Created:** 2026-10-03 · **Trigger:** user report —
"the complete treatment (e.g. `/treatments/new/?animal_year_number=1982/26&back=/animals/10291/edit#nav-treatment`)
should be superseded by the new engine" and "animal 10291 Plan tab shows an
*older* treatment (2026/09/30) next to the Protocoles — there should not be
multiple entries, everything should be under protocols".

This document logs every legacy-treatment surface found in the code, the
evidence that they are still live, and the discrete goals to retire them.
It complements `docs/care-expert.md` (§2, §7, §8.3) which already declared
the intent: *"extends the existing feeding/treatment/care features and
replaces their scheduling parts at rollout"* and *"rule + application log
replaces it cleanly"* — but never named `/treatments` CRUD as a page to
retire. That gap is what this plan closes.

---

## 1. Bug fixed on the way (tab corruption, 2026-10-03)

While reproducing the 10291 report, a **structural template bug** was found
and fixed in `templates/animals/show.plush.html` (all 4 locales):

`#nav-treatment` was closed **twice** — a stray `</div>` + stray `<% } %>`
right after the engine-days block (old lines 505-506). Consequences before
the fix:

- the legacy-treatments accordion and **every later pane** (`#nav-plan`,
  `#nav-media`, `#nav-audit`) fell **outside `.tab-content`**, so Bootstrap
  never hid them: loading `/animals/{id}#nav-plan` rendered General +
  Protocol + Media + Audit **stacked in one scroll** ("repeat of the animal
  content");
- the legacy treatment-day card (e.g. "2026/09/30 — Finadyne, Myogaster-E,
  TRANQUINERVIN, Vitamines B") leaked onto the **Protocol tab**, which is
  what made it look like "multiple entries" next to the protocols.

After the fix all panes are inside `.tab-content` and only the active one
renders (verified in DOM + screenshots, EN/FR). The legacy card now lives
only on the Treatment tab.

---

## 2. Inventory — where the "old" treatment approach is still live

| # | Surface | Location | State |
|---|---------|----------|-------|
| L1 | Global navbar **New ▸ Treatment** (all locales) | `templates/application.plush*.html:162` → `newTreatmentsPath()` | **Live, one click from every page** |
| L2 | Animal show ▸ Treatment tab: **"Add New treatment"** button | `templates/animals/show.plush*.html:449` | **Live** |
| L3 | Animal **edit** ▸ Treatment tab: same button (the URL the user cited) | `templates/animals/_form.plush*.html:751-772` | **Live** |
| L4 | Legacy **full CRUD** resource + pages (index/new/show/edit, 4 locales each) | `actions/treatments.go` (`TreatmentsResource`), `templates/treatments/*` | **Live** |
| L5 | Legacy **bitmap toggle** endpoint (`POST /treatments/{id}/entries/{id}/toggle`) | `actions/app.go:168`, used by `templates/treatments/show.plush*.html:103` | **Live** (spec §8.3 only made `/treatmentschedule` read-only, not this) |
| L6 | **Legacy read-only history** block under the engine days, Treatment tab (date cards, bitmap badges) | `templates/animals/show.plush*.html:~500-680` | Live **by design** (history), but visually competes with the engine cards |
| L7 | `animalPlanToday` dedupe layer (engine occurrences merged into the legacy list look) | `actions/care_plan_animal_page.go` (`animalPlanToday*`), show template ~636-674 | Live — transitional shim |
| L8 | `TreatmentEntriesMap` / bitmap helpers (`ScheduleStatusMorning/Noon/Evening`, `TreatmentBoolToBitmap`) | `models/treatment.go` | Live — legacy data model still first-class |

### What is NOT legacy (keep)

- The `treatments` **table** itself: the engine's apply path writes
  fulfillments there (`fulfillment_type='treatment'`,
  `actions/care_plan_fulfillment.go:107`) and links them to applications
  (undo, view-record links). The storage model stays; the **direct-CRUD
  entry points** are what must go.

## 3. Evidence the old path still dominates (dev DB, prod-shaped copy)

```
total treatments ................................ 58,045
linked to care_plan_applications (engine) ....... 4
created since cutover 2026-09-27 ................ 249
… of which NOT via the engine ................... 246  (98.8%)
```

The engine has been live for a week and ~99% of new treatment rows still
bypass it. The user's instinct is correct: the old approach is not
"superseded in practice" — it is still the path of least resistance
(navbar + animal-page buttons all point at it).

## 4. Why this matters (critical assessment)

1. **Double-recording risk, by design impossible but practically easy.**
   The engine dedupes/overrides protocol occurrences; a treatment created
   via L1-L4 is invisible to the engine (no plan, no occurrence), so a
   caregiver can run a protocol (engine) *and* hand-created treatment rows
   for the same drug — the day plan shows one, the Treatment tab shows both.
2. **Two truths for "what was given".** Engine applications carry
   fulfillment links and undo; legacy bitmap rows have their own
   done-marks. The 10291 report is the visible tip: the same animal had a
   legacy multi-drug treatment day *and* protocol-driven series.
3. **The spec's promise is only half-implemented.** §8.3 retired `/feeding`
   and `/treatmentschedule`; nothing retired `/treatments` CRUD. The
   conversion moved old *data* (102 rows/day plans), but the old *workflow*
   stayed one click away — so new legacy rows keep accumulating (246 in a
   week) and will need yet another conversion later.
4. **UX confusion is the leading indicator** — the 10291 report and the tab
   corruption were both caused by legacy/engine content mixing in one view.

## 5. Migration goals (discrete, ordered, each independently verifiable)

**G1 — Stop advertising the legacy create path (UI).**
Remove/hide: navbar New ▸ Treatment (L1), "Add New treatment" on the animal
show Treatment tab (L2) and on the edit page (L3). The animal Protocol tab's
"**Nouveau protocole**" (carePlanModal) is the replacement for direct
one-animal creation (spec §7: "treatment-style — no matcher").
*Accept:* no `newTreatmentsPath` link rendered anywhere outside
`/treatments/*` pages; all 4 locales.

**G2 — Make one-off ad-hoc medication a first-class engine source.**
Today the engine covers *recurring* schedules well; a true one-shot ("give
Finadyne once now") is still easier as a legacy row — that is why caregivers
keep using L1-L4. Add a minimal flow: protocol modal preset to a single
occurrence today (schedule with one slot, or a "one-off treatment" preset of
`care_animal_plans`), landing on the Plan tab.
*Accept:* a caregiver can record a one-off drug from the animal page in ≤3
clicks, and it appears in the day plan + Treatment tab engine series.

**G3 — Read-only the legacy CRUD (routes).**
Keep `GET /treatments`, `GET /treatments/{id}` (history + fulfillment
records); gate `POST/PUT/DELETE /treatments*` and the bitmap toggle (L5)
to admin-only, or 410/redirect to the plan modal for non-admins.
*Accept:* non-admin cannot create/update/delete legacy treatments; engine
fulfillment records still viewable/undoable.

**G4 — Migrate the residual open legacy schedule rows.**
The 102 animal-plan conversions covered feeding; **open (not-done) legacy
treatment rows** (spec §2: 281 open slots at review time) were converted —
verify and re-run the converter for animals still relying on open legacy
schedules created *after* the cutover (246 rows since 2026-09-27, of which
the open ones should become care_animal_plans).
*Accept:* no in-care animal relies on an open bitmap-only treatment for
future days; `careplan:fixnames`-style idempotent grift available.

**G5 — Collapse the Treatment tab into the Plan tab (single entry).**
Once G1-G4 hold, the animal Treatment tab keeps only: engine series
(medication) + read-only fulfillment history. The legacy date-card block
(L6) and the `animalPlanToday` shim (L7) merge into the engine history
rendering; the tab becomes a filtered view of the same engine data the
Plan tab shows (its stated design, R3-6/R4-7.23).
*Accept:* no legacy bitmap badges rendered for dates after the G4
migration horizon; the two tabs cannot show contradictory rows for the
same drug/day (regression test).

**G6 — Cleanup (post-cutover, later).**
Delete `TreatmentTemplate` flow, bitmap helpers (L8), `treatments/new|edit`
templates; keep List/Show for history. Update `docs/care-expert.md` §8.3 to
name `/treatments` CRUD retired (it currently doesn't).

### Risks / notes

- **Wound-care (`NoDrug`) flow** (issue #96) lives in the legacy form —
  G2 must carry it (a `care_animal_plans` variant or care-kind rule) before
  G3 gates the form.
- **Fulfillment storage stays**: plan applications keep writing
  `treatments` rows; reports/export reading `treatments` are unaffected.
- **Roles**: `roleAllows` extensions (spec line ~877) already let `spw` view
  records; gating writes must respect the same role map.

## 6. Traceability

- User reports (2026-10-03): 10291 "older treatment … should not be
  multiple entries"; "`/treatments/new` should be superseded"; "log all
  these elements and plan the work using dedicated goals" → this document.
- Fixed same day: tab-structure corruption (`show.plush*.html`, all 4
  locales) — see §1.
- Related: `docs/care-expert.md` §2 (treatments wound), §7 (two planning
  levels), §8.3 (retired pages), `care_plan_animal_page.go` (engine day
  assembly, `dayItemsWithoutMisses`, `mergeSameSourceFeedings`).
