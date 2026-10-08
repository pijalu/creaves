# Issue #199-8 — Recurring todos

## Observed
Recurring care tasks (weekly/monthly/yearly) had to be re-created manually after each completion.

## Expected
A todo can be marked as recurring; marking it done closes the current occurrence and spawns the next one automatically.

## Decision (confirmed with reporter)
Marking done spawns the next occurrence due +1 week / +1 month / +1 year and closes the current occurrence.

## Fix
- Migration `20261021093000_todos_recurrence` (additive): `todos.recurrence varchar(16) NOT NULL DEFAULT ''`.
- `models/todo.go`: `Recurrence` field; `TodoRecurrenceNone/Weekly/Monthly/Yearly` constants; `NextOccurrence()` returns a copy due +7d / +1mo / +1yr (nil for none/unknown).
- `actions/todos.go`: `todoRecurrenceParam` whitelist (invalid → one-shot); `TodosCreate`/`TodosUpdate` persist recurrence; `TodosDone` creates `NextOccurrence()` after closing.
- Templates (4 locales): recurrence selector on `todos/new` + `todos/edit`; `badge-info` recurrence badge on open cards in `todos/todos`.
- Locales: `todos.recurrence`, `.none`, `.weekly`, `.monthly`, `.yearly` in en-us/fr/de/nl.

## Tests
- `models/todo_test.go` `TestTodoNextOccurrence` — table-driven intervals and nil cases.
- `actions/todos_test.go` `TestTodosRecurrence` — weekly todo done → new open todo at +7d, still weekly; invalid recurrence value → one-shot, no spawn.
- e2e (agent-browser): created weekly todo, badge visible, marked done → spawned todo due +7 days; rows cleaned up.

## Gates
- go vet, staticcheck: clean.
- gocognit/gocyclo: only test-func growth (accepted).
- Full `go test -count=1 -race -cover ./...`: green except documented pre-existing grifts debt.
- Template parity: no NEW drift on todos pairs.

## Commits
- `0420160` (branch `feature/open-issues-2026-10`)
