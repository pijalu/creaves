# Admin multilingual/reference-data fix plan

## Scope reviewed

Admin reference-data resources and templates reviewed: animal ages, animal types, care types, drugs, outtake types, species, zones, entry causes, native statuses, and subside groups. Translation-aware handlers call `setTranslationValues` on New/Edit and `saveTranslations` on Create/Update. Forms use the shared `translations/_fields` partial, which exposes `en-US`, `de`, and `nl` inputs for each configured field. Display templates use shared `tname`, `tdesc`, `tfield`, or `tbase` helpers.

## Reproduction evidence

Authenticated `agent-browser` checks against `http://127.0.0.1:3000`:

- `/animalages/` rendered descriptions as `{ true}` even where database descriptions existed.
- Switching through `/lang/?lang=de&url=/animalages/` translated names (`Jugendlich`, `Baby`, `Erwachsen`) correctly, while descriptions still rendered `{ true}`.
- `/animalages/<id>/edit/` exposed six translation inputs: `tr_en_US_name`, `tr_en_US_description`, `tr_de_name`, `tr_de_description`, `tr_nl_name`, and `tr_nl_description`.

## Root cause

`tdesc` passes nullable `nulls.String` values into shared `baseString`. The generic coercion used `fmt.Sprintf("%v", value)` for `nulls.String`; because that type does not provide a string representation, Go formatted its struct fields, producing `{ true}` instead of the contained description (or empty text when invalid).

## Execution order

1. Fix `baseString` with an explicit `nulls.String` case (`Valid == false` => empty; otherwise `String`).
2. Add unit coverage for valid and invalid nullable strings and translation fallback behavior.
3. Run targeted Go tests, then the project verification command.
4. Re-run authenticated browser checks in French and German on list/show/edit pages; assert no brace-style values and translated names remain visible.

## Residual risks

- Full browser validation depends on local MySQL/schema state; current running instance reports unrelated schema drift (`resync_runs` and `event_streams.content_hash` missing), although admin pages remain usable.
- Translation persistence intentionally skips empty submitted values, so clearing an existing translation remains separate behavior and is not changed by this fix.
