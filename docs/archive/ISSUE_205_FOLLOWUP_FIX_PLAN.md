# Issue #205 Follow-up — Fix Plan (2026-11-07)

Follow-up on unresolved points from the added comment on
<https://github.com/pijalu/creaves/issues/205>. Four bugs, all in `creaves/`.

## Bug 1 — Media tab file picker text clipped (fr/de/nl)

**Symptom**: "Choisir un fichier (photo ou vidéo)" does not start at the left
edge of its frame and is cut off; OK in English, broken in other locales.

**Root cause**: Bootstrap 4.6 `.custom-file-label::after` renders the "Browse"
button as a right-side overlay (`z-index: 3`) with **no reserved space** — the
label text runs *underneath* the button and is clipped by `overflow: hidden`.
Longer translations (fr/de/nl) + wider localized browse buttons get clipped;
short English text survives. The earlier fix (#205 item 4) only reduced
`padding-left` and added ellipsis; it never reserved right-side space for the
button.

**Fix** (`assets/css/application.scss`):
- `.custom-file-label[for="attachmentFile"]`: add `padding-right: 7rem` so
  text never flows under the Browse/Parcourir/Durchsuchen/Bladeren overlay;
  keep `padding-left: 0.25rem` (glued to left edge) and ellipsis.

**Fix** (`templates/animals/show.plush{,.fr,.de,.nl}.html`): widen the picker
frame from `max-width:360px` to `width:100%; max-width:480px` so long
placeholders fit.

## Bug 2 — Treatment created from animal record lands on edit mode

**Symptom**: after creating a treatment from the animal record, the user is
redirected to the animal record **in edit mode** and must click "Sauvegarder".

**Root cause**: `templates/animals/show.plush{,.fr,.de,.nl}.html` — the
"New treatment" button passes
`back: "/animals/"+animal.ID+"/edit#nav-treatment"` (read-mode template!). The
edit-mode template (`_form.plush*.html`) already uses the correct read-mode
back URL.

**Fix**: change the back param to `"/animals/"+animal.ID+"#nav-treatment"` in
all 4 show templates. `TreatmentsResource.Create` (actions/treatments.go:323)
already honors `back` via `safeBackParam`.

## Bug 3 — Veterinaryvisit "Veterinary" field not translated

**Symptom**: in the FR version the vet field label shows "Veterinary".

**Root cause**: `templates/veterinaryvisits/_form.plush*.html` uses
`f.InputTag("Veterinary")` with no explicit label — Buffalo derives the label
from the field name in every locale.

**Fix**: explicit label in each locale:
- en: `Veterinary`, fr: `Vétérinaire`, de: `Tierarzt`, nl: `Dierenarts`.

## Bug 4 — Outtake crash with feeding end at midnight (Error 1292)

**Symptom**: creating an outtake for an animal whose feeding end is set to
00:00 crashes:
`mysql update: Error 1292 (22007): Incorrect datetime value: '0000-00-00' for column 'feeding_end' at row 1`.

**Root cause**: `timeToNullTime()` (actions/helper.go:40) stores time-only
feeding windows as `time.Date(1, 1, 1, H, M, 0, 1, UTC)` — year 1. MySQL
`DATETIME` columns drop sub-second precision, so `feeding_end = 00:00` is
stored as `0001-01-01 00:00:00`. On read-back (parseTime), that is exactly Go's
zero `time.Time{}`; go-sql-driver serializes zero times as `'0000-00-00'`, and
the next `tx.Update(animal)` (e.g. outtake creation) fails on strict-mode
servers (NO_ZERO_DATE). Any feeding window containing midnight (start or end)
is affected.

Migration `20261007140000_animal_feeding_zero_dates_null_205_item15` only
cleans **legacy** rows (and loses the configured window by setting it NULL);
the write path still produces corrupt rows today, so the crash is reproducible
on a freshly edited animal.

**Fix**:
1. `actions/helper.go` — sentinel date `2000-01-01` (inside DATETIME range,
   never equals Go zero time), drop the `nsec: 1` hack:
   `time.Date(2000, 1, 1, t.Hour(), t.Minute(), 0, 0, time.UTC)`.
   All consumers (calculateFeeding, FeedingStartFmt/EndFmt, feeding sort) use
   only Hour()/Minute()/Format("15:04") — date part is irrelevant.
2. New migration `20261107090000_animal_feeding_window_year2000`:
   convert any residual year-1 windows to year 2000, **preserving** HH:MM
   (no-op on DBs where the item-15 migration already NULLed them):
   ```sql
   UPDATE animals
   SET feeding_start = CONCAT('2000-01-01 ', TIME(feeding_start))
   WHERE feeding_start IS NOT NULL AND feeding_start < '1000-01-01';
   -- same for feeding_end
   ```
   No down migration (data repair).
3. Regression test: `TestTimeToNullTime` — midnight input must not produce a
   zero `time.Time` and must use year 2000.

## Validation

- Load production dump `creaves-db-2026-11-07.gz` into the local `creaves` DB
  (backup of current dev DB taken first), then `buffalo pop migrate up` —
  this replays the production upgrade path incl. the new migration.
- `go test ./actions ./models` (MySQL test DB).
- Chrome DevTools E2E against the migrated production copy:
  1. Bug 4: set an animal's feeding end to 00:00, save, create an outtake →
     no Error 1292; feeding window still displayed correctly.
  2. Bug 2: create a treatment from the animal record → lands on read-mode
     record (`#nav-treatment`), not edit mode.
  3. Bug 1: media tab placeholder fully visible & left-aligned in fr/de/nl.
  4. Bug 3: veterinaryvisit new/edit form shows "Vétérinaire" (fr),
     "Tierarzt" (de), "Dierenarts" (nl), "Veterinary" (en).
