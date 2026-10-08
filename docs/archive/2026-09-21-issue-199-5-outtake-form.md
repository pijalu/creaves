# Fix archive — 2026-09-21 — Issue #199 item 5 (outtake form rework)

Source: https://github.com/pijalu/creaves/issues/199 ("Correctifs 21/09").
Branch: `feature/open-issues-2026-10`.

## Original report
> (a) No stay-duration bucket field. (b) `+24h`/`+48h` buttons still present in
> `templates/outtakes/_form.plush*.html` — they mutate the real outtake date.
> (c) FR label of `precise_location` not exactly "Adresse, lieu précis".
> (d) When the outtake type's `location_mode` is not `free`, the Location field
> is hidden but Precise location is only greyed out.
>
> Expected: display-only 4-bucket select (-12H / 12–24H / 24–48H / +48H)
> auto-selected from the computed stay duration, user-adjustable, never writing
> back to the date; buttons removed; exact FR label; precise location hidden
> for non-free modes.

## Fix
All in `templates/outtakes/_form.plush{,.fr,.de,.nl}.html`:
- Removed the `+24h`/`+48h` buttons and the `setStay()` helper (they silently
  rewrote the real outtake date).
- Added `<select id="stayDurationBucket">` with the 4 buckets. On every
  outtake-date change, `updateDuration()` recomputes the stay hours from the
  intake date and auto-selects the matching bucket (`<12h → -12H`,
  `<24h → 12-24H`, `<48h → 24-48H`, else `+48H`). The select has no `name`, is
  never submitted, and changing it never touches the date field — purely a
  display aid (per reporter decision: not stored, derived from dates). The
  localized human-readable duration text (`39 h`, `2 j - 3 h`…) is kept.
- FR label of `precise_location` is now exactly "Adresse, lieu précis" in both
  the location-modes block and the fallback `f.InputTag`.
- Ported the base template's `applyMode()` fix to fr/de/nl (previously only
  the EN template had it): the precise-location group is hidden and disabled
  whenever the selected type's `location_mode` is not `free`.
- New locale key `outtake.duration.bucket-label` (en/fr/de/nl) used as the
  select's `aria-label`.

## Tests / quality
- `go vet ./...`, `staticcheck ./...` clean; gocognit/gocyclo counts unchanged
  (templates only).
- `GO_ENV=test go test ./actions -count=1` green; template parity test reports
  only pre-existing KNOWN/frozen drift for `outtakes/_form` pairs (the 8 NEW
  drift flags on `config/_sync_form` + `guest.plush.*` exist on the clean tree
  too). Full `-race` run shows pre-existing flaky tests (random failures also
  reproducible on the clean tree; each failing test passes standalone).

## Verification (e2e, agent-browser, authenticated admin)
```
open /outtakes/new?animal_year_number=1923/26
-> #stayDurationBucket = "24-48" with text "39 h"; #stay24h/#stay48h absent
set bucket to -12H -> #outtakeDate unchanged (display-only)
move outtake date -10h -> text "29 h", bucket auto-reselected "24-48"
type "Released" (mode free)  -> precise location visible + enabled
type "Transferred" (mode list) -> precise hidden + disabled, location list shown
type "DCD" (mode none) -> location + precise hidden
FR variant (/lang/?lang=fr): label exactly "Adresse, lieu précis", same behavior
```

## Commit
- creaves `8b820df` fix(outtakes): stay-duration bucket select, drop
  +24h/+48h, precise-location label/hide (#199-5)
