# CSV export encoding: accented characters render as boxes/mojibake

Archived resolved entry from bugs.md on 2026-09-16.

## Reported bug

CSV exports may show odd characters for values with accentuated characters (e.g. the
`animal_age` / `entry_age` column in the configured `/export/csv` exports). Accented
characters (é, è, à…) may display as multiple boxes or mojibake when the file is opened
in Excel or other spreadsheet tools — pointing to an encoding mismatch between the CSV
file and the table/reader encoding.

## Resolution

- Root cause: `export.RunQuery` (`creaves/export/export.go`) streamed raw UTF-8 with no
  byte-order mark and `Content-Type: text/csv` without a charset, so Excel assumed the
  local ANSI codepage and garbled accented values. The newer animal-search and annual
  report exports already wrote a UTF-8 BOM (`actions/csv_export.go`).
- Fix: `/export/csv` now sends `Content-Type: text/csv; charset=utf-8` and prefixes the
  body with the UTF-8 BOM (`EF BB BF`) via a new tested `writeCSV` helper. Delimiter
  unchanged (comma).
- Plan + validation: `creaves/docs/csv-export-encoding-plan.md` (includes unit/HTTP tests
  and agent-browser e2e evidence: BOM bytes + `juvénile`/`Année` decoded correctly on
  http://127.0.0.1:3000/export/csv?query=entry_age).
