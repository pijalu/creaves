# Plan: Menu/template desync — admin submenus

**Date:** 2026-09-11
**Status:** Implemented, tested, closed
**Bugs.md item:** "Menu/template desync: not all template are in sync — Menu
should follow english group on administration *but* use sub-menu (and small
extension of group): Configuration, System, Synchronization."

## Problem

The EN `application.plush.html` admin dropdown was grouped with
`dropdown-header` sections (Configuration / System), while the FR/DE/NL
templates were flat link lists with ad-hoc dividers, a different link order
and no group labels — the locales had drifted apart.

## Fix

All four locale templates (`application.plush.{html,fr,de,nl}.html`) now
share one identical structure: the Administration dropdown holds three
sub-menus (small extension of the EN groups):

| Sub-menu | Items |
|----------|-------|
| Configuration / Configuration / Konfiguration / Configuratie | Drugs, Animal age, Animal types, Outcome types, Care types, Travel types, Localities, Zones, Translations |
| System / Système / System / Systeem | All Routes, Users, Maintenance, Configuration (settings page) |
| Synchronization / Synchronisation / Synchronisation / Synchronisatie | Event Stream, Webhook resync |

Same 15 links, same order, same grouping in every locale — only the labels
are translated.

### Submenu mechanics (Bootstrap 4.6 has no native nested dropdowns)

- Markup: `.dropdown-submenu` wrappers inside the admin `.dropdown-menu`;
  the submenu toggle carries no `data-toggle="dropdown"` (that would
  re-trigger the parent handler).
- `assets/js/application.js`: delegated click handler toggles `.show` on the
  submenu + its `.dropdown-menu`, closes sibling submenus, and a
  `hide.bs.dropdown` handler resets submenus when the parent dropdown closes.
- `assets/css/_navbar.scss`: flyout positioning (`top:0; left:100%`),
  right-pointing caret; below the sm breakpoint (navbar-expand-sm) the
  submenus stack inline and indented instead of flying out of the viewport.
- Webpack assets rebuilt (`yarn build`); `public/assets` is gitignored and
  rebuilt by `buffalo dev` / at deploy time.

## Test approach & validation

- Structural check: the admin dropdown's href sequence and submenu count are
  byte-identical across the four templates (compared programmatically).
- **E2E (agent-browser, http://127.0.0.1:3000, admin login)** — per locale
  (EN/FR/DE/NL via `/lang/?lang=…`): Administration dropdown shows exactly
  the three translated sub-menus; clicking a sub-menu opens its items while
  siblings close; all expected items present (e.g. FR: Médicaments…
  Traductions; DE: Ereignisstrom, Webhook-Neusynchronisation); clicking
  "Flux d'événements" navigates to `/event_streams`. Also verified at a
  500px-wide viewport: hamburger menu opens, sub-menus expand inline.
- **Code quality** (each run separately): `go vet` clean; `staticcheck` 53
  finding lines before and after (all pre-existing, unrelated); `gocognit
  -over 15 .` 62 / `gocyclo -over 12 .` 47 warnings before and after (no Go
  files touched); `go test -count=1 -race -cover ./...` all packages green.

## Commit

`6869dbb` fix(navbar): sync admin menu across locales with
Configuration/System/Synchronization submenus
