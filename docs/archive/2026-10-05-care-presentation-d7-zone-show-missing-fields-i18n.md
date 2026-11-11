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

**Verified** (Phase 7). Fix: `templates/zones/show.plush.html` rewritten —
every label/value through `t()` (`zone.details`, `zone.back_to_all`,
`zone.zone`, `zone.type`, `zone.type.external`, `zone.type.internal`,
`zone.default`, `zone.edit`, `zone.destroy`, `zone.destroy.confirm`,
`zone.requires_cleanup`), RequiresCleanup row added via `bool2html`; fr/de/nl
forks byte-identical (copied from the base file); the 9 new keys translated
in `locales/zones.{en-us,fr,de,nl}.yaml`.

Evidence: `go test ./actions/ -run 'TestZoneShow' -count=1` → ok
(`actions/zones_show_test.go`: 7-T1 all fields + back/edit/delete, 7-T1b
internal branch, 7-T2 4-locale sweep with banned-English assertions).
Live agent-browser sweep on dev (`/zones/4eafb532-…`, requires_cleanup=1):
fr "Détails de la zone"/"externe au centre"/"Nettoyage requis ✓",
en-US "Zone Details"/"external to the center"/"Requires cleanup ✓",
de "Zonedetails"/"extern zum Zentrum"/"Reinigung erforderlich ✓",
nl "Zonedetails"/"extern aan het centrum"/"Schoonmaak vereist ✓" —
no English literal in fr/de/nl.

---

## Phase 8 regression (2026-10-05) — ARCHIVED

Fix commit: `aab947c` (Phase 7: `/zones/{id}` shows RequiresCleanup + full fr/en/de/nl i18n).
- `go test ./actions ./models` → all green (7-T1 `TestZoneShowAllFields`, 7-T2 `TestZoneShowAllLocales` included).
- Sweep ×4 locales: `/zones/4eafb532-2dee-48dc-96df-9cfd48c58d49` → HTTP 200 in all locales; console + page errors empty; localized labels re-confirmed live (fr "Nettoyage requis", en-US "Requires cleanup", de "Reinigung erforderlich", nl "Schoonmaak vereist").
