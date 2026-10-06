# Bugs — open list

Running bug list for the current round. Add every new report here, fix it with a
fix plan, validate, then move resolved entries to `docs/archive/` when the round
is closed.

**Guideline** (same convention as archived rounds):
1. Create a detailed fix plan for each bug; execute and validate it.
2. Fix all issues found during testing and update the fix plan.
3. Archive each resolved entry and its plan under `docs/archive/`.
4. Test all changes, including authenticated agent-browser E2E; record commands,
   URLs, and captured output.
5. Check quality gates separately: `go vet ./...`, `staticcheck ./...`,
   `gocognit -over 15 .`, `gocyclo -over 12 .`,
   `go test -count=1 -race -cover ./...`.
6. Commit each fix with a clear, descriptive message.

**Session constraints:** Creaves contains production data; never destroy data.
Every UI change must be implemented in all four locales (`en-US`, `fr`, `de`,
`nl`).

---

## Open items

### Third user review batch (2026-10-06, evening) — 8 items + hors-délai note

**Status:** FIXED and VALIDATED (2026-10-06 late round) — pending user acceptance
before archiving. All four locales for every touched template; quality gates
green (`go vet ./...`, `staticcheck ./...` clean after removing two pre-existing
findings in untouched-by-this-round files, `go test -count=1 -race -cover ./...`
all packages OK, `git diff --check` clean; gocognit/gocyclo show only
pre-existing offenders in files this round did not touch). New regression pins
in `actions/care_plan_third_batch_test.go` (template pins over all four locale
forks + viewmodel tests); phase0b DOM baselines re-recorded
(`PHASE0B_RECORD=1`).

**Fixes (implementation):**
1. Feed-table column blow-up: removed the `width: max-content` cell rule
   (Chrome resolves it against the table's one-line width), added
   `max-width: 100%` to tier tables + wrap for the diet text — all four
   index locale forks. Pinned by `TestFeedTableColumnNeverMaxContent` and the
   goal3 layout test (now asserts the rule is GONE, not present).
2. Feeding/cleanup History: `historyRows()` no longer skips grouped kinds —
   terminal + superseded occurrences land in the same History section as
   medication (dedupe per source × animal, undo + fulfillment links kept).
   Pinned by `TestFeedingTerminalOccurrencesLandInHistory` and the updated
   cleanup-history pin in `care_plan_links_test.go`.
3. Preferences: one bulk form (`#preference-bulk`, HTML5 `form=` wiring,
   per-kind names `late[<kind>]/future[<kind>]/now[<kind>]`) + ONE Save button
   + bulk handler `PreferencesSaveAll` (`POST /preferences/save`); the
   per-row route/handler removed. Pinned by `TestPreferencesBulkSaveContract`.
4. Uniform toggles: `_plan_slot_toggle` late-recordable now speaks the same
   `○` to-do glyph (title keeps "record late"); skipped/deferred render as
   same-size non-interactive markers (`⊘/⏸`, new `.plan-med-state` btn-sm
   metrics); hors-délai renders a `🔒 HH:MM` non-interactive marker
   (`.plan-med-locked`); `_plan_item_line` late branch uses `○`, its
   out-of-window branch uses the 🔒 marker. Pinned by
   `TestSlotToggleUniformGlyphsAndMarkers`, `TestItemLineNoDisabledButtonsInSlots`,
   updated `TestMedSeriesPartialRenders`/`TestMedSeriesPartialTierClasses`.
5. Confirm ⇒ execute: `_plan_med_toggle.applySlot` now SENDS `late: true`
   (and keeps it through the dosage dialog); `_apply_toggle.instantApply`
   forwards `data-late` and retries ONCE as a late record on a 409 for
   past-due refs (stale render race, incl. a modal held open across a slot
   boundary); the batch flow retries apply-window failures per item
   (`lateRetryOne`); `openApply` honors `data-late` (modal path); the
   history Record-late `isInputKind(string)` TypeError fixed (routes the
   element). Server bound unchanged and pinned: `TestServerKeepsRejectingFutureLateRecord`
   (future-due + late still 409s). Live-validated on :3002: late slot confirm
   → `✓ 08:00`, no error modal.
6. Medication merge: `mergeMedLinesByAnimal` — one line (one animal cell)
   per animal per tier, its series stacked inside, most-urgent first; Done
   lines merge the same way while the badge still counts SERIES.
   Pinned by `TestMedLinesMergePerAnimalWithinTier`,
   `TestMedTierViewHoldsOneLinePerAnimalPerTier`, updated done-tier pin.
7. Care line = medication line: `_plan_item_line` renders
   `[animal][ℹ][label][buttons]` (medication order) with the same computed
   shared animal-column width (`cardAnimalColCh` → `view.MedAnimalColCh`,
   computed for row kinds too). Pinned by
   `TestItemLineAnimalCellPrecedesInfoButton`, `TestCareViewSharesAnimalColumnWidth`,
   `TestItemLineRendersMergedAnimalCell`.
8. Protocol tab: the medication block carries its own
   `.plan-kind-separator` title (`care_plan.kind.medication`, localized) and
   the animal-page med line uses the shared `plan-item-line` band
   (`_med_series` medSeriesRow). No animal cell on the tab (unchanged).
   Pinned by `TestAnimalProtocolTabMedicationSeparator`,
   `TestMedSeriesAnimalPageLineSharesItemLineBand`.

**E2E evidence (agent-browser; screenshots under `/tmp/e2e_3rd_*.png`):**
- :3000 read-only (production DB): feeding `group=animal` — 185 toggles
  visible, 0 clipped, table 1108 px inside the 1110 px panel
  (`e2e_3rd_1_...png`); medication — 23 rows / 23 unique animals (was
  1905/26 ×4), only `○`/`✓` glyphs, merged 1905/26 block with 4 series in
  one row (`e2e_3rd_4_6_...png`); care — row child order
  `[animal, lead, label, btns]` + `min-width: 40ch` shared cell
  (`e2e_3rd_7_...png`); protocol tab — `Medication/Feeding/Cleanup`
  separator bands per day, med line on the shared band, 0 animal cells,
  hors-délai feeding slot renders the 🔒 marker (`e2e_3rd_8_...png`).
- :3002 fixture (clone DB, mutations allowed): late medication slot
  confirm → executed, button flipped `✓ 08:00`, no error modal (fix 5;
  reloaded view captured in `e2e_3rd_5_late_confirm_executed.png`: the
  applied `✓ 08:00` toggle on the 1990/26 amoxiclav series inside the Late
  tier, with the "History 5" section header directly below — every visible
  toggle carries the uniform `○`/`✓` glyphs and the merged per-animal
  blocks); feeding apply → History section "History 392" with the applied
  row, fulfillment link and undo (fix 2, `e2e_3rd_2_...png`); preferences —
  exactly 1 Save, 0 per-row forms, Cleanup late 24→20 + Care now 1→2 both
  persisted in one click, untouched kinds preserved, success flash, fixture
  values restored afterwards (fix 3, `e2e_3rd_3_...png`).
- Four locales live (fr/de/nl + en): localized kind titles ("Médication /
  Nourrissage / Nettoyage"), localized page titles, only `○`/`✓` glyphs and
  zero animal-cell repeats on every locale's medication view.

**Open (carried to acceptance):** the two flagged working assumptions were
used as specified in the items below (🔒 marker for hors-délai; within-tier
merge) — re-confirm with the user before archiving.

**User directives for this batch (2026-10-06):**
- Bug 5 refinement (mid-review note): *if an item is hors délai to apply — instead
  of returning an error, DO NOT SHOW THE ACTION.* Combined with the original report
  ("confirm ⇒ execute"), the target behavior is: an occurrence that is late but
  still in its apply window must apply on confirm without error; an occurrence past
  its apply window must present no action affordance at all (server + client), so
  the 409 `hors délai` path becomes unreachable from the UI.
- Bug 8 hard requirement: reuse ONE component between the day plan and the animal
  Protocol tab to *enforce* a global approach instead of divergent views.

Per-item qualification — each item states: REQUIRED (the user requirement, fully
qualified), EVIDENCE (reproduction / root cause from this round's review + audit),
and ACCEPTANCE (the checks fix-validation must pass; a fix is done only when every
check of its item passes).

1. **Feeding `group=animal` — Apply buttons must be visible.**
   - REQUIRED: on `/care_plan?kind=feeding&group=animal`, every feeding row whose
     occurrence is actionable renders its per-animal toggle (`○ HH:MM`) fully
     visible inside the tier panel at desktop and narrow widths; the row's action
     column must never be clipped, hidden, or pushed out of the panel.
   - EVIDENCE: buttons exist in the DOM (185 `.plan-feeding-one`, none `d-none`)
     but render off-panel: `.plan-tier-body td.plan-med-animal { width: max-content; }`
     (index.plush.html ~line 79) + `.plan-feed-animal { white-space: nowrap; }`
     inflate column 1 to 1466 px (table 1553 px inside an 1110 px `.plan-tier`
     with `overflow: hidden`); live bisect (`width:auto` on the column) collapses
     the table to 1108 px and the buttons reappear.
   - ACCEPTANCE:
     a) Automated DOM-geometry assertion (not presence): for every
        `.plan-feeding-row` containing `.plan-feeding-one`, the button's bounding
        rect lies fully inside its enclosing `.plan-tier` rect (right edge ≤ panel
        right edge; no intersection with the clip boundary), run at ≥1440 px and
        ~1024 px viewports.
        Presence-only checks are explicitly insufficient — they passed in the
        previous round while the buttons were invisible.
     b) Rendered table width ≤ tier panel width on `kind=feeding` for BOTH group
        values (group=cage layout unchanged).
     c) Browser E2E screenshot on the fixture shows toggles on screen for late,
        now and later tier rows (later rows are applicable, so they must show
        `○` toggles too).
     d) No locale-file change required (layout-only), but the four locale pages
        still render (smoke).

2. **Feeding/Cleanup — done actions move to History, never disappear.**
   - REQUIRED: after any action (single apply, cage batch apply, skip, defer) on
     `kind=feeding` or `kind=cleanup`, the occurrence stops being work AND remains
     visible in a History section on the same page — same mechanism as medication
     (collapsed History tier; applied state + time, undo, fulfillment-record link).
     An applied action must never vanish from the view.
   - EVIDENCE: `historyRows()` (care_plan_viewmodel.go:1311) drops every
     `groupedKind()` occurrence; verified live — `#plan-history` absent on
     `kind=feeding` and `kind=cleanup`, present on `kind=medication` (11 rows).
   - ACCEPTANCE (on the :3002 fixture — mutations allowed there only):
     a) Apply one feeding occurrence (single-animal row, group=animal) → after
        reload the page shows a History section containing that occurrence with
        applied time, undo, and a link to the created cares row.
     b) Apply one cage batch (group=cage) → History contains the applied
        occurrence(s); the batched animals appear (grouped rows acceptable).
     c) Undo from History returns the occurrence to its urgency tier.
     d) `#plan-history` present on `kind=feeding` AND `kind=cleanup` whenever
        terminal/superseded occurrences exist in the window; History header count
        equals rendered row count.
     e) Same checks for a cleanup batch (per-animal care rows in History).
     f) History strings localized in en-US/fr/de/nl.

3. **Preferences — exactly ONE Save that persists ALL modified kinds.**
   - REQUIRED: `/preferences` renders a single Save control; one click persists
     every modified value on the page (all six kinds), not just one row.
   - EVIDENCE: one `<form id="pref-<ID>">` per row posting
     `/preferences/<id>/save` (actions/preferences.go:126); 6 rows / 6 forms /
     6 Save buttons counted live.
   - ACCEPTANCE:
     a) DOM: exactly one submit Save control on the page (count == 1); per-row
        forms are gone or merged.
     b) Functional (fixture): change ≥ 2 kinds (e.g. Cleanup late 24→20 and Care
        now-window 1→2), click the one Save → both persisted after reload; all
        unmodified rows keep their stored values (no accidental overwrite).
     c) Semantics preserved: empty field = "no cap" (NULL); `NowWindowHours`
        stored ×60 as minutes; out-of-range values still rejected with the
        existing validation flash — shown once, not per row.
     d) Stepper JS (−/+) still works for every row.
     e) Any new/changed label localized in the four locales.

4. **Uniform to-do buttons — same sign, same size; hors-délai shows no action.**
   - REQUIRED: every actionable to-do toggle in the plan UI shows the SAME sign
     (`○`) at the SAME size (`btn-sm`); no clickable control may render for a
     hors-délai (out-of-apply-window) position — replaced by a non-interactive
     marker (working assumption: muted `🔒 HH:MM`, same precedent as the
     item-line lock; flagged to the user, no answer yet). Skipped/deferred keep
     their state visible as non-oversized non-action markers; `✓` undo stays.
   - EVIDENCE: `_plan_slot_toggle` renders `– HH:MM` for late-recordable and for
     disabled slots, and its skipped/deferred/disabled branches omit `btn-sm`
     (≈38 px vs 26 px — the "bigger buttons"); `_plan_item_line` uses `–` for its
     late/disabled slots too. 6 `–` buttons counted live on `kind=medication`.
   - ACCEPTANCE:
     a) DOM census on `/care_plan` for every kind + the animal Protocol tab +
        dashboard med table: zero toggle buttons whose text starts with `–`;
        every clickable to-do button starts with `○`.
     b) Size census: every rendered action/toggle button (○/✓ and any badge-style
        markers) has the same computed height (btn-sm, ≈26 px); zero
        default-size (`no btn-sm`) plan buttons.
     c) Hors-délai positions render no `button`/`a` element (marker only); zero
        clickable controls carry `data-late="true"` for occurrences whose
        occurrence is past the apply window AND not late-recordable — server
        `Applicable=false` + not `LateAllowed` ⇒ no action affordance.
     d) Late-recordable occurrences (past due, still recordable) keep an ○-signed,
        btn-sm toggle; its title/tooltip still says "record late" (localized).
     e) All four locales.

5. **Confirm ⇒ execute; the hors-délai 409 must be unreachable from the UI.**
   - REQUIRED (user's two statements combined):
     (a) confirming a late-but-recordable action executes it — no error dialog;
     (b) an occurrence hors délai to apply shows NO action at all (directive
     mid-review), so the 409 "occurrence is out of its apply window (hors délai,
     §10-A1)" can never be triggered by a normal UI path.
   - EVIDENCE: `_plan_med_toggle.applySlot()` POSTs `/care_plan/apply` WITHOUT
     `late:true` even after the `data-late` confirm gate → server 409
     (care_plan.go:305). Same class: `_apply_toggle.instantApply()` cannot
     recover render→click staleness (auto-refresh pauses while a modal is open —
     an apply modal open across the next slot boundary 409s on confirm);
     skip/defer POSTs have no late path; batch refs carry no late flag. Bonus
     defect: history `.plan-late-btn` calls `isInputKind(btn.getAttribute(...))`
     with a STRING → TypeError → Record-late silently dead for
     observation/weighing history rows.
   - ACCEPTANCE (fixture only for mutations):
     a) With a late-recordable medication slot: click ○ → confirm → HTTP 201,
        button flips to ✓, NO error modal; the request body contains
        `late: true` (assert in test/network).
     b) Same for a feeding/care late-recordable toggle and for a cage batch
        containing a late-recordable occurrence: batch succeeds or reports the
        item as recorded — never surfaces the hors-délai 409 text.
     c) An apply modal held open across a slot boundary still executes on
        confirm (client sends late for past-due refs, or retries once with
        late) — no 409 shown.
     d) Skip/defer of a late-recordable occurrence does not 409 (late-aware), or
        the control is absent per item 4c — either way the message is
        unreachable.
     e) Record-late on an observation/weighing history row opens the prefilled
        apply modal (isInputKind fixed) and submits with `late:true` → 201.
     f) Server-side pin: `checkPlanApplyItem` keeps rejecting FUTURE-due applies
        (regression test), so the bound moves to the UI layer, not away.
     g) Zero occurrences of the string "hors délai" reachable as an error modal
        in an E2E pass over all kinds on the fixture.

6. **Medication — one merged animal cell for all of an animal's treatments.**
   - REQUIRED: on `/care_plan?kind=medication`, all treatment lines of one
     animal read as ONE block: the animal identity cell is rendered once per
     animal (merged across its drug-series lines), not repeated per series.
     Scope (working assumption, question sent to the user, no answer yet):
     merge WITHIN each urgency tier — the Late → Now → Later sections stay; an
     animal appearing in several tiers still shows once per tier it occupies.
   - EVIDENCE: one row per (animal × series) today — 1905/26 renders 4 rows in
     the Late tier, 2040/26 3 rows, 6 more animals 2 rows (live census).
     `buildMedGroups` already groups per animal; `fillMedTiers` splits series
     across tiers and each series re-renders the animal cell.
   - ACCEPTANCE:
     a) DOM: per tier table, every animal label occurs exactly once as an animal
        cell; the animal's series lines are visually grouped under/next to that
        single cell (rowspan or grouped-block layout).
     b) Each series keeps its own toggles: applying one series' slot flips only
        that series' button; other series of the same animal unaffected; undo
        works per slot.
     c) Tier header counts unchanged (still occurrence-based).
     d) The ℹ detail modal still resolves per series (correct drug/dosage data).
     e) Grouped layout verified on a fixture animal with ≥3 series (e.g.
        1905/26) in en-US/fr/de/nl.

7. **Care table must match the medication table.**
   - REQUIRED: `/care_plan?kind=care` renders through the SAME line component
     and the SAME visual grammar as `kind=medication` — identical column order,
     animal-cell treatment (incl. shared column width), label treatment, and
     toggle rhythm. No per-kind bespoke layout.
   - EVIDENCE: both use `.plan-med-row` but diverge — care order
     `[ℹ][animal][label][buttons]` vs medication `[animal][ℹ][label][buttons]`
     (live child-class census); medication's animal column carries the computed
     shared width (`view.MedAnimalColCh`), care's does not.
   - ACCEPTANCE:
     a) Automated structural check: the child-class sequence of a care line
        equals that of a medication line (both rendered by the ONE shared
        partial; template-level evidence cited in the fix: same component file
        used by both paths).
     b) Geometry: animal cells of care and medication lines share column width
        behavior (aligned down each table; no per-row free-for-all).
     c) Side-by-side screenshots (care vs medication) approved as matching.
     d) Four locales.

8. **Animal Protocol tab — per-type titles, day-plan style, ONE shared component.**
   - REQUIRED on `/animals/{id}#nav-plan`:
     (a) every action kind's section is introduced by its OWN type title —
     medication, care, feeding, observation, weighing, cleanup (in the
     importance order Medication, Care, Feeding, Observation, Weighing,
     Cleanup);
     (b) each type's table matches the `/care_plan` per-animal line style;
     (c) NO animal-details cell (single-animal page — identity comes from the
     page header, lines must not repeat a label column);
     (d) HARD REQUIREMENT: the same component renders the day plan lines and
     this tab's lines (template-level reuse, enforced — not two lookalikes).
   - EVIDENCE: verified live on `/animals/10299#nav-plan` — the medication
     series render as a full-width tinted band with NO Medication title, while
     Feeding/Cleanup have `.plan-kind-separator` bands (23 separators counted,
     all Feeding/Cleanup); med band layout ≠ item-line layout; medication line
     carries an animal/T-row structure inherited from the day-plan med row.
   - ACCEPTANCE:
     a) DOM: for each day group, sections appear under `.plan-kind-separator`
        titles for EVERY kind present, in importance order (Medication → Care →
        Feeding → Observation → Weighing → Cleanup); count of separators ==
        count of kind sections.
     b) Structural: the line markup on the tab is produced by the same partial
        as the day plan per-animal lines (grep/template evidence recorded in
        the fix; no duplicated layout markup).
     c) No per-line animal cell on the tab (checked: zero `.plan-med-animal`
        blocks in the tab's lines; identity in the header only).
     d) Toggles/apply/undo behave identically to the day plan (shared
        `_apply_toggle` bindings; one apply flips in place).
     e) Type titles localized in en-US/fr/de/nl (kind names already keyed —
        verify all four render).
     f) Side-by-side screenshot (tab vs `/care_plan` per-animal rendering)
        approved as matching style.

**Round validation gate (applies to every item):** fixes are validated on the
:3002 fixture (clone DB, `admin`/`admin`) — any apply/confirm-path mutation is
forbidden on :3000 (production data). **User directive (2026-10-06): every fix
must be fully tested with agent-browser E2E AND captured screenshots** — each of
the 8 items needs browser-verified evidence (DOM assertions where applicable,
screenshots of the fixed views), not code-only/unit validation. Every touched
template string ships in all four locales (en-US, fr, de, nl; localized template
variants included). Gates: `go vet ./...`, `staticcheck ./...`,
`gocognit -over 15 .`, `gocyclo -over 12 .`,
`go test -count=1 -race -cover ./...`, `git diff --check`. E2E evidence per item:
recorded commands, URLs and screenshots under `/tmp/e2e_3rd_*`. An item is done
only when ALL its acceptance checks pass; items with a flagged working assumption
(4 marker style, 6 merge scope) are re-confirmed with the user before being
archived.

### Second user review batch (2026-10-27, evening)

**Status:** Fixed and validated (2026-10-27 late round) — pending user acceptance before archiving. Same method: fixture on :3002 (DB `creaves_mig` restored from `creaves-db-2026-10-05.gz`), RED→GREEN tests, four locales, then agent-browser E2E with screenshots (`/tmp/e2e_1_protocol_tab.png`, `/tmp/e2e_1_protocol_day.png`, `/tmp/e2e_2_legacy_journal*.png`, `/tmp/e2e_3_4_feeding_group_animal.png`, `/tmp/e2e_5_care_rules.png`, `/tmp/e2e_6_matchers_filter.png`, `/tmp/e2e_7_upload_form_filled.png`, `/tmp/e2e_7_media_comment_edited.png`). Gates green (`go vet ./... && go test -count=1 ./... && git diff --check`).

Per-item resolution and evidence:

1. **Resolved.** Every kind's entries render through the ONE shared line component (`.plan-med-line`: `care_plan/med_series.plush.html` for medication series, `care_plan/plan_item_line.plush.html` for the rest), each type announced by a `.plan-kind-separator` title band. The label column now grows to take all available space (`div.plan-med-row .plan-med-label { flex: 1 1 auto; min-width: 0 }`, `assets/css/care-plan.scss` — the old 34-ch cap is gone) and the button group is right-aligned with identical spacing for every type (`.plan-med-btns { margin-left: auto }`). Measured live on `/animals/10273#nav-plan`: feeding vs cleanup label widths are equal per day (887/887, 841/839, 863/861 px) and take 77–86 % of the row; the last visible toggle is flush right (card padding) for every line. Pinned by `TestAnimalProtocolDayItemsFollowImportanceOrder`, `TestCareKindTabOrderFollowsImportance` and the re-recorded Phase0b DOM baselines.
2. **Resolved — tab retired.** The animal SHOW page no longer has a Treatment tab in any locale: its medication series render inside the Protocol tab, and the manual-treatment history is preserved as a collapsed "Manual treatments (history)" card (`#planLegacyJournal`) at the foot of the Protocol tab (verified live: tab absent, journal present and collapsed, opens on click; fr/de/nl titles verified). `?med=` deep links from the dashboard now activate `#nav-plan` and highlight the series (B10-4 retargeted). Treatment CRUD still works via the animal edit form and navbar (unchanged legacy surface). Legacy `Add New treatment` shortcut that lived on the retired tab is gone with it.
3. **Resolved.** The feed tables no longer print a time above the animal — the collapsed group header keeps an earliest-time hint and the Apply button carries the exact time (`○ HH:MM`). Per-animal button spans sit in a `d-flex align-items-center ml-2` wrapper so multi-animal cages get guaranteed spacing. Pinned by `TestFeedTableDropsTimeLabelKeepsChipSpacing` + updated render tests; screenshot `/tmp/e2e_3_4_feeding_group_animal.png`.
4. **Resolved (regression).** Apply buttons are back in `group=animal`: 354 `○/✓` toggle buttons counted live on `/care_plan?kind=feeding&group=animal`. The regression tests from the earlier fix round pin the apply wiring for the animal grouping.
5. **Resolved.** `creaves_mig` (clean dump + boot conversion + startup_v3 + all migrations) audited: 13 care_rules (7 active, 6 disabled), 18 care_matchers — zero converter cage-name matchers, hand-edited rules kept, the over-sweep rule dropped, no zombie/duplicate plans (177 feeding / 63 medication plans generated). The 6 disabled rules are intentional seed rules superseded by the protocol system (treatment-style durated rules `Puces → Sarnacuran`, `Tiques → Ivomec 5j`, `Hérisson — Catosal + Réhydratation`, the global daily clean/wash superseded by `Nettoyage des cages occupées`, `Blessés — contrôle quotidien`, `Pesée hebdo juvéniles`) — nothing nonsensical survives. Screenshot `/tmp/e2e_5_care_rules.png`.
6. **Resolved.** `/care_matchers` gained a client-side filter (input + row-text match + `n / N` hit counter) in all four locales. Live: typing “pigeon” filters 18 → 2 and the counter reads `2 / 18`; French placeholder verified (“Filtrer par nom, description ou expression…”). Pinned by `TestCareMatchersIndexHasFilterSearch`.
7. **Resolved.** The file picker is wide enough for its hint (360 px cap → `width:24rem`, hint measured fully visible at 384 px, no ellipsis) and the hint text is localized (`attachments.upload.choose_file`). Uploads accept an optional comment (≤ 500 chars, new nullable `attachments.comment` column — migration `20261027120000_attachment_comment` **must be applied at deploy**); every gallery card offers a prefilled comment edit form (owner or admin, same rule as delete; non-editors see the comment as text). E2E: uploaded with comment “left wing wound, day 3 — E2E”, edited to “edited: infection check Thursday”, persisted; fr/de/nl strings verified live. Pinned by `TestAttachmentCommentSetAtUploadAndEditable`, `TestAttachmentCommentOwnershipAndCap`, `TestAttachmentCommentBlankClears`, `TestAnimalShowMediaPaneHasCommentUploadAndEdit`.
8. **Resolved.** `actionKinds` order is now Medication, Care, Feeding, Observation, Weighing, Cleanup — it drives the `/care_plan` chips (verified live: “Medication 38 / Care 14 / Feeding 99+ / Observation / Weighing / Cleanup 99+”), the per-day groups of the animal Protocol tab, preference kinds and the default work kind. Day items sort by `actionKindRank`; pinned by `TestCareKindTabOrderFollowsImportance` and `TestAnimalProtocolDayItemsFollowImportanceOrder`.

Fixture/method notes for this round: the late/future apply window default was widened 8 h → 24 h for testing with the user's authorization (`preferenceDefaults`); **existing production preference rows keep their stored 8 h values** — the default only covers rows without a preference. The asset manifest regenerating out of sync with `public/assets` (stale `manifest.json` after a partial webpack run) briefly broke all JS on the fixture; re-running `npx webpack --mode production` + rebuilding the binary (assets are `//go:embed`-ed) fixed it.

1. **Animal protocol (page `#nav-plan` tab, e.g. `/animals/10273?...#nav-plan`)** — do NOT style the care tables as separate/distinct presentations. Required layout per care type: **prefix each type with a title for the type, followed by that type's table**, all tables using the same presentation:
   `<info> <description> | (right aligned) <toggle buttons1>…<toggle buttons2>`.
   The description must take **all available space**; buttons must have **similar spacing** across types. Evidence: Medication table is bigger; Cleanup and Feeding descriptions take less than 50 % of available space.
2. **Animal view: the “Traitement” (Treatment) tab should likely be obsolete** — its function is taken over by the protocol (plans) tab. Verify overlap, then remove/deprecate without losing needed information.
3. **`/care_plan` (feeding / cage grouping)** — do not show the time on top of the animal; the Apply button already carries the time. For multiple animals per cage, allow some space between the animal and its associated button.
4. **`/care_plan?kind=feeding&group=animal` — REOPEN: the view is missing Apply button(s).** Treat as regression of the earlier fix round; reproduce and fix.
5. **Clean-DB migration validation** — recreate the DB from `/Users/muaddib/dev/creaves.project/creaves-db-2026-10-05.gz` and validate that all `care_rules` on the clean “to migrate” DB make sense and match the previously open issues (no cage-name matchers from the converter, hand-edited rules kept, over-sweep dropped, zombies/duplicates handled).
6. **`/care_matchers` should have filter/search.**
7. **Media upload** (e.g. `/animals/10351?...#nav-media`): the input should be **long enough to show the hint**; upload should allow adding a **comment/details on the media**, and the user should be allowed to **edit it** afterwards.
8. **Care-plan tab order** — by importance: **Medication, Care, Feeding, Observation, Weighting (Pesée), Cleanup**. The same order should apply in the animal view.

### Care-plan deliverable review: five reported regressions (2026-10-27)

**Status:** Fixed and validated (2026-10-27 round) — pending user acceptance before archiving. Every case was reproduced first on the approved fixture (clone `creaves_review` on port 3002, login `admin`/`admin`, Apply allowed on the clone only), then fixed with RED→GREEN focused regression tests, then revalidated in agent-browser across `en-US`/`fr`/`de`/`nl`.

**User clarifications (2026-10-27, acceptance Q&A):**
- Defects **1 and 2 are one defect**: the issue is on `/care_plan/?kind=feeding&group=cage` — the group Apply button shows a **clock icon instead of the due time**. The button must carry the due-hour text. The `group=animal` feeding toggle (○/✓ HH:MM) is not the reported problem.
- The **108 frozen template-parity drifts will be fixed** as part of this work (user decision) — the full `go test ./...` gate must end green.
- **E2E fixture approved**: run a second local app instance on port **3002** against the DB clone `creaves_review` (non-production copy), login `admin`/`admin`, Apply/toggle activation and fixture-record creation allowed **on the clone only**; production DB `creaves` untouched; webhook/event-stream disabled in the clone.

Resolution per item:

1. `/care_plan/?kind=feeding&group=cage` — group Apply button now carries the due-hour text in place of the bare clock icon (covers former items 1+2). Regression tests + DOM assertions in all four locales; revalidated on the fixture.
2. `/care_plan/?kind=medication&group=animal` — animal cell width fixed; preference-window filtering now hides stale entries. Sub-item **3b** (evidence line `1905/26 VI19 · VI · Hérisson / Baycox 5 % (PER OS) — 0.10 ml / hier` — “hier” is “yesterday” in French): validated on the fixture animal page `/animals/10214` → Traitement tab renders `Baycox 5 % (PER OS) — 0.10 ml` with `○ 12:00` due-time text instead of a relative “hier” label; treatments show “Repris par le protocole” where superseded.
3. `/care_plan/?kind=care&group=animal` — table now matches the medication row styling with a distinct task-text treatment; four locales verified.
4. `/care_plan/?kind=cleanup&group=cage` — single-animal cage no longer shows an “O” Apply control; expanded multi-animal cages put each animal on its own line. **Honest residual:** the clone fixture contains no single-animal cage in `kind=cleanup`, so that path is covered by viewmodel/contract tests rather than browser DOM evidence; the multi-animal path is browser-verified.
5. Template parity — all **108** frozen drift pairs resolved; `grifts.TestTemplateVariantStructuralParity` passes with a zero whitelist; full `go test -count=1 ./...` green.

Gates rerun after the round: `go vet ./...` PASS; `go test -count=1 ./...` PASS (all packages); `git diff --check` PASS.

### Care matchers: cage-name-specific default rules (2026-10-27)

**Status:** Resolved (2026-10-27) — converter cage matchers eliminated everywhere; pending user acceptance before archiving.

User directives (2026-10-27):
- `/care_matchers` — rules whose matchers reference **cage names** must be eliminated from the defaults. Cage names are **per center**; a rule keyed to a specific cage name cannot generalize across centers.
- Cage-scoped rules are applied at the **animal level during migration** instead of as general matcher rules.
- General rule: a default rule must be applicable to **all** centers; **when in doubt — migrate it to the animal**. Applies to **all centers**.

**Design implemented (converter v3 + startup_v3 migration):**
- Feeding conversion now produces **per-animal `care_animal_plans` only** — the converter never creates cluster rules or matchers (`actions/care_plan_convert_data.go`; fenced by `TestConvertDataEmitsNoCageMatchers` source fence and the extended `TestSeedMatcherExpressionsParse`, which reject any `cage ` in seed/converter expressions).
- New marker-guarded one-shot migration (`actions/care_plan_migration.go`, run at boot from `cmd/app/main.go`, or manually via `buffalo task careplan:migrate-cage-rules`): retires **all converter-owned active feeding rules** (name `% (conversion)` or description containing `care_plan_converter`), creates per-animal plans with rule payload+schedule **verbatim** only where the animal's own normalized diet equals the rule's payload food (`NormalizeDiet`), **drops over-sweep coverage** (same species/cage, different diet — the original bug), retires the rule (active=0, matcher unlinked, row kept — `care_plan_applications` FK), deletes the matcher when no rule references it, and **keeps hand-edited rules untouched** (updated_at ≠ created_at within 2 s ⇒ administrator's adjustment wins).
- The `cage` **field stays in the DSL registry** for hand-made rules; only seed/converter matchers are fenced from it (documented decision).
- v3.1 completion pass: inactive-but-still-linked converter rules are unlinked and their matchers deleted (`care_rules.matcher_id` is ON DELETE RESTRICT, so unlink must precede delete).

**Production exposure incident (reported truthfully):** the developer `buffalo dev` server on :3000 runs with `GO_ENV=development`, whose `database.yml` dev block points at the **production `creaves` DB**, and hot-reloads worktree edits, running boot migrations on each rebuild. At 19:50:22 CEST it hot-loaded an **intermediate v3 build** (before the matcher-unlink fix): 8 rules retired, +84 per-animal plans, 80 over-sweep coverages dropped, **0 matchers deleted**, marker written. Remediation: v3.1 completion pass + marker deletion + controlled re-run (`buffalo task careplan:migrate-cage-rules`). A second early-return bug (zero active converter rules ⇒ completion pass skipped) was caught by the controlled grift run, fixed, and pinned RED→GREEN by `TestCarePlanMigrationV3CompletionPassWithoutActiveRules`. **Final production state (audited via SQL):** 8 rules retired total, +84 plans, 80 over-sweep dropped, **0 conversion/cage matchers**, 18 seed matchers remain, 7 active hand-edited linked rules untouched, `startup_v3` marker present (idempotent). Why the 20:01 dev-server rebuild did not write the marker remains unexplained (most plausible: failed hot-reload during a mid-edit window kept the previous process; buffalo dev logs live in the operator's terminal); the currently running server (rebuilt 20:10) runs the final code and a boot no-ops on the marker, verified on :3002.
Timestamp note: `care_plan_conversion.finished_at` is stored UTC (`18:11:23` = 20:11:23 CEST) — same go-sql-driver `loc=UTC` skew as the known unapply `due_at` Z-suffix quirk; display-level only.

**Clone-only artifact cleaned:** 12 duplicate plan pairs on `creaves_review` — v1 byte-cut names (2026-09-27 14:08:18) duplicated by the v2 re-conversion with word-safe-cut names (2026-10-06 17:12:13) after the R4-7.24 name fix defeated the by-name upsert. Per pair: payload and full schedule JSON verified identical; the older copy deactivated (12 rows, `active=0`). Production has `dup_pairs = 0` (never re-ran v2 post-rename).

**Validation (agent-browser on :3002, fr locale):** `/care_matchers` renders only the 18 seed matchers — zero “conversion”/“cage” entries; hedgehog 1905/26 (cage VI19) shows exactly **one** feeding line (own diet, 18:00) on `/care_plan?kind=feeding`; VE2/VE3: all 43 animals have exactly one plan per own normalized diet (SQL), spot-checked on-screen — 9792 `grains pigeons` (no “eau”), 9823 `graines tourterelles grit eau`, 9600 `grains pigeons eau` — one NOURRISSAGE section each; formerly over-swept animals are no longer covered by the majority diet. (The day-plan “En retard” tab shows a rolling window — 16:00+ at 20:17 — so 10:00 slots are off-window by existing design, not a v3 effect.)

**Tests added/updated:** `TestConvertDataEmitsNoCageMatchers`, `TestSeedMatcherExpressionsParse` (no `cage `), `TestCarePlanMigrationV3MigratesRulesToAnimals` (full v3 scenario: retire/diet-gate/over-sweep/hand-edited/residue/marker/idempotency), `TestCarePlanMigrationV3CompletionPassWithoutActiveRules` (the zero-active-rules production corner), RoundTrip MySQL test (per-animal plans, no cage matchers), name-truncation tests updated. Gates: `go vet ./...`, `go test -count=1 ./...`, `git diff --check` all PASS.

## Archived rounds

| Round | File |
|---|---|
| Round 10 (care-plan UX + timestamp localization) | `docs/archive/2026-10-06-care-plan-round-10-bugs.md`; handover: `docs/archive/2026-10-06-care-plan-round-10-handover.md` |
| Round 9 + 8 (caregiver UX / data quality / i18n + revalidation sweep) | `docs/archive/2026-10-05-care-plan-round-9-bugs.md` |
| Round 7 (care plan / animal page UX) | `docs/archive/2026-10-03-care-plan-round-7-bugs.md` |
| Round 4 (care plan / treatment / dashboard UX) | `docs/archive/2026-10-02-care-plan-round-4-bugs.md` |
| Round 3 (care plan / treatment / navbar UX) | `docs/archive/2026-10-02-care-plan-round-3-bugs.md` |
| Round 2 (care plan UX) | `docs/archive/2026-10-01-care-plan-ux-round2-bugs.md` |

Round-10 companion plan: `docs/care-plan-round-10-fix-plan.md`.
