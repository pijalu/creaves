# Bug 7 — Excel export (stat_communes / registre_detail) triggers Excel repair prompt

Archived: 2026-09-09. Root cause confirmed + fix already landed on 2026-09-08:
creaves `1824329`, creaves-console `a1fb0fa`. This session: root-cause
pinpointed to excelize source, fresh exports from BOTH apps validated, bug
closed.

## Observed

`GET /export/excel?query=stat_communes&instance_id=` (creaves:3000 and
creaves-console:3001 — identical pipeline) produced an .xlsx that Microsoft
Excel (macOS) flags for repair. Recovery log: `Removed Part: /xl/styles.xml
part with XML error (HRESULT 0x8000ffff)` followed by cascade repairs of
worksheets (cell/column information) and PivotTable reports
(pivotTable1..9). Both apps affected (same embedded templates,
near-identical `excel/` packages, excelize v2.8.0).

## Root cause (confirmed 2026-09-09)

excelize re-serializes `xl/styles.xml` on `WriteTo`, replacing the template's
root element with its hardcoded `templateNamespaceIDMap`
(`excelize/templates.go:48`). That constant's `mc:Ignorable` attribute lists
the `x15` token TWICE (`... dgm14 x15 x12ac ... xr15 x15 x16 ...`). The
Markup Compatibility spec (ECMA-376 Part 3) forbids duplicate tokens in
`mc:Ignorable`; Microsoft Excel rejects the part outright, LibreOffice and
openpyxl tolerate it — which is why only Excel users saw the repair prompt.

Bisect evidence (files in `~/Downloads`, generated 2026-09-08):

| file | content | styles.xml root |
|------|---------|-----------------|
| TEST0_raw_template.xlsx | untouched embedded template | `mc:Ignorable="x14ac x16r2 xr xr9"` — clean |
| TEST1_excelize_roundtrip.xlsx | template → excelize open+save, no data | bloated root, `x15` duplicated — broken |
| TEST2_pivotpatch.xlsx | TEST1 + pivot-cache patch | same broken root |
| TEST3_full_export.xlsx | served export (pre-fix) | same broken root |

→ excelize serialization is the single cause; present in v2.8.0 AND still in
v2.11.0 (`lib.go:694` `ignorableNS` literal + `templates.go:48`), so upgrading
excelize does NOT fix it.

## Fix (landed 2026-09-08, both repos)

`excel/styles.go` + hook in `writeExcelResponse`: after serializing, replace
`xl/styles.xml` in the final zip with the template's original part, appending
only the extra `cellXfs` `<xf>` entries excelize legitimately created while
writing cells (default date style etc.). Merge rules:

- template prolog + root start tag copied verbatim (kills the duplicate-token
  `mc:Ignorable`);
- extra xf entries sanitized (empty `<alignment/>` dropped, `apply*="false"`
  removed, booleans normalized to `1`, custom numFmt ids remapped or appended
  with fresh ids);
- unsafe merges (excelize added fonts/fills/borders the template lacks)
  abort the patch and keep the excelize document (logged), never shipping an
  invalid file.

Identical copies in `creaves/excel` and `creaves-console/excel` (verified
`diff` — byte-identical `styles.go`). Unit tests in `excel/styles_test.go`
(11 tests incl. `TestMergeStylesXML_NoDuplicateIgnorableTokens`,
root-verbatim, numFmt remap, error paths).

## Validation (2026-09-09, this session)

### Unit tests
- creaves: `go test -count=1 ./excel/...` — ok.
- console: `CGO_ENABLED=1 go test -count=1 -tags sqlite ./excel/... ./actions/` — ok.

### Quality gates
- creaves: `go vet ./...` clean; full-suite `go test -count=1 -race -cover ./...`
  green earlier this session (Bug 9 gate; no excel changes since).
- console: `go vet`/`staticcheck` clean; `CGO_ENABLED=1 go test -count=1 -race
  -tags sqlite ./...` green earlier this session.

### E2E — fresh exports from both apps, both templates (agent-browser)

Servers: `buffalo dev` creaves:3000 (admin/e2e-test-pass),
creaves-console:3001 (admin/admin123, LaGrange instance, 10047 animals
post-Bug-9 resync). Downloads via `agent-browser open <url>` (net::ERR_ABORTED
= file download, expected):

| export | URL | file |
|--------|-----|------|
| creaves stat_communes | `http://localhost:3000/export/excel?query=stat_communes` | 1,015,174 B |
| console stat_communes | `http://localhost:3001/export/excel?query=stat_communes` | 881,249 B |
| creaves registre_detail | `http://localhost:3000/export/excel?query=registre_detail` | 1.6 MB |
| console registre_detail | `http://localhost:3001/export/excel?query=registre_detail` | 1,672,669 B |

Static analysis of all four fresh files (extracted to /tmp/b7/{fresh,freshc,
rd,rdc}):

- `mc:Ignorable` — no duplicate tokens in any file (stat_communes files carry
  the template's original `"x14ac x16r2 xr xr9"`; registre templates carry
  none);
- every XML part well-formed (`xml.dom.minidom` parse of all parts: 0
  malformed);
- style references in range: max cell `s=` index 49 < cellXfs 50
  (stat_communes), 14 < 15 (registre_detail);
- pivot caches: `refreshOnLoad="1"` set on every pivotCacheDefinition (1 per
  file);
- stat_communes data sheets hold 10047 (console) / 10074 (creaves) rows with
  populated locality columns (Bug 9 verified at the same time);
- LibreOffice headless opens both console files and converts them cleanly
  (`soffice --headless --convert-to xlsx` — no repair, no warnings).

**Residual risk**: no physical Microsoft Excel available in this environment —
the final "opens in Excel with zero prompts" confirmation rests on the static
proof (duplicate-token root cause eliminated; template-original styles.xml
restored) plus LibreOffice/openpyxl acceptance. If a repair prompt reappears
for a user, reopen with the recovery log attached.
