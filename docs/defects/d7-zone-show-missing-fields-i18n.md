# D7 — `/zones/{id}`: `RequiresCleanup` field missing, hardcoded English labels

**Date**: 2026-10 (review round). **URL**: `/zones/{id}` (any zone id).
**Severity**: medium (data completeness + project all-language rule violation
on an admin detail page). **Guideline**: — (i18n rule: every user-facing
string localized in en-US/fr/de/nl; the zone's cleanup flag is now core
care-plan data — it drives the D3 cleanup pipeline via SM14).

## Observed (measured)

1. **Missing field**: `templates/zones/show.plush.html` never renders
   `RequiresCleanup`, although the model carries it
   (`models/zone.go:23` — `RequiresCleanup bool … db:"requires_cleanup"`) and
   the zone **form** exposes the checkbox. Since R9-4 flagged every zone
   `requires_cleanup = 1` (migration
   `20261005090080_zones_requires_cleanup_default_true`) and SM14/SR13 drive
   the daily cleanup plan from this flag, the detail page hides the very
   attribute that decides whether the zone appears in `/care_plan?kind=cleanup`.
2. **Hardcoded English**: the page ships literal English strings in every
   locale fork — heading "Zone Details" (l.2), "Back to all Zones" (l.6), and
   the type captions "external to the center" / "internal to the center"
   (l.26-29). No `t()` keys; the fr/de/nl forks render the same English text
   (verified: the strings are literals, not translation lookups).

## Expected

- A `RequiresCleanup` row on the detail view, rendered with the project
  `bool2html` convention and a localized label.
- Type / Default / RequiresCleanup labels **and values** resolved via `t()`
  keys added to all four locale files; zero hardcoded English in any fork.
- fr/de/nl template forks kept byte-identical to the base file (project
  fork-propagation rule).

## Reproduction steps

1. Log in as admin; open `/zones`, pick any zone → `/zones/{id}`.
2. Observe: no "requires cleanup" indication anywhere on the page, although
   the zone edit form shows the checkbox checked (post-R9-4 migration).
3. Switch locale to fr/de/nl: the heading still reads "Zone Details", the back
   link "Back to all Zones", and an external zone's caption "external to the
   center" — English in every locale.

## Root cause

`templates/zones/show.plush.html` (and its fr/de/nl forks) — the detail
template predates the `requires_cleanup` column and was never internationalized:
labels/values are literal strings and the `RequiresCleanup` field is absent
from the markup.

## Mapped test IDs

TM-8 (RequiresCleanup row rendered via bool2html; all labels/values through
`t()`; no English literals in fr/de/nl) — all four locales.

## Status

**Open.** Fix owner: Phase 7 (zones show completeness + i18n).
