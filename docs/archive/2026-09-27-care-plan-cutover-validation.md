# Care-plan production cutover — Phase 7 E2E validation (2026-09-27)

End-to-end proof of the "replace legacy feeding/treatments" claim on a **fresh**
production-dump rig, per `../bugs.md` (workspace root) Phase 7 + rollout notes.
Rig: scratch database `creaves_p7` (MySQL 8.4.11), dump
`creaves-db-2026-09-26.gz` (10 242 animals / 374 044 cares / 57 801 treatments),
48 migrations applied (`buffalo pop migrate up`), app booted from the
`buffalo build` binary (`tmp/creaves-p7-app`, HEAD `5e68a31`) with
`GO_ENV=production`, so the converter ran through the **deployed `cmd/app/main.go`
boot path** (post-migration, pre-HTTP, fail-fast) — not a grift.

**Verdict: GO.** `UNCOVERED == 0` for feeding and treatments; `DEGRADED == 0`;
every Phase 7 check passed (details below). Evidence files: `tmp/p7_evidence/`
(report, day-plan JSON dumps, coverage script output, API log) and
`tmp/browser_evidence/p7-*.png` (screenshots) — both gitignored, regenerated on
demand by re-running the rig.

## 1. Converter at boot (main.go path)

Boot log (`tmp/p7_evidence/p7_boot.log`):

```
2026/09/27 14:21:55 care_plan_converter: done — seeds +12 rules /17 matchers, feeding 8 rules + 93 plans, treatments 85 plans
```

Persisted JSON report (`care_plan_conversion` key `startup_v1`, 51 040 bytes):

| Section | Numbers |
|---|---|
| Seeds | 12 rules inserted, 17 matchers inserted, 0 skipped |
| Feeding | 195 animals considered → 75 clusters → **8 rules + 93 plans** |
| Treatments | 86 series → **85 plans** + 1 series deduplicated onto an identical-name converted plan by the idempotency guard (counted OK by the reconciliation step); 85 conversion lines ↔ 85 plan IDs ↔ 85 series groups per animal, all matched |

Marker decision (rollout notes): the prod dump contains **no** `care_plan_*`
tables — the v1 converter never ran in production — so the marker key stays
`startup_v1` and no v1-cleanup migration is shipped. Pre-prod databases that ran
a v1 converter must be dropped/re-migrated (documented in release notes).

## 2. Coverage go/no-go (Phase 1.6 reconciliation)

Reconciliation section of the persisted report
(`tmp/p7_evidence/p7_report_pretty.json`):

```
feeding_ok: 195   feeding_degraded: 0   feeding_uncovered: 0
treatment_ok: 86  treatment_uncovered: 0
reconciliation complaint lines: 0
```

**`UNCOVERED == 0` for feeding and treatments; `DEGRADED == 0` — gate passed.**

Day-plan cross-check (`tmp/p7_evidence/p7_coverage_check.txt`, script output):
for each of the 195 legacy-feeding animals, the legacy daily slot list
(`feeding_start + n·period ≤ feeding_end`, `start==end` → single slot, overnight
`end<start` via +24 h) was compared against the rendered day plan
(`GET /care_plan?kind=feeding`, 2026-09-26 → 2026-09-29, 5 516 items / 218
animals):

- animals with zero in-window feeding items: **0 / 195**
- animal-days where the converted occurrences do not fully cover the legacy
  slot set: **0** (same-or-richer everywhere)
- `nb 1/2` cohort and the 81-animal `10:00–10:00` cohort fully covered.

Treatment series: all 39 animals of the 86 series have medication/observation
occurrences. 34 appear in the default window; the 5 remaining ones
(10196, 10199, 10244, 10260, 10264) carry `anchor: fixed` plans starting
2026-09-30 → 2026-10-02 (series begin after the window) and appear with the
exact expected day counts (1/9/8/17(=9+8)/9 occurrences) when the window is
shifted (`?from=2026-09-30&to=2026-10-10`, `tmp/p7_evidence/p7_dayplan_future.json`).

## 3. Feature-parity sweep

| Legacy surface | Result |
|---|---|
| `GET /feeding` bookmark | **302** → `/care_plan?kind=feeding` |
| `GET /feeding/close` (removed, M4) | **404** |
| Dashboard treatment panel | renders 200, read-only (inert `.btn-schedule` indicators, no PUT route) |
| Animal page → Plan tab | converted plan visible (e.g. animal 9800: « Alimentation — 1/2 poussin en morceaux… (conversion) », 18:00; screenshot `p7-animal-plan-tab.png`) |
| Guest status page | renders « Prochain nourrissage : 08:00 » from frozen legacy columns (animal 1957/26; screenshot `p7-guest-fr.png`) |
| Animal form legacy feeding schedule | read-only display + hint in 4 locales; no editable `feeding_*` inputs in DOM (fr hint screenshot `p7-animal-form-fr.png`; de/nl strings asserted) |
| Apply row shape | `cares` row for a feeding apply: same table/columns/caretype as legacy flow (weight/clean NULL instead of ''/0 — nulls, not loss); medication apply writes `treatments` row with legacy columns (`drug`, `dosage`, `timebitmap`, `timedonebitmap`) — shape diff: none |

## 4. Browser / API walkthrough (agent-browser + curl, evidence `tmp/p7_evidence/p7_api_evidence.txt`)

Authenticated as a scratch admin (`p7admin`, inserted into the rig DB only).

1. **Day plan render** — `/care_plan` 200, 5 642 items (5 516 feeding, 91
   medication, 35 observation), screenshots in 4 locales.
2. **Apply feeding** — `POST /care_plan/apply` (rule `932f8872…`, animal 8429,
   2026-09-27 08:00) → **201**; `care_plan_applications` row + `cares` row
   (feeding caretype `fe439281…`) written.
3. **Double apply** — same ref re-POSTed → **409**
   `{"error":"occurrence already recorded (idempotent, §4.5)"}` in
   `GO_ENV=production` (H1 contract holds outside development). UI-level proof:
   two synchronous clicks on the row's Apply button produced `201` + `409` in
   the page's own fetch log (`agent-browser network requests`); after reload the
   row shows « Done » and no longer offers Apply (`p7-dayplan-after-apply.png`).
4. **Defer + clamp** — defer requested to 2026-10-15 → **201** with
   `deferred_until` clamped to **2026-09-28T07:59** (just before the next 08:00
   occurrence).
5. **Medication without resolvable dosage** — test plan (empty dosage,
   `dosage_from_dosages_table`) on animal 8635 → apply → **422**
   `{"error":"dosage_required","reason":"no_dosage_row","last_weight":"633","last_weight_at":"2026-09-25T16:08:00Z"}`;
   UI opened the dosage modal showing « Last recorded weight: 633 g (2026-09-25) »
   (screenshot `p7-dosage-modal.png`); resubmit with manual dosage
   `0.12 ml (manuel UI)` → **201**, `treatments.dosage` carries the manual value,
   application snapshot records `dosage_source: "manual"` (§10-B6/L2).
6. **Cage batch apply** — `POST /care_plan/apply_batch` (Enclos Renards, 2
   items) → **200** `{"applied":2,"cage":"Enclos Renards"}`.
7. **Admin unapply** — `POST /care_plan/unapply` (animal 8433) → **200**
   `{"status":"unapplied"}`; application row gone.
8. **Same-bucket append (M2)** — second medication apply same drug/day/bucket:
   `treatments.remarks` gains `"; 14:42"` (append, not overwrite).

## 5. Landing badge == day-plan counters

Landing badge `Day plan: 1501` == `open + late` from `CountOpenItems` ==
independent recount from `GET /care_plan` JSON at the same minute
(09-26 late 300 + missing 590 + 09-27 late 595 + due 16 = **1501**; dropped to
1500 after one apply — consistent). Screenshot `p7-landing-badge.png`.

## 6. Locale sweep (en-US / fr / de / nl)

| Screen | en-US | fr | de | nl |
|---|---|---|---|---|
| Day plan title/headers | Day plan / Animal·Plan·Due·Status | Plan de la journée / Échéance·Statut | Tagesplan / Tier·Fällig·Status | Dagplan / Dier·Te doen om·Status |
| Landing badge | Day plan: 1500 | Plan du jour: 1500 | Tagesplan: 1500 | Dagplan: 1500 |
| Animal form frozen-schedule hint | (template key `animals.feeding.legacy_frozen`) | « Horaire historique (lecture seule)… » | « Historischer Fütterungsplan (schreibgeschützt)… » | « Historisch voedingsschema (alleen-lezen)… » |
| Animal Plan tab | Plan / New plan | Plan (tab) | Plan / Neuer Plan | — |
| Guest status | « Next feeding: 08:00 » | « Prochain nourrissage : 08:00 » | — | — |

Screenshots: `p7-dayplan-{en,fr,de,nl}.png`, `p7-landing-badge.png`,
`p7-animal-form-fr.png`, `p7-guest-fr.png`.

## 7. Test suites

Run after the walkthrough, before rig cleanup:

```
GO_ENV=test go test ./actions/ -count=1   → ok  creaves/actions  20.346s
go test ./models/... -count=1             → ok  creaves/models 0.567s
                                            ok  creaves/models/careplan 0.299s
```

## 8. Rollout notes applied

- **Marker**: `startup_v1` kept — production never ran a converter (dump has no
  `care_plan_*` tables); no cleanup migration needed. Pre-prod DBs that ran the
  buggy v1 converter must be dropped/re-migrated.
- **Go/no-go**: this document + `tmp/p7_evidence/p7_report_pretty.json` are the
  reconciliation evidence to attach to the release notes: `UNCOVERED == 0`,
  `DEGRADED == 0`, zero review lines to sign off.
- **Rehearsal timing**: full rig (drop → import 28 MB dump → 48 migrations →
  build → boot with conversion) ≈ 6 minutes wall clock; the converter itself
  completes in < 1 s inside the boot sequence.
- **Cutover day**: DB backup immediately before boot; converter is insert-only
  (rollback = restore backup, or delete rows where `created_by` is the converter
  marker + marker row); legacy `feeding_start/end/period` and `treatments` are
  never modified (audit trail + guest-page source).
- **Rollback**: restore backup / delete tagged rows + marker, redeploy previous
  binary — legacy columns untouched, old binary works unchanged (§8.2).

## 9. Rig cleanup

After evidence capture: scratch DB `creaves_p7` dropped, `p7admin` scratch user
gone with it, app process stopped, `tmp/creaves-p7-app` binary and
`/tmp/database_p7.yml` removed. No repository files touched (`database.yml`
unchanged; `migrations/schema.sql` restored after the migrate dump-header
rewrite; working tree clean except this document).
