# Fix Plan — Issue #205 follow-up comment (2026-11)

Source: https://github.com/pijalu/creaves/issues/205 latest comment (FR). Four unresolved points.

## Bug A — Media tab file chooser clipped / not left-aligned (non-EN locales)

**Symptom**: In the animal page "Média" tab, the label "Choisir un fichier (photo ou vidéo)" does not start at the left edge of its frame and its tail is clipped. OK in English, still broken in fr/de/nl.

**Localization**:
- `templates/animals/show.plush.{html,fr,de,nl}.html` — `<div class="custom-file mr-2" style="max-width:360px">` + `.custom-file-label[for="attachmentFile"]`.
- Previous partial fix: `assets/css/application.scss` sets `padding-left: 0.25rem` + ellipsis. Insufficient: label `padding-right` is only `.75rem`, so the text runs *under* the Bootstrap `::after` "Browse/Parcourir/Durchsuchen/Bladeren" button and is clipped/ellipsized; `max-width:360px` is too narrow once the right-side button width is accounted for.

**Fix**:
1. `assets/css/application.scss`: add `padding-right: 7rem` (room for the widest locale button "Durchsuchen") to `.custom-file-label[for="attachmentFile"]`.
2. All 4 `show.plush.*` templates: widen frame `style="max-width:360px"` → `style="width:100%; max-width:480px"`.

## Bug B — Creating a treatment from the animal record returns to edit mode

**Symptom**: After creating a treatment from the animal sheet, the redirect lands on `/animals/<id>/edit#nav-treatment`, forcing the user to click "Sauvegarder".

**Localization**: `templates/animals/show.plush.html:449`, `show.plush.fr.html:452`, `show.plush.de.html:449`, `show.plush.nl.html:449` — `newTreatmentsPath({..., back: "/animals/"+animal.ID+"/edit(#|/)#nav-treatment"})`. The edit-mode partial (`_form.plush.*`) already uses the read-mode back URL.

**Fix**: change the 4 `show.plush.*` back params to `"/animals/"+animal.ID+"#nav-treatment"` (read mode), matching `_form.plush.*`.

## Bug C — veterinaryvisits form: FR label still "Veterinary"

**Symptom**: In create/edit veterinary visit, the veterinarian free-text field label reads "veterinary" in the French UI.

**Localization**: `templates/veterinaryvisits/_form.plush.{html,fr,de,nl}.html` — `<%= f.InputTag("Veterinary")%>` derives its label from the struct field name, untranslated in every locale.

**Fix**: add explicit `label:` to the InputTag in all 4 variants:
- base (en): `"Veterinary"`
- fr: `"Vétérinaire"`
- de: `"Tierarzt"`
- nl: `"Dierenarts"`

## Bug D — Outtake crash when feeding end is 00:00 — `Error 1292: Incorrect datetime value: '0000-00-00'`

**Root cause** (confirmed in driver source, `go-sql-driver/mysql@v1.10.1/packets.go:1208`):
1. `actions/helper.go:timeToNullTime` builds time-only values as `time.Date(1,1,1,h,m,0,1,UTC)` (the `nsec=1` hack was meant to dodge `IsZero()`).
2. MySQL `DATETIME` (no fractional seconds) truncates the nanosecond → midnight end stored as `'0001-01-01 00:00:00'`.
3. Reading back with `parseTime` yields exactly the Go zero `time.Time{}` → `nulls.Time{Valid:true, Time:zero}`.
4. On outtake creation, `OuttakesResource.Create` does `tx.Update(animal)` (ring save / outtake link); the driver serializes the zero time as `'0000-00-00'` → strict-mode MySQL rejects with Error 1292 on `feeding_end`. Same applies to `feeding_start` set to 00:00.

**Fix** (migrate mode — data change, rollback documented):
1. `actions/helper.go:timeToNullTime` — use a non-zero sentinel date `time.Date(2000, 1, 1, h, m, 0, 0, time.UTC)`. Only `Hour()`/`Minute()`/`Format("15:04")` are ever consumed (`feeding.go`, `stuff.go`, `animal.go` fmt helpers), so the date part change is behavior-preserving.
2. Migration (raw SQL, up + down) rewriting existing rows:
   ```sql
   UPDATE animals SET feeding_start = CONCAT('2000-01-01 ', TIME(feeding_start)) WHERE feeding_start IS NOT NULL;
   UPDATE animals SET feeding_end   = CONCAT('2000-01-01 ', TIME(feeding_end))   WHERE feeding_end   IS NOT NULL;
   ```
   Down migration rewrites the date part back to `'0001-01-01'`. (Legacy rows already at `'0001-01-01 00:00:00'` remain readable under the old code; new code never writes year 1.)
3. Regression test: animal with feeding end `00:00` re-saved (outtake-style `tx.Update`) must not error.

## Validation strategy

- Load production dump `creaves-db-2026-11-07.gz` into the local MySQL `creaves` dev DB (backup current first), run `buffalo pop migrate up` to bring it to current schema incl. the new migration, verify feeding values converted.
- `go test ./actions ./models` (real checks for bug D + timeToNullTime).
- Browser E2E (Chrome DevTools MCP, per AGENTS.md mandatory testing) in fr **and** en/de/nl spot checks:
  - Media tab: label fully visible, flush left, in all 4 locales.
  - Create treatment from animal sheet → lands on read-mode `#nav-treatment`.
  - veterinaryvisits new/edit: label = Vétérinaire/Tierarzt/Dierenarts/Veterinary.
  - Outtake on animal with feeding end 00:00 → no crash.

## Risk

- Bug D touches stored data: migration is additive-in-place, reversible, idempotent-ish (re-running keeps 2000-date). Rollback = down migration.
- Bug A: pure CSS/inline style, no behavior change.
- Bugs B/C: template-only, no logic change.
