# Bug 1: Excel export shows as "corrupted" (PivotTable removed by Excel repair)

**Status**: fixed & verified (2026-09-08)
**Projects touched**: creaves + creaves-console (kept in sync)

## Symptom

`registre_detail.xlsx` (and any template-based Excel export) triggered Excel's
repair prompt on open; the recovery log reported:

```
Removed Feature: PivotTable report from /xl/pivotTables/pivotTable1.xml part (PivotTable view)
```

## Root cause

The earlier "Bug 5" fix (`excel/pivot.go`) repointed the pivot cache at the
written range but also **emptied** `sharedItems` and replaced the cached
`pivotCacheRecords` with an empty set, relying on `refreshOnLoad="1"` to make
Excel rebuild the cache.

Empirical testing against Microsoft Excel (macOS) showed:

1. Template round-tripped through excelize untouched → opens clean.
2. Template + written cells (no pivot patching) → opens clean.
3. Template + written cells + rewritten/emptied pivot cache parts → **repair
   prompt** (pivot dropped), even though all parts are schema-valid XML.
4. Template + written cells + pivot cache **kept verbatim** (only
   `worksheetSource` ref repointed + `refreshOnLoad="1"` forced,
   `recordCount` left matching the cached records) → **opens clean, pivot
   intact**, Excel refreshes the cache from live data on open.

A full typed rebuild of sharedItems+records from the exported rows
(distinct-value indexes, inlined unique values, `<n>/<s>/<m>/<x>` records —
mirroring Excel's own layout) was implemented and tried first: Excel rejected
it even on a 5-row file. Conclusion: Excel validates pivot cache parts beyond
the schema (likely signature/structure heuristics) and only accepts cache
content it wrote itself; third-party cache writes are treated as corruption.

## Fix

`excel/pivot.go` `updatePivotCaches()` now:

- repoints `worksheetSource` at the written range (`A1:<lastCol><lastRow>`),
- forces `refreshOnLoad="1"`,
- **keeps the template's cached sharedItems/records verbatim** and keeps
  `recordCount` consistent with the cached records count.

The sheet-row truncation + `_xlnm._FilterDatabase` rewrite (companion fix)
are unchanged. Identical logic synced to `creaves-console/excel/pivot.go`.

## Test approach & validation

- Unit: `creaves/excel/pivot_test.go` — ref rewrite, refreshOnLoad, template
  cache/records kept verbatim, recordCount fallback when template lacks one.
- e2e (Go): `actions/export_excel_test.go` drives `/export/excel` end-to-end
  and asserts the invariants (ref, sheet, refreshOnLoad, recordCount ==
  records count). Console equivalent: `export_excel_sqlite_test.go`.
- e2e (real Excel): generated `registre_detail` export from the dev DB
  (10 047 rows) opened in Microsoft Excel via AppleScript/UI scripting —
  **zero repair prompts**, workbook opened, sheet 2 "Stats" pivot table
  `DataPilot1` present and intact. Before the fix the same file produced the
  exact repair dialog quoted above (captured via accessibility API).
- Quality gates: `go vet` clean; `staticcheck` clean on changed packages
  (pre-existing warnings elsewhere only); `gocognit -over 15` /
  `gocyclo -over 12` — no new warnings; `go test -count=1 -race -cover`
  green on both projects.

## Commits

- creaves: `ce55b31` — fix(export): stop Excel repair prompt on exports with pivot tables
- creaves-console: `2f76ec8` — fix(export): sync pivot-cache fix with creaves

## Residual risk

- If a template's pivot cache range is smaller than the exported data, Excel
  still refreshes on load from the repointed full range — no action needed.
- If a future excelize version gains pivot-cache write APIs, re-test before
  adopting: Excel's acceptance of third-party cache writes is the limiting
  factor, not schema validity.
