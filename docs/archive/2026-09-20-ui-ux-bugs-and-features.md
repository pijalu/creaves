# 2026-09-20 — UI/UX bugs & features batch (6 items)

Archived from `bugs.md` after all 6 items were fixed, e2e-validated
(agent-browser) and committed on branch `feature/open-issues-2026-10`
(creaves repo). Original entries kept verbatim below the resolution notes.

## Resolutions

| # | Item | Commit | Resolution |
|---|------|--------|------------|
| 1 | Animal fiche tabs overlap action buttons (small screens, gh#198) | `05be5c6` | Header got `clearfix` + button group `float-right mb-2`; `.nav-tabs` scrolls horizontally (`flex-wrap: nowrap; overflow-x: auto`) below `md`. Applied to all 4 `animals/show.plush.*`. E2E at 360/768/1024 px: no overlap, edit clickable, all tabs reachable via horizontal scroll. Hamburger approach evaluated — horizontal scroll chosen (keeps tab semantics, no extra JS). |
| 2 | Navbar icon/label misalignment | `c6c57bc` | `#navbarSupportedContent .navbar-nav .nav-link` set to `display:flex; align-items:center; white-space:nowrap` — labels never wrap under icons; existing `d-none d-xl-inline` breakpoint already hides labels before overflow. E2E viewport sweep 320→1400 px: no two-line entries, no clipped labels. |
| 3 | Navbar search in-care suggestions | `28d1d35` | Navbar search input (`id="navbarAnimalSearch"`) wired to the same `autoComplete` plugin + `/suggestions/animal_in_care` source as the landing page, in all 4 `application.plush.*` variants. E2E: typed partial number → 5 suggestions → click fills `1986/25` → submit lands on `/animals/7775`. (Note for future e2e: jquery.auto-complete listens to `keyup` — dispatch `jQuery.Event("keyup")` after `fill`.) |
| 4 | TODOs page: collapse Done + post-it cards | `a649843` | Full rewrite of `todos/todos.plush.*` (4 locales): open TODOs as a post-it card grid (`#openTodoCards`), click toggles an actions footer (inline Done without `confirm()`, delete keeps `confirm()`); Done table wrapped in collapsed-by-default `#doneTodosCollapse` with count badge + chevron. Existing POST endpoints unchanged. E2E: collapse default → expand → reopen works; card Done/cancel work; mobile width OK. |
| 5 | Dashboard TODOs: Done button + clickable row | `8f1d9d5` | Dashboard TODO table (4 locales): description links to `/todos/:id/edit` (no show route exists — cheap option per plan), new Actions cell with inline Done form POST `/todos/:id/done`. E2E: Done → row gone after reload + listed under Done on `/todos`; description click → edit page. |
| 6 | Calendar widgets ignore UI language | `f96c57f` | (a) `<html lang="<%= uiLang() %>">` on application + guest layouts (all 4 locales) via new `uiLang()` plush helper (`actions/render.go`, lang cookie via `normalizeUILang`, default `fr`). (b) flatpickr: require fr/de/nl l10n packs and `flatpickr.localize(flatpickr.l10ns[lang])` in the existing select2/DataTables language IIFE. **Gotcha**: the packs' named UMD exports (`require('flatpickr/dist/l10n/de.js').de`) come back `undefined` through webpack interop and silently no-op `localize` — read the self-registered `flatpickr.l10ns` entries instead. Per-call `dateFormat` (`Y/m/d H:i`) unaffected. E2E verified on fresh loads: de (`Januar`, Mo–So week, `firstDayOfWeek: 1`, opened outtake picker German), nl, fr, en-US. Native `datetime-local` (todo edit) follows browser locale — documented as acceptable. |

### Quality gates (guideline #7), 2026-09-20

- `go vet ./...` — clean
- `staticcheck ./...` — clean
- `gocognit -over 15 .` / `gocyclo -over 12 .` — findings only in
  pre-existing functions (e.g. `buildEventPayloadInto`, `syncStartupTable`);
  none introduced by these 6 commits.
- `go test -count=1 -race -cover ./actions ./models ./utils ./stuff/...` —
  all ok (actions 46.9%, models 71.9%, utils 90.5%, feeding 73.8%).
- **Known pre-existing failure (not from this batch)**:
  `grifts TestMigrationsReplayOnEmptyDatabase` fails on MySQL 8 with
  `Duplicate entry 'Relacher' for key 'outtaketypes.outtaketypes_name_idx'`.
  The unique `name` index + server collation `utf8mb4_0900_ai_ci`
  (accent-insensitive) makes the test's seeded 'Relacher'/'Relaché' rows
  collide. The test was green at its introduction commit `2f3f583`
  (MySQL 5.7/MariaDB-era collations where accented forms sort differently)
  and is red at `1eb4854` before any of these 6 commits — environment
  (MySQL 8 upgrade) issue, not a code regression from this batch.
- Flaky note: `actions TestScientistVetVisitFormOnOuttakenAnimal` failed once
  inside a broad `-run "Outtake|FormWallClock"` parallel run (HTTP 409) and
  passes 3/3 in isolation and on re-run — pre-existing suite interference.

---

## Original bugs.md entries (verbatim)

## Bug 1 — Animal fiche: tab menu overlaps the action buttons on small screens

**Observed**: on small screens (systematically on mobile), the tab bar
("Général", "Découverte", "Admission", "Suivis", …) of the animal sheet
overlaps the floated action menu ("Back to animals in care", guest QR, outtake,
edit, delete), making e.g. the animal edit button unusable.
(`creaves/templates/animals/show.plush.html`: header `.py-4.mb-2` contains
`div.float-right` with the buttons; the parent is not clearfixed and the
`nav.nav-tabs` bar right below has no wrapping, so at ~575–990 px the button
block drops onto the tab row.)

**Expected**: action buttons stay fully visible and clickable at every
viewport; they must never overload/overlap the tab bar.

## Bug 2 — Navbar: inconsistent icon/label alignment (icon above text on some entries)

**Observed**: at some viewport widths some navbar entries wrap into two
lines (icon, then text under it) while neighbours keep icon + text side by
side, producing an unaligned/odd menu bar.
(`creaves/templates/application.plush.html`: labels use
`d-none d-xl-inline`; between `lg` and `xl` some entries fit on one line and
others wrap depending on label length.)

**Expected**: consistent layout — an entry either shows icon + full text side
by side or icon only; if it must be two lines, icon/text centered. Preferred:
only show the text when it can be fully displayed next to the icon.

## Feature — Navbar animal search box: in-care suggestions

**Observed**: the navbar search box (`application.plush.html`, form
`action=animalsPath()`, input `animal_year_number`, pattern `[0-9]+(/[0-9]{2})?`)
is free-text only. The landing page "Animal number" field already autocompletes
via `GET /suggestions/animal_in_care` (returns `"123/24"` strings for animals
without outtake) using the `autoComplete` plugin.

**Expected**: the navbar search offers the same in-care animal suggestions.

## UX — Review TODOs page

**Observed**: `/todos` renders two stacked tables ("Open" then "Done",
`creaves/templates/todos/todos.plush.html`). The Done table is always fully
expanded and grows unbounded, pushing the page down; the row-based layout is
dry and the Done action uses a blocking `confirm()`.

**Expected**:
1. The "Done" section is collapsed by default (Bootstrap collapse around the
   Done table, header shows count, e.g. `Done (12)` with a chevron toggle).
2. Nicer page proposal: card/"post-it" board for open TODOs (color badge as
   card accent); clicking a card flips/expands it to reveal the actions
   (Done / edit / delete) with inline Done + cancel instead of the browser
   `confirm()` dialog. Keep the existing POST endpoints
   (`/todos/:id/done|reopen|delete`) — no backend change expected.

## UX — Dashboard TODOs: Done button + clickable row

**Observed**: the dashboard TODO table
(`creaves/templates/dashboard/dashboard.plush.html:321-342`) is read-only:
badge + description + date. Marking a todo done requires going to `/todos`,
and clicking a row does nothing.

**Expected**:
1. Each dashboard todo row has an inline "Done" button (POST
   `/todos/:todo_id/done`, endpoint already exists — same form as the todos
   page, with `authenticity_token`).
2. Clicking a todo (description/row) opens its view. There is no todo show
   route today (only `/todos/:id/edit`); decide: link to the edit page
   (cheap) or add a read-only `GET /todos/{todo_id}` view (nicer, matches
   "open its view"). Done button must not trigger row navigation
   (`event.stopPropagation` / separate cell).

## Bug — Calendar widgets ignore the UI language (English in German UI)

**Observed**: calendar/date pickers render in English even when the UI is in
German (e.g. todo edit). Two root causes:
1. Native pickers: `todos/edit.plush.html:17` and `new.plush.html` use
   `<input type="datetime-local">` — its calendar language follows the
   browser locale, and `application.plush.html` has `<html>` with **no
   `lang` attribute**, so an English browser shows an English calendar in a
   German UI.
2. Flatpickr pickers (40 templates, e.g. outtake/intake/discovery forms):
   `assets/js/application.js` does bare `require("flatpickr")` and no
   template passes a `locale:` option — flatpickr always renders English
   month/day names.

**Expected**: calendar widgets follow the active UI language (fr/en/de/nl).
