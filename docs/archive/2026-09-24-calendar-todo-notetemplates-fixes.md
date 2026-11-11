# Archive — bugs.md items fixed 2026-09-24 (calendar TZ, TODO filtering, note-templates i18n)

Plan executed: `docs/plans/2026-10-18-calendar-todo-notetemplates-fixes.md`
(created 2026-10-18 per repo clock; session system date 2026-09-24 CEST).

All three bugs lived in the **creaves** app. Each was fixed, tested
(unit + action tests), e2e-validated with agent-browser, and committed
separately.

## Bug 1 — Calendar / datetime language & timezone

> Calendar / date time should match the language of the UI — currently, on a
> FR page, dates show in English and not within the current time zone (e.g.
> todos are shown 2h earlier than local time).

- **Language part** was already fixed by commit `f96c57f` (flatpickr fr/de/nl
  l10n packs + `<html lang>` on all layouts); e2e re-verified it still holds.
- **Timezone part**: `parseTodoDate()` in `actions/todos.go` parsed the
  datetime-local form value with `time.ParseInLocation(time.Local)` while the
  MySQL DSN has no `loc` parameter → the driver stored the UTC wall clock in
  the naive DATETIME column and a 10:00 local pick displayed as 08:00. Fixed
  by parsing in the UTC frame (same convention as the buffalo binder, cf.
  `models.FormWallClockNow`) and by using `FormWallClockNow()` for the default
  todo_date, done_at and the Status classification "now".
- **Commit**: `a85c177 fix(todos): keep picked todo times in local wall clock, not UTC`
- **Tests**: `TestParseTodoDateWallClock` (new), `TestTodosCreateDoneDelete`
  expectation corrected (10:00 pick round-trips as 10:00).
- **E2E evidence** (agent-browser, http://127.0.0.1:3000, admin login,
  system time 2026-09-24 09:23 CEST):
  - POST /todos with `todo_date=2026-09-24T11:23` (local now + 2h)
  - GET /todos card text: `"E2E-TZ-CHECK 24/09/2026 11:23 24/09/2026 Done"`
    → picked 11:23 local displayed as **11:23** (pre-fix would show 09:23).
  - GET /dashboard TODO row: `"E2E-TZ-CHECK 24/09/2026 11:23 Done"`.
  - flatpickr on /registersnapshot (FR UI): weekdays
    `["lun","mar","mer","jeu","ven","sam","dim"]`, month dropdown
    `janvier…décembre` — French, not English.
  - Browser console: no errors (only Buffalo live-reload logs).

## Bug 2 — TODO dashboard filtering + collapsed "later" section

> Dashboard: only show TODOs that need to be done within the next 8h (or too
> late). TODO screen: add a collapsed "to be done" section and move all todos
> that are not within the next 8h out of the main list.

- Interpretation recorded in the plan: the 8h window matches the existing
  `models.TodoOverdueAfter`; future todos are moved to a collapsed display
  section, **not** auto-marked done in the DB (display-only split; done
  semantics unchanged).
- `models.Todo.DueSoon(now)` + `TodoDueSoonWindow = 8h` added;
  `listDashboardTodos` filters `done_at IS NULL AND todo_date <= now+8h`;
  `TodosIndex` splits open todos into due/later sections; collapsed
  "to be done later" section added to `templates/todos/todos.plush.html` and
  the fr/de/nl variants ("À faire plus tard", "Später zu erledigen",
  "Later te doen").
- **Commit**: `1e27061 feat(todos): dashboard shows only todos due within 8h; later todos collapsed on /todos`
- **Tests**: `TestTodoDueSoon` (8h boundary table test),
  `TestTodosDueSoonFiltering` (dashboard filter + section placement +
  no-auto-done invariant).
- **E2E evidence**:
  - Created `E2E-TZ-CHECK` (due +2h) and `E2E-LATER-CHECK` (due +48h).
  - GET /dashboard: TODO header `TODOs (1)`; table contains E2E-TZ-CHECK,
    does NOT contain E2E-LATER-CHECK.
  - GET /todos: `{"openHasTZ":true,"openHasLater":false,
    "laterSectionClass":"collapse","laterHasLater":true,"laterHasTZ":false}`
    → later section exists, collapsed by default, holds only the +48h todo.
  - Cleanup: both E2E todos deleted from the dev DB after validation.

## Bug 3 — Care note templates UI in all languages

> It should be in *all* languages but currently only in English.

- `templates/care_templates/{index,new,edit,_form}.plush.html` had hardcoded
  English chrome. Replaced with `t()` keys; translations added to
  `locales/care_templates.{fr,de,nl,en-us}.yaml` (titles, table headers
  Name/Content/Owner, Save/Cancel/Edit/Destroy, delete confirmation).
- **Commit**: `1e6bc6c i18n(care_templates): translate note-templates admin UI into fr/de/nl`
- **E2E evidence** (UI switched to French via /lang/?lang=fr):
  - GET /care_templates: heading `"Modèles de note pour soin"`, button
    `"Créer un nouveau modèle"`, headers `["Nom","Contenu","Propriétaire",""]`.
  - GET /care_templates/new: heading `"Nouveau modèle de note pour soin"`,
    labels `["Nom","Contenu"]`, buttons `"Enregistrer"`, `"Annuler"`.

## Validation summary

- `go vet ./...` — clean
- `staticcheck ./...` — clean
- `gocognit -over 15 .` — no new offenders (only pre-existing baseline ones)
- `gocyclo -over 12 .` — no new offenders (only pre-existing baseline ones)
- `go test -count=1 -race -cover ./...` — ok (actions 51.1%, models 70.1%,
  grifts, utils 90.5%)
- e2e: all checks above, no browser console errors
