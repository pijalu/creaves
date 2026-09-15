# Fix plan — CSV export encoding (accented characters render as boxes/mojibake)

## Bug

CSV exports may show odd characters for values with accentuated characters (e.g. the
`animal_age` / `entry_age` column). Accented characters (é, è, à…) may display as
multiple boxes or mojibake when the file is opened in Excel or other spreadsheet tools —
encoding mismatch between the CSV file bytes and the encoding the reader assumes.

## Root cause

Three CSV download paths exist:

| Path | Handler | BOM | Content-Type charset | Status |
|------|---------|-----|----------------------|--------|
| `/animals/search/export.csv` | `AnimalSearchExportCSV` → `writeCSV` | yes | `text/csv; charset=utf-8` | OK |
| `/reports/annual/export.csv` | `ReportsAnnualExportCSV` → `writeCSV` | yes | `text/csv; charset=utf-8` | OK |
| `/export/csv?query=…` (configured exports) | `export.RunQuery` | **no** | `text/csv` (no charset) | **BUGGY** |

`export.RunQuery` (`export/export.go`) streams raw UTF-8 bytes with no byte-order mark
and no charset parameter. Excel (and other locale-dependent readers) then assume the
local ANSI codepage (e.g. Windows-1252), so multi-byte UTF-8 sequences for accented
characters (`é`, `à`, `è`, `’` — present in column headers like `année`, `Espèce` and
in reference values like animal ages `Juvénile`) render as multiple garbage characters.

## Fix

In `export/export.go` `RunQuery`:

1. Set `Content-Type: text/csv; charset=utf-8` (explicit charset).
2. Write the UTF-8 BOM (`EF BB BF`) before the CSV writer emits anything, so Excel
   auto-detects UTF-8 regardless of the HTTP header.
3. Extract the writer loop into a testable helper `writeCSV(w io.Writer, cols []string,
   rows [][]string) error` (BOM + header + rows) so the behavior is unit-testable
   without a database.

Delimiter stays the comma (existing configured-export consumers parse comma); only the
encoding signalling changes.

## Test approach

- **Unit** (`export/export_test.go`): new test asserting the helper output starts with
  the UTF-8 BOM and that accented values (`Juvénile`, `Mort à l'arrivée`) round-trip
  through `encoding/csv` unchanged.
- **HTTP** (`actions/export_view_test.go`): extend the existing
  `TestExportCsvStillDownloads` coverage with assertions that `/export/csv?query=register`
  returns `charset=utf-8` in Content-Type and a body starting with the UTF-8 BOM.
- **E2E** (agent-browser): log in as admin on the dev app, download
  `/export/csv?query=entry_age`, verify the downloaded bytes start with `EF BB BF` and
  accented values decode as proper UTF-8.

## Validation steps

1. `go vet ./...`
2. `staticcheck ./...`
3. `gocognit -over 15 .`
4. `gocyclo -over 12 .`
5. `go test -count=1 -race -cover ./...`
6. E2E via agent-browser against `buffalo dev` (hexdump of the downloaded file).

## Results (all steps executed)

- `go vet ./...` — exit 0, silent.
- `staticcheck ./...` — exit 0, silent.
- `gocognit -over 15 .` / `gocyclo -over 12 .` — no findings in `export/export.go`;
  listed findings are pre-existing baseline (event_producer, startup_seed, animals, …).
- `go test -count=1 -race -cover ./...` — all packages ok, zero failures
  (actions 33.1%, export 23.9%, models 72.1%, utils 90.5%).
- New tests: `TestWriteCSV_UTF8BOM`, `TestWriteCSV_AccentsRoundTrip` (export package),
  `TestExportCsvUTF8Encoding` (actions, full HTTP via App()).

## E2E evidence (agent-browser, 2026-09-16)

Authenticated admin session on http://127.0.0.1:3000 (buffalo dev), then downloaded
`http://127.0.0.1:3000/export/csv?query=entry_age` with the session cookie:

```
$ curl -s -D /tmp/headers.txt -o /tmp/entry_age.csv -H "Cookie: _creaves_session=…" \
    "http://127.0.0.1:3000/export/csv?query=entry_age"
HTTP/1.1 200 OK
Content-Disposition: attachment; filename="entry_age.csv"
Content-Type: text/csv; charset=utf-8

$ head -c 32 /tmp/entry_age.csv | xxd
00000000: efbb bf41 6e6e c3a9 652c 4167 6520 c3a0  ...Ann..e,Age ..
00000010: 206c 2745 6e74 72c3 a965 2c4e 6f6d 6272   l'Entr..e,Nombr

$ cat /tmp/entry_age.csv
﻿Année,Age à l'Entrée,Nombre
2026,adulte,772
2026,bébé,289
2026,juvénile,797
…
$ grep -c "juvénile" /tmp/entry_age.csv
6
```

File starts with `EF BB BF` (UTF-8 BOM), response declares `charset=utf-8`, and accented
values (`Année`, `Age à l'Entrée`, `juvénile`, `bébé`) decode as proper UTF-8.
