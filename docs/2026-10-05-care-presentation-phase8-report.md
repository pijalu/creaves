# Phase 8 — Full regression sweep + defect archive — final report

**Date**: 2026-10-05 · **Round**: care presentation contract (2026-10) · **Result**: ALL GREEN — round closed.

Normative reference: [docs/care-presentation-guideline.md](./care-presentation-guideline.md).
Defect log (closed index): [docs/defects/README.md](./defects/README.md).

## 1. Defects closed (D1–D7)

| ID | Title | Fix commit | Phase | Archived entry |
|----|-------|-----------|-------|----------------|
| D1 | Medication line responsive stacking + bucket divider | `5f4242b` (+`da3701f`) | 1 | [archive](archive/2026-10-05-care-presentation-d1-medication-line-responsive-bucket-divider.md) |
| D2 | `/care_rules/{id}` raw JSON for browser GET | `2f51d6e` | 2 | [archive](archive/2026-10-05-care-presentation-d2-care-rules-show-raw-json.md) |
| D3 | Cleanup format divergent (tiers, toggles, zone i18n) | `2099f6c` | 3 | [archive](archive/2026-10-05-care-presentation-d3-cleanup-format-divergent.md) |
| D4 | Observation/care/weighing toggle parity | `a08e4b2` | 0b+4 | [archive](archive/2026-10-05-care-presentation-d4-row-kind-toggle-parity.md) |
| D5 | Animal `#nav-plan` non-med format divergence | `b65fe35` | 5 | [archive](archive/2026-10-05-care-presentation-d5-animal-nav-plan-format-divergence.md) |
| D6 | `/care_rules` list sort/filter/search/paging | `1da9e7e` | 6 | [archive](archive/2026-10-05-care-presentation-d6-care-rules-list-no-sort-filter.md) |
| D7 | `/zones/{id}` RequiresCleanup + i18n | `aab947c` | 7 | [archive](archive/2026-10-05-care-presentation-d7-zone-show-missing-fields-i18n.md) |

Each archived entry carries: measured observation, guideline § reference, root
cause (file:line), repro steps, mapped test IDs, per-phase verification output,
and a Phase 8 regression block with the sweep evidence below.

## 2. Tests added (all green)

51 test pins across 10 test files, by phase:

- **Phase 0b** (shared components): `TestPhase0bComponentForksByteIdentical`, `TestPhase0bNoOutlineOnToggles`, `TestCarePlanPhase0bDOMEquivalence`, `TestFillTierBucketsGeneric`, `TestFillTierBucketsRealTypes`, `TestBuildDayPlanViewFillsCareTiers`
- **Phase 1 / D1**: `TestPhase1LadderCSS`, `TestPhase1MedRowAnimalYear`, `TestPhase1MedGroupAnimalYear`, `TestPhase1BucketCaptionGate`, `TestPhase1UndoRestoresTierClass`, `TestPhase1ForksByteIdentical`, `TestPhase1DashboardOverrideIntact`
- **Phase 2 / D2**: `TestCareRuleShowHTMLBrowserGet`, `TestCareRuleShowJSONUnchanged`, `TestCareRuleShowClickThrough`, `TestCareRuleShowNonAdminGate`, `TestCareRuleShowAllLocales`, `TestCareRuleShowBackSanitized`
- **Phase 3 / D3**: `TestPhase3CleanupRendersTieredSections`, `TestPhase3CageRowStatesTimeOncePerOccurrenceGroup`, `TestPhase3CleanupTogglePairContract`, `TestPhase3LateCleanupToggleIsRed`, `TestPhase3BatchApplyCageFlipsTheWholeRow`, `TestPhase3GroupAnimalOneLinePerAnimal`, `TestPhase3GroupChoicePersists`, `TestPhase3GroupAnimalRendersHTTP`, `TestPhase3TierBadgesSpeakSummaryStripUnit`, `TestPhase3CleanupTierTableContract`
- **Phase 4 / D4**: `TestPhase4LateObservationToggleIsRed`, `TestPhase4MergedLineColourFollowsMostUrgentSlot`, `TestPhase4ThreeTimesADayRendersOneLine`, `TestPhase4ToggleFlipsGreenImmediately`, `TestPhase4UndoRestoresTierColour`, `TestPhase4RowLineWalksTheLadder`, `TestPhase4WeighingKeepsTheInputModal`, `TestPhase4WeighingSlotFlagsInput`
- **Phase 5 / D5**: `TestPhase5AnimalRowsUseSharedLineNoAnimalCell`, `TestPhase5MedicationBlockUnchanged`, `TestPhase5ApplyFlipViaSharedToggle`, `TestPhase5MergeDayItemsMergesRepeats`, `TestPhase5OutOfWindowLocksNotDeadButton`, `TestPhase5SrcDeepLinkStillOpensDetails`
- **Phase 6 / D6**: `TestCareRulesListSort`, `TestCareRulesListFilters`, `TestCareRulesListSearch`, `TestCareRulesListPagination`, `TestCareRulesListJSONUnchanged`, `TestCareRulesListControlsAllLocales`
- **Phase 7 / D7**: `TestZoneShowAllFields`, `TestZoneShowAllLocales`, `TestZoneShowInternalType`

## 3. Phase 8 verification evidence

### 3.1 Full test suite
```
$ go test ./actions ./models
ok  	creaves/actions	17.157s
ok  	creaves/models	0.581s
```

### 3.2 Production assets rebuilt
```
$ npm run build
webpack 5.111.1 compiled with 3 warnings in 3703 ms   (size-limit warnings only, pre-existing)
```
New digests in `public/assets/manifest.json`:
`application.5e6078ac981991b8c195.js`, `application.3923f4c5795d07e387e8.css`,
`care-plan.31d6cfe0d16ae931b73c.js`, `care-plan.10a93947c7882a177f75.css`.

### 3.3 Browser sweep — every defect URL × 4 locales
Authenticated admin session (agent-browser) on the dev server.

| URL | fr | en-US | de | nl |
|-----|----|-------|----|----|
| `/care_plan?kind=medication` | 200 | 200 | 200 | 200 |
| `/care_plan?kind=cleanup` | 200 | 200 | 200 | 200 |
| `/care_plan?kind=observation` | 200 | 200 | 200 | 200 |
| `/care_plan?kind=feeding` | 200 | 200 | 200 | 200 |
| `/animals/2058` (`#nav-plan`) | 200 | 200 | 200 | 200 |
| `/care_rules` | 200 | 200 | 200 | 200 |
| `/care_rules/02aae3db-bd94-41ea-b9b5-d4e214bf95b8` | 200 | 200 | 200 | 200 |
| `/zones/4eafb532-2dee-48dc-96df-9cfd48c58d49` | 200 | 200 | 200 | 200 |
| `/dashboard` | 200 | 200 | 200 | 200 |

**36/36 HTTP 200. Browser console: empty in all four locales. Page errors: none.**
Locale spot-checks on the zone page: fr "Nettoyage requis", en-US "Requires
cleanup", de "Reinigung erforderlich", nl "Schoonmaak vereist".

### 3.4 Auto-refresh floor (R4-4.1) — e2e
On `/care_plan?kind=medication`: page loaded, marker planted, medication slot
applied at **T+54s**. Naive 60s timer would have reloaded at T+60s and erased
the fresh state; observed: marker alive at T+64/69/74/80/85s polls, reload at
**T+85–90s** — i.e. ≥30s after the last action (floor = apply+30s = T+84s).
In-place toggle flip (`○` → `✓` btn-success) confirmed immediately after apply.

### 3.5 Dashboard medication cell unchanged (R9-3 guard)
`/dashboard` `.dash-med-cell` computed styles: line `flex-wrap: wrap`, label
`white-space: normal; flex: 1 1 auto` (no ellipsis clip), button group
`flex-basis: 100%; flex-wrap: wrap` — the R9-3 scoped override intact, D1
care-plan ladder did not leak into the dashboard cell. In-place apply from the
dashboard cell works (no reload, console clean). Unit pin:
`TestPhase1DashboardOverrideIntact`.

## 4. Archive actions

- All 7 defect entries moved: `docs/defects/d*.md` →
  `docs/archive/2026-10-05-care-presentation-d*.md` (git mv, history preserved).
- `docs/defects/README.md` rewritten as the closed index (links + commit refs +
  sweep evidence); new defects follow the format documented there.
- AGENTS.md Known Issues: no update required — the section covers webhook
  forwarding only and never referenced D1–D7.

## 5. Residual risks

- Asset-size webpack warnings (bundle >244 KiB) are pre-existing and unchanged.
- R4-4.1 reload timing verified to a 5s poll granularity; floor semantics
  (defer while `planActionAge() < 30s`) additionally pinned by code inspection
  (`index.plush.html:374-382`, `_apply_toggle.plush.html:143-144,607`).
