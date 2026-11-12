# bugs.md fix plan — 2026-10-18 (calendar TZ, TODO filtering, note-templates i18n)

Bugs from `/bugs.md` "To do" section. All three live in the **creaves** app.

## Bug 1 — Calendar / datetime language & timezone

> Calendar / date time should match the language of the UI — currently, on a FR
> page, dates show in English and not in the current time zone (e.g. todos are
> shown 2h earlier than local time).

### Investigation findings

- **Language part**: already fixed by commit `f96c57f` ("i18n: calendar widgets
  follow the UI language") — flatpickr fr/de/nl l10n packs + `<html lang>` on
  all layouts. No work left there; e2e must confirm it still holds.
- **Timezone part (still broken)**: `actions/todos.go parseTodoDate()` parses
  the `datetime-local` form value with `time.ParseInLocation(..., time.Local)`,
  and `TodosNew`/`TodosDone` use `time.Now()` directly. But the MySQL DSN has
  **no `loc` parameter** → go-sql-driver converts to UTC on write and reads
  back as UTC instants. Result: a todo picked at 10:00 local (CEST, UTC+2) is
  stored as 08:00 and displayed back as "08:00" — 2h earlier than local time,
  exactly the reported symptom. The existing test
  `TestTodosCreateDoneDelete` even encodes the wrong behaviour
  (`todo.TodoDate.UTC()... != "2026-09-19 08:00"`).
- All other date forms in the app go through the buffalo binder, which parses
  custom layouts with `time.Parse` (UTC frame) — i.e. the picked wall clock is
  stored verbatim in the naive DATETIME column and displayed back unchanged
  (see `models/outtake.go FormWallClockNow` comment). Todos are the odd one
  out.

### Fix

- `parseTodoDate`: parse with `time.Parse` (UTC frame) instead of
  `time.ParseInLocation(..., time.Local)` so the picked wall clock is stored
  verbatim, consistent with every other form in the app.
- `TodosNew` default date and `TodosDone` done_at: use
  `models.FormWallClockNow()` (local wall clock in UTC frame) so stored values
  stay in the same display frame.
- No schema change (DATETIME column is naive; values written by the fixed code
  are in the display frame — same convention as cares/outtakes). Existing
  production rows written with the old code are shifted by the UTC offset;
  they will keep displaying as stored (historical data, acceptable — the
  guideline forbids destructive rewrites; a one-off UPDATE is not safe across
  DST boundaries).

### Test approach

- Update `TestTodosCreateDoneDelete`: posted `2026-09-19T10:00` must be
  retrieved displaying `10:00` (`.UTC().Format == "2026-09-19 10:00"`).
- Unit test `parseTodoDate`: parsed value's wall clock equals the input wall
  clock regardless of server TZ (run under `TZ=Europe/Paris`).
- e2e (agent-browser): set UI language FR, create a todo at a known local
  time, verify dashboard + /todos display the same wall clock; open a flatpickr
  date picker and verify French month/day names.

## Bug 2 — TODO dashboard filtering + collapsed "later" section

> Dashboard: Only show TODOs that need to be done within the next 8h (or too
> late).
> TODO screen: add a collapsed "to be done" section and move all todos that are
> not within the next 8h out of the main (open) list.

### Interpretation

The 8h window matches the existing `models.TodoOverdueAfter = 8 * time.Hour`
semantics. "Set all todos that are not within the next 8h to 'done'" is read
as: future todos (due later than now+8h) must leave the main open list — shown
in their own collapsed section on /todos, NOT marked done in the DB (no data
mutation; reopening flows stay intact).

### Fix

- `models.Todo`: add `DueWithin(now, 8h) bool` helper (open && todo_date <=
  now+8h) — i.e. overdue OR due within the next 8h.
- `actions/dashboard.go listDashboardTodos`: query
  `done_at IS NULL AND todo_date <= ?` with `now+8h`.
- `actions/todos.go TodosIndex`: split open todos into `openTodos` (due ≤
  now+8h) and `laterTodos` (due > now+8h); pass both to the template.
- `templates/todos/todos.plush.html` (+ fr/de/nl variants if present — check):
  new collapsed "To be done (later)" section between Open and Done, reusing the
  existing collapse/chevron pattern.

### Test approach

- Action test: seed todos at now−1h, now+2h, now+9h → dashboard lists only the
  first two; /todos page puts the +9h one in the collapsed section.
- Update any dashboard test asserting all open todos appear.
- e2e: create todos at +30min and +2days, verify dashboard shows only the
  +30min one and /todos shows the +2days one collapsed.

## Bug 3 — Care note templates UI in all languages

> It should be in *all* languages but currently only in English.

### Investigation findings

`templates/care_templates/{index,new,edit,_form}.plush.html` exist only in the
base (English) variant with hardcoded English strings: page titles, "Create
New Template", table headers (Name/Content/Owner), Save/Cancel, confirm "Are
you sure?", tooltips. `locales/care_templates.{fr,de,nl,en-us}.yaml` only
contain the four flash messages. There are no `.fr/.de/.nl` template variants
for care_templates.

### Fix

Follow the established `t()` pattern (same as todos templates):

- Add keys to `locales/care_templates.{fr,de,nl,en-us}.yaml`:
  `care_templates.index.title`, `care_templates.new`, `care_templates.edit`,
  `care_templates.name`, `care_templates.content`, `care_templates.owner`,
  `care_templates.save`, `care_templates.cancel`, `care_templates.confirm_delete`,
  `care_templates.edit_title`, `care_templates.destroy_title` (review exact set
  while editing).
- Replace hardcoded strings in the 4 base templates with `<%= t("...") %>`
  calls so all locales render translated chrome.
- Form labels via `f.InputTag("Name")`/`f.TextAreaTag("Content")` render the
  field name as label — keep, but wrap label text with the new keys if the
  helper output is the raw field name (verify in e2e).

### Test approach

- `go test` (no logic change; existing care_templates tests must stay green).
- e2e: switch UI to fr/de/nl/en-US, open /care_templates, verify headings,
  buttons, and column headers are translated in each language.

## Validation steps (all bugs)

1. `go vet ./...`
2. `staticcheck ./...`
3. `gocognit -over 15 .`
4. `gocyclo -over 12 .`
5. `go test -count=1 -race -cover ./...` (run each tool separately)
6. e2e via agent-browser skill with real server (`buffalo dev`), evidence per
   bug (URLs + captured output), interactive shell/filmstrip verification of
   actual terminal output.
7. One commit per bug with a descriptive message.
8. Move bug entries from `/bugs.md` to `docs/archive/` (one archive file per
   bug, including this plan + e2e evidence); bugs.md keeps only the guidelines.
