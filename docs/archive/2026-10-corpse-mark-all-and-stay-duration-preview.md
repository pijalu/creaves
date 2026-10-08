# Corpse register mark-all + outtake stay-duration JS preview (RESOLVED)

Two fixes on branch `feature/open-issues-2026-10` (creaves repo), bugs.md items 1 and 2.

## 1. Corpses — multi-select "mark" becomes "mark all"

**Bug**: On `/reports/corpses`, clicking a row "Mark" button with multiple entries
selected reset the selection to that single row — impossible to mark more than one
entry at a time.

**Fix** (`545cef8`, templates `reports/corpses.plush.{html,fr,de,nl}.html`, JS only):
- With ≥2 rows checked, every `.quick-mark` button relabels to "Mark all"
  (en) / "Tout marquer" (fr) / "Alle markieren" (de) / "Alles markeren" (nl) via
  `refreshMarkLabels()`, bound to checkbox + checkAll change events.
- The quick-mark click keeps the selection untouched when ≥2 rows are checked and
  opens the destination modal for all of them; with 0/1 checked it keeps the old
  row-only behavior.
- No server change: `/reports/corpses/mark` already accepts multiple `outtake_ids`.

**E2E** (agent-browser, dev :3000, `/reports/corpses?year=2026`, 1051 rows):
- Check 2 rows → buttons relabel "Mark all" (fr: "Tout marquer").
- Click a row Mark-all → 2 checkboxes stay checked, modal text "2 corpse(s) selected".
- Uncheck all, click a row Mark → exactly 1 checkbox checked (that row), "1 corpse(s)
  selected" — single-row behavior preserved.
- Pre-existing test failures (TestCorpseRegisterMark/Query/Unmark) identical to
  baseline (test-DB state), unrelated to this template-only change.

## 2. Outtakes — stay duration live preview (incl. cage batch form)

**Bug**: On `/outtakes/new`, `/outtakes/{id}/edit` **and `/outtakes/cage?cage=…`**,
the live stay-duration preview (`stayDurationText` in shared partial
`outtakes/_form.plush*.html`) always rendered bare hours (e.g. `1008 h`) — never the
human form; units hard-coded, not localized.

**Fix** (`b2b1fa1`):
- JS `updateDuration()` in `templates/outtakes/_form.plush.{html,fr,de,nl}.html`
  mirrors the server-side `stayDurationHours` helper: plain hours ≤48h, then
  "days - leftover hours".
- Units localized server-side via new i18n keys `outtake.duration.day-unit` /
  `outtake.duration.hour-unit` in `locales/outtakes.{en-us,fr,de,nl}.yaml`
  (d/j/T + h), consistent with the show pages. (Note: localized partial variants
  are not resolved by the render engine for partials — units therefore come from
  `t()` keys in the shared base partial, and all 4 variants carry identical JS.)

**E2E** (agent-browser, `/outtakes/cage?cage=B2`, outtake date set to 2026/11/20):
- en: `104 d - 20 h`, fr: `104 j - 20 h`, de: `104 T - 20 h` — was `2516 h` before.
- Single-animal form `/outtakes/new?animal_year_number=927/22`: days-hours shown too.
- `go test ./actions -run 'StayDuration|Outtake'` — pass (only the 2 known
  pre-existing failures); `grifts -run LocaleKeyParity` — pass.

## Quality

`go vet ./...`, `staticcheck ./...` clean; `gocognit -over 15` / `gocyclo -over 12`
hits pre-existing; full `go test -count=1 -race ./...` failures identical to the
pre-session baseline (26, environment/test-DB related).
