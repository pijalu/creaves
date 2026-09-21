# Issue #199-11 — Outtake type OT7 carries no rating (nullable rating)

Date: 2026-09-21
Commit: `e058d16` on `feature/open-issues-2026-10`

## Request (from pijalu/creaves#199)

Item 11: outtake type OT7 (`6be11a46-07c1-43c6-95d3-ae3630c8e5c0`, "Doublon")
must carry no rating; the UI/reports must not show a rating for it.

Decision (established): make `outtaketypes.rating` nullable, set OT7 to NULL,
hide the rating display when NULL.

## Implementation

- **Migration** `20261022090000_outtaketypes_rating_nullable` — makes
  `outtaketypes.rating` `INT NULL DEFAULT NULL` (additive/backward-compatible,
  existing ratings untouched) and clears OT7's rating.
- **Model** `models/outtaketype.go` — `Rating int` → `nulls.Int` with a
  comment documenting the NULL = no-outcome semantics.
- **Seed** `grifts/create_outtaketype.go` — canonical types use `nulls.Int`;
  OT7 seeds with `nulls.Int{}` (NULL, was -1). A duplicate file is an
  administrative error, not a Positive/Negative/Neutral outcome.
- **Webhook contract unchanged** — `actions/event_producer.go` and
  `actions/webhook_resync.go` forward `rating.Int` (0 when NULL) because the
  event payload keeps `Rating int` (a NULL simply becomes 0 = neutral on the
  wire, matching prior behavior for neutral types).
- **UI hides rating when NULL** (all 4 locales):
  - `templates/outtaketypes/show.plush*` — Type cell.
  - `templates/outtaketypes/index.plush*` — Outcome column.
  - `templates/animals/show.plush*` — outtake Type badge.
  - `templates/animals/index.plush*` — exit-status badge (#199-10 sort already
    NULL-safe: NULLs first on `asc`).
- **Form** `templates/outtaketypes/_form.plush*` — SelectTag gains a localized
  "None"/"Aucune"/"Keine"/"Geen" option with empty value so a rating can be
  cleared back to NULL.

## Tests

- `actions/outtaketype_rating_test.go` — `TestAnimalsShowHidesBadgeWhenRatingNull`
  seeds two animals (NULL-rating type vs −1 type) and asserts the animal show
  page renders no outcome badge for NULL but a danger/Negative badge for the
  rated type.
- `grifts/create_outtaketype_test.go` — `TestCanonicalOuttakeTypes` updated to
  expect OT7 = NULL.
- Full suite: `GO_ENV=test go test -count=1 -race -cover ./...` passes with
  only the two pre-existing grifts debts (template parity known-drift,
  migrations replay accent-collation duplicate). No NEW template-parity drift
  on any touched template pair.

## e2e verification (agent-browser, dev DB)

- Animal 4274 (outtake "Doublon", rating NULL): type name shown, **no**
  Positive/Negative/Neutral badge.
- Animal 8326 (outtake "DCD", rating −1): "Negative" badge rendered.
- `/outtaketypes/6be11a46-…` (Doublon show): Type label present, value empty.
